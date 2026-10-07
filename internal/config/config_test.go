package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, name, text string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// Line numbers in messages follow Python's universal newlines plus
// str.splitlines: \r\n, a lone \r, \f and U+2028 each end a line.
func TestRepoErrorsNameThePythonLineNumber(t *testing.T) {
	for name, text := range map[string]string{
		"crlf":    "# h\r\nok https://github.com/acme/ok folder\r\nbad http://github.com/evil/x folder\r\n",
		"lone cr": "# h\rok https://github.com/acme/ok folder\rbad http://github.com/evil/x folder\r",
		"ff":      "# h\fok https://github.com/acme/ok folder\nbad http://github.com/evil/x folder\n",
		"u2028":   "# h\u2028ok https://github.com/acme/ok folder\nbad http://github.com/evil/x folder\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseRepos(write(t, "repos.txt", text))
			if err == nil || !strings.HasPrefix(err.Error(), "repos.txt:3: bad ") {
				t.Fatalf("err = %v, want repos.txt:3: bad ...", err)
			}
		})
	}
}

func TestChecksRows(t *testing.T) {
	repos, err := ParseRepos(write(t, "repos.txt", "api https://github.com/acme/api.git folder\n"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ row, err string }{
		{`api ci.yml "contract test" process:ci/contract contracts/a services/b`, ""},
		{`api ci.yml test human:alice contracts/a`, "actor must be process:<name>"},
		{`api ci.yml test "process:x, at: 2099-01-01T00:00:00Z }" contracts/a`, "actor must be process:<name>"},
		{`api ci.yml test process: contracts/a`, "actor must be process:<name>"},
		{`api ../ci.yml test process:x contracts/a`, "workflow must be a file name"},
		{`api ci.yml test process:x contracts/a.md`, "bad concept id"},
		{`api ci.yml test process:x a/../b`, "bad concept id"},
		{`api ci.yml test process:x`, "expected <repo-name>"},
		{`web ci.yml test process:x contracts/a`, `repo "web" is not in repos.txt`},
		{`api ci.yml "test process:x contracts/a`, "No closing quotation"},
	} {
		rows, err := ParseChecks(write(t, "checks.txt", "# header\n"+tc.row+"\n"), repos)
		if tc.err == "" {
			if err != nil {
				t.Errorf("%s: %v", tc.row, err)
			} else if r := rows[0]; r.Line != 2 || r.Job != "contract test" || r.Actor != "process:ci/contract" || len(r.Concepts) != 2 {
				t.Errorf("%s: parsed %+v", tc.row, r)
			}
			continue
		}
		if err == nil || !strings.HasPrefix(err.Error(), "checks.txt:2: "+tc.row+": ") || !strings.Contains(err.Error(), tc.err) {
			t.Errorf("%s: err = %v, want checks.txt:2 ... %s", tc.row, err, tc.err)
		}
	}
}

// Any GitHub owner is accepted, and the owner comes from each URL (plan
// decision 7); anything but https://github.com/<owner>/<repo> is refused.
func TestReposAcceptAnyOwner(t *testing.T) {
	repos, err := ParseRepos(write(t, "repos.txt",
		"api https://github.com/acme/api.git folder\nweb https://github.com/other-co/web-app branch\n"))
	if err != nil {
		t.Fatal(err)
	}
	if r := repos.ByName["api"]; r.FullName() != "acme/api" || r.API() != "/repos/acme/api" {
		t.Errorf("api: %+v", r)
	}
	if r := repos.ByName["web"]; r.FullName() != "other-co/web-app" || r.Mode != "branch" {
		t.Errorf("web: %+v", r)
	}
	for _, u := range []string{
		"http://github.com/acme/x",
		"https://github.com/acme/../x",
		"https://github.com/acme/..",
		"https://github.com/-acme/x",
		"https://github.com/acme/x/y",
		"git@github.com:acme/x.git",
		"https://gitlab.com/acme/x",
	} {
		if _, err := ParseRepos(write(t, "repos.txt", "x "+u+" folder\n")); err == nil || !strings.Contains(err.Error(), "URL must be") {
			t.Errorf("%s: err = %v", u, err)
		}
	}
}

func TestHubDir(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "compass", "config")
	env := func(k string) string {
		if k == "XDG_CONFIG_HOME" {
			return dir
		}
		return ""
	}
	if _, err := HubDir(env); err == nil || !IsError(err) || !strings.Contains(err.Error(), "recorded none") {
		t.Errorf("no config: err = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(cfg), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg, []byte("# Written by compass setup.\nOKF_HUB=acme/hub\nOKF_HUB_DIR=/src/my hub\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := HubDir(env); err != nil || got != "/src/my hub" {
		t.Errorf("recorded: got %q, %v", got, err)
	}
	// An unreadable user config is an error, not "no hub recorded".
	if err := os.Remove(cfg); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(cfg, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := HubDir(env); err == nil || !IsError(err) || !strings.Contains(err.Error(), "cannot read") {
		t.Errorf("unreadable user config: err = %v", err)
	}
}
