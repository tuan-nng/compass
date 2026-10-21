// Package syncjob is `compass sync`: merge approved hub pull requests once
// their code pull requests merge, and clean up hub branches.
//
// The hub is the GitHub repo its clone's origin names, and repos.txt comes
// from the clone's fetched default branch, never the working tree (plan
// decision 7). For every id in repos.txt and every hub branch <id>/<b>, the
// job applies the state table in plan Interfaces, "compass sync", with the
// merge conditions in plan Trust boundaries. A hub branch pairs with branch
// <b> of the code repo repos.txt names <id> (plan decision 5).
//
// Code repos are only read (plan decision 6): every comment goes on the hub
// pull request, and what a code pull request comment used to say is a line
// of sync's output. Before a merge, sync applies the path and file-mode rules
// to the branch, then builds the merge of the default head and the pull
// request head in the hub clone, with no worktree, hook, filter or attribute,
// and runs `compass hub check` on both trees; it refuses only findings the
// merge has that the default head lacks (plan decision 11). It keeps no
// state: every run re-reads GitHub, and each comment carries a hidden marker
// so it is never posted twice.
package syncjob

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"compass/internal/config"
	"compass/internal/github"
	"compass/internal/hub"
)

const (
	prog         = "sync"
	abandonAfter = 7 * 24 * time.Hour
	// maxListed caps the paths or findings one comment lists, so a broken
	// branch can't exceed GitHub's comment size.
	maxListed = 20
	// emptyTree is git's empty tree. As merge-tree's attribute source it
	// replaces the .gitattributes files in the trees being merged; with
	// core.attributesFile set to the null device, the user's attributes file
	// is ignored too, so no merge driver or filter a branch names runs.
	// $GIT_DIR/info/attributes still applies: it is the hub clone owner's own
	// configuration, which no branch can write.
	emptyTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
)

var writeRoles = map[string]bool{"admin": true, "maintain": true, "write": true}

func marker(kind string, pr int) string {
	return fmt.Sprintf("<!-- okf-sync:%s:%d -->", kind, pr)
}

// repoFailure stops work on one repo.
type repoFailure struct{ msg string }

func (e *repoFailure) Error() string { return e.msg }

// missingKey mirrors a Python KeyError on a required response field.
func missingKey(key string) error { return &repoFailure{fmt.Sprintf("'%s'", key)} }

type pull struct {
	Number    int     `json:"number"`
	State     string  `json:"state"`
	Body      string  `json:"body"`
	HTMLURL   string  `json:"html_url"`
	MergedAt  *string `json:"merged_at"`
	Merged    bool    `json:"merged"`
	Mergeable *bool   `json:"mergeable"`
	Head      struct {
		Ref  string `json:"ref"`
		SHA  string `json:"sha"`
		Repo *struct {
			FullName string `json:"full_name"`
		} `json:"repo"`
	} `json:"head"`
	Base struct {
		Ref string `json:"ref"`
	} `json:"base"`
}

func (p *pull) merged() bool { return p.MergedAt != nil && *p.MergedAt != "" }

type ref struct {
	Ref    string `json:"ref"`
	Object *struct {
		SHA string `json:"sha"`
	} `json:"object"`
}

type user struct {
	Login string `json:"login"`
	Type  string `json:"type"`
}

type review struct {
	User     *user  `json:"user"`
	State    string `json:"state"`
	CommitID string `json:"commit_id"`
}

type comment struct {
	Body string `json:"body"`
}

// sameRepo reports whether p's head lives in repo fullName itself, not a
// fork.
func sameRepo(p *pull, fullName string) bool {
	name := ""
	if p.Head.Repo != nil {
		name = p.Head.Repo.FullName
	}
	return strings.EqualFold(name, fullName)
}

// refPattern is an owner/repo#N reference, matched anchored at each
// candidate position; links applies the Python look-behind by hand. Python's
// \d is any Unicode decimal digit, \p{Nd}. A short #N has no repo, so it
// never matches: the two pull requests live in different repos (plan
// decision 5).
var refPattern = regexp.MustCompile(`^([\p{L}\p{N}_.-]+/[\p{L}\p{N}_.-]+)#(\p{Nd}+)`)

func isWord(r rune) bool { return r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r) }

func digitAt(s string, i int) bool {
	if i >= len(s) {
		return false
	}
	r, _ := utf8.DecodeRuneInString(s[i:])
	return unicode.IsDigit(r)
}

// digitValue is the value of a Unicode decimal digit. Every Nd block is a
// run of ten code points, 0 to 9, so the value is the offset from the start
// of the run, modulo 10.
func digitValue(r rune) int {
	start := r
	for unicode.IsDigit(start - 1) {
		start--
	}
	return int(r-start) % 10
}

// refIs reports whether digits, read as Python's int() reads them, equal n.
func refIs(digits string, n int) bool {
	v := 0
	for _, r := range digits {
		v = v*10 + digitValue(r)
		if v > n {
			return false
		}
	}
	return v == n
}

// links reports whether src's body names pull request target, which lives in
// repo targetRepo (owner/repo): by its URL or as targetRepo#N.
func links(src, target *pull, targetRepo string) bool {
	body := src.Body
	n := target.Number
	if u := strings.TrimRight(target.HTMLURL, "/"); u != "" {
		for i := 0; i <= len(body)-len(u); {
			j := strings.Index(body[i:], u)
			if j < 0 {
				break
			}
			if !digitAt(body, i+j+len(u)) {
				return true
			}
			i += j + 1
		}
	}
	for i := 0; i < len(body); {
		if i > 0 {
			prev, _ := utf8.DecodeLastRuneInString(body[:i])
			if isWord(prev) || prev == '#' || prev == '/' || prev == '&' {
				_, size := utf8.DecodeRuneInString(body[i:])
				i += size
				continue
			}
		}
		m := refPattern.FindStringSubmatchIndex(body[i:])
		if m == nil {
			_, size := utf8.DecodeRuneInString(body[i:])
			i += size
			continue
		}
		if refIs(body[i+m[4]:i+m[5]], n) && strings.EqualFold(body[i+m[2]:i+m[3]], targetRepo) {
			return true
		}
		i += m[1]
	}
	return false
}

// --- the hub clone ---------------------------------------------------------

// hubClone is the hub: the GitHub repo its clone's origin names, and the
// clone, fetched once per run.
type hubClone struct {
	dir    string
	owner  string
	name   string // owner/repo
	api    string
	def    string // default branch
	defSHA string // fetched default head, the one every check compares with
	// defFindings and defErr cache `compass hub check` on defSHA until a merge
	// moves it.
	defFindings []hub.Finding
	defErr      error
	defChecked  bool
}

// gitStatus runs git in the hub clone with prompts off and no hooks. code is
// git's exit status; err is set when git could not run or exited with a
// status other than 0 or 1, and names git's stderr.
func (h *hubClone) gitStatus(stdin io.Reader, args ...string) (out []byte, code int, err error) {
	full := append([]string{"-C", h.dir, "-c", "core.hooksPath=" + os.DevNull}, args...)
	cmd := exec.Command("git", full...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	cmd.Stdin = stdin
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err = cmd.Run()
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() == 1 {
		return stdout.Bytes(), 1, nil
	}
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, -1, &repoFailure{fmt.Sprintf("git %s: %s", strings.Join(args, " "), msg)}
	}
	return stdout.Bytes(), 0, nil
}

// git runs git in the hub clone and returns its trimmed stdout; exit 1 is an
// error too.
func (h *hubClone) git(args ...string) (string, error) {
	out, code, err := h.gitStatus(nil, args...)
	if err == nil && code != 0 {
		err = &repoFailure{fmt.Sprintf("git %s: exit status %d", strings.Join(args, " "), code)}
	}
	return strings.TrimSpace(string(out)), err
}

type hubBranch struct{ name, b, sha string }

// branches returns the fetched hub branches <id>/<b>.
func (h *hubClone) branches(id string) ([]hubBranch, error) {
	out, err := h.git("for-each-ref", "--format=%(refname)%00%(objectname)", "refs/remotes/origin/"+id)
	if err != nil {
		return nil, err
	}
	var list []hubBranch
	for _, l := range strings.Split(out, "\n") {
		name, sha, ok := strings.Cut(l, "\x00")
		if !ok {
			continue
		}
		name = strings.TrimPrefix(name, "refs/remotes/origin/")
		if b, ok := strings.CutPrefix(name, id+"/"); ok && b != "" {
			list = append(list, hubBranch{name, b, sha})
		}
	}
	return list, nil
}

func (h *hubClone) commitTime(sha string) (time.Time, error) {
	out, err := h.git("log", "-1", "--format=%ct", sha)
	if err != nil {
		return time.Time{}, err
	}
	secs, err := strconv.ParseInt(out, 10, 64)
	if err != nil {
		return time.Time{}, &repoFailure{fmt.Sprintf("commit time of %s: %q", short(sha), out)}
	}
	return time.Unix(secs, 0).UTC(), nil
}

// onDefault reports whether every commit of sha is on the fetched default
// head.
func (h *hubClone) onDefault(sha string) (bool, error) {
	_, code, err := h.gitStatus(nil, "merge-base", "--is-ancestor", sha, h.defSHA)
	return code == 0, err
}

type change struct {
	path, mode string // mode is the new side's, 000000 for a deletion
}

// changes lists every path that differs between base and head, with rename
// detection off, so both sides of a move count.
func (h *hubClone) changes(base, head string) ([]change, error) {
	out, _, err := h.gitStatus(nil, "diff-tree", "-r", "-z", "--no-renames", "--no-ext-diff", "--no-textconv", base, head)
	if err != nil {
		return nil, err
	}
	fields := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
	var list []change
	for i := 0; i+1 < len(fields); i += 2 {
		meta := strings.Fields(strings.TrimPrefix(fields[i], ":"))
		if len(meta) < 5 {
			return nil, &repoFailure{fmt.Sprintf("git diff-tree: unexpected line %q", fields[i])}
		}
		list = append(list, change{fields[i+1], meta[1]})
	}
	return list, nil
}

// mergeTree builds the merge of the default head and head in the object
// store only: no worktree and no hook. Attributes come from the empty tree
// and core.attributesFile is the null device, so neither the trees'
// .gitattributes nor the user's attributes file applies ($GIT_DIR/info/
// attributes still does; see emptyTree). clean is false on a conflict.
func (h *hubClone) mergeTree(head string) (tree string, clean bool, err error) {
	out, code, err := h.gitStatus(nil, "-c", "core.attributesFile="+os.DevNull, "--attr-source="+emptyTree,
		"merge-tree", "--write-tree", "--no-messages", h.defSHA, head)
	if err != nil || code != 0 {
		return "", false, err
	}
	tree, _, _ = strings.Cut(string(out), "\n")
	return strings.TrimSpace(tree), true, nil
}

// unsafePath is a tree path materialize refuses to write out. check turns it
// into a blocker on the branch whose merge holds it.
type unsafePath struct{ tree, path string }

func (e *unsafePath) Error() string {
	return fmt.Sprintf("tree %s holds the unsafe path %s", short(e.tree), quoted(e.path))
}

// materialize writes tree's regular files into a new temporary folder, from
// raw blobs, so no git filter or attribute applies, and returns the folder.
// Symlinks and submodule links are skipped: a branch can add neither (the
// file-mode rule), and one already on the default head is skipped on both
// sides of the comparison. A path that would leave the folder or name .git
// is an *unsafePath error.
func (h *hubClone) materialize(tree string) (string, error) {
	out, _, err := h.gitStatus(nil, "ls-tree", "-r", "-z", "--full-tree", tree)
	if err != nil {
		return "", err
	}
	var paths []string
	var batch strings.Builder
	for _, rec := range strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00") {
		meta, path, ok := strings.Cut(rec, "\t")
		f := strings.Fields(meta)
		if !ok || len(f) != 3 {
			continue
		}
		if f[1] != "blob" || (f[0] != "100644" && f[0] != "100755") {
			continue
		}
		if !safePath(path) {
			return "", &unsafePath{tree, path}
		}
		paths = append(paths, path)
		batch.WriteString(f[2] + "\n")
	}
	dir, err := os.MkdirTemp("", "compass-sync-")
	if err != nil {
		return "", err
	}
	if err := h.writeBlobs(dir, batch.String(), paths); err != nil {
		os.RemoveAll(dir)
		return "", err
	}
	return dir, nil
}

// writeBlobs reads one blob per path through one `git cat-file --batch` fed
// oids, one per line, and writes blob i to dir/paths[i].
func (h *hubClone) writeBlobs(dir, oids string, paths []string) error {
	cmd := exec.Command("git", "-C", h.dir, "cat-file", "--batch")
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	cmd.Stdin = strings.NewReader(oids)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	r := bufio.NewReader(pipe)
	var werr error
	for _, p := range paths {
		if werr = writeBlob(r, filepath.Join(dir, filepath.FromSlash(p))); werr != nil {
			break
		}
	}
	if werr != nil {
		_, _ = io.Copy(io.Discard, r)
	}
	if err := cmd.Wait(); err != nil {
		return &repoFailure{fmt.Sprintf("git cat-file --batch: %v %s", err, strings.TrimSpace(stderr.String()))}
	}
	return werr
}

// writeBlob reads one `git cat-file --batch` record and writes its content to
// path.
func writeBlob(r *bufio.Reader, path string) error {
	header, err := r.ReadString('\n')
	if err != nil {
		return &repoFailure{"git cat-file --batch: " + err.Error()}
	}
	f := strings.Fields(header)
	if len(f) != 3 || f[1] != "blob" {
		return &repoFailure{fmt.Sprintf("git cat-file --batch: unexpected %q", strings.TrimSpace(header))}
	}
	size, err := strconv.Atoi(f[2])
	if err != nil {
		return &repoFailure{fmt.Sprintf("git cat-file --batch: bad size in %q", strings.TrimSpace(header))}
	}
	data := make([]byte, size+1) // content and its trailing newline
	if _, err := io.ReadFull(r, data); err != nil {
		return &repoFailure{"git cat-file --batch: " + err.Error()}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data[:size], 0o644)
}

// safePath reports whether a tree path stays inside the folder it is written
// to and names no .git.
func safePath(p string) bool {
	for _, part := range strings.Split(p, "/") {
		if part == "" || part == "." || part == ".." || strings.EqualFold(part, ".git") || strings.ContainsRune(part, '\\') {
			return false
		}
	}
	return true
}

// findings runs `compass hub check` on a materialized tree.
func (h *hubClone) findings(tree string) ([]hub.Finding, error) {
	dir, err := h.materialize(tree)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	found, err := hub.Check(dir)
	if err != nil {
		return nil, &repoFailure{"hub check: " + err.Error()}
	}
	return found, nil
}

// defaultFindings is `compass hub check` on the fetched default head, run
// once per default head.
func (h *hubClone) defaultFindings() ([]hub.Finding, error) {
	if !h.defChecked {
		h.defFindings, h.defErr = h.findings(h.defSHA)
		h.defChecked = true
	}
	return h.defFindings, h.defErr
}

// refresh fetches the hub again and moves the default head to the fetched
// one, dropping its cached findings, so a branch checked after a merge in the
// same run compares with the default head that merge made.
func (h *hubClone) refresh() error {
	if err := config.Fetch(h.dir); err != nil {
		return &repoFailure{"fetching the hub after a merge: " + err.Error()}
	}
	sha, err := h.git("rev-parse", "--verify", "refs/remotes/origin/"+h.def+"^{commit}")
	if err != nil {
		return err
	}
	h.defSHA, h.defFindings, h.defErr, h.defChecked = sha, nil, nil, false
	return nil
}

// pathAllowed is the path rule in plan Trust boundaries: a branch for id may
// add, change or delete only repos/<id>/**, repos/index.md, and files outside
// repos/ other than repos.txt, checks.txt, any CODEOWNERS, any .gitattributes
// or .gitmodules, .gitignore, and anything under .github/. Names compare
// case-insensitively wherever a case-insensitive checkout would make two
// spellings one file: a first component that folds to repos but is spelled
// otherwise is refused, as are root control files in any case.
func pathAllowed(id, p string) bool {
	base := p[strings.LastIndexByte(p, '/')+1:]
	for _, name := range []string{"CODEOWNERS", ".gitattributes", ".gitmodules"} {
		if strings.EqualFold(base, name) {
			return false
		}
	}
	first, rest, _ := strings.Cut(p, "/")
	if strings.EqualFold(first, "repos") {
		return first == "repos" && (rest == "index.md" || strings.HasPrefix(rest, id+"/"))
	}
	lower := strings.ToLower(p)
	return lower != "repos.txt" && lower != "checks.txt" && lower != ".gitignore" && !strings.HasPrefix(lower, ".github/")
}

// quoted renders an untrusted path as inline code in a comment.
func quoted(p string) string {
	if strings.ContainsAny(p, "`\n\r") || !utf8.ValidString(p) {
		p = strconv.Quote(p)
	}
	return "`" + p + "`"
}

// listed joins at most maxListed items, saying how many more there are.
func listed(items []string) string {
	if len(items) > maxListed {
		return strings.Join(items[:maxListed], ", ") + fmt.Sprintf(" and %d more", len(items)-maxListed)
	}
	return strings.Join(items, ", ")
}

// --- one repo --------------------------------------------------------------

type syncer struct {
	gh     *github.Client
	hub    *hubClone
	repo   *config.Repo
	now    time.Time
	dryRun bool
	ignore map[string]bool
	log    func(string)
	api    string // the code repo's REST path; only read
	perm   map[string]bool
	// codeDefault is the code repo's default branch, read once when needed.
	codeDefault string
}

type blocker struct{ kind, reason string }

// --- reads ---------------------------------------------------------------

// codeBranch reports whether branch b is on the code repo's remote.
func (s *syncer) codeBranch(b string) (bool, error) {
	var r ref
	return s.gh.GetOptional(s.api+"/git/ref/heads/"+github.QuotePath(b), nil, &r)
}

func (s *syncer) codeDefaultBranch() (string, error) {
	if s.codeDefault == "" {
		var info struct {
			DefaultBranch *string `json:"default_branch"`
		}
		if err := s.gh.Get(s.api, nil, &info); err != nil {
			return "", err
		}
		if info.DefaultBranch == nil {
			return "", missingKey("default_branch")
		}
		s.codeDefault = *info.DefaultBranch
	}
	return s.codeDefault, nil
}

// latestCodePR is the newest pull request from b in the code repo itself.
func (s *syncer) latestCodePR(b string) (*pull, error) {
	pulls, err := github.Paginate[*pull](s.gh, s.api+"/pulls", url.Values{
		"state": {"all"}, "head": {s.repo.Owner + ":" + b}, "sort": {"created"}, "direction": {"desc"}}, "")
	if err != nil {
		return nil, err
	}
	var best *pull
	for _, p := range pulls {
		if p.Head.Ref == b && sameRepo(p, s.repo.FullName()) {
			if best == nil || p.Number > best.Number {
				best = p
			}
		}
	}
	return best, nil
}

// hubPRs returns the open hub pull request from hub branch hb into the hub's
// default branch, and the latest closed one. A pull request from a fork or
// into another base never counts.
func (s *syncer) hubPRs(hb string) (open, closed *pull, err error) {
	pulls, err := github.Paginate[*pull](s.gh, s.hub.api+"/pulls", url.Values{
		"state": {"all"}, "head": {s.hub.owner + ":" + hb}, "sort": {"created"}, "direction": {"desc"}}, "")
	if err != nil {
		return nil, nil, err
	}
	for _, p := range pulls {
		if p.Head.Ref != hb || p.Base.Ref != s.hub.def || !sameRepo(p, s.hub.name) {
			continue
		}
		if p.State == "open" {
			if open == nil || p.Number < open.Number {
				open = p
			}
		} else if closed == nil || p.Number > closed.Number {
			closed = p
		}
	}
	return open, closed, nil
}

// hasWrite reports whether login has write access to the hub.
func (s *syncer) hasWrite(login string) (bool, error) {
	if v, ok := s.perm[login]; ok {
		return v, nil
	}
	var p struct {
		RoleName   string `json:"role_name"`
		Permission string `json:"permission"`
	}
	if _, err := s.gh.GetOptional(s.hub.api+"/collaborators/"+github.QuotePath(login)+"/permission", nil, &p); err != nil {
		return false, err
	}
	v := writeRoles[p.RoleName] || p.Permission == "admin" || p.Permission == "write"
	s.perm[login] = v
	return v, nil
}

// approver returns a human with write access to the hub whose latest review
// approves head.
func (s *syncer) approver(number int, head string) (string, error) {
	reviews, err := github.Paginate[review](s.gh, fmt.Sprintf("%s/pulls/%d/reviews", s.hub.api, number), nil, "")
	if err != nil {
		return "", err
	}
	var order []string
	latest := map[string]review{}
	for _, r := range reviews {
		u := user{}
		if r.User != nil {
			u = *r.User
		}
		login := u.Login
		if login == "" || u.Type == "Bot" || strings.HasSuffix(login, "[bot]") || s.ignore[strings.ToLower(login)] {
			continue
		}
		switch r.State {
		case "APPROVED", "CHANGES_REQUESTED", "DISMISSED":
			if _, seen := latest[login]; !seen {
				order = append(order, login)
			}
			latest[login] = r
		}
	}
	for _, login := range order {
		r := latest[login]
		if r.State == "APPROVED" && r.CommitID == head {
			ok, err := s.hasWrite(login)
			if err != nil {
				return "", err
			}
			if ok {
				return login, nil
			}
		}
	}
	return "", nil
}

// --- writes, all on the hub (each one checks dry-run first) ---------------

// comment posts one marked comment on hub pull request number unless it is
// already there, and returns a log fragment.
func (s *syncer) comment(number int, kind, text string) (string, error) {
	m := marker(kind, number)
	path := fmt.Sprintf("%s/issues/%d/comments", s.hub.api, number)
	comments, err := github.Paginate[comment](s.gh, path, nil, "")
	if err != nil {
		return "", err
	}
	for _, c := range comments {
		if strings.Contains(c.Body, m) {
			return fmt.Sprintf("already commented on hub #%d", number), nil
		}
	}
	if s.dryRun {
		return fmt.Sprintf("would comment on hub #%d", number), nil
	}
	if _, err := s.gh.Post(path, map[string]string{"body": m + "\n" + text}, nil); err != nil {
		return "", err
	}
	return fmt.Sprintf("commented on hub #%d", number), nil
}

// deleteBranch deletes hub branch hb, never the hub's default branch.
func (s *syncer) deleteBranch(hb string) (string, error) {
	if hb == s.hub.def || !strings.HasPrefix(hb, s.repo.Name+"/") || hb == s.repo.Name+"/" {
		return "", &repoFailure{"refusing to delete " + hb}
	}
	if s.dryRun {
		return "would delete " + hb, nil
	}
	if _, err := s.gh.Delete(s.hub.api + "/git/refs/heads/" + github.QuotePath(hb)); err != nil {
		return "", err
	}
	return "deleted " + hb, nil
}

func (s *syncer) closeAndDelete(hb string, hpr *pull, kind, why string) (string, error) {
	var done []string
	if hpr != nil {
		d, err := s.comment(hpr.Number, kind, fmt.Sprintf("okf-sync closed this hub pull request: %s.", why))
		if err != nil {
			return "", err
		}
		done = append(done, d)
		if s.dryRun {
			done = append(done, fmt.Sprintf("would close hub #%d", hpr.Number))
		} else {
			if _, err := s.gh.Patch(fmt.Sprintf("%s/pulls/%d", s.hub.api, hpr.Number), map[string]string{"state": "closed"}, nil); err != nil {
				return "", err
			}
			done = append(done, fmt.Sprintf("closed hub #%d", hpr.Number))
		}
	}
	d, err := s.deleteBranch(hb)
	if err != nil {
		return "", err
	}
	done = append(done, d)
	return why + ": " + strings.Join(done, "; "), nil
}

// --- the state table -----------------------------------------------------

func (s *syncer) run() error {
	branches, err := s.hub.branches(s.repo.Name)
	if err != nil {
		return err
	}
	if len(branches) == 0 {
		s.log(fmt.Sprintf("%s: %s: no hub branches %s/<b>", prog, s.repo.Name, s.repo.Name))
	}
	for _, br := range branches {
		line, err := s.branch(br)
		if line != "" {
			s.log(fmt.Sprintf("%s: %s: %s", prog, br.name, line))
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// branch applies the first matching row of the state table to one hub
// branch and returns its log line.
func (s *syncer) branch(br hubBranch) (string, error) {
	hb, b := br.name, br.b
	open, closed, err := s.hubPRs(hb)
	if err != nil {
		return "", err
	}
	// Any merge, a squash or rebase included, finishes the branch; only <b>
	// still on the remote keeps it.
	if open == nil && closed != nil && closed.merged() {
		what := fmt.Sprintf("hub #%d merged", closed.Number)
		return s.deleteOnceGone(b, hb, "nothing: "+what, what)
	}
	code, err := s.latestCodePR(b)
	if err != nil {
		return "", err
	}
	repo := s.repo.FullName()
	if code == nil {
		on, err := s.codeBranch(b)
		if err != nil {
			return "", err
		}
		if on {
			return fmt.Sprintf("nothing: no pull request from %s in %s; %s is on the remote", b, repo, b), nil
		}
		last, err := s.hub.commitTime(br.sha)
		if err != nil {
			return "", err
		}
		age := s.now.Sub(last)
		if age <= abandonAfter {
			days := int(math.Floor(age.Hours() / 24))
			return fmt.Sprintf("nothing: no pull request from %s in %s; last commit %dd ago", b, repo, days), nil
		}
		return s.closeAndDelete(hb, open, "abandoned",
			fmt.Sprintf("no pull request from %s in %s, %s is gone there and %s has no commits for 7 days", b, repo, b, hb))
	}
	c := fmt.Sprintf("%s#%d", repo, code.Number)
	if code.State == "open" {
		return fmt.Sprintf("nothing: %s is open", c), nil
	}
	if !code.merged() {
		on, err := s.codeBranch(b)
		if err != nil {
			return "", err
		}
		if on {
			return fmt.Sprintf("nothing: %s closed unmerged; %s is on the remote", c, b), nil
		}
		return s.closeAndDelete(hb, open, "closed-unmerged", fmt.Sprintf("%s closed without merging and %s is gone", c, b))
	}
	def, err := s.codeDefaultBranch()
	if err != nil {
		return "", err
	}
	if code.Base.Ref != def {
		return fmt.Sprintf("nothing: %s merged into %s, not %s; treated as not merged", c, code.Base.Ref, def), nil
	}
	if open != nil {
		return s.tryMerge(b, code, open)
	}
	if closed != nil {
		what := fmt.Sprintf("hub pull request rejected: hub #%d closed unmerged", closed.Number)
		return s.deleteOnceGone(b, hb, what, what)
	}
	on, err := s.hub.onDefault(br.sha)
	if err != nil {
		return "", err
	}
	if !on {
		return fmt.Sprintf("%s merged; %s has commits not on %s and no hub pull request: needs a hub pull request",
			c, hb, s.hub.def), nil
	}
	return s.deleteOnceGone(b, hb,
		fmt.Sprintf("nothing: %s merged and %s is on %s", c, hb, s.hub.def),
		fmt.Sprintf("%s merged and %s is on %s", c, hb, s.hub.def))
}

// deleteOnceGone deletes hub branch hb when b is gone from the code repo.
// kept and deleted prefix the log line of each case.
func (s *syncer) deleteOnceGone(b, hb, kept, deleted string) (string, error) {
	on, err := s.codeBranch(b)
	if err != nil {
		return "", err
	}
	if on {
		return fmt.Sprintf("%s; %s is on the remote", kept, b), nil
	}
	d, err := s.deleteBranch(hb)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s, %s is gone: %s", deleted, b, d), nil
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// tryMerge merges hub pull request hpr when every merge condition in plan
// Trust boundaries holds, and otherwise comments once per blocker on it.
func (s *syncer) tryMerge(b string, code, hpr *pull) (string, error) {
	c, k := fmt.Sprintf("%s#%d", s.repo.FullName(), code.Number), fmt.Sprintf("hub #%d", hpr.Number)
	if !(links(code, hpr, s.hub.name) || links(hpr, code, s.repo.FullName())) {
		done, err := s.comment(hpr.Number, "unlinked", fmt.Sprintf(
			"okf-sync: %s, the latest pull request from %s, merged, but neither pull request links the "+
				"other, so this one is not merged. If they belong together, write %s in this description.", c, b, c))
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("not merged: %s and %s do not link each other: %s", c, k, done), nil
	}
	var pr pull
	if err := s.gh.Get(fmt.Sprintf("%s/pulls/%d", s.hub.api, hpr.Number), nil, &pr); err != nil {
		return "", err
	}
	if pr.Merged || pr.State != "open" {
		state := pr.State
		if pr.Merged {
			state = "merged"
		} else if state == "" {
			state = "None"
		}
		return fmt.Sprintf("nothing: %s is already %s", k, state), nil
	}
	if pr.Base.Ref != s.hub.def || !sameRepo(&pr, s.hub.name) {
		return fmt.Sprintf("nothing: %s no longer goes from the hub's %s into %s", k, hpr.Head.Ref, s.hub.def), nil
	}
	head := pr.Head.SHA
	if head == "" {
		return "", missingKey("sha")
	}
	approved, err := s.approver(pr.Number, head)
	if err != nil {
		return "", err
	}
	var blockers []blocker
	if approved == "" {
		blockers = append(blockers, blocker{"unapproved", fmt.Sprintf("no approval of its head commit %s from a person with write access to the hub", short(head))})
	}
	conflict := pr.Mergeable != nil && !*pr.Mergeable
	if conflict {
		blockers = append(blockers, blocker{"conflict", "GitHub reports that it conflicts with " + s.hub.def})
	}
	local, waiting, err := s.check(head, !conflict)
	if err != nil {
		return "", err
	}
	blockers = append(blockers, local...)
	if len(blockers) > 0 {
		var done, reasons []string
		for _, bl := range blockers {
			d, err := s.comment(hpr.Number, bl.kind, fmt.Sprintf(
				"okf-sync: %s merged, but this hub pull request cannot merge yet: %s. compass sync tries again on its next run.", c, bl.reason))
			if err != nil {
				return "", err
			}
			done = append(done, d)
			reasons = append(reasons, bl.reason)
		}
		return fmt.Sprintf("not merged: %s: %s", strings.Join(reasons, "; "), strings.Join(done, "; ")), nil
	}
	if waiting != "" {
		return "waiting: " + waiting, nil
	}
	if pr.Mergeable == nil {
		return fmt.Sprintf("waiting: GitHub has not computed whether %s can merge", k), nil
	}
	if s.dryRun {
		return fmt.Sprintf("would merge %s at %s (approved by %s)", k, short(head), approved), nil
	}
	// The check holds for the default head it compared; a move since then
	// waits for the next run (plan Failure modes).
	var now ref
	if err := s.gh.Get(s.hub.api+"/git/ref/heads/"+github.QuotePath(s.hub.def), nil, &now); err != nil {
		return "", err
	}
	if now.Object == nil {
		return "", missingKey("object")
	}
	if now.Object.SHA != s.hub.defSHA {
		return fmt.Sprintf("not merged: %s moved from %s to %s after the check; next run retries",
			s.hub.def, short(s.hub.defSHA), short(now.Object.SHA)), nil
	}
	var body struct {
		Message string `json:"message"`
	}
	status, err := s.gh.Put(fmt.Sprintf("%s/pulls/%d/merge", s.hub.api, hpr.Number),
		map[string]string{"merge_method": "merge", "sha": head}, &body, http.StatusMethodNotAllowed, http.StatusConflict)
	if err != nil {
		var he *github.HTTPError
		// A 405/409 body that is not a JSON object still means a refusal.
		if !(errors.As(err, &he) && (status == http.StatusMethodNotAllowed || status == http.StatusConflict)) {
			return "", err
		}
		body.Message = ""
	}
	if status == http.StatusMethodNotAllowed || status == http.StatusConflict {
		return fmt.Sprintf("not merged: GitHub refused the merge of %s at %s (%d %s); next run retries", k, short(head), status, body.Message), nil
	}
	line := fmt.Sprintf("merged %s at %s (approved by %s; %s merged)", k, short(head), approved, c)
	// The merge moved the default head: the next branch in this run is checked
	// against, and merged onto, the head it made.
	return line, s.hub.refresh()
}

// check is the local consistency check on head. The path and file-mode rules
// run first, on the branch's changes alone, and when they refuse, nothing
// reads the branch's files. Then, when merge is set, it builds the merge with
// the default head, applies both rules again to what the merge changes on the
// default head (a rename on the default head can carry a branch's edit into
// another folder), and blocks only on findings the merge has that the default
// head lacks (plan decision 11). waiting is set when head is not in the
// fetched hub yet.
func (s *syncer) check(head string, merge bool) (blockers []blocker, waiting string, err error) {
	if _, code, err := s.hub.gitStatus(nil, "rev-parse", "--verify", "--quiet", head+"^{commit}"); err != nil || code != 0 {
		if err != nil {
			return nil, "", err
		}
		return nil, fmt.Sprintf("head %s is not in the fetched hub yet; next run retries", short(head)), nil
	}
	out, code, err := s.hub.gitStatus(nil, "merge-base", s.hub.defSHA, head)
	if err != nil {
		return nil, "", err
	}
	if code != 0 {
		return []blocker{{"conflict", "it shares no history with " + s.hub.def}}, "", nil
	}
	changes, err := s.hub.changes(strings.TrimSpace(string(out)), head)
	if err != nil {
		return nil, "", err
	}
	if blockers = s.rules("it", changes); len(blockers) > 0 || !merge {
		return blockers, "", nil
	}
	tree, clean, err := s.hub.mergeTree(head)
	if err != nil {
		return nil, "", err
	}
	if !clean {
		return []blocker{{"conflict", fmt.Sprintf("it conflicts with %s when merged; merge %s into it and run `okf index .`", s.hub.def, s.hub.def)}}, "", nil
	}
	if changes, err = s.hub.changes(s.hub.defSHA, tree); err != nil {
		return nil, "", err
	}
	if blockers = s.rules("its merge with "+s.hub.def, changes); len(blockers) > 0 {
		return blockers, "", nil
	}
	before, err := s.hub.defaultFindings()
	if err == nil {
		var after []hub.Finding
		if after, err = s.hub.findings(tree); err == nil {
			return s.compare(before, after), "", nil
		}
	}
	var unsafe *unsafePath
	if errors.As(err, &unsafe) {
		which := "its merge with " + s.hub.def
		if unsafe.tree == s.hub.defSHA {
			which = s.hub.def
		}
		return []blocker{{"unsafe-path", fmt.Sprintf("%s (tree %s) holds the path %s, which leaves the folder it is written to "+
			"or names .git, so `compass hub check` cannot read it", which, short(unsafe.tree), quoted(unsafe.path))}}, "", nil
	}
	return nil, "", err
}

// rules applies the path rule and the file-mode rule to changes; who names
// what made them, the branch ("it") or its merge, at the start of each
// reason.
func (s *syncer) rules(who string, changes []change) []blocker {
	var blockers []blocker
	var paths, linked []string
	for _, ch := range changes {
		if !pathAllowed(s.repo.Name, ch.path) {
			paths = append(paths, quoted(ch.path))
		}
		switch ch.mode {
		case "120000":
			linked = append(linked, "the symlink "+quoted(ch.path))
		case "160000":
			linked = append(linked, "the submodule link "+quoted(ch.path))
		}
	}
	if len(paths) > 0 {
		blockers = append(blockers, blocker{"paths", fmt.Sprintf(
			"%s changes %s, but a hub branch for %s may change only repos/%s/, repos/index.md and files outside "+
				"repos/ other than repos.txt, checks.txt, CODEOWNERS, .gitattributes, .gitmodules, .gitignore and .github/",
			who, listed(paths), s.repo.Name, s.repo.Name)})
	}
	if len(linked) > 0 {
		blockers = append(blockers, blocker{"file-mode", fmt.Sprintf(
			"%s adds %s; a hub holds no links", who, listed(linked))})
	}
	return blockers
}

// compare blocks on the findings after has that before lacks.
func (s *syncer) compare(before, after []hub.Finding) []blocker {
	had := map[hub.Finding]bool{}
	for _, f := range before {
		had[f] = true
	}
	var added []string
	for _, f := range after {
		if !had[f] {
			added = append(added, quoted(f.String()))
		}
	}
	if len(added) == 0 {
		return nil
	}
	return []blocker{{"failing", fmt.Sprintf("`compass hub check` on its merge with %s finds what %s lacks: %s",
		s.hub.def, s.hub.def, listed(added))}}
}

// Run syncs every hub branch of every repo in repos.txt and returns the exit
// code: 0 when every repo synced, 1 when any repo failed. err is a
// configuration error, exit 2. hubDir is the hub clone: its origin names the
// hub, and repos.txt comes from its default branch as fetched now (plan
// decision 7).
func Run(hubDir string, gh *github.Client, now time.Time, dryRun bool, ignore []string, log func(string)) (int, error) {
	owner, name, err := config.HubRepo(hubDir)
	if err != nil {
		return 2, err
	}
	h := &hubClone{dir: hubDir, owner: owner, name: owner + "/" + name, api: "/repos/" + owner + "/" + name}
	if err := config.Fetch(hubDir); err != nil {
		log(fmt.Sprintf("%s: error: hub %s: %v", prog, h.name, err))
		return 1, nil
	}
	if h.def, err = config.RemoteDefault(hubDir); err != nil {
		return 2, err
	}
	repos, err := config.HubRepos(hubDir, h.def)
	if err != nil {
		return 2, err
	}
	if h.defSHA, err = h.git("rev-parse", "--verify", "refs/remotes/origin/"+h.def+"^{commit}"); err != nil {
		return 2, err
	}
	ign := map[string]bool{}
	for _, i := range ignore {
		ign[strings.ToLower(i)] = true
	}
	ok := true
	perm := map[string]bool{} // hub permissions, the same for every repo
	for _, repo := range repos.List {
		s := &syncer{gh: gh, hub: h, repo: repo, now: now, dryRun: dryRun, ignore: ign, log: log,
			api: repo.API(), perm: perm}
		if err := s.run(); err != nil {
			log(fmt.Sprintf("%s: error: %s: %v", prog, repo.Name, err))
			ok = false
		}
	}
	if ok {
		return 0, nil
	}
	return 1, nil
}

// Main is `compass sync`.
func Main(args []string, stdout, stderr io.Writer) int {
	return run(args, stdout, stderr, osGetenv, time.Now, github.New)
}
