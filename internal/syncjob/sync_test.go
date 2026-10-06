package syncjob

// compass sync against recorded GitHub API responses (testdata/fixtures/).
//
// One test per row of the branch-mode state table (research report section
// 5), plus the merge-condition cases in phase 02's list.

import (
	"bytes"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"compass/internal/github"
	"compass/internal/github/ghfake"
)

const (
	repoPath = "/repos/acme/web-app"
	reposTxt = "billing-api https://github.com/acme/billing-api folder\n" +
		"web-app https://github.com/acme/web-app.git branch\n"
	defaultNow = "2026-10-06T00:30:00Z"
	brName     = "feat/retry"
)

var (
	codeQ = map[string]string{"state": "all", "head": "acme:" + brName}
	kprQ  = map[string]string{"state": "open", "head": "acme:okf/" + brName, "base": "okf/main"}
)

type world struct {
	t    *testing.T
	f    *ghfake.Fake
	logs []string
	code map[string]any // #41 feat/retry -> main, merged, body links #42
	kpr  map[string]any // #42 okf/feat/retry -> okf/main, open
	kh   string
	tm   string
}

func fx(t *testing.T, name string) any { return ghfake.Fixture(t, "testdata", name) }

func obj(v any) map[string]any { return v.(map[string]any) }

func newWorld(t *testing.T) *world {
	w := &world{t: t, f: ghfake.New(t)}
	w.code = obj(fx(t, "pull-code.json"))
	w.kpr = obj(fx(t, "pull-knowledge.json"))
	w.kh = obj(w.kpr["head"])["sha"].(string)
	w.tm = obj(obj(fx(t, "ref.json"))["object"])["sha"].(string)
	f := w.f
	f.Add("GET", repoPath, fx(t, "repo.json"))
	f.Add("GET", repoPath+"/git/ref/heads/okf/main", fx(t, "ref.json"))
	f.Add("GET", repoPath+"/git/matching-refs/heads/okf/", fx(t, "matching-refs.json"))
	w.codeBranch(false)
	w.codePRs(w.code)
	w.knowledgePRs(w.kpr)
	f.Add("GET", repoPath+"/pulls/42", fx(t, "pull-knowledge-detail.json"))
	f.Add("GET", repoPath+"/pulls/42/reviews", fx(t, "reviews.json"))
	f.Add("GET", repoPath+"/collaborators/carol/permission", fx(t, "permission-write.json"))
	f.Add("GET", repoPath+"/collaborators/dave/permission", fx(t, "permission-read.json"))
	f.Add("GET", repoPath+"/commits/"+w.kh+"/check-runs", fx(t, "check-runs.json"),
		ghfake.Query(map[string]string{"check_name": "bundle-check", "filter": "latest"}))
	w.onTrunk(false)
	f.Add("GET", repoPath+"/git/commits/"+w.kh, fx(t, "git-commit.json"))
	for _, n := range []string{"41", "42"} {
		f.Add("GET", repoPath+"/issues/"+n+"/comments", []any{})
		f.Add("POST", repoPath+"/issues/"+n+"/comments", fx(t, "comment-created.json"), ghfake.Status(201))
	}
	f.Add("PUT", repoPath+"/pulls/42/merge", fx(t, "merge-ok.json"))
	f.Add("PATCH", repoPath+"/pulls/42", fx(t, "pull-closed.json"))
	f.Add("DELETE", repoPath+"/git/refs/heads/okf/"+brName, nil, ghfake.Status(204))
	return w
}

// --- world setters -------------------------------------------------------

func (w *world) codeBranch(onRemote bool) {
	if onRemote {
		w.f.Add("GET", repoPath+"/git/ref/heads/"+brName, ghfake.With(fx(w.t, "ref.json"), map[string]any{"ref": "refs/heads/" + brName}))
	} else {
		w.f.Add("GET", repoPath+"/git/ref/heads/"+brName, fx(w.t, "not-found.json"), ghfake.Status(404))
	}
}

func (w *world) codePRs(prs ...any) {
	w.f.Add("GET", repoPath+"/pulls", append([]any{}, prs...), ghfake.Query(codeQ))
}

func (w *world) knowledgePRs(prs ...any) {
	w.f.Add("GET", repoPath+"/pulls", append([]any{}, prs...), ghfake.Query(kprQ))
}

func (w *world) onTrunk(yes bool) {
	name := "compare-ahead.json"
	if yes {
		name = "compare-behind.json"
	}
	w.f.Add("GET", repoPath+"/compare/"+w.tm+"..."+w.kh, fx(w.t, name))
}

func (w *world) detail(changes map[string]any) {
	w.f.Add("GET", repoPath+"/pulls/42", ghfake.With(fx(w.t, "pull-knowledge-detail.json"), changes))
}

func (w *world) reviews(reviews ...any) {
	w.f.Add("GET", repoPath+"/pulls/42/reviews", append([]any{}, reviews...))
}

func (w *world) checks(runs ...any) {
	w.f.Add("GET", repoPath+"/commits/"+w.kh+"/check-runs", map[string]any{"total_count": len(runs), "check_runs": append([]any{}, runs...)})
}

func (w *world) checkRun(changes map[string]any) map[string]any {
	return ghfake.With(obj(fx(w.t, "check-runs.json"))["check_runs"].([]any)[0], changes)
}

func (w *world) reset() {
	w.f.Calls = nil
	w.logs = nil
}

// --- running -------------------------------------------------------------

func (w *world) runSyncAt(now string, extra ...string) int {
	hub := ghfake.WriteHub(w.t, w.t.TempDir(), reposTxt, "")
	args := append([]string{"--hub", hub, "--api-url", ghfake.API, "--now", now}, extra...)
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
		if strings.Contains(x, " okf/") {
			lines = append(lines, x)
		}
	}
	if len(lines) != 1 { // one log line per okf/<b>
		w.t.Fatalf("want one okf/ log line, got %q", w.logs)
	}
	if !strings.HasPrefix(lines[0], "sync: web-app okf/"+brName+": ") {
		w.t.Fatalf("bad prefix: %s", lines[0])
	}
	return lines[0]
}

func (w *world) commentBodies() map[string]string {
	out := map[string]string{}
	for _, c := range w.f.Writes() {
		if c.Method == "POST" {
			out[c.Path] = obj(c.Body)["body"].(string)
		}
	}
	return out
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
		w.t.Fatalf("writes %v, want %v", got, want)
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

func (w *world) assertBlocked(kind, reason string) {
	w.t.Helper()
	w.expectCode(0, w.runSync())
	w.expectWrites(mp{"POST", repoPath + "/issues/42/comments"}, mp{"POST", repoPath + "/issues/41/comments"})
	for _, body := range w.commentBodies() {
		if !strings.HasPrefix(body, "<!-- okf-sync:"+kind+":42 -->\n") {
			w.t.Errorf("body %q", body)
		}
		if !strings.Contains(body, reason) {
			w.t.Errorf("body %q lacks %q", body, reason)
		}
	}
	w.expectLine("not merged")
}

func with(o any, changes map[string]any) map[string]any { return ghfake.With(o, changes) }

// --- state table rows ----------------------------------------------------

func TestRow1CodePrOpenDoesNothing(t *testing.T) {
	w := newWorld(t)
	w.codeBranch(true)
	w.codePRs(with(w.code, map[string]any{"state": "open", "merged_at": nil, "closed_at": nil}))
	w.assertNothing("code #41 is open")
}

func TestRow1CodePrClosedWhileBranchOnRemoteDoesNothing(t *testing.T) {
	w := newWorld(t)
	w.codeBranch(true)
	w.codePRs(with(w.code, map[string]any{"merged_at": nil}))
	w.assertNothing("closed unmerged")
}

func TestRow1NoCodePrWhileBranchOnRemoteDoesNothing(t *testing.T) {
	w := newWorld(t)
	w.codeBranch(true)
	w.codePRs()
	w.assertNothing("no code pull request")
}

func TestRow2MergedApprovedGreenKnowledgePrIsMerged(t *testing.T) {
	w := newWorld(t)
	w.expectCode(0, w.runSync())
	w.expectWrites(mp{"PUT", repoPath + "/pulls/42/merge"})
	if got, want := w.f.Writes()[0].Body, map[string]any{"merge_method": "merge", "sha": w.kh}; !reflect.DeepEqual(got, want) {
		t.Fatalf("merge body %v, want %v", got, want)
	}
	w.expectLine("merged knowledge #42")
	w.expectLine("approved by carol")
}

func TestRow3UnapprovedCommentsOnBothPullRequests(t *testing.T) {
	w := newWorld(t)
	w.reviews(fx(t, "reviews.json").([]any)[:1]...) // only the changes request
	w.assertBlocked("unapproved", "no approval of its head commit")
}

func TestRow3FailingCheckCommentsOnBothPullRequests(t *testing.T) {
	w := newWorld(t)
	w.checks(w.checkRun(map[string]any{"conclusion": "failure"}))
	w.assertBlocked("failing", "bundle-check failed")
}

func TestRow3ConflictingCommentsOnBothPullRequests(t *testing.T) {
	w := newWorld(t)
	w.detail(map[string]any{"mergeable": false, "mergeable_state": "dirty"})
	w.assertBlocked("conflict", "conflicts with okf/main")
}

func TestRow4MergedUnmergedKnowledgeWithoutPrCommentsOnCodePr(t *testing.T) {
	w := newWorld(t)
	w.knowledgePRs()
	w.expectCode(0, w.runSync())
	w.expectWrites(mp{"POST", repoPath + "/issues/41/comments"})
	if body := w.commentBodies()[repoPath+"/issues/41/comments"]; !strings.HasPrefix(body, "<!-- okf-sync:missing-knowledge-pr:41 -->\n") {
		t.Fatalf("body %q", body)
	}
	w.expectLine("commented on #41")
}

func TestRow5MergedBranchGoneEverythingOnOkfMainDeletesBranch(t *testing.T) {
	w := newWorld(t)
	w.knowledgePRs()
	w.onTrunk(true)
	w.expectCode(0, w.runSync())
	w.expectWrites(mp{"DELETE", repoPath + "/git/refs/heads/okf/" + brName})
	w.expectLine("deleted okf/" + brName)
}

func TestRow5BranchStillOnRemoteKeepsOkfBranch(t *testing.T) {
	w := newWorld(t)
	w.knowledgePRs()
	w.onTrunk(true)
	w.codeBranch(true)
	w.assertNothing(brName + " is on the remote")
}

func TestRow6ClosedUnmergedBranchGoneClosesAndDeletes(t *testing.T) {
	w := newWorld(t)
	w.codePRs(with(w.code, map[string]any{"merged_at": nil}))
	w.expectCode(0, w.runSync())
	w.expectWrites(mp{"POST", repoPath + "/issues/42/comments"},
		mp{"PATCH", repoPath + "/pulls/42"},
		mp{"DELETE", repoPath + "/git/refs/heads/okf/" + brName})
	if got := w.f.Writes()[1].Body; !reflect.DeepEqual(got, map[string]any{"state": "closed"}) {
		t.Fatalf("patch body %v", got)
	}
	if body := w.commentBodies()[repoPath+"/issues/42/comments"]; !strings.Contains(body, "<!-- okf-sync:closed-unmerged:42 -->") {
		t.Fatalf("body %q", body)
	}
}

func TestRow7NoCodePrBranchGoneAfter7DaysClosesAndDeletes(t *testing.T) {
	w := newWorld(t)
	w.codePRs()
	// last commit 2026-10-04T08:59:00Z; 7 days later plus a minute
	w.expectCode(0, w.runSyncAt("2026-10-11T09:00:00Z"))
	w.expectWrites(mp{"POST", repoPath + "/issues/42/comments"},
		mp{"PATCH", repoPath + "/pulls/42"},
		mp{"DELETE", repoPath + "/git/refs/heads/okf/" + brName})
	if body := w.commentBodies()[repoPath+"/issues/42/comments"]; !strings.Contains(body, "<!-- okf-sync:abandoned:42 -->") {
		t.Fatalf("body %q", body)
	}
}

func TestRow7NoCodePrBranchGoneWithin7DaysWaits(t *testing.T) {
	w := newWorld(t)
	w.codePRs()
	w.expectCode(0, w.runSyncAt("2026-10-11T08:58:00Z"))
	w.expectWrites()
	w.expectLine("last commit 6d ago")
}

// --- merge conditions and safety -----------------------------------------

func TestPullRequestsFromForksAreIgnored(t *testing.T) {
	w := newWorld(t)
	fork := map[string]any{"id": 1, "name": "web-app", "full_name": "mallory/web-app"}
	w.codePRs(with(w.code, map[string]any{"head": with(w.code["head"], map[string]any{"repo": fork})}))
	w.knowledgePRs(with(w.kpr, map[string]any{"head": with(w.kpr["head"], map[string]any{"repo": fork})}))
	// No same-repo code PR, and the knowledge branch is young: nothing at all.
	w.expectCode(0, w.runSync())
	w.expectWrites()
	w.expectLine("no code pull request")
	for _, c := range w.f.Calls {
		if strings.HasSuffix(c.Path, "/merge") {
			t.Fatalf("merge call: %v", c)
		}
	}
}

func TestAppApprovalsAndApprovalsOfOlderCommitsDoNotCount(t *testing.T) {
	w := newWorld(t)
	bot := func(login string) map[string]any { return map[string]any{"login": login, "id": 9, "type": "Bot"} }
	base := fx(t, "reviews.json").([]any)[1]
	w.reviews(
		with(base, map[string]any{"user": bot("okf-writer[bot]")}),
		with(base, map[string]any{"user": bot("okf-reader[bot]")}),
		with(base, map[string]any{"user": map[string]any{"login": "ci-robot", "id": 8, "type": "User"}}), // --ignore-login
		with(base, map[string]any{"user": map[string]any{"login": "dave", "id": 7, "type": "User"}}),     // read access only
		with(base, map[string]any{"commit_id": "6666666666666666666666666666666666666666"}),              // carol, older head
	)
	w.expectCode(0, w.runSync("--ignore-login", "CI-Robot"))
	for _, x := range w.writes() {
		if x == (mp{"PUT", repoPath + "/pulls/42/merge"}) {
			t.Fatal("merged")
		}
	}
	w.expectLine("no approval of its head commit")
	for _, c := range w.f.Calls {
		if strings.Contains(c.Path, "/collaborators/ci-robot") {
			t.Fatalf("asked permission of an ignored login: %v", c)
		}
	}
}

func TestNoBundleCheckResultOnHeadIsNotMerged(t *testing.T) {
	w := newWorld(t)
	otherApp := w.checkRun(map[string]any{"app": map[string]any{"id": 1, "slug": "evil-ci", "name": "Evil CI"}})
	cases := map[string][]any{
		"none":        {},
		"other-app":   {otherApp},
		"in-progress": {w.checkRun(map[string]any{"status": "in_progress", "conclusion": nil})},
	}
	for _, name := range []string{"none", "other-app", "in-progress"} {
		t.Run(name, func(t *testing.T) {
			w.t = t
			w.reset()
			w.checks(cases[name]...)
			w.expectCode(0, w.runSync())
			w.expectWrites()
			w.expectLine("waiting: bundle-check")
		})
	}
}

func TestCodePrMergedIntoAnotherBranchIsNotMerged(t *testing.T) {
	w := newWorld(t)
	w.codePRs(with(w.code, map[string]any{"base": with(w.code["base"], map[string]any{"ref": "release/1.x"})}))
	w.assertNothing("treated as not merged")
}

func TestReusedBranchWithOldUnlinkedCodePrNeverMerges(t *testing.T) {
	w := newWorld(t)
	w.codePRs(with(w.code, map[string]any{"body": "Old change from last year.", "created_at": "2025-01-02T00:00:00Z"}))
	w.expectCode(0, w.runSync())
	w.expectWrites(mp{"POST", repoPath + "/issues/42/comments"})
	if body := w.commentBodies()[repoPath+"/issues/42/comments"]; !strings.Contains(body, "<!-- okf-sync:unlinked:42 -->") {
		t.Fatalf("body %q", body)
	}
	w.expectLine("do not link each other")
}

func TestLinkFromKnowledgePrByUrlCounts(t *testing.T) {
	w := newWorld(t)
	w.codePRs(with(w.code, map[string]any{"body": "No links here; #420 and acme/other#42 are not it."}))
	w.knowledgePRs(with(w.kpr, map[string]any{"body": "Pairs with https://github.com/acme/web-app/pull/41."}))
	w.expectCode(0, w.runSync())
	w.expectWrites(mp{"PUT", repoPath + "/pulls/42/merge"})
}

func TestMergePassesCheckedHeadSoALaterPushFailsIt(t *testing.T) {
	w := newWorld(t)
	w.f.Add("PUT", repoPath+"/pulls/42/merge", fx(t, "merge-head-moved.json"), ghfake.Status(409))
	w.expectCode(0, w.runSync())
	w.expectWrites(mp{"PUT", repoPath + "/pulls/42/merge"})
	if got := obj(w.f.Writes()[0].Body)["sha"]; got != w.kh {
		t.Fatalf("merge sha %v, want %s", got, w.kh)
	}
	w.expectLine("GitHub refused the merge")
}

func TestMergedPullRequestIsNeverMergedAgain(t *testing.T) {
	w := newWorld(t)
	w.detail(map[string]any{"state": "closed", "merged": true, "merged_at": "2026-10-06T00:00:00Z"})
	w.expectCode(0, w.runSync())
	w.expectWrites()
	w.expectLine("already merged")
	// The run after: the knowledge PR is no longer open and okf/<b> is on okf/main.
	w.reset()
	w.knowledgePRs()
	w.onTrunk(true)
	w.expectCode(0, w.runSync())
	w.expectWrites(mp{"DELETE", repoPath + "/git/refs/heads/okf/" + brName})
}

func TestSecondRunPostsNoDuplicateComment(t *testing.T) {
	w := newWorld(t)
	w.reviews()
	w.expectCode(0, w.runSync())
	posted := w.f.Writes()
	if len(posted) != 2 {
		t.Fatalf("posted %v", posted)
	}
	for _, c := range posted {
		parts := strings.Split(c.Path, "/")
		n := parts[len(parts)-2]
		w.f.Add("GET", repoPath+"/issues/"+n+"/comments",
			[]any{with(fx(t, "comment-created.json"), map[string]any{"body": obj(c.Body)["body"]})})
	}
	w.reset()
	w.expectCode(0, w.runSync())
	w.expectWrites()
	w.expectLine("already commented on #42; already commented on #41")
}

func TestDryRunMakesNoApiWrites(t *testing.T) {
	cases := []struct {
		name  string
		setup func(w *world)
	}{
		{"merge", func(w *world) {}},
		{"comment", func(w *world) { w.reviews() }},
		{"delete", func(w *world) { w.knowledgePRs(); w.onTrunk(true) }},
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

func TestOkfMainIsNeverSyncedOrDeletedAndFolderReposAreSkipped(t *testing.T) {
	w := newWorld(t)
	w.knowledgePRs()
	w.onTrunk(true)
	w.expectCode(0, w.runSync())
	w.expectWrites(mp{"DELETE", repoPath + "/git/refs/heads/okf/" + brName})
	for _, c := range w.f.Calls {
		if strings.Contains(c.Path, "billing-api") {
			t.Fatalf("folder repo called: %v", c)
		}
		if strings.HasSuffix(c.Path, "okf/main") && c.Method != "GET" {
			t.Fatalf("okf/main written: %v", c)
		}
	}
	for _, x := range w.logs {
		if strings.Contains(x, "okf/main") && strings.Contains(x, "sync: web-app okf/main") {
			t.Fatalf("okf/main synced: %s", x)
		}
	}
}

func TestApiErrorExitsNonzeroNamingRepo(t *testing.T) {
	w := newWorld(t)
	w.f.Add("GET", repoPath+"/pulls/42/reviews", map[string]any{"message": "Server Error"}, ghfake.Status(502))
	w.expectCode(1, w.runSync())
	if all := strings.Join(w.logs, "\n"); !strings.Contains(all, "sync: error: web-app:") {
		t.Fatalf("logs %q", all)
	}
	w.expectWrites()
}
