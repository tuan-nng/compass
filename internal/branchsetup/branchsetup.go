// Package branchsetup implements `compass branch setup`: set up branch-mode
// OKF knowledge in one clone.
//
// Result: okf/ is a git worktree holding the knowledge branch that pairs with
// the current code branch (okf/main for the default branch, okf/<b> for <b>).
// Two local hooks keep it paired. Nothing is committed to the code branches.
// Safe to run again.
//
// Needs git 2.28 or newer (the reference-transaction hook), git 2.42 or newer
// when --init has to create okf/main (git worktree add --orphan), and a remote
// named origin whose HEAD is known.
//
// It refuses, before changing anything, when git is too old, there is no remote
// named origin, origin/HEAD is unknown, core.hooksPath is set or a hook already
// exists that is not ours (both unless --no-hooks), or a top-level okf path
// exists that is not this clone's okf worktree. After fetching origin, it
// refuses when there is no okf/main and --init was not given.
//
// For tests only: OKF_GIT_VERSION="git version 2.41.0" replaces the output of
// `git --version` in the version checks.
package branchsetup

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const usage = `Usage: compass branch setup [--init] [--no-hooks]
       compass branch setup --print-hook post-checkout|reference-transaction
  --init        create the okf/main branch; only the first person to set up a repo needs it
  --no-hooks    skip installing the two hooks into .git/hooks, for a repo whose hook
                manager (husky, lefthook) runs them; see templates/hooks/ in compass
  --print-hook  print one hook script to stdout, for adding it to a hook manager
`

// exitError ends the run with a code; msg, when set, goes to stderr.
type exitError struct {
	code int
	msg  string
}

func (e *exitError) Error() string { return e.msg }

func die(parts ...string) error { return &exitError{code: 1, msg: strings.Join(parts, " ")} }

// Main runs `compass branch setup`.
func Main(args []string, stdout, stderr io.Writer) int {
	initTrunk, hooks := false, true
	for i := range args {
		switch args[i] {
		case "--init":
			initTrunk = true
		case "--no-hooks":
			hooks = false
		case "--print-hook":
			next := ""
			if i+1 < len(args) {
				next = args[i+1]
			}
			switch next {
			case "post-checkout":
				io.WriteString(stdout, hookPostCheckout)
				return 0
			case "reference-transaction":
				io.WriteString(stdout, hookReferenceTransaction)
				return 0
			}
			fmt.Fprintln(stderr, "--print-hook needs post-checkout or reference-transaction")
			return 1
		case "-h", "--help":
			io.WriteString(stdout, usage)
			return 0
		default:
			fmt.Fprintf(stderr, "unknown argument: %s (see --help)\n", args[i])
			return 1
		}
	}
	s := &setup{stdout: stdout, stderr: stderr}
	if err := s.run(initTrunk, hooks); err != nil {
		var e *exitError
		if errors.As(err, &e) {
			if e.msg != "" {
				fmt.Fprintln(stderr, e.msg)
			}
			return e.code
		}
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

type setup struct {
	stdout, stderr io.Writer
	top            string
}

// git runs git in dir (the top level when empty), passing its stderr through,
// and returns trimmed stdout. A failure becomes an exitError with git's code.
func (s *setup) git(dir string, args ...string) (string, error) {
	return s.cmd(dir, s.stderr, args...)
}

// gitQuiet is git with stderr discarded.
func (s *setup) gitQuiet(dir string, args ...string) (string, error) {
	return s.cmd(dir, io.Discard, args...)
}

func (s *setup) cmd(dir string, stderr io.Writer, args ...string) (string, error) {
	if dir == "" {
		dir = s.top
	}
	c := exec.Command("git", args...)
	c.Dir = dir
	var out bytes.Buffer
	c.Stdout = &out
	c.Stderr = stderr
	err := c.Run()
	return strings.TrimRight(out.String(), "\n"), exitCode(err)
}

// gitPass runs git with stdout and stderr passed through.
func (s *setup) gitPass(args ...string) error {
	c := exec.Command("git", args...)
	c.Dir = s.top
	c.Stdout, c.Stderr = s.stdout, s.stderr
	return exitCode(c.Run())
}

func exitCode(err error) error {
	if err == nil {
		return nil
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return &exitError{code: ee.ExitCode()}
	}
	return err
}

func (s *setup) abs(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(s.top, p)
}

var versionRE = regexp.MustCompile(`^[^0-9]*([0-9]+)\.([0-9]+)`)

// gitVersionText is OKF_GIT_VERSION or the output of `git --version`.
func (s *setup) gitVersionText() string {
	if v := os.Getenv("OKF_GIT_VERSION"); v != "" {
		return v
	}
	out, _ := s.gitQuiet("", "--version")
	return out
}

// gitVersion returns major, minor from the first line that has them.
func (s *setup) gitVersion() (int, int, error) {
	text := s.gitVersionText()
	for _, line := range strings.Split(text, "\n") {
		if m := versionRE.FindStringSubmatch(line); m != nil {
			major, _ := strconv.Atoi(m[1])
			minor, _ := strconv.Atoi(m[2])
			return major, minor, nil
		}
	}
	return 0, 0, die("Cannot read the git version from: " + text)
}

func (s *setup) gitAtLeast(wantMajor, wantMinor int) (bool, string, error) {
	major, minor, err := s.gitVersion()
	if err != nil {
		return false, "", err
	}
	ok := major > wantMajor || (major == wantMajor && minor >= wantMinor)
	return ok, fmt.Sprintf("%d.%d", major, minor), nil
}

func exists(p string) bool { _, err := os.Lstat(p); return err == nil }

func realDir(p string) (string, error) { return filepath.EvalSymlinks(p) }

// isOKFWorktree reports whether ./okf is this clone's own knowledge worktree.
func (s *setup) isOKFWorktree() bool {
	okf := filepath.Join(s.top, "okf")
	fi, err := os.Lstat(okf)
	if err != nil || fi.Mode()&os.ModeSymlink != 0 || !fi.IsDir() || !exists(filepath.Join(okf, ".git")) {
		return false
	}
	if t, err := s.gitQuiet(okf, "rev-parse", "--show-toplevel"); err != nil || t != okf {
		return false
	}
	hereRaw, err := s.gitQuiet("", "rev-parse", "--git-common-dir")
	if err != nil {
		return false
	}
	here, err := realDir(s.abs(hereRaw))
	if err != nil {
		return false
	}
	thereRaw, err := s.gitQuiet(okf, "rev-parse", "--git-common-dir")
	if err != nil {
		return false
	}
	if !filepath.IsAbs(thereRaw) {
		thereRaw = filepath.Join(okf, thereRaw)
	}
	there, err := realDir(thereRaw)
	if err != nil || here != there {
		return false
	}
	ref, err := s.gitQuiet(okf, "symbolic-ref", "-q", "HEAD")
	if err != nil {
		return true // detached
	}
	return strings.HasPrefix(ref, "refs/heads/okf/")
}

func (s *setup) run(initTrunk, hooks bool) error {
	top, err := s.cmd(".", s.stderr, "rev-parse", "--show-toplevel")
	if err != nil {
		return err
	}
	s.top = top

	// 0. Checks that change nothing.
	ok, ver, err := s.gitAtLeast(2, 28)
	if err != nil {
		return err
	}
	if !ok {
		return die("git "+ver+" is too old: branch mode needs git 2.28 or newer",
			"for the reference-transaction hook. Upgrade git, then re-run.")
	}
	if hooks {
		if hp, _ := s.gitQuiet("", "config", "core.hooksPath"); hp != "" {
			return die("core.hooksPath is set, so git ignores .git/hooks. Add the two hooks",
				"from this script to that hook manager by hand (see --print-hook),",
				"then re-run with --no-hooks.")
		}
	}
	if _, err := s.gitQuiet("", "remote", "get-url", "origin"); err != nil {
		return die("This clone has no remote named origin. Branch mode needs it; rename the",
			"remote with: git remote rename <name> origin")
	}
	def, err := s.git("", "symbolic-ref", "-q", "--short", "refs/remotes/origin/HEAD")
	if err != nil {
		return die("origin/HEAD is unknown; run: git remote set-head origin --auto")
	}
	okf := filepath.Join(top, "okf")
	if exists(okf) {
		if !s.isOKFWorktree() {
			return die(okf+" already exists and is not this clone's okf worktree.",
				"Branch mode needs that path: move it away first. If the code branches",
				"hold the bundle in okf/, this repo uses folder mode, not branch mode.")
		}
	} else if tracked, err := s.git("", "ls-files", "--", "okf"); err != nil {
		return err
	} else if tracked != "" {
		return die("okf is tracked on this code branch, so this repo uses folder mode, not branch mode.")
	}
	common, err := s.git("", "rev-parse", "--git-common-dir")
	if err != nil {
		return err
	}
	hooksDir := common + "/hooks"
	if hooks {
		for _, h := range []string{"post-checkout", "reference-transaction"} {
			p := s.abs(hooksDir + "/" + h)
			if !exists(p) {
				continue
			}
			if b, err := os.ReadFile(p); err != nil || !bytes.Contains(b, []byte("okf-branch-hook")) {
				return die(hooksDir+"/"+h+" already exists. Merge the okf hook into it by hand",
					"(see --print-hook), then re-run with --no-hooks.")
			}
		}
	}

	// 1. Check out the knowledge trunk at okf/ and hide it from the code branches.
	if _, err := s.gitQuiet("", "fetch", "-q", "origin"); err != nil {
		fmt.Fprintln(s.stderr, "Could not fetch origin; using local refs.")
	}
	if err := s.gitPass("worktree", "prune"); err != nil { // forget an okf/ folder deleted by hand
		return err
	}
	if !exists(okf) {
		has := func(ref string) bool {
			_, err := s.gitQuiet("", "show-ref", "-q", "--verify", ref)
			return err == nil
		}
		switch {
		case has("refs/heads/okf/main"):
			err = s.gitPass("worktree", "add", "-q", "okf", "okf/main")
		case has("refs/remotes/origin/okf/main"):
			err = s.gitPass("worktree", "add", "-q", "--track", "-b", "okf/main", "okf", "origin/okf/main")
		case initTrunk:
			err = s.createTrunk(okf)
		default:
			err = die("This repo has no okf/main branch yet. Re-run with --init to create it.")
		}
		if err != nil {
			return err
		}
	}
	if err := s.gitPass("config", "okf.defaultBranch", strings.TrimPrefix(def, "origin/")); err != nil {
		return err
	}
	excludeRaw, err := s.git("", "rev-parse", "--git-path", "info/exclude")
	if err != nil {
		return err
	}
	exclude := s.abs(excludeRaw)
	if err := os.MkdirAll(filepath.Dir(exclude), 0o777); err != nil {
		return err
	}
	if !hasLine(exclude, "/okf/") { // top level only
		f, err := os.OpenFile(exclude, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o666)
		if err != nil {
			return err
		}
		_, err = f.WriteString("/okf/\n")
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return err
		}
	}

	// 2. Install the hooks; step 0 refused hooks that are not ours.
	if hooks {
		dir := s.abs(hooksDir)
		if err := os.MkdirAll(dir, 0o777); err != nil {
			return err
		}
		for name, body := range map[string]string{
			"post-checkout":         hookPostCheckout,
			"reference-transaction": hookReferenceTransaction,
		} {
			p := filepath.Join(dir, name)
			if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
				return err
			}
			if err := os.Chmod(p, 0o755); err != nil {
				return err
			}
		}
	}

	// 3. Pair okf/ with the branch that is checked out now.
	c := exec.Command("sh", "-c", strings.TrimRight(hookPostCheckout, "\n"), "post-checkout", "x", "x", "1")
	c.Dir = top
	c.Stdout, c.Stderr = s.stdout, s.stderr
	return exitCode(c.Run())
}

// createTrunk creates okf/main as an orphan worktree with a root index.md.
func (s *setup) createTrunk(okf string) error {
	ok, ver, err := s.gitAtLeast(2, 42)
	if err != nil {
		return err
	}
	if !ok {
		return die("--init needs git 2.42 or newer to create okf/main (git worktree add",
			"--orphan); this is git "+ver+". Upgrade git, or ask",
			"someone with a newer git to run --init and push okf/main.")
	}
	if err := s.gitPass("worktree", "add", "-q", "--orphan", "-b", "okf/main", "okf"); err != nil {
		return err
	}
	index := fmt.Sprintf("---\nokf_version: \"0.2\"\n---\n# %s\n", filepath.Base(s.top))
	if err := os.WriteFile(filepath.Join(okf, "index.md"), []byte(index), 0o666); err != nil {
		return err
	}
	if _, err := s.git(okf, "add", "index.md"); err != nil {
		return err
	}
	if err := s.gitPassIn(okf, "commit", "-q", "-m", "okf: create knowledge branch"); err != nil {
		return err
	}
	fmt.Fprintln(s.stdout, "Created okf/main. Publish it with: git -C okf push -u origin okf/main")
	return nil
}

func (s *setup) gitPassIn(dir string, args ...string) error {
	c := exec.Command("git", args...)
	c.Dir = dir
	c.Stdout, c.Stderr = s.stdout, s.stderr
	return exitCode(c.Run())
}

func hasLine(path, line string) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	for _, l := range strings.Split(string(b), "\n") {
		if l == line {
			return true
		}
	}
	return false
}
