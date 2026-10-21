package syncjob

// compass sync against recorded GitHub API responses (testdata/fixtures/)
// and a real local hub (ghfake.TemplateHub) holding the branches those
// responses name, so the local consistency check runs on real files.
//
// One test per row of the state table in plan Interfaces, one refusal per
// merge condition in plan Trust boundaries, and phase 03's other cases. Every
// world marks the code repo read-only, so any write under it fails the test
// (plan decision 6).

import (
	"bytes"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"compass/internal/github"
	"compass/internal/github/ghfake"
	"compass/internal/okftest"
)

const (
	codePath   = "/repos/acme/web-app"
	hubPath    = ghfake.HubPath
	reposRow   = "web-app https://github.com/acme/web-app.git"
	defaultNow = "2026-10-06T00:30:00Z"
	brName     = "feat/retry"
	hubBr      = "web-app/" + brName
)

var (
	codeQ = map[string]string{"state": "all", "head": "acme:" + brName}
	hubQ  = map[string]string{"state": "all", "head": "acme:" + hubBr}
)

type world struct {
	t    *testing.T
	f    *ghfake.Fake
	h    *ghfake.Hub
	okf  string
	logs []string
	code map[string]any // acme/web-app#41 feat/retry -> main, merged, links acme/hub#42
	hpr  map[string]any // acme/hub#42 web-app/feat/retry -> main, open
	kh   string         // head of the hub branch
	dh   string         // head of the hub's main
}

func fx(t *testing.T, name string) any { return ghfake.Fixture(t, "testdata", name) }

func obj(v any) map[string]any { return v.(map[string]any) }

func with(o any, changes map[string]any) map[string]any { return ghfake.With(o, changes) }

// concept is a concept file the strict validator passes.
func concept(typ, title, body string) string {
	return "---\ntype: " + typ + "\ntitle: " + title + "\ndescription: " + title + ".\ntags: [test]\n" +
		"status: stable\ngenerated: { by: process:test, at: 2026-10-01T00:00:00Z }\n---\n# " + title + "\n\n" + body + "\n"
}

func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, text := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// index runs `okf index .` at the root of dir, as an agent does before it
// commits.
func (w *world) index(dir string) {
	w.t.Helper()
	cmd := exec.Command(w.okf, "index", ".")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		w.t.Fatalf("okf index: %v\n%s", err, out)
	}
}

// edit returns a change that writes files, removes paths, and re-indexes.
func (w *world) edit(files map[string]string, remove ...string) func(string) {
	return func(dir string) {
		writeFiles(w.t, dir, files)
		for _, p := range remove {
			if err := os.Remove(filepath.Join(dir, filepath.FromSlash(p))); err != nil {
				w.t.Fatal(err)
			}
		}
		w.index(dir)
	}
}

// baseContent is the default branch every world starts from beyond the
// template: a repo folder for web-app and two hub-level decisions.
var baseContent = map[string]string{
	"repos/web-app/overview.md":  concept("Overview", "Web app", "The web app."),
	"repos/web-app/gotchas/a.md": concept("Gotcha", "Timeouts", "Calls time out after 5 s."),
	"decisions/retry.md":         concept("Decision", "Retry", "Retry with a key."),
	"decisions/keep.md":          concept("Decision", "Keep", "Keep this one."),
}

// branchContent is what the hub branch adds: a concept that links a
// hub-level decision.
var branchContent = map[string]string{
	"repos/web-app/gotchas/idempotency-key.md": concept("Gotcha", "Idempotency key",
		"Retries reuse the key; see [the retry decision](/decisions/retry.md)."),
}

func newWorld(t *testing.T) *world {
	w := newBareWorld(t)
	w.moveMain(w.edit(baseContent))
	w.branch(w.edit(branchContent))
	return w
}

// newBareWorld is a world whose hub's main is the template alone, with no
// hub branch yet.
func newBareWorld(t *testing.T) *world {
	w := &world{t: t, f: ghfake.New(t), okf: okftest.Binary(t)}
	w.h = ghfake.TemplateHub(t, reposRow, "")
	w.dh = w.h.Head("main")
	w.code = obj(fx(t, "pull-code.json"))
	w.hpr = obj(fx(t, "pull-hub.json"))
	f := w.f
	f.ReadOnly(codePath)
	f.Add("GET", codePath, fx(t, "repo.json"))
	w.codeBranch(false)
	w.codePRs(w.code)
	f.Add("GET", hubPath+"/collaborators/carol/permission", fx(t, "permission-write.json"))
	f.Add("GET", hubPath+"/collaborators/dave/permission", fx(t, "permission-read.json"))
	f.Add("GET", hubPath+"/issues/42/comments", []any{})
	f.Add("POST", hubPath+"/issues/42/comments", fx(t, "comment-created.json"), ghfake.Status(201))
	f.Add("PUT", hubPath+"/pulls/42/merge", fx(t, "merge-ok.json"))
	f.Add("PATCH", hubPath+"/pulls/42", fx(t, "pull-closed.json"))
	f.Add("DELETE", hubPath+"/git/refs/heads/"+hubBr, nil, ghfake.Status(204))
	w.mainRef(w.dh)
	return w
}

// --- world setters -------------------------------------------------------

// moveMain commits change onto the hub's main and routes the re-read of
// main to the new head.
func (w *world) moveMain(change func(string)) {
	w.dh = w.h.Commit("main", "", change)
	w.mainRef(w.dh)
}

func (w *world) mainRef(sha string) {
	ref := obj(fx(w.t, "ref.json"))
	w.f.Add("GET", hubPath+"/git/ref/heads/main", with(ref, map[string]any{
		"object": with(ref["object"], map[string]any{"sha": sha})}))
}

// branch makes the hub branch one commit on main holding change, and routes
// the open hub pull request, its detail and an approval of carol's at that
// head.
func (w *world) branch(change func(string)) {
	w.kh = w.h.Commit(hubBr, "main", change)
	w.hpr = with(w.hpr, map[string]any{"head": with(w.hpr["head"], map[string]any{"sha": w.kh})})
	w.hubPRs(w.hpr)
	w.detail(nil)
	revs := fx(w.t, "reviews.json").([]any)
	w.reviews(revs[0], with(revs[1], map[string]any{"commit_id": w.kh}))
}

func (w *world) codeBranch(onRemote bool) {
	if onRemote {
		w.f.Add("GET", codePath+"/git/ref/heads/"+brName, with(fx(w.t, "ref.json"), map[string]any{
			"ref": "refs/heads/" + brName, "url": "https://api.github.com" + codePath + "/git/refs/heads/" + brName}))
	} else {
		w.f.Add("GET", codePath+"/git/ref/heads/"+brName, fx(w.t, "not-found.json"), ghfake.Status(404))
	}
}

func (w *world) codePRs(prs ...any) {
	w.f.Add("GET", codePath+"/pulls", append([]any{}, prs...), ghfake.Query(codeQ))
}

func (w *world) hubPRs(prs ...any) {
	w.f.Add("GET", hubPath+"/pulls", append([]any{}, prs...), ghfake.Query(hubQ))
}

func (w *world) detail(changes map[string]any) {
	d := with(fx(w.t, "pull-hub-detail.json"), map[string]any{
		"head": with(obj(fx(w.t, "pull-hub-detail.json"))["head"], map[string]any{"sha": w.kh})})
	w.f.Add("GET", hubPath+"/pulls/42", with(d, changes))
}

func (w *world) reviews(reviews ...any) {
	w.f.Add("GET", hubPath+"/pulls/42/reviews", append([]any{}, reviews...))
}

// closedHubPR is the hub pull request closed, merged at mergedAt or, when
// mergedAt is nil, unmerged.
func (w *world) closedHubPR(mergedAt any) map[string]any {
	return with(w.hpr, map[string]any{"state": "closed", "closed_at": "2026-10-05T16:00:00Z", "merged_at": mergedAt})
}

func (w *world) reset() {
	w.f.Calls = nil
	w.logs = nil
}

// --- running -------------------------------------------------------------

func (w *world) runSyncAt(now string, extra ...string) int {
	args := append([]string{"--hub", w.h.Dir, "--api-url", ghfake.API, "--now", now}, extra...)
	var out, errb bytes.Buffer
	newClient := func(token, base string, dry bool) *github.Client {
		c := github.New(token, base, dry)
		c.HTTP = &http.Client{Transport: w.f}
		return c
	}
	clock := func() time.Time { w.t.Fatal("clock read despite --now"); return time.Time{} }
	code := run(args, &out, &errb, ghfake.Getenv, clock, newClient)
	if s := strings.TrimRight(out.String(), "\n"); s != "" {
		w.logs = append(w.logs, strings.Split(s, "\n")...)
	}
	if errb.Len() > 0 {
		w.t.Errorf("stderr: %s", errb.String())
	}
	return code
}

func (w *world) runSync(extra ...string) int { return w.runSyncAt(defaultNow, extra...) }

type mp struct{ method, path string }

func (w *world) writes() []mp {
	out := []mp{}
	for _, c := range w.f.Writes() {
		out = append(out, mp{c.Method, c.Path})
	}
	return out
}

func (w *world) line() string {
	w.t.Helper()
	var lines []string
	for _, x := range w.logs {
		if strings.HasPrefix(x, "sync: web-app/") {
			lines = append(lines, x)
		}
	}
	if len(lines) != 1 { // one log line per hub branch
		w.t.Fatalf("want one hub branch log line, got %q", w.logs)
	}
	if !strings.HasPrefix(lines[0], "sync: "+hubBr+": ") {
		w.t.Fatalf("bad prefix: %s", lines[0])
	}
	return lines[0]
}

func (w *world) commentBody() string {
	w.t.Helper()
	for _, c := range w.f.Writes() {
		if c.Method == "POST" && c.Path == hubPath+"/issues/42/comments" {
			return obj(c.Body)["body"].(string)
		}
	}
	w.t.Fatalf("no comment posted; writes %v", w.writes())
	return ""
}

func (w *world) expectCode(want, got int) {
	w.t.Helper()
	if got != want {
		w.t.Fatalf("exit %d, want %d; logs %q", got, want, w.logs)
	}
}

func (w *world) expectWrites(want ...mp) {
	w.t.Helper()
	if want == nil {
		want = []mp{}
	}
	if got := w.writes(); !reflect.DeepEqual(got, want) {
		w.t.Fatalf("writes %v, want %v; logs %q", got, want, w.logs)
	}
}

func (w *world) expectLine(sub string) {
	w.t.Helper()
	if l := w.line(); !strings.Contains(l, sub) {
		w.t.Fatalf("log line %q lacks %q", l, sub)
	}
}

func (w *world) assertNothing(why string) {
	w.t.Helper()
	w.expectCode(0, w.runSync())
	w.expectWrites()
	w.expectLine("nothing")
	w.expectLine(why)
}

var (
	mergeCall   = mp{"PUT", hubPath + "/pulls/42/merge"}
	commentCall = mp{"POST", hubPath + "/issues/42/comments"}
	closeCall   = mp{"PATCH", hubPath + "/pulls/42"}
	deleteCall  = mp{"DELETE", hubPath + "/git/refs/heads/" + hubBr}
)

// assertBlocked expects one comment on the hub pull request, of kind and
// naming reason, and no merge.
func (w *world) assertBlocked(kind, reason string) string {
	w.t.Helper()
	w.expectCode(0, w.runSync())
	w.expectWrites(commentCall)
	body := w.commentBody()
	if !strings.HasPrefix(body, "<!-- okf-sync:"+kind+":42 -->\n") {
		w.t.Errorf("body %q, want kind %s", body, kind)
	}
	if !strings.Contains(body, reason) {
		w.t.Errorf("body %q lacks %q", body, reason)
	}
	w.expectLine("not merged")
	return body
}

// noCheck makes any run of `compass hub check` fail the run, so a test
// proves the check never ran.
func (w *world) noCheck() { w.t.Setenv("OKF", filepath.Join(w.t.TempDir(), "no-okf")) }

// --- state table rows ----------------------------------------------------

// Row 1: a hub pull request merged by any method, a squash here (the branch
// is not on main), finishes the branch once <b> is gone.
func TestRowHubPrMergedBySquashDeletesBranchOnceCodeBranchGone(t *testing.T) {
	t.Run("gone", func(t *testing.T) {
		w := newWorld(t)
		w.hubPRs(w.closedHubPR("2026-10-05T16:00:00Z"))
		w.codePRs(with(w.code, map[string]any{"state": "open", "merged_at": nil})) // any code state
		w.expectCode(0, w.runSync())
		w.expectWrites(deleteCall)
		w.expectLine("hub #42 merged, feat/retry is gone: deleted " + hubBr)
	})
	t.Run("on the remote", func(t *testing.T) {
		w := newWorld(t)
		w.hubPRs(w.closedHubPR("2026-10-05T16:00:00Z"))
		w.codeBranch(true)
		w.assertNothing("hub #42 merged; feat/retry is on the remote")
	})
}

func TestRowNoCodePrWhileBranchOnRemoteDoesNothing(t *testing.T) {
	w := newWorld(t)
	w.codeBranch(true)
	w.codePRs()
	w.assertNothing("no pull request from feat/retry in acme/web-app")
}

func TestRowNoCodePrBranchGoneWithin7DaysWaits(t *testing.T) {
	w := newWorld(t)
	w.codePRs()
	// The hub branch's last commit is ghfake.HubDate, 2026-10-04T08:59:00Z.
	w.expectCode(0, w.runSyncAt("2026-10-11T08:58:00Z"))
	w.expectWrites()
	w.expectLine("last commit 6d ago")
}

func TestRowNoCodePrBranchGoneAfter7DaysClosesAndDeletes(t *testing.T) {
	w := newWorld(t)
	w.codePRs()
	w.expectCode(0, w.runSyncAt("2026-10-11T09:00:00Z"))
	w.expectWrites(commentCall, closeCall, deleteCall)
	if body := w.commentBody(); !strings.HasPrefix(body, "<!-- okf-sync:abandoned:42 -->\n") {
		t.Fatalf("body %q", body)
	}
	if got := w.f.Writes()[1].Body; !reflect.DeepEqual(got, map[string]any{"state": "closed"}) {
		t.Fatalf("patch body %v", got)
	}
}

func TestRowNoCodePrNoHubPrAfter7DaysDeletesBranch(t *testing.T) {
	w := newWorld(t)
	w.codePRs()
	w.hubPRs()
	w.expectCode(0, w.runSyncAt("2026-10-11T09:00:00Z"))
	w.expectWrites(deleteCall)
}

// An open code pull request costs at most 5 requests (plan Quality targets).
func TestRowCodePrOpenDoesNothingWithinFiveRequests(t *testing.T) {
	w := newWorld(t)
	w.codeBranch(true)
	w.codePRs(with(w.code, map[string]any{"state": "open", "merged_at": nil, "closed_at": nil}))
	w.assertNothing("acme/web-app#41 is open")
	if len(w.f.Calls) > 5 {
		t.Fatalf("%d requests, want at most 5: %v", len(w.f.Calls), w.f.Calls)
	}
}

func TestRowCodePrClosedUnmergedWhileBranchOnRemoteDoesNothing(t *testing.T) {
	w := newWorld(t)
	w.codeBranch(true)
	w.codePRs(with(w.code, map[string]any{"merged_at": nil}))
	w.assertNothing("closed unmerged; feat/retry is on the remote")
}

func TestRowCodePrClosedUnmergedBranchGoneClosesAndDeletes(t *testing.T) {
	w := newWorld(t)
	w.codePRs(with(w.code, map[string]any{"merged_at": nil}))
	w.expectCode(0, w.runSync())
	w.expectWrites(commentCall, closeCall, deleteCall)
	if body := w.commentBody(); !strings.Contains(body, "<!-- okf-sync:closed-unmerged:42 -->") {
		t.Fatalf("body %q", body)
	}
}

func TestRowCodePrMergedIntoAnotherBranchDoesNothing(t *testing.T) {
	w := newWorld(t)
	w.codePRs(with(w.code, map[string]any{"base": with(w.code["base"], map[string]any{"ref": "release/1.x"})}))
	w.assertNothing("treated as not merged")
}

// The merge passes the head SHA the check ran on.
func TestRowMergedApprovedCheckedHubPrIsMergedAtCheckedHead(t *testing.T) {
	w := newWorld(t)
	w.expectCode(0, w.runSync())
	w.expectWrites(mergeCall)
	if got, want := w.f.Writes()[0].Body, map[string]any{"merge_method": "merge", "sha": w.kh}; !reflect.DeepEqual(got, want) {
		t.Fatalf("merge body %v, want %v", got, want)
	}
	w.expectLine("merged hub #42 at " + w.kh[:7])
	w.expectLine("approved by carol; acme/web-app#41 merged")
}

func TestRowHubPrClosedUnmergedReportsRejectedAndDeletesOnceGone(t *testing.T) {
	t.Run("gone", func(t *testing.T) {
		w := newWorld(t)
		w.hubPRs(w.closedHubPR(nil))
		w.expectCode(0, w.runSync())
		w.expectWrites(deleteCall)
		w.expectLine("hub pull request rejected: hub #42 closed unmerged, feat/retry is gone: deleted " + hubBr)
	})
	t.Run("on the remote", func(t *testing.T) {
		w := newWorld(t)
		w.hubPRs(w.closedHubPR(nil))
		w.codeBranch(true)
		w.expectCode(0, w.runSync())
		w.expectWrites()
		w.expectLine("hub pull request rejected: hub #42 closed unmerged; feat/retry is on the remote")
	})
}

// What used to be a comment on the code pull request is a line of output.
func TestRowNoHubPrBranchAheadReportsNeedsHubPr(t *testing.T) {
	w := newWorld(t)
	w.hubPRs()
	w.expectCode(0, w.runSync())
	w.expectWrites()
	w.expectLine("needs a hub pull request")
}

func TestRowNoHubPrNothingLeftToMerge(t *testing.T) {
	t.Run("on the remote", func(t *testing.T) {
		w := newWorld(t)
		w.hubPRs()
		w.h.Git(w.h.Remote, "update-ref", "refs/heads/main", w.kh)
		w.codeBranch(true)
		w.assertNothing(hubBr + " is on main; feat/retry is on the remote")
	})
	t.Run("gone", func(t *testing.T) {
		w := newWorld(t)
		w.hubPRs()
		w.h.Git(w.h.Remote, "update-ref", "refs/heads/main", w.kh)
		w.expectCode(0, w.runSync())
		w.expectWrites(deleteCall)
		w.expectLine("deleted " + hubBr)
	})
}

// --- merge conditions: who and what --------------------------------------

func TestHubPrFromForkNeverCounts(t *testing.T) {
	w := newWorld(t)
	fork := map[string]any{"id": 1, "name": "hub", "full_name": "mallory/hub"}
	w.hubPRs(with(w.hpr, map[string]any{"head": with(w.hpr["head"], map[string]any{"repo": fork})}))
	w.expectCode(0, w.runSync())
	w.expectWrites()
	w.expectLine("needs a hub pull request")
}

func TestHubPrIntoAnotherBaseNeverCounts(t *testing.T) {
	w := newWorld(t)
	w.hubPRs(with(w.hpr, map[string]any{"base": with(w.hpr["base"], map[string]any{"ref": "release"})}))
	w.expectCode(0, w.runSync())
	w.expectWrites()
	w.expectLine("needs a hub pull request")
}

// A hub branch whose id repos.txt does not list is never scanned: no request
// names it, and the listed repo's branch still merges.
func TestUnlistedIdIsNeverScanned(t *testing.T) {
	w := newWorld(t)
	w.h.Commit("ghost/feat", "main", w.edit(map[string]string{
		"repos/ghost/overview.md": concept("Overview", "Ghost", "Not listed.")}))
	w.expectCode(0, w.runSync())
	w.expectWrites(mergeCall)
	for _, c := range w.f.Calls {
		if strings.Contains(c.Path, "ghost") || strings.Contains(c.Query["head"], "ghost") {
			t.Fatalf("unlisted id requested: %v", c)
		}
	}
	if strings.Contains(strings.Join(w.logs, "\n"), "ghost") {
		t.Fatalf("unlisted id synced: %q", w.logs)
	}
}

// --- merge conditions: links (plan decision 5) ---------------------------

func TestUnlinkedPairNeverMerges(t *testing.T) {
	w := newWorld(t)
	w.codePRs(with(w.code, map[string]any{"body": "Old change from last year.", "created_at": "2025-01-02T00:00:00Z"}))
	w.hubPRs(with(w.hpr, map[string]any{"body": "Knowledge."}))
	body := w.assertBlocked("unlinked", "write acme/web-app#41 in this description")
	if !strings.Contains(body, "neither pull request links the other") {
		t.Errorf("body %q", body)
	}
	w.expectLine("do not link each other")
}

// A short #N names a pull request in the same repo, so it never links across
// the code repo and the hub.
func TestShortReferenceIsNotALink(t *testing.T) {
	w := newWorld(t)
	w.codePRs(with(w.code, map[string]any{"body": "Knowledge: #42"}))
	w.hubPRs(with(w.hpr, map[string]any{"body": "Knowledge for #41."}))
	w.assertBlocked("unlinked", "acme/web-app#41")
}

func TestLinkFromHubPrByUrlCounts(t *testing.T) {
	w := newWorld(t)
	w.codePRs(with(w.code, map[string]any{"body": "No links here; #42 and acme/other#42 are not it."}))
	w.hubPRs(with(w.hpr, map[string]any{"body": "Pairs with https://github.com/acme/web-app/pull/41."}))
	w.expectCode(0, w.runSync())
	w.expectWrites(mergeCall)
}

func TestLinkFromCodePrAloneCounts(t *testing.T) {
	w := newWorld(t)
	w.hubPRs(with(w.hpr, map[string]any{"body": "Knowledge."}))
	w.expectCode(0, w.runSync())
	w.expectWrites(mergeCall)
}

// --- merge conditions: approval (plan decision 2) ------------------------

func TestStaleApprovalDoesNotCount(t *testing.T) {
	w := newWorld(t)
	w.reviews(with(fx(t, "reviews.json").([]any)[1], map[string]any{"commit_id": strings.Repeat("6", 40)}))
	w.assertBlocked("unapproved", "no approval of its head commit "+w.kh[:7])
}

func TestApprovalFromReaderDoesNotCount(t *testing.T) {
	w := newWorld(t)
	w.reviews(with(fx(t, "reviews.json").([]any)[1], map[string]any{
		"commit_id": w.kh, "user": map[string]any{"login": "dave", "id": 7, "type": "User"}}))
	w.assertBlocked("unapproved", "from a person with write access to the hub")
	for _, c := range w.f.Calls {
		if strings.Contains(c.Path, "/collaborators/") && !strings.HasPrefix(c.Path, hubPath+"/") {
			t.Fatalf("permission read outside the hub: %v", c)
		}
	}
}

func TestAppApprovalsAndIgnoredLoginsDoNotCount(t *testing.T) {
	w := newWorld(t)
	bot := func(login string) map[string]any { return map[string]any{"login": login, "id": 9, "type": "Bot"} }
	base := with(fx(t, "reviews.json").([]any)[1], map[string]any{"commit_id": w.kh})
	w.reviews(
		with(base, map[string]any{"user": bot("okf-writer[bot]")}),
		with(base, map[string]any{"user": bot("okf-reader[bot]")}),
		with(base, map[string]any{"user": map[string]any{"login": "ci-robot", "id": 8, "type": "User"}}),
	)
	w.expectCode(0, w.runSync("--ignore-login", "CI-Robot"))
	w.expectWrites(commentCall)
	w.expectLine("no approval of its head commit")
	for _, c := range w.f.Calls {
		if strings.Contains(c.Path, "/collaborators/ci-robot") {
			t.Fatalf("asked permission of an ignored login: %v", c)
		}
	}
}

// --- merge conditions: conflicts -----------------------------------------

func TestGitHubConflictBlocks(t *testing.T) {
	w := newWorld(t)
	w.detail(map[string]any{"mergeable": false, "mergeable_state": "dirty"})
	w.assertBlocked("conflict", "GitHub reports that it conflicts with main")
}

// main added another gotcha after the branch started, so both rewrote
// repos/web-app/gotchas/index.md.
func TestLocalMergeConflictBlocks(t *testing.T) {
	w := newWorld(t)
	w.moveMain(w.edit(map[string]string{"repos/web-app/gotchas/b.md": concept("Gotcha", "Pagination", "Pages hold 50.")}))
	w.assertBlocked("conflict", "it conflicts with main when merged")
}

// --- merge conditions: the path rule -------------------------------------

func TestPathRuleRefusals(t *testing.T) {
	other := map[string]string{"repos/billing-api/x.md": concept("Gotcha", "Other", "Another repo's.")}
	cases := []struct {
		name, path string
		main       map[string]string // committed on main before the branch starts
		change     func(w *world) func(string)
	}{
		{"change in another repo's folder", "repos/billing-api/x.md", nil,
			func(w *world) func(string) { return w.edit(other) }},
		{"rename out of another repo's folder", "repos/billing-api/x.md", other,
			func(w *world) func(string) {
				return func(dir string) { w.h.Git(dir, "mv", "repos/billing-api/x.md", "repos/web-app/x.md") }
			}},
		{"deletion in another repo's folder", "repos/billing-api/x.md", other,
			func(w *world) func(string) { return w.edit(nil, "repos/billing-api/x.md") }},
		{"sibling id web-app-v2", "repos/web-app-v2/x.md", nil,
			func(w *world) func(string) {
				return w.edit(map[string]string{"repos/web-app-v2/x.md": concept("Gotcha", "Sibling", "Not web-app.")})
			}},
		{"checks.txt", "checks.txt", nil,
			func(w *world) func(string) {
				return func(dir string) {
					writeFiles(w.t, dir, map[string]string{"checks.txt": "web-app ci.yml test process:ci repos/web-app/overview\n"})
				}
			}},
		{".gitattributes in the repo folder", "repos/web-app/.gitattributes", nil,
			func(w *world) func(string) {
				return func(dir string) {
					writeFiles(w.t, dir, map[string]string{"repos/web-app/.gitattributes": "*.md filter=evil\n"})
				}
			}},
		{"repos spelled Repos", "Repos/billing-api/x.md", nil,
			func(w *world) func(string) {
				return func(dir string) {
					writeFiles(w.t, dir, map[string]string{"Repos/billing-api/x.md": concept("Gotcha", "Folded", "Repos is repos on a Mac.")})
				}
			}},
		{"checks.txt spelled Checks.TXT", "Checks.TXT", nil,
			func(w *world) func(string) {
				return func(dir string) {
					writeFiles(w.t, dir, map[string]string{"Checks.TXT": "web-app ci.yml test process:ci repos/web-app/overview\n"})
				}
			}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t)
			if tc.main != nil {
				w.moveMain(func(dir string) { writeFiles(t, dir, tc.main) })
			}
			w.branch(tc.change(w))
			w.noCheck()
			body := w.assertBlocked("paths", "`"+tc.path+"`")
			if !strings.Contains(body, "may change only repos/web-app/") {
				t.Errorf("body %q", body)
			}
		})
	}
}

// main renamed repos/web-app/z.md into billing-api's folder after the branch
// edited it: the branch's own changes stay in repos/web-app/, but its merge
// carries the edit into repos/billing-api/z.md, so the rules run on the
// merge too.
func TestRenameOnMainCarryingBranchEditIntoAnotherFolderIsRefused(t *testing.T) {
	w := newWorld(t)
	w.moveMain(w.edit(map[string]string{"repos/web-app/z.md": concept("Gotcha", "Zones", "Every time is in UTC.")}))
	w.branch(func(dir string) {
		writeFiles(t, dir, map[string]string{"repos/web-app/z.md": concept("Gotcha", "Zones", "Every time is in UTC, stored and shown.")})
	})
	branchOnly := w.h.Git(w.h.Remote, "diff", "--name-only", "main", hubBr)
	w.moveMain(func(dir string) {
		if err := os.MkdirAll(filepath.Join(dir, "repos", "billing-api"), 0o755); err != nil {
			t.Fatal(err)
		}
		w.h.Git(dir, "mv", "repos/web-app/z.md", "repos/billing-api/z.md")
	})
	if branchOnly != "repos/web-app/z.md" {
		t.Fatalf("the branch changes %q, want only repos/web-app/z.md", branchOnly)
	}
	w.noCheck()
	w.assertBlocked("paths", "its merge with main changes `repos/billing-api/z.md`, but a hub branch for web-app may change only repos/web-app/")
}

// --- merge conditions: the file-mode rule --------------------------------

// The link points at a file outside the hub; nothing reads it, so the
// comment quotes none of it.
func TestSymlinkIsRefusedWithoutReadingIt(t *testing.T) {
	w := newWorld(t)
	secret := filepath.Join(t.TempDir(), "secret.md")
	writeFiles(t, filepath.Dir(secret), map[string]string{"secret.md": "---\ntitle: s3cr3t\n---\ns3cr3t\n"})
	w.branch(func(dir string) {
		if err := os.Symlink(secret, filepath.Join(dir, "repos", "web-app", "leak.md")); err != nil {
			t.Fatal(err)
		}
	})
	w.noCheck()
	body := w.assertBlocked("file-mode", "it adds the symlink `repos/web-app/leak.md`; a hub holds no links")
	for _, leak := range []string{"s3cr3t", secret} {
		if strings.Contains(body, leak) || strings.Contains(strings.Join(w.logs, "\n"), leak) {
			t.Errorf("comment or log quotes %q:\n%s\n%q", leak, body, w.logs)
		}
	}
}

func TestSubmoduleLinkIsRefused(t *testing.T) {
	w := newWorld(t)
	w.branch(func(dir string) {
		if err := os.MkdirAll(filepath.Join(dir, "repos", "web-app", "vendor"), 0o755); err != nil {
			t.Fatal(err)
		}
		w.h.Git(dir, "update-index", "--add", "--cacheinfo", "160000,"+w.dh+",repos/web-app/vendor")
	})
	w.noCheck()
	w.assertBlocked("file-mode", "it adds the submodule link `repos/web-app/vendor`")
}

// --- merge conditions: the check (plan decision 11) ----------------------

// The branch links decisions/retry.md, which main deleted after the branch
// started: the branch alone passes, the merge does not.
func TestCheckFailsOnMergeResultOnly(t *testing.T) {
	w := newWorld(t)
	w.moveMain(w.edit(nil, "decisions/retry.md"))
	body := w.assertBlocked("failing", "repos/web-app/gotchas/idempotency-key.md: cross-link target not found")
	if !strings.Contains(body, "`compass hub check` on its merge with main finds what main lacks") {
		t.Errorf("body %q", body)
	}
	h := &hubClone{dir: w.h.Dir}
	if found, err := h.findings(w.kh); err != nil || len(found) != 0 {
		t.Fatalf("the branch alone: findings %v, err %v", found, err)
	}
}

// main already carries a broken link; a branch that adds none still merges.
func TestCleanBranchMergesDespiteFindingOnMain(t *testing.T) {
	w := newWorld(t)
	w.moveMain(w.edit(map[string]string{
		"decisions/broken.md": concept("Decision", "Broken", "See [gone](/decisions/missing.md).")}))
	h := &hubClone{dir: w.h.Dir}
	w.h.Git(w.h.Dir, "fetch", "-q", "origin")
	if found, err := h.findings(w.dh); err != nil || len(found) == 0 {
		t.Fatalf("main: findings %v, err %v; want a finding", found, err)
	}
	w.expectCode(0, w.runSync())
	w.expectWrites(mergeCall)
}

// The first branch for a new id changes repos/<id>/, repos/index.md and the
// root index.md, and merges (plan Acceptance 1).
func TestFirstBranchForNewIdMerges(t *testing.T) {
	w := newBareWorld(t)
	w.branch(w.edit(map[string]string{"repos/web-app/overview.md": concept("Overview", "Web app", "The web app.")}))
	changed := w.h.Git(w.h.Remote, "diff", "--name-only", "main", hubBr)
	if want := "index.md\nrepos/index.md\nrepos/web-app/index.md\nrepos/web-app/overview.md"; changed != want {
		t.Fatalf("branch changes\n%s\nwant\n%s", changed, want)
	}
	w.expectCode(0, w.runSync())
	w.expectWrites(mergeCall)
}

// --- merge conditions: timing --------------------------------------------

func TestDefaultHeadMovedAfterCheckSkipsMerge(t *testing.T) {
	w := newWorld(t)
	w.mainRef(strings.Repeat("e", 40))
	w.expectCode(0, w.runSync())
	w.expectWrites()
	w.expectLine("not merged: main moved from " + w.dh[:7] + " to eeeeeee after the check; next run retries")
}

func TestMergeRefusedByGitHubRetriesNextRun(t *testing.T) {
	w := newWorld(t)
	w.f.Add("PUT", hubPath+"/pulls/42/merge", fx(t, "merge-head-moved.json"), ghfake.Status(409))
	w.expectCode(0, w.runSync())
	w.expectWrites(mergeCall)
	if got := obj(w.f.Writes()[0].Body)["sha"]; got != w.kh {
		t.Fatalf("merge sha %v, want %s", got, w.kh)
	}
	w.expectLine("GitHub refused the merge")
}

// otherBranch makes a second hub branch, web-app/feat/other, one commit on
// main holding change. It pairs with acme/web-app#51, merged, as hub #43,
// open and approved by carol at its head; GitHub's merge of #43 really merges
// it into main and moves the ref sync re-reads. It returns the branch head.
func (w *world) otherBranch(change func(string)) string {
	t := w.t
	const hb = "web-app/feat/other"
	head := w.h.Commit(hb, "main", change)
	code := with(w.code, map[string]any{"number": 51, "html_url": "https://github.com/acme/web-app/pull/51",
		"body": "Knowledge: acme/hub#43\n",
		"head": with(w.code["head"], map[string]any{"ref": "feat/other", "label": "acme:feat/other"})})
	w.f.Add("GET", codePath+"/pulls", []any{code}, ghfake.Query(map[string]string{"state": "all", "head": "acme:feat/other"}))
	w.f.Add("GET", codePath+"/git/ref/heads/feat/other", fx(t, "not-found.json"), ghfake.Status(404))
	pr := map[string]any{"number": 43, "html_url": "https://github.com/acme/hub/pull/43", "body": "Knowledge for acme/web-app#51.\n"}
	headOf := func(p map[string]any) map[string]any {
		return with(p["head"], map[string]any{"ref": hb, "label": "acme:" + hb, "sha": head})
	}
	hpr := with(w.hpr, pr)
	hpr["head"] = headOf(hpr)
	w.f.Add("GET", hubPath+"/pulls", []any{hpr}, ghfake.Query(map[string]string{"state": "all", "head": "acme:" + hb}))
	d := with(fx(t, "pull-hub-detail.json"), pr)
	d["head"] = headOf(d)
	w.f.Add("GET", hubPath+"/pulls/43", d)
	w.f.Add("GET", hubPath+"/pulls/43/reviews", []any{with(fx(t, "reviews.json").([]any)[1], map[string]any{"commit_id": head})})
	w.f.Add("GET", hubPath+"/issues/43/comments", []any{})
	w.f.Add("POST", hubPath+"/issues/43/comments", fx(t, "comment-created.json"), ghfake.Status(201))
	w.f.Add("PUT", hubPath+"/pulls/43/merge", fx(t, "merge-ok.json"),
		ghfake.Then(func() { w.mainRef(w.h.Merge(hb, "main")) }))
	return head
}

// The first merge moves main; sync fetches again, so the second ready branch
// is checked against, and merges onto, the main the first merge made.
func TestTwoReadyBranchesMergeInOneRun(t *testing.T) {
	w := newWorld(t)
	w.f.Add("PUT", hubPath+"/pulls/42/merge", fx(t, "merge-ok.json"),
		ghfake.Then(func() { w.mainRef(w.h.Merge(hubBr, "main")) }))
	other := w.otherBranch(w.edit(map[string]string{
		"repos/web-app/overview.md": concept("Overview", "Web app", "The web app, which retries.")}))
	firstMain := w.dh
	w.expectCode(0, w.runSync())
	w.expectWrites(mp{"PUT", hubPath + "/pulls/43/merge"}, mergeCall)
	logs := strings.Join(w.logs, "\n")
	for _, want := range []string{
		"sync: web-app/feat/other: merged hub #43 at " + other[:7],
		"sync: " + hubBr + ": merged hub #42 at " + w.kh[:7],
	} {
		if !strings.Contains(logs, want) {
			t.Errorf("logs lack %q:\n%s", want, logs)
		}
	}
	if merged := w.h.Git(w.h.Remote, "log", "--merges", "--format=%s", firstMain+"..main"); strings.Count(merged, "\n") != 1 {
		t.Errorf("main gained merges %q, want two", merged)
	}
}

// A merge holding a path materialize refuses blocks that branch with a
// comment; the repo's other branches still sync.
func TestUnsafePathBlocksOnlyItsBranch(t *testing.T) {
	w := newWorld(t)
	w.otherBranch(func(dir string) {
		writeFiles(t, dir, map[string]string{`repos/web-app/a\b.md`: concept("Gotcha", "Backslash", "A name Windows splits.")})
	})
	w.expectCode(0, w.runSync())
	w.expectWrites(mp{"POST", hubPath + "/issues/43/comments"}, mergeCall)
	body, _ := obj(w.f.Writes()[0].Body)["body"].(string)
	if !strings.HasPrefix(body, "<!-- okf-sync:unsafe-path:43 -->\n") ||
		!strings.Contains(body, "its merge with main (tree ") ||
		!strings.Contains(body, "holds the path `repos/web-app/a\\b.md`, which leaves the folder it is written to or names .git") {
		t.Errorf("comment %q", body)
	}
	logs := strings.Join(w.logs, "\n")
	for _, want := range []string{"sync: web-app/feat/other: not merged: its merge with main", "sync: " + hubBr + ": merged hub #42"} {
		if !strings.Contains(logs, want) {
			t.Errorf("logs lack %q:\n%s", want, logs)
		}
	}
}

// --- control files (plan decision 7) -------------------------------------

// The working tree lists another repo, and main gained a row after the clone;
// sync reads repos.txt from main as it fetches it.
func TestReposTxtComesFromFetchedDefaultBranch(t *testing.T) {
	w := newWorld(t)
	w.moveMain(func(dir string) {
		writeFiles(t, dir, map[string]string{"repos.txt": reposRow + "\nbilling-api https://github.com/acme/billing-api\n"})
	})
	writeFiles(t, w.h.Dir, map[string]string{"repos.txt": "other https://github.com/acme/other\n"})
	w.f.ReadOnly("/repos/acme/billing-api", "/repos/acme/other")
	w.expectCode(0, w.runSync())
	w.expectWrites(mergeCall)
	all := strings.Join(w.logs, "\n")
	if !strings.Contains(all, "sync: billing-api: no hub branches billing-api/<b>") || strings.Contains(all, "other") {
		t.Fatalf("logs %q", w.logs)
	}
}

func TestHubCloneWhoseOriginIsNotGitHubIsAConfigError(t *testing.T) {
	w := newWorld(t)
	w.h.Git(w.h.Dir, "remote", "set-url", "origin", "https://gitlab.com/acme/hub")
	w.expectCode(2, w.runSync())
	if all := strings.Join(w.logs, "\n"); !strings.Contains(all, "is not a GitHub repo") {
		t.Fatalf("logs %q", w.logs)
	}
	if len(w.f.Calls) != 0 {
		t.Fatalf("requests: %v", w.f.Calls)
	}
}

// --- other cases ---------------------------------------------------------

func TestAlreadyMergedHubPrIsNeverMergedAgain(t *testing.T) {
	w := newWorld(t)
	w.detail(map[string]any{"state": "closed", "merged": true, "merged_at": "2026-10-06T00:00:00Z"})
	w.expectCode(0, w.runSync())
	w.expectWrites()
	w.expectLine("already merged")
}

func TestSecondRunPostsNoDuplicateComment(t *testing.T) {
	w := newWorld(t)
	w.reviews()
	w.expectCode(0, w.runSync())
	posted := w.f.Writes()
	if len(posted) != 1 {
		t.Fatalf("posted %v", posted)
	}
	w.f.Add("GET", hubPath+"/issues/42/comments",
		[]any{with(fx(t, "comment-created.json"), map[string]any{"body": obj(posted[0].Body)["body"]})})
	w.reset()
	w.expectCode(0, w.runSync())
	w.expectWrites()
	w.expectLine("already commented on hub #42")
}

func TestDryRunMakesNoApiWrites(t *testing.T) {
	cases := []struct {
		name  string
		setup func(w *world)
	}{
		{"merge", func(w *world) {}},
		{"comment", func(w *world) { w.reviews() }},
		{"delete", func(w *world) { w.hubPRs(w.closedHubPR("2026-10-05T16:00:00Z")) }},
		{"close", func(w *world) { w.codePRs(with(w.code, map[string]any{"merged_at": nil})) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t)
			tc.setup(w)
			w.expectCode(0, w.runSync("--dry-run"))
			for _, c := range w.f.Calls {
				if c.Method != "GET" {
					t.Fatalf("write call in dry run: %v", c)
				}
			}
			w.expectLine("would ")
		})
	}
}

func TestApiErrorExitsNonzeroNamingRepo(t *testing.T) {
	w := newWorld(t)
	w.f.Add("GET", hubPath+"/pulls/42/reviews", map[string]any{"message": "Server Error"}, ghfake.Status(502))
	w.expectCode(1, w.runSync())
	if all := strings.Join(w.logs, "\n"); !strings.Contains(all, "sync: error: web-app:") {
		t.Fatalf("logs %q", w.logs)
	}
	w.expectWrites()
}
