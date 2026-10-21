// Package hub implements the commands that work on a hub clone.
package hub

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"compass/internal/config"
	"compass/internal/validate"
)

const checkHelp = `Check a hub, without changing it:
  1. no symlink exists anywhere in it; the other checks would follow one, so
     they don't run while one does;
  2. the strict validator passes, so a broken link between repos fails;
  3. for each repos/<id>/, the concepts ` + "`okf list`" + ` sees there equal its concept
     files, and are more than zero;
  4. every index.md is what ` + "`okf index .`" + ` at the hub root would write.

Usage: compass hub check [<hub-dir>]
       <hub-dir> defaults to the hub clone that ` + "`compass setup`" + ` recorded.
Exit:  0 clean, 1 on any finding, 2 when the check could not run.
Env:   OKF  path to the okf binary (default: okf on PATH)
`

// Finding is one problem in a hub, keyed by the hub-relative path it
// concerns, so the findings of two trees compare.
type Finding struct{ Path, Message string }

func (f Finding) String() string { return f.Path + ": " + f.Message }

// CheckMain runs `compass hub check`.
func CheckMain(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		fmt.Fprint(stdout, checkHelp)
		return 0
	}
	if len(args) > 1 || (len(args) == 1 && strings.HasPrefix(args[0], "-")) {
		fmt.Fprintln(stderr, "usage: compass hub check [<hub-dir>]")
		return 2
	}
	var hub string
	if len(args) == 1 {
		hub = args[0]
	} else {
		var err error
		if hub, err = config.HubDir(os.Getenv); err != nil {
			fmt.Fprintf(stderr, "hub check: %v\n", err)
			return 2
		}
	}
	if st, err := os.Stat(hub); err != nil || !st.IsDir() {
		fmt.Fprintf(stderr, "hub check: %s is not a directory\n", hub)
		return 2
	}
	fmt.Fprintf(stdout, "hub check: %s\n", hub)
	findings, err := Check(hub)
	if err != nil {
		fmt.Fprintf(stderr, "hub check: %v\n", err)
		return 2
	}
	for _, f := range findings {
		fmt.Fprintf(stderr, "hub check: %s\n", f)
	}
	if len(findings) > 0 {
		fmt.Fprintf(stdout, "hub check: %d finding(s)\n", len(findings))
		return 1
	}
	fmt.Fprintln(stdout, "hub check: ok")
	return 0
}

// Check runs every hub check on dir and returns its findings; dir is never
// modified. err is set when a check could not run, such as okf missing.
func Check(dir string) ([]Finding, error) {
	links, err := symlinks(dir)
	if err != nil {
		return nil, err
	}
	if len(links) > 0 {
		return links, nil
	}
	vf, err := validate.Findings(dir)
	if err != nil {
		return nil, fmt.Errorf("validator: %v", err)
	}
	var out []Finding
	for _, f := range vf {
		out = append(out, Finding{f.Path, f.Message})
	}
	counts, err := conceptCounts(dir)
	if err != nil {
		return nil, err
	}
	out = append(out, counts...)
	stale, err := staleIndexes(dir)
	if err != nil {
		return nil, err
	}
	return append(out, stale...), nil
}

// symlinks reports every symlink under dir, other than a top-level .git
// (a folder in a clone, a file in a worktree), without following any.
func symlinks(dir string) ([]Finding, error) {
	var out []Finding
	err := walkHub(dir, func(rel string, d fs.DirEntry) error {
		if d.Type()&fs.ModeSymlink != 0 {
			out = append(out, Finding{rel, "is a symlink; a hub holds no links"})
		}
		return nil
	})
	return out, err
}

// walkHub calls fn for every entry under dir with its slash-separated
// hub-relative path, skipping the top-level .git and never following a link.
func walkHub(dir string, fn func(rel string, d fs.DirEntry) error) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == dir {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == ".git" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		return fn(rel, d)
	})
}

// conceptCounts checks that, for each repos/<id>/, the concepts `okf list`
// sees there equal its concept files and are more than zero. This catches a
// folder the tools silently skip (research report, finding 1). A hub with no
// repos/ has nothing to count.
func conceptCounts(dir string) ([]Finding, error) {
	names, err := repoDirs(filepath.Join(dir, "repos"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, nil
	}
	listed, err := listIDs(dir)
	if err != nil {
		return nil, err
	}
	var out []Finding
	for _, name := range names {
		files, err := conceptFiles(filepath.Join(dir, "repos", name))
		if err != nil {
			return nil, err
		}
		seen := 0
		for _, id := range listed {
			if strings.HasPrefix(id, "repos/"+name+"/") {
				seen++
			}
		}
		switch {
		case files == 0:
			out = append(out, Finding{"repos/" + name, "holds no concept files"})
		case seen != files:
			out = append(out, Finding{"repos/" + name, fmt.Sprintf("okf sees %d concepts in %d concept files", seen, files)})
		}
	}
	return out, nil
}

// staleIndexes runs `okf index` on a copy of dir and reports each index.md
// it wrote that dir lacks or holds with other content.
func staleIndexes(dir string) ([]Finding, error) {
	tmp, err := os.MkdirTemp("", "compass-hub-check-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	if tmp, err = filepath.EvalSymlinks(tmp); err != nil {
		return nil, err
	}
	if err := copyTree(dir, tmp); err != nil {
		return nil, fmt.Errorf("copying %s: %v", dir, err)
	}
	out, err := runOKF("index", tmp)
	if err != nil {
		return nil, err
	}
	var res struct {
		Written []string `json:"indexes_written"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		return nil, fmt.Errorf("cannot parse `okf index` output: %v", err)
	}
	var found []Finding
	for _, w := range res.Written {
		rel, err := filepath.Rel(tmp, w)
		if err != nil || strings.HasPrefix(rel, "..") {
			return nil, fmt.Errorf("okf index wrote %s, outside the copy", w)
		}
		want, err := os.ReadFile(w)
		if err != nil {
			return nil, err
		}
		have, err := os.ReadFile(filepath.Join(dir, rel))
		rel = filepath.ToSlash(rel)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			found = append(found, Finding{rel, "is missing; run `okf index .` at the hub root"})
		case err != nil:
			return nil, err
		case !bytes.Equal(have, want):
			found = append(found, Finding{rel, "is stale; run `okf index .` at the hub root"})
		}
	}
	return found, nil
}

// copyTree copies the folders and regular files under src into dst, skipping
// the top-level .git. symlinks has already refused any link.
func copyTree(src, dst string) error {
	return walkHub(src, func(rel string, d fs.DirEntry) error {
		target := filepath.Join(dst, filepath.FromSlash(rel))
		switch {
		case d.IsDir():
			return os.Mkdir(target, 0o755)
		case d.Type().IsRegular():
			data, err := os.ReadFile(filepath.Join(src, filepath.FromSlash(rel)))
			if err != nil {
				return err
			}
			return os.WriteFile(target, data, 0o644)
		}
		return nil
	})
}

// runOKF runs the okf binary and returns its stdout. A failure names okf's
// exit code and its stderr.
func runOKF(args ...string) ([]byte, error) {
	okf := config.OKFBinary()
	var out, errOut bytes.Buffer
	cmd := exec.Command(okf, args...)
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errOut.String())
		if i := strings.LastIndexByte(msg, '\n'); i >= 0 {
			msg = msg[i+1:]
		}
		return nil, fmt.Errorf("`%s %s`: %v %s", okf, strings.Join(args, " "), err, msg)
	}
	return out.Bytes(), nil
}

// listIDs runs `okf list <hub>` and returns every concept id.
func listIDs(hub string) ([]string, error) {
	out, err := runOKF("list", hub)
	if err != nil {
		return nil, err
	}
	var list struct {
		Concepts []struct {
			ID string `json:"id"`
		} `json:"concepts"`
	}
	if err := json.Unmarshal(out, &list); err != nil {
		return nil, fmt.Errorf("cannot parse `okf list %s` output: %v", hub, err)
	}
	ids := make([]string, len(list.Concepts))
	for i, c := range list.Concepts {
		ids[i] = c.ID
	}
	return ids, nil
}

// repoDirs returns the sorted names of the directories in dir, skipping
// hidden entries, as the glob dir/*/ does.
func repoDirs(dir string) ([]string, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range ents {
		if !strings.HasPrefix(e.Name(), ".") && e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// conceptFiles counts the regular *.md files under dir other than index.md
// and log.md.
func conceptFiles(dir string) (int, error) {
	n := 0
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.Type().IsRegular() && name != "index.md" && name != "log.md" && strings.HasSuffix(name, ".md") {
			n++
		}
		return nil
	})
	return n, err
}
