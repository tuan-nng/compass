// Package setup implements `compass setup`: it connects one machine to the
// end user's knowledge hub (plan decisions 3 and 9). Developer laptops, CI
// runners and cloud agent images all run it; CI passes flags instead of
// answering prompts.
package setup

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"compass"
	"compass/internal/config"
	"compass/internal/okfinstall"
)

const usageText = `usage: compass setup [--hub OWNER/REPO] [--hub-dir DIR] [--yes]
                     [--prefix DIR] [--agents LIST] [--okf-bin FILE]

Set up this machine for OKF knowledge. It asks for your hub repo and the
folder to clone it into (flags skip the questions), checks that the hub is
readable with your git credentials and that the folder is free or already
holds a clone of it, and stops before writing anything if not. Then it:
  - installs the pinned okf into DIR/bin (default DIR: ~/.local);
  - clones the hub into its folder, or reuses the clone already there;
  - records the hub and its folder in $XDG_CONFIG_HOME/compass/config
    (default ~/.config/compass/config); ` + "`compass hub assemble`" + `, ` + "`hub check`" + `,
    ` + "`stamp`" + ` and ` + "`sync`" + ` use that folder when given none;
  - installs the OKF skill into each agent's user-level skill folder:
      claude  ${CLAUDE_CONFIG_DIR:-~/.claude}/skills/okf
      cursor  ~/.cursor/skills/okf
      omp     ${PI_CODING_AGENT_DIR:-~/.omp/agent}/skills/okf
Running it again changes nothing.

flags:
  --hub OWNER/REPO  your hub repo (default: the one already in the user config)
  --hub-dir DIR     folder for the hub clone (default: the recorded folder of
                    the same hub, else ~/src/<repo>)
  --yes             ask nothing; take the flags and defaults (for CI and cloud agents)
  --prefix DIR      install okf into DIR/bin (default: ~/.local)
  --agents LIST     comma-separated agents to give the skill to: claude, cursor, omp
                    (default: each agent whose folder exists on this machine)
  --okf-bin FILE    copy this okf binary instead of downloading the pinned
                    release; for tests and offline machines, not checksummed
`

var knownAgents = []string{"claude", "cursor", "omp"}

// agentRoot is the agent's user-level folder; its skills live below it.
func agentRoot(agent string, getenv func(string) string) string {
	home := getenv("HOME")
	switch agent {
	case "claude":
		if d := getenv("CLAUDE_CONFIG_DIR"); d != "" {
			return d
		}
		return filepath.Join(home, ".claude")
	case "cursor":
		return filepath.Join(home, ".cursor")
	case "omp":
		if d := getenv("PI_CODING_AGENT_DIR"); d != "" {
			return d
		}
		return filepath.Join(home, ".omp", "agent")
	}
	return ""
}

type options struct {
	hub, hubDir, prefix, agents, okfBin string
	yes                                 bool
	hubSet, hubDirSet, agentsSet        bool
}

// Main runs `compass setup`.
func Main(args []string, stdout, stderr io.Writer) int {
	return run(args, os.Stdin, stdout, stderr, os.Getenv)
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer, getenv func(string) string) int {
	fs := flag.NewFlagSet("compass setup", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, usageText) }
	var o options
	fs.StringVar(&o.hub, "hub", "", "")
	fs.StringVar(&o.hubDir, "hub-dir", "", "")
	fs.BoolVar(&o.yes, "yes", false, "")
	fs.StringVar(&o.prefix, "prefix", "", "")
	fs.StringVar(&o.agents, "agents", "", "")
	fs.StringVar(&o.okfBin, "okf-bin", "", "")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, usageText)
			return 0
		}
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "setup: unknown argument: %s\n", fs.Arg(0))
		return 2
	}
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "hub":
			o.hubSet = true
		case "hub-dir":
			o.hubDirSet = true
		case "agents":
			o.agentsSet = true
		}
	})
	if err := setup(o, stdin, stdout, stderr, getenv); err != nil {
		fmt.Fprintf(stderr, "setup: %v\n", err)
		return 1
	}
	return 0
}

func setup(o options, stdin io.Reader, stdout, stderr io.Writer, getenv func(string) string) error {
	home := getenv("HOME")
	if home == "" {
		return errors.New("HOME is not set")
	}
	cfgPath := config.UserConfigPath(getenv)
	current, err := config.UserConfig(getenv)
	if err != nil {
		return fmt.Errorf("%v; nothing written", err)
	}

	// Answers: flags first, then prompts (unless --yes), then defaults.
	in := bufio.NewReader(stdin)
	curHub, curDir := config.EnvValue(current, "OKF_HUB"), config.EnvValue(current, "OKF_HUB_DIR")
	if !o.hubSet {
		o.hub = curHub
		if !o.yes {
			o.hub = ask(in, stdout, "Your hub repo (owner/name)", curHub)
		}
	}

	// Check every input before changing anything.
	if o.hub == "" {
		return errors.New("no hub given: pass --hub OWNER/REPO; nothing written")
	}
	owner, repo, ok := config.ParseRepoURL("https://github.com/" + o.hub)
	if !ok {
		return fmt.Errorf("hub %q is not OWNER/REPO; nothing written", o.hub)
	}
	o.hub = owner + "/" + repo
	if !o.hubDirSet {
		def := filepath.Join(home, "src", repo)
		if curDir != "" && strings.EqualFold(curHub, o.hub) {
			def = curDir
		}
		o.hubDir = def
		if !o.yes {
			o.hubDir = ask(in, stdout, "Folder to clone it into", def)
		}
	}
	hubDir, err := absPath(o.hubDir, home)
	if err != nil {
		return fmt.Errorf("hub folder %q: %v; nothing written", o.hubDir, err)
	}
	agents, err := pickAgents(o, getenv)
	if err != nil {
		return err
	}
	if o.okfBin != "" {
		if st, err := os.Stat(o.okfBin); err != nil || !st.Mode().IsRegular() || st.Mode()&0o111 == 0 {
			return fmt.Errorf("--okf-bin %s is not an executable file; nothing written", o.okfBin)
		}
	}
	if err := checkHub(o.hub); err != nil {
		return fmt.Errorf("cannot read hub %s: %v; nothing written.\n"+
			"setup: git reads the hub, and `compass hub assemble` its repos, over https://github.com/;"+
			" give git credentials for it (for example `gh auth setup-git`) and run setup again", o.hub, err)
	}
	reuse, err := existingClone(hubDir, o.hub)
	if err != nil {
		return fmt.Errorf("%v; nothing written. Pass --hub-dir with an empty or missing folder, or a clone of %s", err, o.hub)
	}

	prefix := o.prefix
	if prefix == "" {
		prefix = filepath.Join(home, ".local")
	}
	bin := filepath.Join(prefix, "bin")

	// okf first: a failed download leaves nothing behind. The hub was read
	// above, so the clone after it rarely fails.
	if err := installOKF(bin, o, stdout, stderr, getenv); err != nil {
		return err
	}

	if reuse {
		fmt.Fprintf(stdout, "setup: reusing the clone of %s at %s\n", o.hub, hubDir)
	} else {
		if err := cloneHub(o.hub, hubDir); err != nil {
			return fmt.Errorf("cloning hub %s into %s: %v; config and skills not written", o.hub, hubDir, err)
		}
		fmt.Fprintf(stdout, "setup: cloned %s into %s\n", o.hub, hubDir)
	}

	want := fmt.Sprintf("# Written by compass setup.\nOKF_HUB=%s\nOKF_HUB_DIR=%s\n", o.hub, hubDir)
	if current == want {
		fmt.Fprintf(stdout, "setup: config at %s is current (hub %s at %s)\n", cfgPath, o.hub, hubDir)
	} else {
		if err := writeFileAtomic(cfgPath, []byte(want), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "setup: recorded hub %s at %s in %s\n", o.hub, hubDir, cfgPath)
	}

	for _, a := range agents {
		dest := filepath.Join(agentRoot(a, getenv), "skills", "okf")
		changed, err := installSkill(dest)
		if err != nil {
			return fmt.Errorf("installing the skill for %s: %v", a, err)
		}
		if changed {
			fmt.Fprintf(stdout, "setup: installed the okf skill for %s to %s\n", a, dest)
		} else {
			fmt.Fprintf(stdout, "setup: the okf skill for %s at %s is current\n", a, dest)
		}
	}
	if len(agents) == 0 {
		fmt.Fprintln(stdout, "setup: note: no agent folder found (claude, cursor, omp); pass --agents to install the skill")
	}

	path := ":" + getenv("PATH") + ":"
	if !strings.Contains(path, ":"+bin+":") {
		fmt.Fprintf(stdout, "setup: note: %s is not on PATH; add it so agents can run okf\n", bin)
	}
	if _, err := exec.LookPath("compass"); err != nil {
		fmt.Fprintln(stdout, "setup: note: compass is not on PATH; agents run `compass branch setup` and `compass hub assemble`")
	}
	return nil
}

func ask(in *bufio.Reader, out io.Writer, question, def string) string {
	if def != "" {
		fmt.Fprintf(out, "%s [%s]: ", question, def)
	} else {
		fmt.Fprintf(out, "%s: ", question)
	}
	line, _ := in.ReadString('\n')
	if line = strings.TrimSpace(line); line != "" {
		return line
	}
	return def
}

func pickAgents(o options, getenv func(string) string) ([]string, error) {
	if !o.agentsSet {
		var found []string
		for _, a := range knownAgents {
			if st, err := os.Stat(agentRoot(a, getenv)); err == nil && st.IsDir() {
				found = append(found, a)
			}
		}
		return found, nil
	}
	var out []string
	for _, a := range strings.Split(o.agents, ",") {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		if agentRoot(a, getenv) == "" {
			return nil, fmt.Errorf("unknown agent '%s' (known: claude, cursor, omp); nothing written", a)
		}
		out = append(out, a)
	}
	if len(out) == 0 {
		return nil, errors.New("--agents is empty; nothing written")
	}
	return out, nil
}

// checkHub reads the hub's refs with the caller's git credentials, never
// prompting for them.
func checkHub(hub string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "ls-remote", "https://github.com/"+hub+".git", "HEAD")
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var errOut bytes.Buffer
	cmd.Stderr = &errOut
	cmd.Stdout = io.Discard
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errOut.String())
		if msg == "" {
			msg = err.Error()
		}
		if i := strings.LastIndexByte(msg, '\n'); i >= 0 {
			msg = msg[i+1:]
		}
		return errors.New(msg)
	}
	return nil
}

// absPath expands a leading ~ to home and makes p absolute.
func absPath(p, home string) (string, error) {
	if p == "" {
		return "", errors.New("empty path")
	}
	if p == "~" {
		p = home
	} else if rest, ok := strings.CutPrefix(p, "~/"); ok {
		p = filepath.Join(home, rest)
	}
	return filepath.Abs(p)
}

// existingClone reports whether dir already holds a clone of hub, so setup
// reuses it. A missing or empty dir is free for a fresh clone; anything else
// is an error.
func existingClone(dir, hub string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) || (err == nil && len(entries) == 0) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("hub folder %s: %v", dir, err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return false, fmt.Errorf("hub folder %s is not empty and is not a git clone", dir)
	}
	// The configured URL first, before any url.<base>.insteadOf rewrite: a
	// user who maps https://github.com/ to ssh still has a clone of the same
	// hub. Then the rewritten one, for host aliases that map to GitHub.
	var origin string
	for _, args := range [][]string{{"config", "--get", "remote.origin.url"}, {"remote", "get-url", "origin"}} {
		out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
		if err != nil {
			continue
		}
		remote := withoutUserinfo(strings.TrimSpace(string(out)))
		if strings.EqualFold(githubRepo(remote), hub) {
			return true, nil
		}
		if origin == "" {
			origin = remote
		}
	}
	if origin == "" {
		origin = "no origin"
	}
	return false, fmt.Errorf("hub folder %s holds a clone of %s, not %s", dir, origin, hub)
}

// withoutUserinfo drops the user and password from a URL-form remote, so a
// token in it is neither matched on nor printed. scp-form remotes
// (git@host:path) carry no password and come back unchanged.
func withoutUserinfo(remote string) string {
	if u, err := url.Parse(remote); err == nil && u.Scheme != "" && u.User != nil {
		u.User = nil
		return u.String()
	}
	return remote
}

var originRe = regexp.MustCompile(`^(?:https://github\.com/|git@github\.com:|ssh://github\.com/)([^/]+/[^/]+?)(?:\.git)?/?$`)

// githubRepo returns OWNER/REPO for a GitHub https or ssh remote URL without
// userinfo, else "".
func githubRepo(remote string) string {
	if m := originRe.FindStringSubmatch(remote); m != nil {
		return m[1]
	}
	return ""
}

// cloneHub clones hub into dir over https with the caller's git credentials,
// never prompting for them, as checkHub reads it.
func cloneHub(hub, dir string) error {
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return err
	}
	cmd := exec.Command("git", "clone", "-q", "https://github.com/"+hub+".git", dir)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var errOut bytes.Buffer
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(errOut.String()); msg != "" {
			return errors.New(msg)
		}
		return err
	}
	return nil
}

func installOKF(bin string, o options, stdout, stderr io.Writer, getenv func(string) string) error {
	target := filepath.Join(bin, "okf")
	if o.okfBin != "" {
		src, err := os.ReadFile(o.okfBin)
		if err != nil {
			return err
		}
		if cur, err := os.ReadFile(target); err == nil && bytes.Equal(cur, src) {
			fmt.Fprintf(stdout, "setup: okf at %s is current\n", target)
			return nil
		}
		if err := writeFileAtomic(target, src, 0o755); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "setup: copied %s to %s\n", o.okfBin, target)
		return nil
	}
	// Download and check the pinned release on every run, as the installer
	// script did, but rewrite okf only when it differs.
	okf, label, err := okfinstall.Fetch(getenv, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "okf install: %v\n", err)
		return errors.New("okf install failed; config and skills not written")
	}
	if cur, err := os.ReadFile(target); err == nil && bytes.Equal(cur, okf) {
		fmt.Fprintf(stdout, "setup: %s at %s is current\n", label, target)
		return nil
	}
	if err := writeFileAtomic(target, okf, 0o755); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "setup: installed %s to %s\n", label, target)
	return nil
}

// installSkill makes dest hold exactly the embedded skill. It reports whether
// anything changed.
func installSkill(dest string) (bool, error) {
	same, err := sameAsEmbedded(dest)
	if err != nil {
		return false, err
	}
	if same {
		return false, nil
	}
	tmp := fmt.Sprintf("%s.tmp.%d", dest, os.Getpid())
	if err := os.RemoveAll(tmp); err != nil {
		return false, err
	}
	err = fs.WalkDir(compass.Skill, "skill/okf", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(strings.TrimPrefix(p, "skill/okf"), "/")
		target := filepath.Join(tmp, filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := compass.Skill.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		os.RemoveAll(tmp)
		return false, err
	}
	if err := os.RemoveAll(dest); err != nil {
		return false, err
	}
	return true, os.Rename(tmp, dest)
}

// sameAsEmbedded reports whether dest is a real folder holding exactly the
// embedded skill files. A symlink or file at dest is not the same, so
// installSkill replaces it with a real folder.
func sameAsEmbedded(dest string) (bool, error) {
	if st, err := os.Lstat(dest); err != nil || !st.IsDir() {
		return false, nil
	}
	want := map[string][]byte{}
	err := fs.WalkDir(compass.Skill, "skill/okf", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := compass.Skill.ReadFile(p)
		want[strings.TrimPrefix(p, "skill/okf/")] = data
		return err
	})
	if err != nil {
		return false, err
	}
	got := map[string][]byte{}
	err = filepath.WalkDir(dest, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dest, p)
		data, err := os.ReadFile(p)
		got[filepath.ToSlash(rel)] = data
		return err
	})
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if len(got) != len(want) {
		return false, nil
	}
	for k, v := range want {
		if !bytes.Equal(got[k], v) {
			return false, nil
		}
	}
	return true, nil
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := fmt.Sprintf("%s.tmp.%d", path, os.Getpid())
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return err
	}
	if err := os.Chmod(tmp, mode); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}
