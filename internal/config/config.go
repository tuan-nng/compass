// Package config reads the user config that `compass setup` writes and the
// hub control files repos.txt and checks.txt (plan decisions 3 and 7, plan
// Design "Interfaces").
package config

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

// Error is a configuration problem: an unreadable user config, no recorded
// hub folder, an unreadable or malformed control file, or a missing token.
type Error struct{ msg string }

func (e *Error) Error() string { return e.msg }

func errorf(format string, a ...any) error { return &Error{fmt.Sprintf(format, a...)} }

// IsError reports whether err is a configuration error.
func IsError(err error) bool {
	var e *Error
	return errors.As(err, &e)
}

// repoURLRe matches https://github.com/<owner>/<repo>, optionally with .git;
// any owner is accepted (plan decision 7).
var repoURLRe = regexp.MustCompile(`^https://github\.com/([A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?)/([A-Za-z0-9._-]+?)(?:\.git)?$`)

// ParseRepoURL splits https://github.com/<owner>/<repo>[.git] into owner and
// repo; ok is false for any other URL.
func ParseRepoURL(u string) (owner, repo string, ok bool) {
	m := repoURLRe.FindStringSubmatch(u)
	if m == nil || m[2] == "." || m[2] == ".." {
		return "", "", false
	}
	return m[1], m[2], true
}

// EnvValue returns the last KEY=value assignment in env-file text, with
// surrounding quotes removed. Comments and blank lines are skipped.
func EnvValue(text, key string) string {
	val := ""
	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if v, ok := strings.CutPrefix(line, key+"="); ok {
			val = strings.Trim(strings.TrimSpace(v), `"'`)
		}
	}
	return val
}

// UserConfig returns the user config text, or "" when there is none.
func UserConfig(getenv func(string) string) (string, error) {
	p := UserConfigPath(getenv)
	if p == "" {
		return "", nil
	}
	raw, err := os.ReadFile(p)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", errorf("cannot read %s: %v", p, err)
	}
	return string(raw), nil
}

// HubDir returns the hub clone folder that `compass setup` recorded as
// OKF_HUB_DIR in the user config. The hub commands use it when given no
// folder.
func HubDir(getenv func(string) string) (string, error) {
	text, err := UserConfig(getenv)
	if err != nil {
		return "", err
	}
	if d := EnvValue(text, "OKF_HUB_DIR"); d != "" {
		return d, nil
	}
	return "", errorf("no hub folder given, and `compass setup` recorded none in %s", UserConfigPath(getenv))
}

// UserConfigPath is $XDG_CONFIG_HOME/compass/config, else
// $HOME/.config/compass/config; empty when neither variable is set.
func UserConfigPath(getenv func(string) string) string {
	if d := getenv("XDG_CONFIG_HOME"); d != "" {
		return filepath.Join(d, "compass", "config")
	}
	if h := getenv("HOME"); h != "" {
		return filepath.Join(h, ".config", "compass", "config")
	}
	return ""
}

// OKFBinary is the okf binary to run: $OKF, else okf on PATH.
func OKFBinary() string {
	if p := os.Getenv("OKF"); p != "" {
		return p
	}
	return "okf"
}

// Token returns the API token held in the environment variable name.
func Token(name string, getenv func(string) string) (string, error) {
	t := getenv(name)
	if t == "" {
		return "", errorf("no token: environment variable %s is empty", name)
	}
	return t, nil
}

// Repo is one repos.txt row.
type Repo struct {
	Name  string
	URL   string
	Mode  string // "folder" or "branch"
	Owner string
	Repo  string
}

// FullName is owner/repo.
func (r *Repo) FullName() string { return r.Owner + "/" + r.Repo }

// API is the repo's REST path, /repos/owner/repo.
func (r *Repo) API() string { return "/repos/" + r.Owner + "/" + r.Repo }

// Repos is repos.txt in file order, with lookup by name.
type Repos struct {
	List   []*Repo
	ByName map[string]*Repo
}

type line struct {
	n    int
	text string
}

// contentLines returns the non-blank, non-comment lines of path, numbered as
// Python's universal-newline read plus str.splitlines numbers them.
func contentLines(path string) ([]line, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, errorf("cannot read %s: %v", path, err)
	}
	text := strings.ReplaceAll(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\r", "\n")
	var out []line
	n, start := 0, 0
	emit := func(end int) {
		n++
		t := strings.TrimFunc(text[start:end], pyIsSpace)
		if t != "" && !strings.HasPrefix(t, "#") {
			out = append(out, line{n, t})
		}
	}
	for i, r := range text {
		if isLineBreak(r) {
			emit(i)
			start = i + len(string(r))
		}
	}
	if start < len(text) {
		emit(len(text))
	}
	return out, nil
}

// isLineBreak reports whether str.splitlines ends a line at r (after \r and
// \r\n became \n).
func isLineBreak(r rune) bool {
	switch r {
	case '\n', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
		return true
	}
	return false
}

// pyIsSpace is Python's str.isspace for one character.
func pyIsSpace(r rune) bool { return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f) }

var repoNameRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// ParseRepos reads repos.txt: "<name> <git URL> <folder|branch>" per line.
// Every URL must be https://github.com/<owner>/<repo>, optionally with .git,
// for any owner (plan decision 7).
func ParseRepos(path string) (*Repos, error) {
	lines, err := contentLines(path)
	if err != nil {
		return nil, err
	}
	rs := &Repos{ByName: map[string]*Repo{}}
	for _, l := range lines {
		parts := strings.Fields(l.text)
		where := fmt.Sprintf("repos.txt:%d: %s", l.n, l.text)
		if len(parts) != 3 {
			return nil, errorf("%s: expected <name> <git URL> <folder|branch>", where)
		}
		name, u, mode := parts[0], parts[1], parts[2]
		if !repoNameRe.MatchString(name) || name == "." || name == ".." {
			return nil, errorf("%s: bad repo name %q", where, name)
		}
		owner, repo, ok := ParseRepoURL(u)
		if !ok {
			return nil, errorf("%s: URL must be https://github.com/<owner>/<repo>", where)
		}
		if mode != "folder" && mode != "branch" {
			return nil, errorf("%s: mode must be folder or branch", where)
		}
		if _, dup := rs.ByName[name]; dup {
			return nil, errorf("%s: duplicate repo name %s", where, name)
		}
		r := &Repo{Name: name, URL: u, Mode: mode, Owner: owner, Repo: repo}
		rs.List = append(rs.List, r)
		rs.ByName[name] = r
	}
	return rs, nil
}

// CheckRow is one checks.txt row.
type CheckRow struct {
	Line     int
	Text     string
	Repo     *Repo
	Workflow string
	Job      string
	Actor    string
	Concepts []string
}

// Where names the row in messages: "checks.txt:<n>: <text>".
func (c *CheckRow) Where() string { return fmt.Sprintf("checks.txt:%d: %s", c.Line, c.Text) }

var (
	workflowRe  = regexp.MustCompile(`^[A-Za-z0-9._-]+\.ya?ml$`)
	actorRe     = regexp.MustCompile(`^process:[A-Za-z0-9._/-]+$`)
	conceptIDRe = regexp.MustCompile(`^[A-Za-z0-9_-][A-Za-z0-9._-]*(?:/[A-Za-z0-9_-][A-Za-z0-9._-]*)*$`)
)

// ParseChecks reads checks.txt rows, split shell-style:
// "<repo-name> <workflow-file> <job-name> <process-actor> <concept-id>...".
func ParseChecks(path string, repos *Repos) ([]*CheckRow, error) {
	lines, err := contentLines(path)
	if err != nil {
		return nil, err
	}
	var rows []*CheckRow
	for _, l := range lines {
		where := fmt.Sprintf("checks.txt:%d: %s", l.n, l.text)
		parts, err := SplitShell(l.text)
		if err != nil {
			return nil, errorf("%s: %v", where, err)
		}
		if len(parts) < 5 {
			return nil, errorf("%s: expected <repo-name> <workflow-file> <job-name> <process-actor> <concept-id>...", where)
		}
		name, workflow, job, actor, concepts := parts[0], parts[1], parts[2], parts[3], parts[4:]
		repo, ok := repos.ByName[name]
		if !ok {
			return nil, errorf("%s: repo %q is not in repos.txt", where, name)
		}
		if !workflowRe.MatchString(workflow) {
			return nil, errorf("%s: workflow must be a file name under .github/workflows/", where)
		}
		if !actorRe.MatchString(actor) {
			return nil, errorf("%s: actor must be process:<name>", where)
		}
		for _, c := range concepts {
			if !conceptIDRe.MatchString(c) || strings.HasSuffix(c, ".md") || hasDotDot(c) {
				return nil, errorf("%s: bad concept id %q (use e.g. contracts/invoice-api)", where, c)
			}
		}
		rows = append(rows, &CheckRow{Line: l.n, Text: l.text, Repo: repo, Workflow: workflow, Job: job, Actor: actor, Concepts: concepts})
	}
	return rows, nil
}

func hasDotDot(id string) bool {
	for _, p := range strings.Split(id, "/") {
		if p == ".." {
			return true
		}
	}
	return false
}

// SplitShell splits s the way Python's shlex.split does in POSIX mode:
// whitespace separates words, single quotes are literal, double quotes allow
// backslash escapes of \ and ", and a backslash outside quotes escapes the
// next character.
func SplitShell(s string) ([]string, error) {
	var words []string
	var cur strings.Builder
	inWord := false
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		c := rs[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			if inWord {
				words = append(words, cur.String())
				cur.Reset()
				inWord = false
			}
		case c == '\'':
			inWord = true
			j := i + 1
			for j < len(rs) && rs[j] != '\'' {
				j++
			}
			if j >= len(rs) {
				return nil, errors.New("No closing quotation")
			}
			cur.WriteString(string(rs[i+1 : j]))
			i = j
		case c == '"':
			inWord = true
			j := i + 1
			for ; j < len(rs) && rs[j] != '"'; j++ {
				if rs[j] == '\\' && j+1 < len(rs) && (rs[j+1] == '\\' || rs[j+1] == '"') {
					j++
				}
				cur.WriteRune(rs[j])
			}
			if j >= len(rs) {
				return nil, errors.New("No closing quotation")
			}
			i = j
		case c == '\\':
			if i+1 >= len(rs) {
				return nil, errors.New("No escaped character")
			}
			inWord = true
			i++
			cur.WriteRune(rs[i])
		default:
			inWord = true
			cur.WriteRune(c)
		}
	}
	if inWord {
		words = append(words, cur.String())
	}
	return words, nil
}
