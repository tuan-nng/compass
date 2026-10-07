// Command compass runs the OKF knowledge system tooling: one binary with a
// subcommand per job (plan decision 2).
package main

import (
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"

	"compass/internal/branchsetup"
	"compass/internal/bundlecheck"
	"compass/internal/hub"
	"compass/internal/okfinstall"
	"compass/internal/setup"
	"compass/internal/stamp"
	"compass/internal/syncjob"
	"compass/internal/validate"
)

// handler runs one subcommand with the arguments after its name and returns
// the exit code.
type handler func(args []string, stdout, stderr io.Writer) int

type command struct {
	name    string
	summary string
	run     handler
}

var commands = []command{
	{"setup", "connect this machine to your hub: okf, config and the skill", setup.Main},
	{"okf install", "install the pinned okf binary", okfinstall.Main},
	{"validate", "run the strict OKF validator on a bundle", validate.Main},
	{"bundle check", "check a bundle as repo CI does", bundlecheck.Main},
	{"hub assemble", "fetch every repos.txt bundle into a hub's repos/", hub.AssembleMain},
	{"hub check", "check an assembled hub", hub.CheckMain},
	{"branch setup", "set up branch-mode okf/ in a clone", branchsetup.Main},
	{"stamp", "write process: stamps from checks.txt", stamp.Main},
	{"sync", "carry out the branch-mode knowledge pull request state table", syncjob.Main},
	{"version", "print the build's commit", versionMain},
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "usage: compass <command> [arguments]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "commands:")
	for _, c := range commands {
		fmt.Fprintf(w, "  %-14s %s\n", c.name, c.summary)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Run `compass <command> --help` for a command's flags.")
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		usage(stdout)
		if len(args) == 0 {
			return 2
		}
		return 0
	}
	// Longest match first: two-word commands before one-word ones.
	for _, words := range []int{2, 1} {
		if len(args) < words {
			continue
		}
		name := strings.Join(args[:words], " ")
		for _, c := range commands {
			if c.name == name {
				return c.run(args[words:], stdout, stderr)
			}
		}
	}
	fmt.Fprintf(stderr, "compass: unknown command: %s\n", strings.Join(args, " "))
	usage(stderr)
	return 2
}

// commit is set by the Makefile's -ldflags when the build has no VCS
// information of its own.
var commit = ""

func versionMain(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		fmt.Fprintln(stderr, "usage: compass version")
		return 2
	}
	rev, dirty := commit, false
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			switch s.Key {
			case "vcs.revision":
				if rev == "" {
					rev = s.Value
				}
			case "vcs.modified":
				dirty = s.Value == "true"
			}
		}
	}
	if rev == "" {
		rev = "unknown"
	}
	if dirty {
		rev += "-dirty"
	}
	fmt.Fprintf(stdout, "compass %s\n", rev)
	return 0
}
