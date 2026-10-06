// Package stamp implements "compass stamp": it writes
// "verified: process:<actor>" stamps from checks.txt rows.
//
// For each row the stamper finds the latest push run of the named workflow
// file on the default branch's head commit, and the named job in it. If that
// job succeeded, it stamps the row's concepts as they are at the commit it
// read (folder mode: that head; branch mode: the okf/main head), dated with
// the job's completion time, and fast-forwards the branch to a new commit.
// See the plan's Design section, "Stamper".
package stamp

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"compass/internal/config"
	"compass/internal/frontmatter"
	"compass/internal/github"
)

// Prog prefixes every log line.
const Prog = "stamp"

// repoFailure stops one repo, such as a missing branch.
type repoFailure struct{ msg string }

func (e *repoFailure) Error() string { return e.msg }

func refSHA(gh *github.Client, repo *config.Repo, branch string) (string, error) {
	var ref struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	found, err := gh.GetOptional(repo.API()+"/git/ref/heads/"+github.QuotePath(branch), nil, &ref)
	if err != nil {
		return "", err
	}
	if !found {
		return "", &repoFailure{fmt.Sprintf("branch %s not found", branch)}
	}
	return ref.Object.SHA, nil
}

type object = map[string]any

func str(o object, key string) string {
	s, _ := o[key].(string)
	return s
}

func num(o object, key string) float64 {
	n, _ := o[key].(float64)
	return n
}

// findJob returns the completed_at of the successful row.Job job of the
// latest push run of .github/workflows/<row.Workflow> on head, or "".
func findJob(gh *github.Client, repo *config.Repo, row *config.CheckRow, def, head string) (string, error) {
	path := ".github/workflows/" + row.Workflow
	runs, err := github.Paginate[object](gh, repo.API()+"/actions/workflows/"+github.QuotePath(row.Workflow)+"/runs",
		url.Values{"branch": {def}, "event": {"push"}, "head_sha": {head}}, "workflow_runs")
	if err != nil {
		return "", err
	}
	var run object
	for _, r := range runs {
		var fullName any = repo.FullName()
		if rr, ok := r["repository"].(object); ok {
			if v, ok := rr["full_name"]; ok {
				fullName = v
			}
		}
		if r["path"] != any(path) || r["event"] != any("push") || r["head_branch"] != any(def) ||
			r["head_sha"] != any(head) || fullName != any(repo.FullName()) {
			continue
		}
		// The first of equal maxima wins, as Python's max does.
		if run == nil || str(r, "created_at") > str(run, "created_at") ||
			(str(r, "created_at") == str(run, "created_at") && num(r, "id") > num(run, "id")) {
			run = r
		}
	}
	if run == nil {
		return "", nil
	}
	jobs, err := github.Paginate[object](gh, fmt.Sprintf("%s/actions/runs/%s/jobs", repo.API(), runID(run["id"])),
		url.Values{"filter": {"latest"}}, "jobs")
	if err != nil {
		return "", err
	}
	var named []object
	for _, j := range jobs {
		if v, ok := j["head_sha"]; ok && v != any(head) {
			continue
		}
		if j["name"] == any(row.Job) {
			named = append(named, j)
		}
	}
	if len(named) == 0 {
		return "", nil
	}
	best := ""
	for _, j := range named {
		if j["conclusion"] != any("success") || str(j, "completed_at") == "" {
			return "", nil
		}
		if c := str(j, "completed_at"); c > best {
			best = c
		}
	}
	return best, nil
}

// runID renders a JSON run id the way Python formats an int.
func runID(v any) string {
	switch n := v.(type) {
	case float64:
		return fmt.Sprintf("%.0f", n)
	case string:
		return n
	default:
		return fmt.Sprint(v)
	}
}

// fetch returns the file text at commit sha; found=false when the path does
// not exist.
func fetch(gh *github.Client, repo *config.Repo, path, sha string) (text string, found bool, err error) {
	var data any
	found, err = gh.GetOptional(repo.API()+"/contents/"+github.QuotePath(path), url.Values{"ref": {sha}}, &data)
	if err != nil || !found {
		return "", found, err
	}
	o, ok := data.(object)
	if !ok || o["type"] != any("file") {
		return "", true, fmtErr("%s is not a regular file", path)
	}
	if o["encoding"] != any("base64") {
		return "", true, fmtErr("%s: content not returned (file too large?)", path)
	}
	raw, err := b64decode(str(o, "content"))
	if err != nil || !utf8.Valid(raw) {
		return "", true, fmtErr("%s: not UTF-8 text", path)
	}
	return string(raw), true, nil
}

// fetchErr is a file the stamper cannot read as a concept; it is reported
// like a frontmatter error.
type fetchErr struct{ msg string }

func (e *fetchErr) Error() string { return e.msg }

func fmtErr(format string, a ...any) error { return &fetchErr{fmt.Sprintf(format, a...)} }

// b64decode is Python's base64.b64decode: characters outside the alphabet
// (the API's line breaks) are dropped before decoding.
func b64decode(s string) ([]byte, error) {
	var b strings.Builder
	for i := range len(s) {
		c := s[i]
		if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '+' || c == '/' || c == '=' {
			b.WriteByte(c)
		}
	}
	return base64.StdEncoding.DecodeString(b.String())
}

func isFileErr(err error) bool {
	var fe *fetchErr
	return frontmatter.IsError(err) || errors.As(err, &fe)
}

type commit struct {
	row     *config.CheckRow
	message string
	changed map[string]string
}

type logLine struct {
	row  *config.CheckRow
	text string
}

// stampRepo stamps every row of one repo from one read. It returns ok=false
// when a concept failed; err stops the repo.
func stampRepo(gh *github.Client, repo *config.Repo, rows []*config.CheckRow, dryRun bool, log func(string)) (bool, error) {
	ok := true
	var info struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := gh.Get(repo.API(), nil, &info); err != nil {
		return false, err
	}
	def := info.DefaultBranch
	head, err := refSHA(gh, repo, def)
	if err != nil {
		return false, err
	}
	branch, readSHA, prefix := def, head, "okf/"
	if repo.Mode != "folder" {
		branch, prefix = "okf/main", ""
		if readSHA, err = refSHA(gh, repo, branch); err != nil {
			return false, err
		}
	}

	files := map[string]string{} // path -> current text, starting from the read commit
	var commits []commit
	var lines []logLine // one per row, printed once the push outcome is known

	for _, row := range rows {
		label := fmt.Sprintf("%s: %s %s/%s %s", Prog, repo.Name, row.Workflow, row.Job, row.Actor)
		completed, err := findJob(gh, repo, row, def, head)
		if err != nil {
			return false, err
		}
		if completed == "" {
			lines = append(lines, logLine{row, fmt.Sprintf("%s: no check run on head %s", label, short(head))})
			continue
		}
		at, err := frontmatter.ParseTime(completed)
		if err != nil {
			return false, err
		}
		changed := map[string]string{}
		var stamped, missing []string
		for _, cid := range row.Concepts {
			path := prefix + cid + ".md"
			var err error
			text, have := files[path]
			if !have {
				var found bool
				text, found, err = fetch(gh, repo, path, readSHA)
				if err != nil && !isFileErr(err) {
					return false, err
				}
				if err == nil && !found {
					missing = append(missing, cid)
					continue
				}
				if err == nil {
					files[path] = text
				}
			}
			var newText, outcome string
			var did bool
			if err == nil {
				newText, outcome, did, err = frontmatter.Stamp(text, row.Actor, at)
			}
			if err != nil {
				log(fmt.Sprintf("%s: error: %s %s: %v; left untouched", Prog, repo.Name, cid, err))
				ok = false
				continue
			}
			if did {
				files[path] = newText
				changed[path] = newText
				stamped = append(stamped, fmt.Sprintf("%s (%s)", cid, outcome))
			}
		}
		if len(missing) > 0 {
			paths := make([]string, len(missing))
			for i, c := range missing {
				paths[i] = prefix + c + ".md"
			}
			log(fmt.Sprintf("%s: error: %s: concept not found at %s %s: %s",
				Prog, row.Where(), branch, short(readSHA), strings.Join(paths, ", ")))
			ok = false
		}
		if len(changed) > 0 {
			message := fmt.Sprintf("okf: process stamps from %s (%s/%s @ %s)", row.Actor, row.Workflow, row.Job, short(head))
			commits = append(commits, commit{row, message, changed})
			verb := "stamped"
			if dryRun {
				verb = "would stamp"
			}
			lines = append(lines, logLine{row, fmt.Sprintf("%s: %s %s on %s at %s",
				label, verb, strings.Join(stamped, ", "), branch, frontmatter.FormatTime(at))})
		} else {
			lines = append(lines, logLine{row, fmt.Sprintf("%s: nothing due on %s %s", label, branch, short(readSHA))})
		}
	}

	suffix := ""
	if len(commits) > 0 && !dryRun {
		parent := readSHA
		var base struct {
			Tree struct {
				SHA string `json:"sha"`
			} `json:"tree"`
		}
		if err := gh.Get(repo.API()+"/git/commits/"+readSHA, nil, &base); err != nil {
			return false, err
		}
		tree := base.Tree.SHA
		for _, c := range commits {
			paths := make([]string, 0, len(c.changed))
			for p := range c.changed {
				paths = append(paths, p)
			}
			sort.Strings(paths)
			entries := make([]map[string]string, len(paths))
			for i, p := range paths {
				entries[i] = map[string]string{"path": p, "mode": "100644", "type": "blob", "content": c.changed[p]}
			}
			var t, nc struct {
				SHA string `json:"sha"`
			}
			if _, err := gh.Post(repo.API()+"/git/trees", map[string]any{"base_tree": tree, "tree": entries}, &t); err != nil {
				return false, err
			}
			if _, err := gh.Post(repo.API()+"/git/commits",
				map[string]any{"message": c.message, "tree": t.SHA, "parents": []string{parent}}, &nc); err != nil {
				return false, err
			}
			tree, parent = t.SHA, nc.SHA
		}
		status, err := gh.Patch(repo.API()+"/git/refs/heads/"+github.QuotePath(branch),
			map[string]any{"sha": parent, "force": false}, nil, 409, 422)
		if err != nil {
			return false, err
		}
		if status == 409 || status == 422 {
			suffix = fmt.Sprintf(" (not pushed: %s moved after %s; next run retries)", branch, short(readSHA))
		} else {
			suffix = fmt.Sprintf(" (commit %s)", short(parent))
		}
	}
	for _, l := range lines {
		committed := false
		for _, c := range commits {
			if c.row == l.row {
				committed = true
			}
		}
		if committed {
			log(l.text + suffix)
		} else {
			log(l.text)
		}
	}
	return ok, nil
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// Run stamps every checks.txt row under hub and returns the exit code: 0 when
// every repo succeeded, 1 when any failed, 2 on a configuration error.
func Run(hub string, gh *github.Client, org string, dryRun bool, log func(string)) int {
	repos, err := config.ParseRepos(filepath.Join(hub, "repos.txt"), org)
	if err != nil {
		log(fmt.Sprintf("%s: error: %v", Prog, err))
		return 2
	}
	rows, err := config.ParseChecks(filepath.Join(hub, "checks.txt"), repos)
	if err != nil {
		log(fmt.Sprintf("%s: error: %v", Prog, err))
		return 2
	}
	var order []string
	byRepo := map[string][]*config.CheckRow{}
	for _, row := range rows {
		if _, seen := byRepo[row.Repo.Name]; !seen {
			order = append(order, row.Repo.Name)
		}
		byRepo[row.Repo.Name] = append(byRepo[row.Repo.Name], row)
	}
	ok := true
	for _, name := range order {
		repoOK, err := stampRepo(gh, repos.ByName[name], byRepo[name], dryRun, log)
		if err != nil {
			log(fmt.Sprintf("%s: error: %s: %v", Prog, name, err))
			ok = false
			continue
		}
		ok = repoOK && ok
	}
	if ok {
		return 0
	}
	return 1
}

const usage = "usage: compass stamp [-h] --hub DIR [--dry-run] [--token-env NAME] [--api-url URL]\n"

const help = usage + `
Write verified: process:<actor> stamps for the concepts each checks.txt row
covers, when its workflow job passed on the default-branch head.

options:
  -h, --help        show this help message and exit
  --hub DIR         hub checkout holding repos.txt (and checks.txt)
  --dry-run         read GitHub and log what would change; make no write calls
  --token-env NAME  environment variable holding the API token (default:
                    GITHUB_TOKEN)
  --api-url URL     GitHub REST API base (default: $GITHUB_API_URL or
                    https://api.github.com)
`

// Options are the parsed command-line flags.
type Options struct {
	Hub      string
	DryRun   bool
	TokenEnv string
	APIURL   string
}

// parseArgs reads the flags with argparse's rules: long options may be
// abbreviated to a unique prefix and take "--opt value" or "--opt=value".
// exit is -1 to continue, else the exit code after help or a usage error.
func parseArgs(args []string, stdout, stderr io.Writer) (o Options, exit int) {
	o.TokenEnv = "GITHUB_TOKEN"
	fail := func(msg string) (Options, int) {
		fmt.Fprint(stderr, usage)
		fmt.Fprintf(stderr, "compass stamp: error: %s\n", msg)
		return o, 2
	}
	names := []string{"--help", "--hub", "--dry-run", "--token-env", "--api-url"}
	hasHub := false
	var unknown []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "-h" {
			fmt.Fprint(stdout, help)
			return o, 0
		}
		if a == "--" {
			unknown = append(unknown, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(a, "--") {
			unknown = append(unknown, a)
			continue
		}
		name, value, hasValue := strings.Cut(a, "=")
		var match []string
		for _, n := range names {
			if n == name {
				match = []string{n}
				break
			}
			if strings.HasPrefix(n, name) {
				match = append(match, n)
			}
		}
		if len(match) > 1 {
			return fail(fmt.Sprintf("ambiguous option: %s could match %s", name, strings.Join(match, ", ")))
		}
		if len(match) == 0 {
			unknown = append(unknown, a)
			continue
		}
		switch opt := match[0]; opt {
		case "--help":
			fmt.Fprint(stdout, help)
			return o, 0
		case "--dry-run":
			if hasValue {
				return fail(fmt.Sprintf("argument --dry-run: ignored explicit argument '%s'", value))
			}
			o.DryRun = true
		default:
			if !hasValue {
				if i+1 >= len(args) || (strings.HasPrefix(args[i+1], "-") && args[i+1] != "-") {
					return fail(fmt.Sprintf("argument %s: expected one argument", opt))
				}
				i++
				value = args[i]
			}
			switch opt {
			case "--hub":
				o.Hub, hasHub = value, true
			case "--token-env":
				o.TokenEnv = value
			case "--api-url":
				o.APIURL = value
			}
		}
	}
	if !hasHub {
		return fail("the following arguments are required: --hub")
	}
	if len(unknown) > 0 {
		return fail("unrecognized arguments: " + strings.Join(unknown, " "))
	}
	return o, -1
}

// Main is "compass stamp". Log lines go to stdout.
func Main(args []string, stdout, stderr io.Writer) int {
	return MainWith(args, stdout, stderr, os.Getenv, github.New)
}

// MainWith is Main with the environment and the client constructor injected,
// for tests.
func MainWith(args []string, stdout, stderr io.Writer, getenv func(string) string,
	newClient func(token, baseURL string, dryRun bool) *github.Client) int {
	o, exit := parseArgs(args, stdout, stderr)
	if exit >= 0 {
		return exit
	}
	log := func(s string) { fmt.Fprintln(stdout, s) }
	org, err := config.Org(getenv)
	if err == nil {
		var token string
		if token, err = config.Token(o.TokenEnv, getenv); err == nil {
			return Run(o.Hub, newClient(token, APIURL(o.APIURL, getenv), o.DryRun), org, o.DryRun, log)
		}
	}
	log(fmt.Sprintf("%s: error: %v", Prog, err))
	return 2
}

// APIURL is the API base: the flag, else $GITHUB_API_URL, else the public API.
func APIURL(flag string, getenv func(string) string) string {
	if flag != "" {
		return flag
	}
	if v := getenv("GITHUB_API_URL"); v != "" {
		return v
	}
	return github.DefaultAPIURL
}
