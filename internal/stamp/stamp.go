// Package stamp implements "compass stamp": it writes
// "verified: process:<actor>" stamps from checks.txt rows on the hub.
//
// The hub is the GitHub repo the hub clone's origin names, and repos.txt and
// checks.txt come from the clone's fetched default branch (plan decision 7).
// For each row the stamper finds the latest push run of the named workflow
// file on the code repo's default-branch head, and the named job in it, and
// only reads the code repo (plan decision 6). If that job succeeded, it
// stamps the row's concepts, repos/<id>/<concept>.md on the hub's default
// branch as it read it, dated with the job's completion time. Every stamp of
// the run chains into commits on that head, and the hub's default branch
// moves to the last one with one fast-forward (plan decision 10).
package stamp

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
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

// refSHA returns branch's head in the repo at REST path api.
func refSHA(gh *github.Client, api, branch string) (string, error) {
	var ref struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	found, err := gh.GetOptional(api+"/git/ref/heads/"+github.QuotePath(branch), nil, &ref)
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
		// GitHub names are case-insensitive, and repos.txt may write the
		// owner in any case.
		fn, isStr := fullName.(string)
		if r["path"] != any(path) || r["event"] != any("push") || r["head_branch"] != any(def) ||
			r["head_sha"] != any(head) || !isStr || !strings.EqualFold(fn, repo.FullName()) {
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

// fetch returns the file text at commit sha of the repo at REST path api;
// found=false when the path does not exist.
func fetch(gh *github.Client, api, path, sha string) (text string, found bool, err error) {
	var data any
	found, err = gh.GetOptional(api+"/contents/"+github.QuotePath(path), url.Values{"ref": {sha}}, &data)
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

// target is where every stamp of a run lands: the hub's default branch at
// the head the run read.
type target struct {
	api, name string // REST path and owner/repo of the hub
	branch    string
	head      string
}

// where names the hub branch in log lines.
func (h *target) where() string { return h.name + " " + h.branch }

// stampRepo stamps every row of one code repo onto the hub head h read. It
// returns the commits to chain and one log line per row, printed once the
// push outcome is known; ok=false when a concept failed. err stops the repo.
func stampRepo(gh *github.Client, h *target, repo *config.Repo, rows []*config.CheckRow, dryRun bool,
	log func(string)) (commits []commit, lines []logLine, ok bool, err error) {
	ok = true
	var info struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := gh.Get(repo.API(), nil, &info); err != nil {
		return nil, nil, false, err
	}
	def := info.DefaultBranch
	head, err := refSHA(gh, repo.API(), def)
	if err != nil {
		return nil, nil, false, err
	}
	prefix := "repos/" + repo.Name + "/"
	files := map[string]string{} // path -> current text, starting from the hub head

	for _, row := range rows {
		label := fmt.Sprintf("%s: %s %s/%s %s", Prog, repo.Name, row.Workflow, row.Job, row.Actor)
		completed, err := findJob(gh, repo, row, def, head)
		if err != nil {
			return nil, nil, false, err
		}
		if completed == "" {
			lines = append(lines, logLine{row, fmt.Sprintf("%s: no check run on head %s", label, short(head))})
			continue
		}
		at, err := frontmatter.ParseTime(completed)
		if err != nil {
			return nil, nil, false, err
		}
		changed := map[string]string{}
		var stamped, missing []string
		for _, cid := range row.Concepts {
			path := prefix + cid + ".md"
			var err error
			text, have := files[path]
			if !have {
				var found bool
				text, found, err = fetch(gh, h.api, path, h.head)
				if err != nil && !isFileErr(err) {
					return nil, nil, false, err
				}
				if err == nil && !found {
					missing = append(missing, path)
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
			log(fmt.Sprintf("%s: error: %s: concept not found at %s %s: %s",
				Prog, row.Where(), h.where(), short(h.head), strings.Join(missing, ", ")))
			ok = false
		}
		if len(changed) > 0 {
			message := fmt.Sprintf("okf: process stamps from %s (%s %s/%s @ %s)", row.Actor, repo.Name, row.Workflow, row.Job, short(head))
			commits = append(commits, commit{row, message, changed})
			verb := "stamped"
			if dryRun {
				verb = "would stamp"
			}
			lines = append(lines, logLine{row, fmt.Sprintf("%s: %s %s on %s at %s",
				label, verb, strings.Join(stamped, ", "), h.where(), frontmatter.FormatTime(at))})
		} else {
			lines = append(lines, logLine{row, fmt.Sprintf("%s: nothing due on %s %s", label, h.where(), short(h.head))})
		}
	}
	return commits, lines, ok, nil
}

// push chains commits onto the hub head h read and moves h.branch to the last
// one with a single fast-forward (plan decision 10). suffix ends each
// committed row's log line; refusal is set when GitHub refused the update
// while the head had not moved.
func push(gh *github.Client, h *target, commits []commit) (suffix, refusal string, err error) {
	parent := h.head
	var base struct {
		Tree struct {
			SHA string `json:"sha"`
		} `json:"tree"`
	}
	if err := gh.Get(h.api+"/git/commits/"+h.head, nil, &base); err != nil {
		return "", "", err
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
		if _, err := gh.Post(h.api+"/git/trees", map[string]any{"base_tree": tree, "tree": entries}, &t); err != nil {
			return "", "", err
		}
		if _, err := gh.Post(h.api+"/git/commits",
			map[string]any{"message": c.message, "tree": t.SHA, "parents": []string{parent}}, &nc); err != nil {
			return "", "", err
		}
		tree, parent = t.SHA, nc.SHA
	}
	var refused struct {
		Message string `json:"message"`
	}
	status, err := gh.Patch(h.api+"/git/refs/heads/"+github.QuotePath(h.branch),
		map[string]any{"sha": parent, "force": false}, &refused, 409, 422)
	if err != nil {
		return "", "", err
	}
	if status != 409 && status != 422 {
		return fmt.Sprintf(" (commit %s)", short(parent)), "", nil
	}
	// A race and a protection refusal share these statuses, and GitHub's
	// message text is not a contract, so the head decides.
	now, err := refSHA(gh, h.api, h.branch)
	if err != nil {
		return "", "", err
	}
	if now != h.head {
		return fmt.Sprintf(" (not pushed: %s moved after %s; next run retries)", h.where(), short(h.head)), "", nil
	}
	return " (not pushed: refused)", fmt.Sprintf("GitHub refused the update to %s at %s with HTTP %d %q;"+
		" branch protection or a ruleset likely blocks this account, which needs a bypass on the hub's %s",
		h.where(), short(h.head), status, refused.Message, h.branch), nil
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// Run stamps every checks.txt row and returns the exit code: 0 when every
// repo succeeded, 1 when any failed or the hub could not be read or written,
// 2 on a configuration error. hubDir is the hub clone: its origin names the
// hub, and repos.txt and checks.txt come from its default branch as fetched
// now (plan decision 7).
func Run(hubDir string, gh *github.Client, dryRun bool, log func(string)) int {
	configErr := func(err error) int {
		log(fmt.Sprintf("%s: error: %v", Prog, err))
		return 2
	}
	owner, name, err := config.HubRepo(hubDir)
	if err != nil {
		return configErr(err)
	}
	h := &target{api: "/repos/" + owner + "/" + name, name: owner + "/" + name}
	hubErr := func(err error) int {
		log(fmt.Sprintf("%s: error: hub %s: %v", Prog, h.name, err))
		return 1
	}
	if err := config.Fetch(hubDir); err != nil {
		return hubErr(err)
	}
	if h.branch, err = config.RemoteDefault(hubDir); err != nil {
		return configErr(err)
	}
	repos, err := config.HubRepos(hubDir, h.branch)
	if err != nil {
		return configErr(err)
	}
	checksTxt, err := config.ControlFile(hubDir, h.branch, "checks.txt")
	if err != nil {
		return configErr(err)
	}
	rows, err := config.ParseChecks(checksTxt, repos)
	if err != nil {
		return configErr(err)
	}
	var order []string
	byRepo := map[string][]*config.CheckRow{}
	for _, row := range rows {
		if _, seen := byRepo[row.Repo.Name]; !seen {
			order = append(order, row.Repo.Name)
		}
		byRepo[row.Repo.Name] = append(byRepo[row.Repo.Name], row)
	}
	if len(order) == 0 {
		return 0
	}
	if h.head, err = refSHA(gh, h.api, h.branch); err != nil {
		return hubErr(err)
	}
	ok := true
	var commits []commit
	var lines []logLine
	for _, name := range order {
		c, l, repoOK, err := stampRepo(gh, h, repos.ByName[name], byRepo[name], dryRun, log)
		if err != nil {
			log(fmt.Sprintf("%s: error: %s: %v", Prog, name, err))
			ok = false
			continue
		}
		commits = append(commits, c...)
		lines = append(lines, l...)
		ok = repoOK && ok
	}
	suffix, refusal := "", ""
	if len(commits) > 0 && !dryRun {
		if suffix, refusal, err = push(gh, h, commits); err != nil {
			return hubErr(err)
		}
	}
	committed := map[*config.CheckRow]bool{}
	for _, c := range commits {
		committed[c.row] = true
	}
	for _, l := range lines {
		if committed[l.row] {
			log(l.text + suffix)
		} else {
			log(l.text)
		}
	}
	if refusal != "" {
		log(fmt.Sprintf("%s: error: hub %s: %s", Prog, h.name, refusal))
		ok = false
	}
	if ok {
		return 0
	}
	return 1
}

const usage = "usage: compass stamp [-h] [--hub DIR] [--dry-run] [--token-env NAME] [--api-url URL]\n"

const help = usage + `
Write verified: process:<actor> stamps for the concepts each checks.txt row
covers, at repos/<id>/<concept>.md on the hub's default branch, when its
workflow job passed on the head of the code repo's default branch. Every
stamp of the run lands with one fast-forward of the hub's default branch,
which needs a ruleset bypass for this account; code repos are only read.

options:
  -h, --help        show this help message and exit
  --hub DIR         hub clone whose origin names the hub repo; repos.txt and
                    checks.txt are read from its default branch as fetched
                    now (default: the hub clone that ` + "`compass setup`" + `
                    recorded)
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
				o.Hub = value
			case "--token-env":
				o.TokenEnv = value
			case "--api-url":
				o.APIURL = value
			}
		}
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
	hub := o.Hub
	var err error
	if hub == "" {
		hub, err = config.HubDir(getenv)
	}
	if err == nil {
		var token string
		if token, err = config.Token(o.TokenEnv, getenv); err == nil {
			return Run(hub, newClient(token, APIURL(o.APIURL, getenv), o.DryRun), o.DryRun, log)
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
