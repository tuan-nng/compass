package stamp

// compass stamp against recorded GitHub API responses (testdata/fixtures/)
// and a real local hub clone (ghfake.WriteHub) whose origin names the hub
// acme/hub. Gates read the code repo, which every test marks read-only (plan
// decision 6); concepts are read from and committed to the hub at
// repos/<id>/<concept>.md (plan decision 10).

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"compass/internal/github"
	"compass/internal/github/ghfake"
)

const (
	repoPath = "/repos/acme/billing-api"
	webPath  = "/repos/acme/web-app"
	hubPath  = ghfake.HubPath
	reposTxt = "# id url\n" +
		"billing-api https://github.com/acme/billing-api.git\n" +
		"web-app https://github.com/acme/web-app\n"
	row  = `billing-api contract.yml "contract test" process:contract-test`
	done = "2026-10-05T09:12:34Z" // completed_at of the "contract test" job in jobs.json
	// concepts is where billing-api's concepts live on the hub.
	concepts = "repos/billing-api/"
)

var proto = filepath.Join("..", "..", "testdata", "proto", "billing-api", "okf")

type obj = map[string]any

func fx(t *testing.T, name string) obj { return ghfake.Fixture(t, "testdata", name).(obj) }

func protoText(t *testing.T, cid string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(proto, cid+".md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func sha(o obj) string { return o["sha"].(string) }

type env struct {
	t   *testing.T
	f   *ghfake.Fake
	out bytes.Buffer
	// head is the code repo's default-branch head; hubHead is the hub main
	// the stamps land on, and tree its tree.
	head, hubHead, tree string
	// newTree and newCommit are the shas the fake returns for writes.
	newTree, newCommit string
}

func setup(t *testing.T) *env {
	e := &env{t: t, f: ghfake.New(t)}
	e.head = fx(t, "ref-main.json")["object"].(obj)["sha"].(string)
	e.hubHead = fx(t, "hub-ref.json")["object"].(obj)["sha"].(string)
	e.tree = fx(t, "hub-commit.json")["tree"].(obj)["sha"].(string)
	e.newTree = sha(fx(t, "tree-created.json"))
	e.newCommit = sha(fx(t, "commit-created.json"))
	f := e.f
	f.ReadOnly(repoPath, webPath)
	f.Add("GET", repoPath, fx(t, "repo.json"))
	f.Add("GET", repoPath+"/git/ref/heads/main", fx(t, "ref-main.json"))
	f.Add("GET", repoPath+"/actions/workflows/contract.yml/runs", fx(t, "workflow-runs.json"),
		ghfake.Query(map[string]string{"branch": "main", "event": "push", "head_sha": e.head}))
	f.Add("GET", repoPath+"/actions/runs/11223344556/jobs", fx(t, "jobs.json"),
		ghfake.Query(map[string]string{"filter": "latest"}))
	f.Add("GET", hubPath+"/git/ref/heads/main", fx(t, "hub-ref.json"))
	f.Add("GET", hubPath+"/git/commits/"+e.hubHead, fx(t, "hub-commit.json"))
	f.Add("POST", hubPath+"/git/trees", fx(t, "tree-created.json"), ghfake.Status(201))
	f.Add("POST", hubPath+"/git/commits", fx(t, "commit-created.json"), ghfake.Status(201))
	f.Add("PATCH", hubPath+"/git/refs/heads/main", fx(t, "ref-updated.json"))
	for _, cid := range []string{"gotchas/idempotency-key", "contracts/invoice-api", "services/billing-api"} {
		e.concept(cid, protoText(t, cid))
	}
	return e
}

// concept routes billing-api's concept cid on the hub head.
func (e *env) concept(cid, text string) { e.conceptAt(concepts+cid, text) }

func (e *env) conceptAt(path, text string) {
	e.f.Add("GET", hubPath+"/contents/"+path+".md", ghfake.Contents(text),
		ghfake.Query(map[string]string{"ref": e.hubHead}))
}

func (e *env) main(args ...string) int {
	var stderr bytes.Buffer
	code := MainWith(args, &e.out, &stderr, ghfake.Getenv, func(token, baseURL string, dryRun bool) *github.Client {
		if token != ghfake.Token || baseURL != ghfake.API {
			e.t.Errorf("client for %q at %q", token, baseURL)
		}
		return e.f.Client(dryRun)
	})
	if stderr.Len() > 0 {
		e.t.Errorf("stderr: %s", stderr.String())
	}
	return code
}

func (e *env) runHub(hub *ghfake.Hub, extra ...string) int {
	return e.main(append([]string{"--hub", hub.Dir, "--api-url", ghfake.API}, extra...)...)
}

func (e *env) runStamp(checks string, extra ...string) int {
	return e.runHub(ghfake.WriteHub(e.t, reposTxt, checks), extra...)
}

func (e *env) logs() []string {
	s := strings.TrimSuffix(e.out.String(), "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func (e *env) logText() string { return strings.Join(e.logs(), "\n") }

func (e *env) code(got, want int) {
	e.t.Helper()
	if got != want {
		e.t.Fatalf("exit %d, want %d; log:\n%s", got, want, e.logText())
	}
}

func (e *env) logHas(s string) {
	e.t.Helper()
	if !strings.Contains(e.logText(), s) {
		e.t.Errorf("log lacks %q:\n%s", s, e.logText())
	}
}

func deepEq(t *testing.T, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %#v\nwant %#v", got, want)
	}
}

func body(c ghfake.Call) obj { return c.Body.(obj) }

func methodPaths(calls []ghfake.Call) [][2]string {
	out := [][2]string{}
	for _, c := range calls {
		out = append(out, [2]string{c.Method, c.Path})
	}
	return out
}

// committed returns {path: text} of the one tree written, after asserting
// the exact writes: one tree and one commit on the hub head, and one
// fast-forward of the hub's main.
func (e *env) committed() map[string]string {
	t := e.t
	t.Helper()
	w := e.f.Writes()
	deepEq(t, methodPaths(w), [][2]string{
		{"POST", hubPath + "/git/trees"}, {"POST", hubPath + "/git/commits"},
		{"PATCH", hubPath + "/git/refs/heads/main"}})
	if len(w) != 3 {
		t.FailNow()
	}
	deepEq(t, body(w[0])["base_tree"], e.tree)
	deepEq(t, body(w[1])["tree"], e.newTree)
	deepEq(t, body(w[1])["parents"], []any{e.hubHead})
	deepEq(t, body(w[2]), obj{"sha": e.newCommit, "force": false})
	out := map[string]string{}
	for _, it := range body(w[0])["tree"].([]any) {
		en := it.(obj)
		deepEq(t, [2]any{en["mode"], en["type"]}, [2]any{"100644", "blob"})
		out[en["path"].(string)] = en["content"].(string)
	}
	return out
}

func (e *env) fetched() []string {
	out := []string{}
	for _, c := range e.f.Calls {
		if strings.Contains(c.Path, "/contents/") {
			out = append(out, c.Path)
		}
	}
	return out
}

func jobsList(o obj) []any { return o["jobs"].([]any) }

func runsList(o obj) []any { return o["workflow_runs"].([]any) }

// 1
func TestStampsCoveredConceptWhenJobSucceededOnHead(t *testing.T) {
	e := setup(t)
	e.code(e.runStamp(row+" gotchas/idempotency-key\n"), 0)
	old := protoText(t, "gotchas/idempotency-key")
	expect := strings.ReplaceAll(old,
		"generated: { by: cursor/gpt-5.6, at: 2026-06-01T12:00:00Z }\n",
		"generated: { by: cursor/gpt-5.6, at: 2026-06-01T12:00:00Z }\n"+
			"verified: { by: process:contract-test, at: "+done+" }\n")
	if expect == old {
		t.Fatal("fixture lacks the generated line")
	}
	deepEq(t, e.committed(), map[string]string{concepts + "gotchas/idempotency-key.md": expect})
	deepEq(t, body(e.f.Writes()[1])["message"],
		"okf: process stamps from process:contract-test (billing-api contract.yml/contract test @ "+e.head[:7]+")")
	e.logHas("stamped gotchas/idempotency-key (added) on acme/hub main")
}

// 2: failed, missing, other commit, other workflow file
func (e *env) assertNoCheckOnHead() {
	t := e.t
	t.Helper()
	e.code(e.runStamp(row+" gotchas/idempotency-key\n"), 0)
	deepEq(t, len(e.f.Writes()), 0)
	e.logHas("no check run on head " + e.head[:7])
	deepEq(t, e.fetched(), []string{})
}

func TestNoStampWhenJobFailed(t *testing.T) {
	e := setup(t)
	jobs := fx(t, "jobs.json")
	jobsList(jobs)[0].(obj)["conclusion"] = "failure"
	e.f.Add("GET", repoPath+"/actions/runs/11223344556/jobs", jobs)
	e.assertNoCheckOnHead()
}

func TestNoStampWhenJobMissing(t *testing.T) {
	e := setup(t)
	jobs := fx(t, "jobs.json")
	kept := []any{}
	for _, j := range jobsList(jobs) {
		if j.(obj)["name"] != "contract test" {
			kept = append(kept, j)
		}
	}
	jobs["jobs"] = kept
	e.f.Add("GET", repoPath+"/actions/runs/11223344556/jobs", jobs)
	e.assertNoCheckOnHead()
}

func TestNoStampWhenRunOnAnotherCommit(t *testing.T) {
	e := setup(t)
	runs := fx(t, "workflow-runs.json")
	runsList(runs)[0].(obj)["head_sha"] = strings.Repeat("0", 40)
	e.f.Add("GET", repoPath+"/actions/workflows/contract.yml/runs", runs)
	e.assertNoCheckOnHead()
}

func TestNoStampWhenRunFromAnotherWorkflowFile(t *testing.T) {
	e := setup(t)
	runs := fx(t, "workflow-runs.json")
	runsList(runs)[0].(obj)["path"] = ".github/workflows/copycat.yml"
	e.f.Add("GET", repoPath+"/actions/workflows/contract.yml/runs", runs)
	e.assertNoCheckOnHead()
}

func TestNoStampWhenRunWasNotAPush(t *testing.T) {
	e := setup(t)
	runs := fx(t, "workflow-runs.json")
	runsList(runs)[0].(obj)["event"] = "workflow_dispatch"
	e.f.Add("GET", repoPath+"/actions/workflows/contract.yml/runs", runs)
	e.assertNoCheckOnHead()
}

// 3
func TestConceptNotInChecksIsNeverStamped(t *testing.T) {
	e := setup(t)
	e.code(e.runStamp(row+" services/billing-api\n"), 0)
	// gotchas/idempotency-key is unstamped and due, but no row lists it.
	got := e.committed()
	deepEq(t, len(got), 1)
	if _, ok := got[concepts+"services/billing-api.md"]; !ok {
		t.Errorf("committed %v", got)
	}
	deepEq(t, e.fetched(), []string{hubPath + "/contents/" + concepts + "services/billing-api.md"})
}

// 4
func TestRefreshesActorEntryOlderThanGenerated(t *testing.T) {
	e := setup(t)
	old := strings.ReplaceAll(protoText(t, "contracts/invoice-api"),
		"verified: { by: process:contract-test, at: 2026-09-20T08:00:00Z }",
		"verified: { by: process:contract-test, at: 2026-06-01T08:00:00Z }")
	e.concept("contracts/invoice-api", old)
	e.code(e.runStamp(row+" contracts/invoice-api\n"), 0)
	expect := strings.ReplaceAll(old, "at: 2026-06-01T08:00:00Z }", "at: "+done+" }")
	deepEq(t, e.committed(), map[string]string{concepts + "contracts/invoice-api.md": expect})
	e.logHas("contracts/invoice-api (refreshed)")
}

// 5
func TestSkipsConceptGeneratedAfterJobCompleted(t *testing.T) {
	e := setup(t)
	text := strings.ReplaceAll(protoText(t, "gotchas/idempotency-key"), "at: 2026-06-01T12:00:00Z", "at: 2026-10-05T10:00:00Z")
	e.concept("gotchas/idempotency-key", text)
	e.code(e.runStamp(row+" gotchas/idempotency-key\n"), 0)
	deepEq(t, len(e.f.Writes()), 0)
	e.logHas("nothing due")
}

// 6
func TestConceptWithoutGeneratedIsStampedOnce(t *testing.T) {
	e := setup(t)
	text := "---\ntype: Runbook\ntitle: Rotate keys\ndescription: How to rotate keys.\n---\n# Rotate keys\n"
	e.concept("runbooks/rotate-keys", text)
	e.code(e.runStamp(row+" runbooks/rotate-keys\n"), 0)
	stamped := e.committed()[concepts+"runbooks/rotate-keys.md"]
	deepEq(t, stamped, strings.ReplaceAll(text,
		"description: How to rotate keys.\n",
		"description: How to rotate keys.\nverified: { by: process:contract-test, at: "+done+" }\n"))
	// Next run: the concept now carries the actor's entry, so it is left alone
	// even when a later job completes.
	e.f.Calls = nil
	e.concept("runbooks/rotate-keys", stamped)
	jobs := fx(t, "jobs.json")
	jobsList(jobs)[0].(obj)["completed_at"] = "2026-10-07T00:00:00Z"
	e.f.Add("GET", repoPath+"/actions/runs/11223344556/jobs", jobs)
	e.code(e.runStamp(row+" runbooks/rotate-keys\n"), 0)
	deepEq(t, len(e.f.Writes()), 0)
}

// 7: no commit when stamps are current
func TestSecondRunMakesNoCommitWhenStampsAreCurrent(t *testing.T) {
	e := setup(t)
	checks := row + " gotchas/idempotency-key contracts/invoice-api\n"
	e.code(e.runStamp(checks), 0)
	first := e.committed()
	deepEq(t, len(first), 1) // invoice-api is current
	if _, ok := first[concepts+"gotchas/idempotency-key.md"]; !ok {
		t.Fatalf("committed %v", first)
	}
	e.f.Calls = nil
	e.concept("gotchas/idempotency-key", first[concepts+"gotchas/idempotency-key.md"])
	e.out.Reset()
	e.code(e.runStamp(checks), 0)
	deepEq(t, len(e.f.Writes()), 0)
	logs := e.logs()
	deepEq(t, len(logs), 1)
	if len(logs) == 1 && !strings.Contains(logs[0], "nothing due on acme/hub main "+e.hubHead[:7]) {
		t.Errorf("log %q", logs[0])
	}
}

// 8
func TestHumanOneLineEntryKeptWhenProcessEntryAdded(t *testing.T) {
	e := setup(t)
	old := protoText(t, "services/billing-api")
	human := "{ by: human:alice, at: 2026-06-12T16:00:00Z }"
	if !strings.Contains(old, "verified: "+human+"\n") {
		t.Fatal("fixture lacks the human entry")
	}
	e.code(e.runStamp(row+" services/billing-api\n"), 0)
	expect := strings.ReplaceAll(old, "verified: "+human+"\n",
		"verified:\n  - "+human+"\n  - { by: process:contract-test, at: "+done+" }\n")
	deepEq(t, e.committed(), map[string]string{concepts + "services/billing-api.md": expect})
}

// 9
func TestUnsupportedVerifiedFormFailsAndLeavesConceptUntouched(t *testing.T) {
	e := setup(t)
	text := strings.ReplaceAll(protoText(t, "services/billing-api"),
		"verified: { by: human:alice, at: 2026-06-12T16:00:00Z }",
		"verified:\n  by: human:alice\n  at: 2026-06-12T16:00:00Z")
	e.concept("services/billing-api", text)
	e.code(e.runStamp(row+" services/billing-api gotchas/idempotency-key\n"), 1)
	e.logHas("error: billing-api services/billing-api")
	got := e.committed()
	deepEq(t, len(got), 1)
	if _, ok := got[concepts+"gotchas/idempotency-key.md"]; !ok {
		t.Errorf("committed %v", got)
	}
}

// 10
func TestMissingConceptExitsNonzeroNamingChecksRow(t *testing.T) {
	e := setup(t)
	e.f.Add("GET", hubPath+"/contents/"+concepts+"contracts/moved.md", fx(t, "not-found.json"), ghfake.Status(404))
	r := row + " contracts/moved"
	e.code(e.runStamp("# covered concepts\n"+r+"\n"), 1)
	e.logHas("checks.txt:2: " + r)
	e.logHas("concept not found at acme/hub main " + e.hubHead[:7] + ": " + concepts + "contracts/moved.md")
	deepEq(t, len(e.f.Writes()), 0)
}

// 11
func TestHubMainMovedAfterReadPushesNothing(t *testing.T) {
	e := setup(t)
	// The first read sees e.hubHead; the re-read after the refusal sees a new head.
	e.f.Add("GET", hubPath+"/git/ref/heads/main", ghfake.With(fx(t, "hub-ref.json"),
		obj{"object": obj{"sha": strings.Repeat("e", 40), "type": "commit"}}))
	e.f.Add("GET", hubPath+"/git/ref/heads/main", fx(t, "hub-ref.json"), ghfake.Times(1))
	e.f.Add("PATCH", hubPath+"/git/refs/heads/main", fx(t, "ref-not-fast-forward.json"), ghfake.Status(422))
	e.code(e.runStamp(row+" gotchas/idempotency-key\n"), 0)
	e.committed() // one tree, one commit, one refused fast-forward; no retry, no force
	e.logHas("not pushed: acme/hub main moved")
}

// A refusal with the head unchanged is protection, not a race; the message
// names the hub branch. The message is synthetic: the stamper must not
// depend on GitHub's wording.
func TestRefusedPushWithUnchangedHeadExitsNonzeroNamingHubBranch(t *testing.T) {
	for _, status := range []int{409, 422} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			e := setup(t)
			e.f.Add("PATCH", hubPath+"/git/refs/heads/main", obj{"message": "refused for the test"}, ghfake.Status(status))
			e.code(e.runStamp(row+" gotchas/idempotency-key\n"), 1)
			e.committed() // one tree, one commit, one refused update; no retry, no force
			e.logHas(fmt.Sprintf(`stamp: error: hub acme/hub: GitHub refused the update to acme/hub main at %s with HTTP %d "refused for the test"`,
				e.hubHead[:7], status))
			e.logHas("branch protection or a ruleset likely blocks this account, which needs a bypass on the hub's main")
			if strings.Contains(e.logText(), "moved") {
				t.Errorf("refusal reported as a move:\n%s", e.logText())
			}
		})
	}
}

func TestRefusedPushThenFailedReReadExitsNonzeroNamingHub(t *testing.T) {
	e := setup(t)
	e.f.Add("GET", hubPath+"/git/ref/heads/main", fx(t, "rate-limited.json"),
		ghfake.Status(403), ghfake.Header("x-ratelimit-remaining", "0"))
	e.f.Add("GET", hubPath+"/git/ref/heads/main", fx(t, "hub-ref.json"), ghfake.Times(1))
	e.f.Add("PATCH", hubPath+"/git/refs/heads/main", fx(t, "ref-not-fast-forward.json"), ghfake.Status(422))
	e.code(e.runStamp(row+" gotchas/idempotency-key\n"), 1)
	e.committed()
	e.logHas("stamp: error: hub acme/hub:")
}

// Beyond the phase list

func TestDryRunMakesNoWrites(t *testing.T) {
	e := setup(t)
	e.code(e.runStamp(row+" gotchas/idempotency-key services/billing-api\n", "--dry-run"), 0)
	deepEq(t, len(e.f.Writes()), 0)
	e.logHas("would stamp gotchas/idempotency-key (added), services/billing-api (added)")
}

func TestTwoRowsForOneRepoChainOntoOneRead(t *testing.T) {
	e := setup(t)
	second := strings.Repeat("c", 40)
	e.f.Add("POST", hubPath+"/git/trees", ghfake.With(fx(t, "tree-created.json"), obj{"sha": strings.Repeat("d", 40)}), ghfake.Times(1))
	e.f.Add("POST", hubPath+"/git/commits", ghfake.With(fx(t, "commit-created.json"), obj{"sha": second}), ghfake.Times(1))
	e.f.Add("POST", hubPath+"/git/trees", fx(t, "tree-created.json"), ghfake.Times(1))
	e.f.Add("POST", hubPath+"/git/commits", fx(t, "commit-created.json"), ghfake.Times(1))
	checks := row + " gotchas/idempotency-key\n" +
		"billing-api contract.yml lint process:lint gotchas/idempotency-key\n"
	e.code(e.runStamp(checks), 0)
	w := e.f.Writes()
	methods := []string{}
	for _, c := range w {
		methods = append(methods, c.Method)
	}
	deepEq(t, methods, []string{"POST", "POST", "POST", "POST", "PATCH"})
	if len(w) != 5 {
		t.FailNow()
	}
	deepEq(t, body(w[1])["parents"], []any{e.hubHead})
	deepEq(t, body(w[2])["base_tree"], e.newTree)
	deepEq(t, body(w[3])["parents"], []any{e.newCommit})
	deepEq(t, body(w[4]), obj{"sha": second, "force": false})
	final := body(w[2])["tree"].([]any)[0].(obj)["content"].(string)
	want := "verified:\n  - { by: process:contract-test, at: " + done + " }\n" +
		"  - { by: process:lint, at: 2026-10-05T09:06:01Z }\n"
	if !strings.Contains(final, want) {
		t.Errorf("final text lacks %q:\n%s", want, final)
	}
}

// Two code repos stamp in one run: their commits chain on the hub head, and
// the hub's main moves once, to the last (plan decision 10).
func TestTwoReposChainIntoOneFastForwardOfHubMain(t *testing.T) {
	e := setup(t)
	webHead := strings.Repeat("7", 40)
	f := e.f
	f.Add("GET", webPath, ghfake.With(fx(t, "repo.json"), obj{"name": "web-app", "full_name": "acme/web-app"}))
	f.Add("GET", webPath+"/git/ref/heads/main", ghfake.With(fx(t, "ref-main.json"), obj{"object": obj{"sha": webHead, "type": "commit"}}))
	runs := fx(t, "workflow-runs.json")
	run := runsList(runs)[0].(obj)
	run["id"], run["head_sha"] = 22334455667.0, webHead
	run["repository"] = obj{"id": 2, "name": "web-app", "full_name": "acme/web-app"}
	f.Add("GET", webPath+"/actions/workflows/contract.yml/runs", runs,
		ghfake.Query(map[string]string{"branch": "main", "event": "push", "head_sha": webHead}))
	jobs := fx(t, "jobs.json")
	for _, j := range jobsList(jobs) {
		j.(obj)["head_sha"] = webHead
	}
	f.Add("GET", webPath+"/actions/runs/22334455667/jobs", jobs)
	e.conceptAt("repos/web-app/gotchas/idempotency-key", protoText(t, "gotchas/idempotency-key"))
	second := strings.Repeat("c", 40)
	f.Add("POST", hubPath+"/git/trees", ghfake.With(fx(t, "tree-created.json"), obj{"sha": strings.Repeat("d", 40)}), ghfake.Times(1))
	f.Add("POST", hubPath+"/git/commits", ghfake.With(fx(t, "commit-created.json"), obj{"sha": second}), ghfake.Times(1))
	f.Add("POST", hubPath+"/git/trees", fx(t, "tree-created.json"), ghfake.Times(1))
	f.Add("POST", hubPath+"/git/commits", fx(t, "commit-created.json"), ghfake.Times(1))
	checks := row + " gotchas/idempotency-key\n" +
		`web-app contract.yml "contract test" process:contract-test gotchas/idempotency-key` + "\n"
	e.code(e.runStamp(checks), 0)
	w := e.f.Writes()
	deepEq(t, methodPaths(w), [][2]string{
		{"POST", hubPath + "/git/trees"}, {"POST", hubPath + "/git/commits"},
		{"POST", hubPath + "/git/trees"}, {"POST", hubPath + "/git/commits"},
		{"PATCH", hubPath + "/git/refs/heads/main"}})
	if len(w) != 5 {
		t.FailNow()
	}
	paths := func(c ghfake.Call) []any {
		var out []any
		for _, en := range body(c)["tree"].([]any) {
			out = append(out, en.(obj)["path"])
		}
		return out
	}
	deepEq(t, paths(w[0]), []any{"repos/billing-api/gotchas/idempotency-key.md"})
	deepEq(t, paths(w[2]), []any{"repos/web-app/gotchas/idempotency-key.md"})
	deepEq(t, body(w[1])["parents"], []any{e.hubHead})
	deepEq(t, body(w[3])["parents"], []any{e.newCommit})
	deepEq(t, body(w[4]), obj{"sha": second, "force": false})
	if msg := body(w[3])["message"].(string); !strings.Contains(msg, "(web-app contract.yml/contract test @ 7777777)") {
		t.Errorf("message %q", msg)
	}
}

// The working tree's checks.txt names another concept, and main gained a
// row after the clone; the stamper reads checks.txt from main as it fetches
// it (plan decision 7).
func TestControlFilesComeFromFetchedDefaultBranch(t *testing.T) {
	e := setup(t)
	hub := ghfake.WriteHub(t, reposTxt, row+" services/billing-api\n")
	hub.Commit("main", "", func(dir string) {
		if err := os.WriteFile(filepath.Join(dir, "checks.txt"), []byte(row+" gotchas/idempotency-key\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	})
	if err := os.WriteFile(filepath.Join(hub.Dir, "checks.txt"), []byte(row+" contracts/invoice-api\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.code(e.runHub(hub), 0)
	got := e.committed()
	if _, ok := got[concepts+"gotchas/idempotency-key.md"]; !ok || len(got) != 1 {
		t.Errorf("committed %v", got)
	}
}

// The shipped hub template parses with one row under each header.
func TestTemplateHubStamps(t *testing.T) {
	e := setup(t)
	hub := ghfake.TemplateHub(t, "billing-api https://github.com/acme/billing-api", row+" gotchas/idempotency-key")
	e.code(e.runHub(hub), 0)
	if _, ok := e.committed()[concepts+"gotchas/idempotency-key.md"]; !ok {
		t.Errorf("committed nothing at %sgotchas/idempotency-key.md", concepts)
	}
}

func TestRateLimitExitsNonzeroNamingRepo(t *testing.T) {
	e := setup(t)
	e.f.Add("GET", repoPath+"/actions/workflows/contract.yml/runs", fx(t, "rate-limited.json"),
		ghfake.Status(403), ghfake.Header("x-ratelimit-remaining", "0"))
	e.code(e.runStamp(row+" gotchas/idempotency-key\n"), 1)
	e.logHas("error: billing-api:")
	e.logHas("rate limited")
	deepEq(t, len(e.f.Writes()), 0)
}

func TestChecksRowForUnknownRepoIsAConfigError(t *testing.T) {
	e := setup(t)
	e.code(e.runStamp("ghost contract.yml test process:x contracts/a\n"), 2)
	e.logHas("checks.txt:1: ghost")
}

// The stamper must never commit a stamp that only review may grant
// (human:) or text that escapes the verified entry.
func TestChecksRowActorMustBeProcess(t *testing.T) {
	for _, actor := range []string{"human:alice", `"process:x, at: 2099-01-01T00:00:00Z }"`} {
		t.Run(actor, func(t *testing.T) {
			e := setup(t)
			e.code(e.runStamp(`billing-api contract.yml "contract test" `+actor+" gotchas/idempotency-key\n"), 2)
			e.logHas("checks.txt:1: ")
			e.logHas("actor must be process:<name>")
			deepEq(t, len(e.f.Writes()), 0)
		})
	}
}

func TestReposURLNotOnGitHubIsRejected(t *testing.T) {
	e := setup(t)
	e.code(e.runHub(ghfake.WriteHub(t, "billing-api https://gitlab.com/acme/billing-api\n", "")), 2)
	e.logHas("repos.txt:1:")
}
