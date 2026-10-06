package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestManifestPermissions(t *testing.T) {
	for role, want := range map[string]map[string]string{
		"reader": {"contents": "read", "metadata": "read"},
		"writer": {"contents": "write", "pull_requests": "write", "actions": "read", "checks": "read", "metadata": "read"},
	} {
		var m struct {
			Name     string            `json:"name"`
			URL      string            `json:"url"`
			Hook     map[string]any    `json:"hook_attributes"`
			Redirect string            `json:"redirect_url"`
			Public   bool              `json:"public"`
			Perms    map[string]string `json:"default_permissions"`
			Events   []string          `json:"default_events"`
		}
		if err := json.Unmarshal([]byte(manifestJSON(role, "n", "acme/hub", "http://127.0.0.1:1/callback")), &m); err != nil {
			t.Fatal(err)
		}
		if fmt.Sprint(m.Perms) != fmt.Sprint(want) || m.Public || m.Events == nil || len(m.Events) != 0 ||
			m.URL != "https://github.com/acme/hub" || m.Hook["active"] != false || m.Redirect != "http://127.0.0.1:1/callback" {
			t.Errorf("%s manifest: %+v", role, m)
		}
	}
}

// fakeGH puts a gh on PATH that logs its arguments and the secret's stdin.
func fakeGH(t *testing.T) string {
	dir := t.TempDir()
	script := `#!/bin/sh
echo "$*" >> "` + dir + `/log"
case "$*" in
  "api users/acme -q .type") echo Organization ;;
  "api repos/acme/hub -q .default_branch") echo main ;;
  *deployment-branch-policies) case "$*" in *POST*) ;; *) echo '{"branch_policies":[]}' ;; esac ;;
  *conversions) echo '{"id":7,"slug":"acme-okf-writer","client_id":"Iv1.abc","pem":"SECRET-PEM"}' ;;
  "secret set"*) cat > "` + dir + `/stdin" ;;
  "api -X PUT"*) cat > /dev/null ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return dir
}

func get(t *testing.T, url string) (int, string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestCreateWriter(t *testing.T) {
	dir := fakeGH(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	base := "http://" + ln.Addr().String()
	var out bytes.Buffer
	errc := make(chan error, 1)
	go func() { errc <- create(options{role: "writer", hub: "acme/hub"}, ln, &out) }()

	var page string
	for range 50 {
		var code int
		if code, page = get(t, base+"/"); code == 200 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	m := regexp.MustCompile(`action='https://github.com/organizations/acme/settings/apps/new\?state=([A-Za-z0-9_-]+)'`).FindStringSubmatch(page)
	if m == nil {
		t.Fatalf("no org form action in page: %s", page)
	}
	if code, _ := get(t, base+"/callback?state=wrong&code=c1"); code != 400 {
		t.Fatalf("state mismatch: status %d, want 400", code)
	}
	if code, _ := get(t, base+"/callback?state="+m[1]); code != 400 {
		t.Fatalf("no code: status %d, want 400", code)
	}
	if code, body := get(t, base+"/callback?state="+m[1]+"&code=c1"); code != 200 || !strings.Contains(body, "apps/acme-okf-writer/installations/new") {
		t.Fatalf("callback: %d %s", code, body)
	}
	if err := <-errc; err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "SECRET-PEM") {
		t.Errorf("private key printed: %s", out.String())
	}
	for _, want := range []string{
		"environment okf-write: only branch main may deploy",
		"stored OKF_WRITER_CLIENT_ID (variable) and OKF_WRITER_PRIVATE_KEY (secret) on acme/hub, environment okf-write",
		"app: acme-okf-writer (id 7, client id Iv1.abc)",
		"install: https://github.com/apps/acme-okf-writer/installations/new",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}
	logb, _ := os.ReadFile(filepath.Join(dir, "log"))
	log := string(logb)
	envAt := strings.Index(log, "api -X PUT repos/acme/hub/environments/okf-write")
	convAt := strings.Index(log, "app-manifests/c1/conversions")
	if envAt < 0 || convAt < envAt || strings.Count(log, "conversions") != 1 {
		t.Errorf("environment must be created before the single conversion:\n%s", log)
	}
	for _, want := range []string{
		"api -X POST repos/acme/hub/environments/okf-write/deployment-branch-policies -f name=main -f type=branch",
		"variable set OKF_WRITER_CLIENT_ID -R acme/hub --env okf-write --body Iv1.abc",
		"secret set OKF_WRITER_PRIVATE_KEY -R acme/hub --env okf-write",
	} {
		if !strings.Contains(log, want) {
			t.Errorf("gh log lacks %q:\n%s", want, log)
		}
	}
	if strings.Contains(log, "SECRET-PEM") {
		t.Errorf("private key passed as an argument:\n%s", log)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "stdin")); string(b) != "SECRET-PEM" {
		t.Errorf("secret stdin = %q", b)
	}
}

func TestHelp(t *testing.T) {
	var out, errb bytes.Buffer
	if code := Main([]string{"--help"}, &out, &errb); code != 0 || !strings.Contains(out.String(), "--role reader|writer") {
		t.Fatalf("help: %d %s", code, out.String())
	}
	if code := Main([]string{"--role", "admin", "--hub", "a/b"}, &out, &errb); code != 2 {
		t.Fatalf("bad role: exit %d", code)
	}
}
