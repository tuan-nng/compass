// Package syncjob is `compass sync`: merge and clean up branch-mode knowledge
// pull requests.
//
// For every branch-mode repo in repos.txt and every remote okf/<b> branch
// (never okf/main), the job applies the state table in the research report,
// section 5, "Branch mode", with the merge conditions under "Merges by the
// sync job" in the plan's trust boundaries. It keeps no state: every run
// re-reads GitHub, and each comment carries a hidden marker so it is never
// posted twice. It never opens pull requests, never writes to code branches,
// and never deletes okf/main.
package syncjob

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"compass/internal/config"
	"compass/internal/frontmatter"
	"compass/internal/github"
)

const (
	prog         = "sync"
	trunk        = "okf/main"
	checkName    = "bundle-check"
	checkApp     = "github-actions"
	abandonAfter = 7 * 24 * time.Hour
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

type checkRun struct {
	Name       string          `json:"name"`
	Status     string          `json:"status"`
	Conclusion *string         `json:"conclusion"`
	HeadSHA    json.RawMessage `json:"head_sha"`
	App        *struct {
		Slug string `json:"slug"`
	} `json:"app"`
}

type comment struct {
	Body string `json:"body"`
}

func sameRepo(p *pull, repo *config.Repo) bool {
	name := ""
	if p.Head.Repo != nil {
		name = p.Head.Repo.FullName
	}
	return strings.EqualFold(name, repo.FullName())
}

// refPattern is the Python pattern without its look-around, matched anchored
// at each candidate position; links applies the look-around by hand. Python's
// \d is any Unicode decimal digit, \p{Nd}.
var refPattern = regexp.MustCompile(`^(?:([\p{L}\p{N}_.-]+/[\p{L}\p{N}_.-]+))?#(\p{Nd}+)`)

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

// links reports whether src's body references pull request target.
func links(src, target *pull, repo *config.Repo) bool {
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
		num := body[i+m[4] : i+m[5]]
		if refIs(num, n) &&
			(m[2] < 0 || strings.EqualFold(body[i+m[2]:i+m[3]], repo.FullName())) {
			return true
		}
		i += m[1]
	}
	return false
}

type syncer struct {
	gh     *github.Client
	repo   *config.Repo
	now    time.Time
	dryRun bool
	ignore map[string]bool
	log    func(string)
	api    string
	perm   map[string]bool
}

// --- reads ---------------------------------------------------------------

func (s *syncer) refSHA(branch string) (string, bool, error) {
	var r ref
	found, err := s.gh.GetOptional(s.api+"/git/ref/heads/"+github.QuotePath(branch), nil, &r)
	if err != nil || !found {
		return "", false, err
	}
	if r.Object == nil {
		return "", false, missingKey("object")
	}
	return r.Object.SHA, true, nil
}

type okfBranch struct{ name, sha string }

func (s *syncer) okfBranches() ([]okfBranch, error) {
	refs, err := github.Paginate[ref](s.gh, s.api+"/git/matching-refs/heads/okf/", nil, "")
	if err != nil {
		return nil, err
	}
	var out []okfBranch
	for _, r := range refs {
		name := strings.TrimPrefix(r.Ref, "refs/heads/")
		if name != trunk && strings.HasPrefix(name, "okf/") && len(name) > len("okf/") {
			if r.Object == nil {
				return nil, missingKey("object")
			}
			out = append(out, okfBranch{strings.TrimPrefix(name, "okf/"), r.Object.SHA})
		}
	}
	return out, nil
}

func (s *syncer) latestCodePR(b string) (*pull, error) {
	pulls, err := github.Paginate[*pull](s.gh, s.api+"/pulls", url.Values{
		"state": {"all"}, "head": {s.repo.Owner + ":" + b}, "sort": {"created"}, "direction": {"desc"}}, "")
	if err != nil {
		return nil, err
	}
	var best *pull
	for _, p := range pulls {
		if p.Head.Ref == b && sameRepo(p, s.repo) && !strings.HasPrefix(p.Base.Ref, "okf/") {
			if best == nil || p.Number > best.Number {
				best = p
			}
		}
	}
	return best, nil
}

func (s *syncer) openKnowledgePR(b string) (*pull, error) {
	pulls, err := github.Paginate[*pull](s.gh, s.api+"/pulls", url.Values{
		"state": {"open"}, "head": {s.repo.Owner + ":okf/" + b}, "base": {trunk}}, "")
	if err != nil {
		return nil, err
	}
	var best *pull
	for _, p := range pulls {
		if p.Head.Ref == "okf/"+b && p.Base.Ref == trunk && sameRepo(p, s.repo) {
			if best == nil || p.Number < best.Number {
				best = p
			}
		}
	}
	return best, nil
}

func (s *syncer) hasWrite(login string) (bool, error) {
	if v, ok := s.perm[login]; ok {
		return v, nil
	}
	var p struct {
		RoleName   string `json:"role_name"`
		Permission string `json:"permission"`
	}
	if _, err := s.gh.GetOptional(s.api+"/collaborators/"+github.QuotePath(login)+"/permission", nil, &p); err != nil {
		return false, err
	}
	v := writeRoles[p.RoleName] || p.Permission == "admin" || p.Permission == "write"
	s.perm[login] = v
	return v, nil
}

// approver returns a human with write access whose latest review approves head.
func (s *syncer) approver(number int, head string) (string, error) {
	reviews, err := github.Paginate[review](s.gh, fmt.Sprintf("%s/pulls/%d/reviews", s.api, number), nil, "")
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

// bundleCheck returns "success", "failure", "pending" or "missing" for
// bundle-check on head.
func (s *syncer) bundleCheck(head string) (string, error) {
	all, err := github.Paginate[checkRun](s.gh, s.api+"/commits/"+head+"/check-runs",
		url.Values{"check_name": {checkName}, "filter": {"latest"}}, "check_runs")
	if err != nil {
		return "", err
	}
	var runs []checkRun
	for _, r := range all {
		if r.Name == checkName && r.App != nil && r.App.Slug == checkApp && headIs(r.HeadSHA, head) {
			runs = append(runs, r)
		}
	}
	if len(runs) == 0 {
		return "missing", nil
	}
	done := 0
	failed := false
	for _, r := range runs {
		if r.Status == "completed" {
			done++
			if r.Conclusion == nil || *r.Conclusion != "success" {
				failed = true
			}
		}
	}
	if failed {
		return "failure", nil
	}
	if done == len(runs) {
		return "success", nil
	}
	return "pending", nil
}

// headIs is Python's r.get("head_sha", head) == head: a missing key matches,
// and a present value, null included, must equal head.
func headIs(raw json.RawMessage, head string) bool {
	if raw == nil {
		return true
	}
	var s string
	return string(raw) != "null" && json.Unmarshal(raw, &s) == nil && s == head
}

func (s *syncer) onTrunk(trunkSHA, sha string) (bool, error) {
	var cmp struct {
		Status string `json:"status"`
	}
	if err := s.gh.Get(s.api+"/compare/"+trunkSHA+"..."+sha, nil, &cmp); err != nil {
		return false, err
	}
	return cmp.Status == "identical" || cmp.Status == "behind", nil
}

func (s *syncer) lastCommitTime(sha string) (time.Time, error) {
	var c struct {
		Committer *struct {
			Date *string `json:"date"`
		} `json:"committer"`
	}
	if err := s.gh.Get(s.api+"/git/commits/"+sha, nil, &c); err != nil {
		return time.Time{}, err
	}
	if c.Committer == nil {
		return time.Time{}, missingKey("committer")
	}
	if c.Committer.Date == nil {
		return time.Time{}, missingKey("date")
	}
	return frontmatter.ParseTime(*c.Committer.Date)
}

// --- writes (each one checks dry-run first) ------------------------------

// comment posts one marked comment unless it is already there and returns a
// log fragment.
func (s *syncer) comment(number int, kind string, key int, text string) (string, error) {
	m := marker(kind, key)
	path := fmt.Sprintf("%s/issues/%d/comments", s.api, number)
	comments, err := github.Paginate[comment](s.gh, path, nil, "")
	if err != nil {
		return "", err
	}
	for _, c := range comments {
		if strings.Contains(c.Body, m) {
			return fmt.Sprintf("already commented on #%d", number), nil
		}
	}
	if s.dryRun {
		return fmt.Sprintf("would comment on #%d", number), nil
	}
	if _, err := s.gh.Post(path, map[string]string{"body": m + "\n" + text}, nil); err != nil {
		return "", err
	}
	return fmt.Sprintf("commented on #%d", number), nil
}

func (s *syncer) deleteBranch(b string) (string, error) {
	name := "okf/" + b
	if b == "" || name == trunk {
		return "", &repoFailure{"refusing to delete " + name}
	}
	if s.dryRun {
		return "would delete " + name, nil
	}
	if _, err := s.gh.Delete(s.api + "/git/refs/heads/" + github.QuotePath(name)); err != nil {
		return "", err
	}
	return "deleted " + name, nil
}

func (s *syncer) closeAndDelete(b string, kpr *pull, kind, why string) (string, error) {
	var done []string
	if kpr != nil {
		d, err := s.comment(kpr.Number, kind, kpr.Number, fmt.Sprintf("okf-sync closed this knowledge pull request: %s.", why))
		if err != nil {
			return "", err
		}
		done = append(done, d)
		if s.dryRun {
			done = append(done, fmt.Sprintf("would close #%d", kpr.Number))
		} else {
			if _, err := s.gh.Patch(fmt.Sprintf("%s/pulls/%d", s.api, kpr.Number), map[string]string{"state": "closed"}, nil); err != nil {
				return "", err
			}
			done = append(done, fmt.Sprintf("closed #%d", kpr.Number))
		}
	}
	d, err := s.deleteBranch(b)
	if err != nil {
		return "", err
	}
	done = append(done, d)
	return why + ": " + strings.Join(done, "; "), nil
}

// --- the state table -----------------------------------------------------

func (s *syncer) run() error {
	var info struct {
		DefaultBranch *string `json:"default_branch"`
	}
	if err := s.gh.Get(s.api, nil, &info); err != nil {
		return err
	}
	if info.DefaultBranch == nil {
		return missingKey("default_branch")
	}
	trunkSHA, found, err := s.refSHA(trunk)
	if err != nil {
		return err
	}
	if !found {
		return &repoFailure{trunk + " not found"}
	}
	branches, err := s.okfBranches()
	if err != nil {
		return err
	}
	if len(branches) == 0 {
		s.log(fmt.Sprintf("%s: %s: no okf/<b> branches", prog, s.repo.Name))
	}
	for _, br := range branches {
		line, err := s.branch(br.name, br.sha, *info.DefaultBranch, trunkSHA)
		if err != nil {
			return err
		}
		s.log(fmt.Sprintf("%s: %s okf/%s: %s", prog, s.repo.Name, br.name, line))
	}
	return nil
}

func (s *syncer) branch(b, sha, defaultBranch, trunkSHA string) (string, error) {
	code, err := s.latestCodePR(b)
	if err != nil {
		return "", err
	}
	kpr, err := s.openKnowledgePR(b)
	if err != nil {
		return "", err
	}
	_, bOnRemote, err := s.refSHA(b)
	if err != nil {
		return "", err
	}

	if code == nil {
		if bOnRemote {
			return fmt.Sprintf("nothing: no code pull request from %s; %s is on the remote", b, b), nil
		}
		last, err := s.lastCommitTime(sha)
		if err != nil {
			return "", err
		}
		age := s.now.Sub(last)
		if age <= abandonAfter {
			days := int(math.Floor(age.Hours() / 24))
			return fmt.Sprintf("nothing: no code pull request from %s; last commit %dd ago", b, days), nil
		}
		return s.closeAndDelete(b, kpr, "abandoned",
			fmt.Sprintf("no code pull request from %s, %s is gone and okf/%s has no commits for 7 days", b, b, b))
	}
	c := fmt.Sprintf("code #%d", code.Number)
	if code.State == "open" {
		return fmt.Sprintf("nothing: %s is open", c), nil
	}
	if code.MergedAt == nil || *code.MergedAt == "" {
		if bOnRemote {
			return fmt.Sprintf("nothing: %s closed unmerged; %s is on the remote", c, b), nil
		}
		return s.closeAndDelete(b, kpr, "closed-unmerged", fmt.Sprintf("%s closed without merging and %s is gone", c, b))
	}
	if code.Base.Ref != defaultBranch {
		return fmt.Sprintf("nothing: %s merged into %s, not %s; treated as not merged", c, code.Base.Ref, defaultBranch), nil
	}

	if kpr != nil {
		return s.tryMerge(code, kpr)
	}
	on, err := s.onTrunk(trunkSHA, sha)
	if err != nil {
		return "", err
	}
	if !on {
		done, err := s.comment(code.Number, "missing-knowledge-pr", code.Number, fmt.Sprintf(
			"okf-sync: this pull request merged, but okf/%s has commits that are not on "+
				"%s and no knowledge pull request is open. Open one from okf/%s into "+
				"%s and link it here.", b, trunk, b, trunk))
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s merged; okf/%s has commits not on %s and no knowledge pull request: %s", c, b, trunk, done), nil
	}
	if bOnRemote {
		return fmt.Sprintf("nothing: %s merged and okf/%s is on %s; %s is on the remote", c, b, trunk, b), nil
	}
	d, err := s.deleteBranch(b)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s merged, %s is gone and okf/%s is on %s: %s", c, b, b, trunk, d), nil
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

func (s *syncer) tryMerge(code, kpr *pull) (string, error) {
	c, k := fmt.Sprintf("code #%d", code.Number), fmt.Sprintf("knowledge #%d", kpr.Number)
	if !(links(code, kpr, s.repo) || links(kpr, code, s.repo)) {
		done, err := s.comment(kpr.Number, "unlinked", kpr.Number, fmt.Sprintf(
			"okf-sync: the latest code pull request from this branch, #%d, merged, "+
				"but neither pull request links the other, so this one is not merged. Link them "+
				"(write #%d in this description) if they belong together.", code.Number, code.Number))
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("not merged: %s and %s do not link each other: %s", c, k, done), nil
	}
	var pr pull
	if err := s.gh.Get(fmt.Sprintf("%s/pulls/%d", s.api, kpr.Number), nil, &pr); err != nil {
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
	head := pr.Head.SHA
	if head == "" {
		return "", missingKey("sha")
	}
	approved, err := s.approver(pr.Number, head)
	if err != nil {
		return "", err
	}
	check, err := s.bundleCheck(head)
	if err != nil {
		return "", err
	}
	mergeable := pr.Mergeable

	type blocker struct{ kind, reason string }
	var blockers []blocker
	if approved == "" {
		blockers = append(blockers, blocker{"unapproved", fmt.Sprintf("no approval of its head commit %s from a person with write access", short(head))})
	}
	if check == "failure" {
		blockers = append(blockers, blocker{"failing", fmt.Sprintf("%s failed on its head commit %s", checkName, short(head))})
	}
	if mergeable != nil && !*mergeable {
		blockers = append(blockers, blocker{"conflict", "it conflicts with " + trunk})
	}
	if len(blockers) > 0 {
		var done, reasons []string
		for _, bl := range blockers {
			text := fmt.Sprintf("okf-sync: %s merged, but %s cannot merge yet: %s. The sync job tries again on its next run.", c, k, bl.reason)
			for _, n := range []int{kpr.Number, code.Number} {
				d, err := s.comment(n, bl.kind, kpr.Number, text)
				if err != nil {
					return "", err
				}
				done = append(done, d)
			}
			reasons = append(reasons, bl.reason)
		}
		return fmt.Sprintf("not merged: %s: %s", strings.Join(reasons, "; "), strings.Join(done, "; ")), nil
	}
	if check != "success" {
		return fmt.Sprintf("waiting: %s on %s is %s", checkName, short(head), check), nil
	}
	if mergeable == nil {
		return fmt.Sprintf("waiting: GitHub has not computed whether %s can merge", k), nil
	}
	if s.dryRun {
		return fmt.Sprintf("would merge %s at %s (approved by %s)", k, short(head), approved), nil
	}
	var body struct {
		Message string `json:"message"`
	}
	status, err := s.gh.Put(fmt.Sprintf("%s/pulls/%d/merge", s.api, kpr.Number),
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
	return fmt.Sprintf("merged %s at %s (approved by %s; %s merged)", k, short(head), approved, c), nil
}

// Run syncs every branch-mode repo in hub's repos.txt and returns the exit
// code: 0 when every repo synced, 1 when any repo failed.
func Run(hub string, gh *github.Client, org string, now time.Time, dryRun bool, ignore []string, log func(string)) (int, error) {
	repos, err := config.ParseRepos(filepath.Join(hub, "repos.txt"), org)
	if err != nil {
		return 2, err
	}
	ign := map[string]bool{}
	for _, i := range ignore {
		ign[strings.ToLower(i)] = true
	}
	ok := true
	for _, repo := range repos.List {
		if repo.Mode != "branch" {
			continue
		}
		s := &syncer{gh: gh, repo: repo, now: now, dryRun: dryRun, ignore: ign, log: log, api: repo.API(), perm: map[string]bool{}}
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
