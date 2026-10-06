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

const checkHelp = `Check an assembled hub, as hub CI does after ` + "`compass hub assemble`" + `:
  1. the strict validator passes, so a broken cross-repo link fails;
  2. for each repos/<name>/, the concepts ` + "`okf list`" + ` sees there equal the
     concept files copied, and are more than zero. This catches a bundle the
     tools silently skip, such as a symlinked one (research report, finding 1).

Usage: compass hub check <hub-dir>
Env:   OKF  path to the okf binary (default: okf on PATH)
`

// CheckMain runs `compass hub check`.
func CheckMain(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		fmt.Fprint(stdout, checkHelp)
		return 0
	}
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: compass hub check <hub-dir>")
		return 2
	}
	hub := args[0]
	if st, err := os.Stat(filepath.Join(hub, "repos")); err != nil || !st.IsDir() {
		fmt.Fprintf(stderr, "hub check: %s/repos is missing; run `compass hub assemble` first\n", hub)
		return 1
	}

	status := 0
	fmt.Fprintf(stdout, "== strict validator: %s\n", hub)
	if validate.Main([]string{hub}, stdout, stderr) != 0 {
		status = 1
	}

	fmt.Fprintln(stdout, "== concept counts")
	listed, code := listIDs(hub, stderr)
	if code != 0 {
		return code
	}
	names, err := repoDirs(filepath.Join(hub, "repos"))
	if err != nil {
		fmt.Fprintf(stderr, "hub check: %v\n", err)
		return 1
	}
	for _, name := range names {
		files, err := conceptFiles(filepath.Join(hub, "repos", name) + "/")
		if err != nil {
			fmt.Fprintf(stderr, "hub check: %v\n", err)
			return 1
		}
		seen := 0
		for _, id := range listed {
			if strings.HasPrefix(id, "repos/"+name+"/") {
				seen++
			}
		}
		if files == 0 || seen != files {
			fmt.Fprintf(stderr, "hub check: %s: okf sees %d concepts, %d concept files were copied\n", name, seen, files)
			status = 1
		} else {
			fmt.Fprintf(stdout, "%s: %d concepts\n", name, seen)
		}
	}
	return status
}

// listIDs runs `okf list <hub>` and returns every concept id. A failure
// returns okf's exit code (127 when okf cannot be started), or 1 for output
// that is not the expected JSON.
func listIDs(hub string, stderr io.Writer) ([]string, int) {
	okf := config.OKFBinary()
	var out bytes.Buffer
	cmd := exec.Command(okf, "list", hub)
	cmd.Stdout, cmd.Stderr = &out, stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ExitCode() > 0 {
			return nil, ee.ExitCode()
		}
		fmt.Fprintf(stderr, "hub check: %s: %v\n", okf, err)
		return nil, 127
	}
	var list struct {
		Concepts []struct {
			ID string `json:"id"`
		} `json:"concepts"`
	}
	if err := json.Unmarshal(out.Bytes(), &list); err != nil {
		fmt.Fprintf(stderr, "hub check: cannot parse `okf list %s` output: %v\n", hub, err)
		return nil, 1
	}
	ids := make([]string, len(list.Concepts))
	for i, c := range list.Concepts {
		ids[i] = c.ID
	}
	return ids, 0
}

// repoDirs returns the sorted names of the directories (or links to
// directories) in dir, skipping hidden entries, as the glob dir/*/ does.
func repoDirs(dir string) ([]string, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if st, err := os.Stat(filepath.Join(dir, e.Name())); err == nil && st.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// conceptFiles counts the regular *.md files under dir other than index.md
// and log.md, without following links below dir.
func conceptFiles(dir string) (int, error) {
	n := 0
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.Type().IsRegular() && name != "index.md" && name != "log.md" {
			if ok, _ := filepath.Match("*.md", name); ok {
				n++
			}
		}
		return nil
	})
	return n, err
}
