package ghfake

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The hub every test hub's origin names, and its API path.
const (
	HubOwner = "acme"
	HubName  = "hub"
	HubPath  = "/repos/" + HubOwner + "/" + HubName
)

// HubDate is the author and committer date of every commit a test hub makes,
// so the 7-day rule reads a known age.
const HubDate = "2026-10-04T08:59:00Z"

// Hub is a real hub for tests: Dir is a clone whose origin is
// https://github.com/acme/hub, the form `compass setup` records, fetched from
// the bare repo Remote through a url.<base>.insteadOf rewrite in a per-test
// git config (plan decisions 7 and 12). Commit pushes branches to Remote from
// a second clone, so Dir only sees them once the job under test fetches, as a
// user's hub clone would.
type Hub struct {
	t      testing.TB
	Dir    string
	Remote string
	work   string
}

// WriteHub builds a hub whose default branch, main, holds only repos.txt and
// checks.txt with the given text.
func WriteHub(t testing.TB, repos, checks string) *Hub {
	t.Helper()
	return newHub(t, func(work string) {
		for name, text := range map[string]string{"repos.txt": repos, "checks.txt": checks} {
			if err := os.WriteFile(filepath.Join(work, name), []byte(text), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	})
}

// TemplateHub builds a hub whose main holds the shipped hub template
// (templates/hub/) with one row appended to its repos.txt and checks.txt
// under their headers. The template itself is never written.
func TemplateHub(t testing.TB, reposRow, checksRow string) *Hub {
	t.Helper()
	src := filepath.Join(moduleRoot(t), "templates", "hub")
	return newHub(t, func(work string) {
		if err := os.CopyFS(work, os.DirFS(src)); err != nil {
			t.Fatal(err)
		}
		for name, row := range map[string]string{"repos.txt": reposRow, "checks.txt": checksRow} {
			f, err := os.OpenFile(filepath.Join(work, name), os.O_APPEND|os.O_WRONLY, 0)
			if err != nil {
				t.Fatal(err)
			}
			_, err = f.WriteString(row + "\n")
			if cerr := f.Close(); err == nil {
				err = cerr
			}
			if err != nil {
				t.Fatal(err)
			}
		}
	})
}

// moduleRoot is the nearest folder above the test's folder that holds go.mod;
// tests run in their package folder.
func moduleRoot(t testing.TB) string {
	t.Helper()
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			return root
		}
		parent := filepath.Dir(root)
		if parent == root {
			t.Fatal("no go.mod above the test's folder")
		}
		root = parent
	}
}

// newHub isolates git for the test, creates the bare remote, commits what
// fill writes as main, and clones Dir.
func newHub(t testing.TB, fill func(work string)) *Hub {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	remotes := filepath.Join(root, "remotes")
	cfg := filepath.Join(root, "gitconfig")
	text := "[user]\n\tname = test\n\temail = test@example.com\n" +
		"[init]\n\tdefaultBranch = main\n" +
		"[commit]\n\tgpgsign = false\n" +
		"[protocol \"file\"]\n\tallow = always\n" +
		"[url \"file://" + remotes + "/\"]\n\tinsteadOf = https://github.com/\n"
	if err := os.WriteFile(cfg, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", cfg)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	url := "https://github.com/" + HubOwner + "/" + HubName
	h := &Hub{
		t:      t,
		Dir:    filepath.Join(root, "hub"),
		Remote: filepath.Join(remotes, HubOwner, HubName),
		work:   filepath.Join(root, "work"),
	}
	h.Git(root, "init", "-q", "--bare", "-b", "main", h.Remote)
	h.Git(root, "clone", "-q", url, h.work)
	fill(h.work)
	h.Git(h.work, "add", "-A")
	h.Git(h.work, "commit", "-q", "-m", "hub")
	h.Git(h.work, "push", "-q", "origin", "HEAD:refs/heads/main")
	h.Git(root, "clone", "-q", url, h.Dir)
	return h
}

// Git runs git in dir, with the hub's commit date, and returns its trimmed
// stdout; a failure fails the test.
func (h *Hub) Git(dir string, args ...string) string {
	h.t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE="+HubDate, "GIT_COMMITTER_DATE="+HubDate, "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.Output()
	if err != nil {
		msg := ""
		if ee, ok := err.(*exec.ExitError); ok {
			msg = string(ee.Stderr)
		}
		h.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, msg)
	}
	return strings.TrimSpace(string(out))
}

// Commit makes one commit on branch in the work clone and pushes it to the
// remote, returning its SHA. The commit starts from origin's from, a branch
// name, or from origin's branch itself when from is "". change edits the work
// clone's files; everything it leaves is committed.
func (h *Hub) Commit(branch, from string, change func(work string)) string {
	h.t.Helper()
	if from == "" {
		from = branch
	}
	h.Git(h.work, "checkout", "-q", "--detach", "refs/remotes/origin/"+from)
	change(h.work)
	h.Git(h.work, "add", "-A")
	h.Git(h.work, "commit", "-q", "--allow-empty", "-m", "test: "+branch)
	sha := h.Git(h.work, "rev-parse", "HEAD")
	h.Git(h.work, "push", "-q", "-f", "origin", "HEAD:refs/heads/"+branch)
	return sha
}

// Merge merges branch into into on the remote, with a merge commit, as
// GitHub's merge of a pull request does, and returns the new head of into.
func (h *Hub) Merge(branch, into string) string {
	h.t.Helper()
	h.Git(h.work, "checkout", "-q", "--detach", "refs/remotes/origin/"+into)
	h.Git(h.work, "merge", "-q", "--no-ff", "--no-edit", "refs/remotes/origin/"+branch)
	sha := h.Git(h.work, "rev-parse", "HEAD")
	h.Git(h.work, "push", "-q", "origin", "HEAD:refs/heads/"+into)
	return sha
}

// Head returns branch's commit on the remote.
func (h *Hub) Head(branch string) string {
	h.t.Helper()
	return h.Git(h.Remote, "rev-parse", "refs/heads/"+branch)
}
