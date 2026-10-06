package hub

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// The awk-equivalent strip: CRLF delimiters are not delimiters, an unclosed
// block drops the rest, a later --- line is kept, a final line gains a newline.
func TestStripFrontmatter(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{"closed block", "---\nokf_version: \"0.2\"\n---\n# Repo\n", "# Repo\n"},
		{"later rule kept", "---\na: 1\n---\nx\n---\ny", "x\n---\ny\n"},
		{"unclosed block", "---\na: 1\n# Repo\n", ""},
		{"no frontmatter", "# Repo\n---\n", "# Repo\n---\n"},
		{"CRLF opener is not a delimiter", "---\r\na: 1\r\n---\r\nx\r\n", "---\r\na: 1\r\n---\r\nx\r\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "index.md")
			if err := os.WriteFile(p, []byte(tc.in), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := stripFrontmatter(p); err != nil {
				t.Fatal(err)
			}
			got, _ := os.ReadFile(p)
			if string(got) != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// Every bad line is reported, comments start anywhere, and only blanks and
// tabs separate fields.
func TestParseReposReportsEveryBadLine(t *testing.T) {
	p := filepath.Join(t.TempDir(), "repos.txt")
	text := "# header\n" +
		"a https://github.com/acme/a folder # trailing comment\n" +
		"b https://github.com/acme/b\n" +
		"bad/name https://github.com/acme/c folder\n" +
		"d https://github.com/acme/d.git branch\n" +
		"e https://github.com/acme/e\x0bfolder\n" +
		"a https://github.com/acme/a2.git branch\n" +
		"f https://github.com/acme/f folder\r\n" +
		"g http://github.com/acme/g folder"
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	repos, ok, err := parseRepos(p, "https://github.com/acme/", &stderr)
	if err != nil {
		t.Fatal(err)
	}
	want := "hub assemble: repos.txt:3: expected '<name> <git URL> <folder|branch>'\n" +
		"hub assemble: repos.txt:4: bad repo name 'bad/name'\n" +
		"hub assemble: repos.txt:6: expected '<name> <git URL> <folder|branch>'\n" +
		"hub assemble: repos.txt:7: duplicate repo name 'a'\n" +
		"hub assemble: repos.txt:8: f: mode must be folder or branch, got 'folder\r'\n" +
		"hub assemble: repos.txt:9: g: URL 'http://github.com/acme/g' is not under https://github.com/acme/; refusing to fetch anything\n"
	if ok || stderr.String() != want {
		t.Errorf("ok=%v, messages:\n%q\nwant:\n%q", ok, stderr.String(), want)
	}
	if len(repos) != 2 || repos[0] != (listed{"a", "https://github.com/acme/a", "folder"}) || repos[1].name != "d" {
		t.Errorf("repos %+v", repos)
	}
}
