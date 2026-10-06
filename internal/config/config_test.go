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
		"crlf":    "# h\r\nok https://github.com/acme/ok folder\r\nbad https://github.com/evil/x folder\r\n",
		"lone cr": "# h\rok https://github.com/acme/ok folder\rbad https://github.com/evil/x folder\r",
		"ff":      "# h\fok https://github.com/acme/ok folder\nbad https://github.com/evil/x folder\n",
		"u2028":   "# h\u2028ok https://github.com/acme/ok folder\nbad https://github.com/evil/x folder\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseRepos(write(t, "repos.txt", text), "acme")
			if err == nil || !strings.HasPrefix(err.Error(), "repos.txt:3: bad ") {
				t.Fatalf("err = %v, want repos.txt:3: bad ...", err)
			}
		})
	}
}

func TestChecksRows(t *testing.T) {
	repos, err := ParseRepos(write(t, "repos.txt", "api https://github.com/acme/api.git folder\n"), "acme")
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

func TestOrgPrecedence(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "compass", "config")
	if err := os.MkdirAll(filepath.Dir(cfg), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg, []byte("# Written by compass setup.\nOKF_ORG=fromuser\nOKF_HUB=fromuser/hub\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	env := func(org string) func(string) string {
		return func(k string) string {
			switch k {
			case "OKF_ORG":
				return org
			case "XDG_CONFIG_HOME":
				return dir
			}
			return ""
		}
	}
	if got, err := Org(env("fromenv")); err != nil || got != "fromenv" {
		t.Errorf("env set: got %q, %v", got, err)
	}
	if got, err := Org(env("")); err != nil || got != "fromuser" {
		t.Errorf("user config: got %q, %v", got, err)
	}
	// An unreadable user config is an error, not a silent fall back to the
	// built-in org.
	if err := os.Remove(cfg); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(cfg, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Org(env("")); err == nil || !IsError(err) || !strings.Contains(err.Error(), "cannot read") {
		t.Errorf("unreadable user config: err = %v", err)
	}
}
