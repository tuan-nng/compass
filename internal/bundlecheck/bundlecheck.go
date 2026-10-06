// Package bundlecheck checks an OKF bundle the way repo CI does (research
// report section 5, "Hub assembly and CI"):
//  1. the strict validator passes;
//  2. `okf index` changes no file, so the committed index files are current.
//
// It runs from inside the git checkout that holds the bundle.
package bundlecheck

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"compass/internal/config"
	"compass/internal/validate"
)

const help = `Check an OKF bundle the way repo CI does:
  1. the strict validator passes;
  2. ` + "`okf index`" + ` changes no file, so the committed index files are current.
Run it from inside the git checkout that holds the bundle.

Usage: compass bundle check <bundle>    (okf in folder mode, . on okf/main)
Env:   OKF  path to the okf binary (default: okf on PATH)
`

// Main runs `compass bundle check`.
func Main(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		fmt.Fprint(stdout, help)
		return 0
	}
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: compass bundle check <bundle>")
		return 2
	}
	bundle := args[0]
	okf := config.OKFBinary()
	if st, err := os.Stat(bundle); err != nil || !st.IsDir() {
		fmt.Fprintf(stderr, "bundle check: no bundle at %s\n", bundle)
		return 1
	}
	if exec.Command("git", "rev-parse", "--is-inside-work-tree").Run() != nil {
		fmt.Fprintf(stderr, "bundle check: run inside the git checkout that holds %s\n", bundle)
		return 2
	}

	status := 0
	fmt.Fprintf(stdout, "== strict validator: %s\n", bundle)
	if validate.Main([]string{bundle}, stdout, stderr) != 0 {
		status = 1
	}

	fmt.Fprintf(stdout, "== index check: %s\n", bundle)
	// Like `[ -n "$(git status ...)" ]`: a failing git status counts as clean.
	before, _ := output(stderr, "git", "status", "--porcelain", "--", bundle)
	if before != "" {
		fmt.Fprintf(stderr, "bundle check: %s has uncommitted changes; commit them before the index check\n", bundle)
		return 1
	}
	if code := run(io.Discard, stderr, okf, "index", bundle); code != 0 {
		return code
	}
	changed, code := output(stderr, "git", "status", "--porcelain", "--", bundle)
	if code != 0 {
		return code
	}
	if changed == "" {
		fmt.Fprintln(stdout, "index files are current")
		return status
	}
	fmt.Fprintf(stderr, "bundle check: index files are out of date. Run `okf index %s` and commit the result:\n", bundle)
	fmt.Fprintln(stderr, changed)
	if code := run(stderr, stderr, "git", "--no-pager", "diff", "--stat", "--", bundle); code != 0 {
		return code
	}
	return 1
}

// run runs a command and returns its exit code, as the shell reports it:
// 127 when the command cannot be started.
func run(stdout, stderr io.Writer, name string, args ...string) int {
	cmd := exec.Command(name, args...)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	return exitCode(cmd.Run(), stderr, name)
}

// output runs a command and returns its stdout without trailing newlines, as
// shell command substitution does, plus its exit code.
func output(stderr io.Writer, name string, args ...string) (string, int) {
	var out bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Stdout, cmd.Stderr = &out, stderr
	code := exitCode(cmd.Run(), stderr, name)
	return strings.TrimRight(out.String(), "\n"), code
}

func exitCode(err error, stderr io.Writer, name string) int {
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() > 0 {
		return ee.ExitCode()
	}
	fmt.Fprintf(stderr, "bundle check: %s: %v\n", name, err)
	return 127
}
