package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Line numbers in messages follow Python's universal newlines plus
// str.splitlines: \r\n, a lone \r, \f and U+2028 each end a line.
func TestRepoErrorsNameThePythonLineNumber(t *testing.T) {
	for name, text := range map[string]string{
		"crlf":    "# h\r\nok https://github.com/acme/ok\r\nbad http://github.com/evil/x\r\n",
		"lone cr": "# h\rok https://github.com/acme/ok\rbad http://github.com/evil/x\r",
		"ff":      "# h\fok https://github.com/acme/ok\nbad http://github.com/evil/x\n",
		"u2028":   "# h\u2028ok https://github.com/acme/ok\nbad http://github.com/evil/x\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseRepos([]byte(text))
			if err == nil || !strings.HasPrefix(err.Error(), "repos.txt:3: bad ") {
				t.Fatalf("err = %v, want repos.txt:3: bad ...", err)
			}
		})
	}
}

func TestChecksRows(t *testing.T) {
	repos, err := ParseRepos([]byte("api https://github.com/acme/api.git\n"))
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
		rows, err := ParseChecks([]byte("# header\n"+tc.row+"\n"), repos)
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
	repos, err := ParseRepos([]byte(
		"api https://github.com/acme/api.git\nweb https://github.com/other-co/web-app\n"))
	if err != nil {
		t.Fatal(err)
	}
	if r := repos.ByName["api"]; r.FullName() != "acme/api" || r.API() != "/repos/acme/api" {
		t.Errorf("api: %+v", r)
	}
	if r := repos.ByName["web"]; r.FullName() != "other-co/web-app" {
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
		if _, err := ParseRepos([]byte("x " + u + "\n")); err == nil || !strings.Contains(err.Error(), "URL must be") {
			t.Errorf("%s: err = %v", u, err)
		}
	}
}

// Each refusal names the line it comes from. Ids name hub branches <id>/<b>,
// so an id that is not a single git ref component is refused, and two rows
// may not share an id up to case or name one owner/repo.
func TestReposRefusals(t *testing.T) {
	for _, tc := range []struct{ text, err string }{
		{"a https://github.com/acme/a folder\n", "repos.txt:1: a https://github.com/acme/a folder: expected <id> <URL>"},
		{"a\n", "repos.txt:1: a: expected <id> <URL>"},
		{".a https://github.com/acme/a\n", `repos.txt:1: .a https://github.com/acme/a: bad repo id ".a"`},
		{"-a https://github.com/acme/a\n", `bad repo id "-a"`},
		{"a..b https://github.com/acme/a\n", `bad repo id "a..b"`},
		{"a.lock https://github.com/acme/a\n", `bad repo id "a.lock"`},
		{"a/b https://github.com/acme/a\n", `bad repo id "a/b"`},
		{"# h\nApi https://github.com/acme/a\napi https://github.com/acme/b\n", "repos.txt:3: api https://github.com/acme/b: duplicate repo id api (already Api)"},
		{"a https://github.com/acme/web\nb https://github.com/Acme/Web.git\n", "repos.txt:2: b https://github.com/Acme/Web.git: Acme/Web is already listed as a"},
	} {
		_, err := ParseRepos([]byte(tc.text))
		if err == nil || !IsError(err) || !strings.HasPrefix(err.Error(), "repos.txt:") || !strings.Contains(err.Error(), tc.err) {
			t.Errorf("%q: err = %v, want %s", tc.text, err, tc.err)
		}
	}
	if rs, err := ParseRepos([]byte("a.b_c-1 https://github.com/acme/a\n")); err != nil || rs.ByName["a.b_c-1"] == nil {
		t.Errorf("a.b_c-1: %v", err)
	}
}

// Every remote form setup accepts maps to OWNER/REPO once its userinfo is
// dropped, and a token never survives WithoutUserinfo.
func TestGitHubRepoForms(t *testing.T) {
	for remote, want := range map[string]string{
		"https://github.com/o/r.git":                   "o/r",
		"https://github.com/o/r":                       "o/r",
		"ssh://git@github.com/o/r.git":                 "o/r",
		"git@github.com:o/r.git":                       "o/r",
		"https://x-access-token:s3cr3t@github.com/o/r": "o/r",
		"https://x:s3cr3t%zz@github.com/o/r":           "o/r",
		"https://gitlab.com/o/r.git":                   "",
		"file:///tmp/r.git":                            "",
	} {
		clean := WithoutUserinfo(remote)
		if strings.Contains(clean, "s3cr3t") {
			t.Errorf("%s: token kept: %s", remote, clean)
		}
		if got := GitHubRepo(clean); got != want {
			t.Errorf("%s: got %q, want %q", remote, got, want)
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
