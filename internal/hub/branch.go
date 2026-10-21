package hub

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"compass/internal/config"
)

const branchUsage = "usage: compass hub branch [-h] [--hub DIR] [--prune]\n"

const branchHelp = branchUsage + `
Open the hub worktree for the code branch you are on (plan decision 8). Run it
inside a clone of a repo that the hub's repos.txt lists as <id>: on branch <b>
it prints the absolute path of a worktree of the hub on branch <id>/<b>.
Read and write knowledge there, then commit, push and open a hub pull request.

The repo is found by matching the clone's origin, as configured and then as
rewritten by url.<base>.insteadOf, against repos.txt on the hub's fetched
default branch, never its working tree. The hub branch is the local <id>/<b>
when one exists, else the hub remote's <id>/<b>, tracked, else a new branch
from the hub's default head. Worktrees live in <hub-clone>.worktrees/<id>/<b>,
one per hub branch. It uses git only: it fetches the hub and never pushes.

With --prune it does not look at the current clone. It removes each worktree
in <hub-clone>.worktrees whose hub branch is gone from the hub remote, whose
tree is clean with no ignored file either, and whose branch has no commit the
hub's default branch lacks, together with its local branch; it keeps every
other one. It prints each worktree it removes and each one it keeps, with the
reason.

options:
  -h, --help  show this help message and exit
  --hub DIR   hub clone (default: the hub clone that ` + "`compass setup`" + ` recorded)
  --prune     remove finished worktrees instead of opening one

Exit: 0 on success, 1 on a refusal or failure (the cause is named), 2 on a
usage error.
`

// BranchMain runs `compass hub branch`.
func BranchMain(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("compass hub branch", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	hubFlag := fs.String("hub", "", "")
	prune := fs.Bool("prune", false, "")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, branchHelp)
			return 0
		}
		fmt.Fprint(stderr, branchUsage)
		fmt.Fprintf(stderr, "compass hub branch: error: %v\n", err)
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprint(stderr, branchUsage)
		fmt.Fprintf(stderr, "compass hub branch: error: unrecognized arguments: %s\n", strings.Join(fs.Args(), " "))
		return 2
	}
	fail := func(err error) int {
		fmt.Fprintf(stderr, "hub branch: %v\n", err)
		return 1
	}
	hub, err := hubClone(*hubFlag)
	if err != nil {
		return fail(err)
	}
	if *prune {
		ok, err := pruneWorktrees(hub, stdout)
		if err != nil {
			return fail(err)
		}
		if !ok {
			return 1
		}
		return 0
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fail(err)
	}
	path, err := openBranch(hub, cwd)
	if err != nil {
		return fail(err)
	}
	fmt.Fprintln(stdout, path)
	return 0
}

// hubClone returns the top folder of the hub clone given by --hub, else the
// one `compass setup` recorded.
func hubClone(flagDir string) (string, error) {
	dir := flagDir
	if dir == "" {
		d, err := config.HubDir(os.Getenv)
		if err != nil {
			return "", err
		}
		dir = d
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	if st, err := os.Stat(abs); err != nil || !st.IsDir() {
		return "", fmt.Errorf("hub folder %s is not a directory", abs)
	}
	top, err := gitOut(abs, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("hub folder %s is not a git clone: %v", abs, err)
	}
	return top, nil
}

// worktreeRoot is the folder next to the hub clone that holds its worktrees.
func worktreeRoot(hub string) string { return filepath.Clean(hub) + ".worktrees" }

// worktreePath is where the worktree of hub branch <id>/<b> lives: the
// branch name's slashes become folders. Two branches that can coexist never
// share a folder or nest, because git refuses a ref that is a path prefix of
// another.
func worktreePath(hub, hubBranch string) string {
	return filepath.Join(worktreeRoot(hub), filepath.FromSlash(hubBranch))
}

// openBranch maps the clone at dir to its repos.txt row and returns the path
// of the hub worktree for its current branch, creating it when needed.
func openBranch(hub, dir string) (string, error) {
	code, err := gitOut(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("%s is not inside a git clone of a code repo: %v", dir, err)
	}
	head, err := gitOut(code, "symbolic-ref", "-q", "HEAD")
	b, onBranch := strings.CutPrefix(head, "refs/heads/")
	if err != nil || !onBranch || b == "" {
		return "", fmt.Errorf("%s has a detached HEAD: switch to the branch you are working on", code)
	}
	cands, origin := config.OriginRepos(code)
	if origin == "" {
		return "", fmt.Errorf("%s has no origin remote", code)
	}
	if err := config.Fetch(hub); err != nil {
		return "", fmt.Errorf("cannot fetch the hub clone %s: %v", hub, err)
	}
	hubDefault, err := config.RemoteDefault(hub)
	if err != nil {
		return "", fmt.Errorf("hub clone %s: %v", hub, err)
	}
	repos, err := config.HubRepos(hub, hubDefault)
	if err != nil {
		return "", err
	}
	row := matchRow(repos, cands)
	if row == nil {
		return "", fmt.Errorf("origin %s is not listed in repos.txt on origin/%s of the hub clone %s", origin, hubDefault, hub)
	}
	codeDefault, err := config.RemoteDefault(code)
	if err != nil {
		return "", fmt.Errorf("cannot find the default branch of %s: %v", row.FullName(), err)
	}
	if b == codeDefault {
		return "", fmt.Errorf("%s is %s's default branch: read its knowledge in the hub clone %s, and write through an ordinary hub branch by switching to a work branch first", b, row.FullName(), hub)
	}
	hb := row.Name + "/" + b

	wts, err := worktrees(hub)
	if err != nil {
		return "", err
	}
	for _, w := range wts {
		if w.branch == hb && !w.prunable {
			return w.path, nil
		}
	}
	refs, err := gitOut(hub, "for-each-ref", "--format=%(refname)", "refs/heads/", "refs/remotes/origin/")
	if err != nil {
		return "", err
	}
	local, remote, conflict := branchRefs(strings.Split(refs, "\n"), hb)
	if conflict != "" {
		return "", fmt.Errorf("hub ref %s conflicts with hub branch %s: git cannot hold both; finish or delete %s first", conflict, hb, conflict)
	}
	path := worktreePath(hub, hb)
	if _, err := os.Lstat(path); err == nil {
		return "", fmt.Errorf("%s already exists and is not a worktree of %s on %s: move it away", path, hub, hb)
	}
	for _, w := range wts {
		if w.prunable {
			// A worktree whose folder was deleted still holds its branch
			// until git forgets it.
			if _, err := gitOut(hub, "worktree", "prune"); err != nil {
				return "", err
			}
			break
		}
	}
	switch {
	case local:
		_, err = gitOut(hub, "worktree", "add", "--quiet", path, hb)
	case remote:
		_, err = gitOut(hub, "worktree", "add", "--quiet", "--track", "-b", hb, path, "refs/remotes/origin/"+hb)
	default:
		if _, err = gitOut(hub, "worktree", "add", "--quiet", "--no-track", "-b", hb, path, "refs/remotes/origin/"+hubDefault); err == nil {
			// Upstream <id>/<b> on origin, so a plain `git push` publishes it.
			if _, err = gitOut(hub, "config", "branch."+hb+".remote", "origin"); err == nil {
				_, err = gitOut(hub, "config", "branch."+hb+".merge", "refs/heads/"+hb)
			}
		}
	}
	if err != nil {
		return "", err
	}
	if wts, err = worktrees(hub); err != nil {
		return "", err
	}
	for _, w := range wts {
		if w.branch == hb {
			return w.path, nil
		}
	}
	return "", fmt.Errorf("git created no worktree of %s on %s", hub, hb)
}

// matchRow returns the repos.txt row naming the first of the origin's
// OWNER/REPO candidates, compared case-insensitively as GitHub does.
func matchRow(repos *config.Repos, cands []string) *config.Repo {
	for _, c := range cands {
		for _, r := range repos.List {
			if strings.EqualFold(r.FullName(), c) {
				return r
			}
		}
	}
	return nil
}

// branchRefs reports whether hub branch hb exists locally and on origin, given
// the hub's refs under refs/heads/ and refs/remotes/origin/, and names the
// first ref that blocks it: one whose name is a path prefix of hb, such as
// <id>/feat for <id>/feat/x or a branch named <id>, or one under hb/. A
// remote one blocks the push. origin/HEAD names no remote branch.
func branchRefs(refs []string, hb string) (local, remote bool, conflict string) {
	for _, ref := range refs {
		name, isLocal := strings.CutPrefix(ref, "refs/heads/")
		shown := name
		if !isLocal {
			var ok bool
			if name, ok = strings.CutPrefix(ref, "refs/remotes/origin/"); !ok || name == "HEAD" {
				continue
			}
			shown = "origin/" + name
		}
		switch {
		case name == hb && isLocal:
			local = true
		case name == hb:
			remote = true
		case conflict == "" && (strings.HasPrefix(hb, name+"/") || strings.HasPrefix(name, hb+"/")):
			conflict = shown
		}
	}
	return local, remote, conflict
}

type worktree struct {
	path, branch     string
	prunable, locked bool
}

// worktrees lists the hub's worktrees as `git worktree list --porcelain`
// reports them; branch is "" for a detached one.
func worktrees(hub string) ([]worktree, error) {
	out, err := gitOut(hub, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	var list []worktree
	for _, block := range strings.Split(out, "\n\n") {
		var w worktree
		for _, l := range strings.Split(block, "\n") {
			key, val, _ := strings.Cut(l, " ")
			switch key {
			case "worktree":
				w.path = val
			case "branch":
				w.branch = strings.TrimPrefix(val, "refs/heads/")
			case "prunable":
				w.prunable = true
			case "locked":
				w.locked = true
			}
		}
		if w.path != "" {
			list = append(list, w)
		}
	}
	return list, nil
}

// pruneWorktrees removes each worktree under the hub's worktree folder that
// holds no work: its branch is gone from the hub remote, its tree is clean
// with no ignored file either, and its branch has no commit the remote
// default branch lacks. Its local branch goes with it, so a later hub branch
// it would block can be made.
// Every other worktree is kept and printed with the reason. ok is false when
// a removal failed.
func pruneWorktrees(hub string, stdout io.Writer) (ok bool, err error) {
	if err := config.Fetch(hub); err != nil {
		return false, fmt.Errorf("cannot fetch the hub clone %s: %v", hub, err)
	}
	hubDefault, err := config.RemoteDefault(hub)
	if err != nil {
		return false, fmt.Errorf("hub clone %s: %v", hub, err)
	}
	if _, err := gitOut(hub, "worktree", "prune"); err != nil {
		return false, err
	}
	wts, err := worktrees(hub)
	if err != nil {
		return false, err
	}
	root := worktreeRoot(hub) + string(filepath.Separator)
	ok = true
	for _, w := range wts {
		if !strings.HasPrefix(w.path, root) {
			continue
		}
		reason, err := keepReason(hub, hubDefault, w)
		if err != nil {
			return false, err
		}
		if reason != "" {
			fmt.Fprintf(stdout, "kept %s: %s\n", w.path, reason)
			continue
		}
		if _, err := gitOut(hub, "worktree", "remove", w.path); err != nil {
			fmt.Fprintf(stdout, "kept %s: cannot remove it: %v\n", w.path, err)
			ok = false
			continue
		}
		if _, err := gitOut(hub, "branch", "-D", "--quiet", w.branch); err != nil {
			fmt.Fprintf(stdout, "removed %s, but kept its branch %s: %v\n", w.path, w.branch, err)
			ok = false
			continue
		}
		removeEmptyParents(filepath.Dir(w.path), worktreeRoot(hub))
		fmt.Fprintf(stdout, "removed %s: %s is not on the hub remote and holds no uncommitted or unpushed work\n", w.path, w.branch)
	}
	return ok, nil
}

// keepReason says why --prune keeps worktree w, or "" when it may go.
func keepReason(hub, hubDefault string, w worktree) (string, error) {
	switch {
	case w.locked:
		return "it is locked", nil
	case w.branch == "":
		return "it has a detached HEAD", nil
	}
	if _, err := gitOut(hub, "rev-parse", "--verify", "--quiet", "refs/remotes/origin/"+w.branch); err == nil {
		return w.branch + " is still on the hub remote", nil
	}
	// Ignored files count too: `git worktree remove` deletes them, and they
	// may be work hidden by a global excludes file or a nested .gitignore.
	status, err := gitOut(w.path, "status", "--porcelain", "--untracked-files=normal", "--ignored")
	if err != nil {
		return "", err
	}
	for _, l := range strings.Split(status, "\n") {
		if l != "" && !strings.HasPrefix(l, "!! ") {
			return "it has uncommitted changes", nil
		}
	}
	if status != "" {
		return "it has ignored files, which removing it would delete", nil
	}
	n, err := gitOut(hub, "rev-list", "--count", "refs/remotes/origin/"+hubDefault+"..refs/heads/"+w.branch)
	if err != nil {
		return "", err
	}
	if c, _ := strconv.Atoi(n); c > 0 {
		return fmt.Sprintf("%s has %d commit(s) not on origin/%s", w.branch, c, hubDefault), nil
	}
	return "", nil
}

// removeEmptyParents removes dir and its parents up to, not including, root
// while they are empty, so a removed <id>/feat/x leaves no empty <id>/feat.
func removeEmptyParents(dir, root string) {
	for dir != root && strings.HasPrefix(dir, root+string(filepath.Separator)) {
		if os.Remove(dir) != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}

// gitOut runs git in dir with prompts off and returns its trimmed stdout. A
// failure names the git command and its stderr.
func gitOut(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(errOut.String()); msg != "" {
			return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
		}
		return "", fmt.Errorf("git %s: %v", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(out.String()), nil
}
