// Package hub assembles a hub view from the repos listed in repos.txt and
// checks an assembled hub (plan Design "Interfaces").
package hub

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"compass/internal/config"
)

const assembleHelp = `Assemble the hub view: copy every repo bundle listed in <hub-dir>/repos.txt
into <hub-dir>/repos/<name>/, fetching each one from its git remote.

Usage: compass hub assemble [--ci | --local] [<hub-dir>]

<hub-dir> defaults to the hub clone that ` + "`compass setup`" + ` recorded.

repos.txt lines: <name> <git URL> <folder|branch>   (# comments allowed)
  folder: the okf/ folder of the remote's default branch
  branch: the root of the remote's okf/main branch (.git and .github skipped)
Only https://github.com/<owner>/<repo> URLs are accepted, for any owner; every
line is checked before anything is fetched.

A repo fails, and is named, when it cannot be fetched, its bundle has no
index.md or overview.md, or its bundle contains a symlink. In CI ($CI=true or
--ci) any failure makes the run exit non-zero. Locally (--local, or $CI
unset) a repo that cannot be fetched keeps its previous copy with a warning;
the other failures still exit non-zero.

Clone cache: $OKF_HUB_CACHE, else ${XDG_CACHE_HOME:-~/.cache}/okf-hub.
Each copied repo index.md loses its frontmatter: the spec allows frontmatter
only in the bundle-root index.md (research report, finding 3).
`

const me = "hub assemble"

// listed is one accepted repos.txt row.
type listed struct{ name, url, kind string }

var nameRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// AssembleMain runs `compass hub assemble`.
func AssembleMain(args []string, stdout, stderr io.Writer) int {
	die := func(format string, a ...any) int {
		fmt.Fprintf(stderr, me+": "+format+"\n", a...)
		return 2
	}
	mode := "local"
	if os.Getenv("CI") == "true" {
		mode = "ci"
	}
	hub := ""
	for _, a := range args {
		switch {
		case a == "--ci":
			mode = "ci"
		case a == "--local":
			mode = "local"
		case a == "-h" || a == "--help":
			fmt.Fprint(stdout, assembleHelp)
			return 0
		case strings.HasPrefix(a, "-"):
			return die("unknown option: %s", a)
		case hub != "":
			return die("only one hub dir allowed")
		default:
			hub = a
		}
	}
	if hub == "" {
		var err error
		if hub, err = config.HubDir(os.Getenv); err != nil {
			return die("%v", err)
		}
	}
	reposTxt := filepath.Join(hub, "repos.txt")
	if st, err := os.Stat(reposTxt); err != nil || !st.Mode().IsRegular() {
		return die("no repos.txt in %s", hub)
	}
	cache := os.Getenv("OKF_HUB_CACHE")
	if cache == "" {
		base := os.Getenv("XDG_CACHE_HOME")
		if base == "" {
			base = os.Getenv("HOME") + "/.cache"
		}
		cache = base + "/okf-hub"
	}

	// 1. Parse and check every line before fetching anything.
	repos, ok, err := parseRepos(reposTxt, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", me, err)
		return 1
	}
	if !ok {
		return 1
	}
	if len(repos) == 0 {
		fmt.Fprintf(stderr, "%s: repos.txt lists no repos\n", me)
		return 1
	}

	code, err := assemble(hub, cache, mode, repos, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", me, err)
		return 1
	}
	return code
}

// parseRepos reads repos.txt, printing one message per bad line; ok is false
// when any line is bad. Text from the first # on a line is a comment.
func parseRepos(path string, stderr io.Writer) (repos []listed, ok bool, err error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, false, err
	}
	ok = true
	bad := func(n int, format string, a ...any) {
		fmt.Fprintf(stderr, "%s: repos.txt:%d: %s\n", me, n, fmt.Sprintf(format, a...))
		ok = false
	}
	text := string(raw)
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return nil, true, nil
	}
lines:
	for i, line := range strings.Split(text, "\n") {
		n := i + 1
		line, _, _ = strings.Cut(line, "#")
		// Split on blanks and tabs only, as the shell's read does.
		f := strings.FieldsFunc(line, func(r rune) bool { return r == ' ' || r == '\t' })
		if len(f) == 0 {
			continue
		}
		if len(f) != 3 {
			bad(n, "expected '<name> <git URL> <folder|branch>'")
			continue
		}
		name, url, kind := f[0], f[1], f[2]
		if !nameRe.MatchString(name) || name == "." || name == ".." {
			bad(n, "bad repo name '%s'", name)
			continue
		}
		if _, _, under := config.ParseRepoURL(url); !under {
			bad(n, "%s: URL '%s' is not https://github.com/<owner>/<repo>; refusing to fetch anything", name, url)
			continue
		}
		if kind != "folder" && kind != "branch" {
			bad(n, "%s: mode must be folder or branch, got '%s'", name, kind)
			continue
		}
		for _, r := range repos {
			if r.name == name {
				bad(n, "duplicate repo name '%s'", name)
				continue lines
			}
		}
		repos = append(repos, listed{name, url, kind})
	}
	return repos, ok, nil
}

// assemble fetches and copies every repo, drops unlisted copies, and
// returns the exit code. err is an unexpected file system failure.
func assemble(hub, cache, mode string, repos []listed, stdout, stderr io.Writer) (int, error) {
	reposDir := filepath.Join(hub, "repos")
	if err := os.MkdirAll(cache, 0o777); err != nil {
		return 0, err
	}
	if err := os.MkdirAll(reposDir, 0o777); err != nil {
		return 0, err
	}

	var failed, warned []string
	for _, r := range repos {
		dir := filepath.Join(cache, r.name+"-"+r.kind)
		dest := filepath.Join(reposDir, r.name)
		if !fetchBundle(r, dir, stdout, stderr) {
			if mode == "local" {
				if isDir(dest) {
					fmt.Fprintf(stderr, "%s: warning: %s: cannot fetch %s; keeping the previous copy\n", me, r.name, r.url)
				} else {
					fmt.Fprintf(stderr, "%s: warning: %s: cannot fetch %s; no previous copy, so %s is missing from the hub\n", me, r.name, r.url, r.name)
				}
				warned = append(warned, r.name)
			} else {
				fmt.Fprintf(stderr, "%s: %s: cannot fetch %s\n", me, r.name, r.url)
				failed = append(failed, r.name)
			}
			continue
		}

		src := dir
		if r.kind == "folder" {
			src = filepath.Join(dir, "okf")
		}
		if !isFile(filepath.Join(src, "index.md")) || !isFile(filepath.Join(src, "overview.md")) {
			fmt.Fprintf(stderr, "%s: %s: bundle has no index.md or overview.md (empty bundle?)\n", me, r.name)
			failed = append(failed, r.name)
			continue
		}
		links, err := symlinks(src)
		if err != nil {
			return 0, err
		}
		if len(links) > 0 {
			fmt.Fprintf(stderr, "%s: %s: bundle contains symlinks, which tools silently skip:\n", me, r.name)
			for _, l := range links {
				fmt.Fprintf(stderr, "  %s\n", l)
			}
			failed = append(failed, r.name)
			continue
		}

		tmp := filepath.Join(reposDir, "."+r.name+".tmp")
		if err := os.RemoveAll(tmp); err != nil {
			return 0, err
		}
		if err := copyBundle(src, tmp); err != nil {
			return 0, err
		}
		if err := stripFrontmatter(filepath.Join(tmp, "index.md")); err != nil {
			return 0, err
		}
		if err := os.RemoveAll(dest); err != nil {
			return 0, err
		}
		if err := os.Rename(tmp, dest); err != nil {
			return 0, err
		}
		n, err := countMD(dest)
		if err != nil {
			return 0, err
		}
		fmt.Fprintf(stdout, "%s: %s: assembled (%s, %d .md files)\n", me, r.name, r.kind, n)
	}

	// Drop copies of repos no longer listed.
	names, err := repoDirs(reposDir)
	if err != nil {
		return 0, err
	}
	for _, base := range names {
		if slices.ContainsFunc(repos, func(r listed) bool { return r.name == base }) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(reposDir, base)); err != nil {
			return 0, err
		}
		fmt.Fprintf(stdout, "%s: %s: removed (not in repos.txt)\n", me, base)
	}

	if len(failed) > 0 {
		fmt.Fprintf(stderr, "%s: failed: %s\n", me, strings.Join(failed, " "))
		return 1, nil
	}
	if len(warned) > 0 {
		fmt.Fprintf(stderr, "%s: done with warnings for: %s\n", me, strings.Join(warned, " "))
	}
	return 0, nil
}

// fetchBundle brings the cache clone dir up to date: a fetch and hard reset
// when dir already clones r.url, else a fresh shallow clone (okf/main for
// branch mode, a sparse okf/ checkout for folder mode).
func fetchBundle(r listed, dir string, stdout, stderr io.Writer) bool {
	git := func(errOut io.Writer, args ...string) bool {
		cmd := exec.Command("git", args...)
		cmd.Stdout, cmd.Stderr = stdout, errOut
		return cmd.Run() == nil
	}
	ref := "HEAD"
	if r.kind == "branch" {
		ref = "okf/main"
	}
	if isDir(filepath.Join(dir, ".git")) {
		var origin bytes.Buffer
		cmd := exec.Command("git", "-C", dir, "remote", "get-url", "origin")
		cmd.Stdout = &origin
		cmd.Run()
		if strings.TrimRight(origin.String(), "\n") == r.url {
			return git(stderr, "-C", dir, "fetch", "-q", "--depth", "1", "origin", ref) &&
				git(stderr, "-C", dir, "reset", "-q", "--hard", "FETCH_HEAD")
		}
	}
	if os.RemoveAll(dir) != nil {
		return false
	}
	if r.kind == "branch" {
		return git(stderr, "clone", "-q", "--depth", "1", "--single-branch", "--branch", "okf/main", r.url, dir)
	}
	return git(io.Discard, "clone", "-q", "--depth", "1", "--filter=blob:none", "--sparse", r.url, dir) &&
		git(stderr, "-C", dir, "sparse-checkout", "set", "okf")
}

// skipped reports whether a walk skips this entry: .git and .github, at any
// depth, are not part of a bundle.
func skipped(path, root string, d fs.DirEntry) bool {
	return path != root && (d.Name() == ".git" || d.Name() == ".github")
}

// symlinks lists the symlinks under src outside .git and .github, relative
// to src (src itself, when it is a link, by its full path).
func symlinks(src string) ([]string, error) {
	var links []string
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if skipped(path, src, d) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			links = append(links, strings.TrimPrefix(path, src+"/"))
		}
		return nil
	})
	return links, err
}

// copyBundle copies src into the new folder dst, leaving out src's own .git
// and .github. Files and folders keep their permission bits (less the umask)
// and their modification times to the second, as a tar pipe does.
func copyBundle(src, dst string) error {
	type dirTime struct {
		path  string
		mtime time.Time
	}
	var dirs []dirTime
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == ".git" || rel == ".github" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		mtime := info.ModTime().Truncate(time.Second)
		switch {
		case d.IsDir():
			if err := os.MkdirAll(target, info.Mode().Perm()); err != nil {
				return err
			}
			dirs = append(dirs, dirTime{target, mtime})
		case d.Type()&fs.ModeSymlink != 0:
			to, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(to, target)
		case d.Type().IsRegular():
			if err := copyFile(path, target, info.Mode().Perm()); err != nil {
				return err
			}
			return os.Chtimes(target, mtime, mtime)
		}
		return nil
	})
	if err != nil {
		return err
	}
	// Directory times last, deepest first, since copying into a directory
	// changes its modification time.
	for i := len(dirs) - 1; i >= 0; i-- {
		if err := os.Chtimes(dirs[i].path, dirs[i].mtime, dirs[i].mtime); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string, perm fs.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// stripFrontmatter drops a leading ----delimited block from index.md, line
// for line as `awk 'NR==1 && /^---$/ {f=1; next} f && /^---$/ {f=0; next} !f'`
// does: an unclosed block drops the rest of the file, and every kept line
// ends in a newline.
func stripFrontmatter(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	first, _, _ := strings.Cut(string(raw), "\n")
	if first != "---" {
		return nil
	}
	text := strings.TrimSuffix(string(raw), "\n")
	var out strings.Builder
	in := false
	for i, line := range strings.Split(text, "\n") {
		switch {
		case i == 0 && line == "---":
			in = true
		case in && line == "---":
			in = false
		case !in:
			out.WriteString(line)
			out.WriteByte('\n')
		}
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(out.String()), 0o666); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// countMD counts the entries named *.md under dir, dir included, as
// find dir -name '*.md' does.
func countMD(dir string) (int, error) {
	n := 0
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ok, _ := filepath.Match("*.md", d.Name()); ok {
			n++
		}
		return nil
	})
	return n, err
}

func isDir(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}

func isFile(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.Mode().IsRegular()
}
