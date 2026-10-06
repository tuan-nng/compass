package syncjob

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"compass/internal/config"
	"compass/internal/frontmatter"
	"compass/internal/github"
)

func osGetenv(name string) string { return os.Getenv(name) }

const usage = "usage: compass sync [-h] --hub DIR [--dry-run] [--token-env NAME] [--api-url URL]\n" +
	"                    [--ignore-login LOGIN] [--now DATETIME]\n"

const help = usage + `
Merge approved, green branch-mode knowledge pull requests after their code
pull request merges, and clean up okf/<b> branches (research report section
5, Branch mode).

options:
  -h, --help            show this help message and exit
  --hub DIR             hub checkout holding repos.txt (and checks.txt)
  --dry-run             read GitHub and log what would change; make no write
                        calls
  --token-env NAME      environment variable holding the API token (default:
                        GITHUB_TOKEN)
  --api-url URL         GitHub REST API base (default: $GITHUB_API_URL or
                        https://api.github.com)
  --ignore-login LOGIN  review login whose approval never counts
                        (repeatable); [bot] logins and Bot users are always
                        ignored
  --now DATETIME        current time for the 7-day rule (RFC 3339; default:
                        the clock)
`

type listFlag []string

func (l *listFlag) String() string     { return strings.Join(*l, ",") }
func (l *listFlag) Set(v string) error { *l = append(*l, v); return nil }

func usageError(stderr io.Writer, msg string) int {
	fmt.Fprint(stderr, usage)
	fmt.Fprintf(stderr, "compass sync: error: %s\n", msg)
	return 2
}

// run is Main with the environment, clock and client constructor injected.
// Log lines go to stdout; usage errors go to stderr.
func run(args []string, stdout, stderr io.Writer, getenv func(string) string, clock func() time.Time,
	newClient func(token, baseURL string, dryRun bool) *github.Client) int {
	fs := flag.NewFlagSet("compass sync", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	hub := fs.String("hub", "", "")
	dryRun := fs.Bool("dry-run", false, "")
	tokenEnv := fs.String("token-env", "GITHUB_TOKEN", "")
	apiURL := fs.String("api-url", "", "")
	var ignore listFlag
	fs.Var(&ignore, "ignore-login", "")
	nowFlag := fs.String("now", "", "")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, help)
			return 0
		}
		return usageError(stderr, err.Error())
	}
	if fs.NArg() > 0 {
		return usageError(stderr, "unrecognized arguments: "+strings.Join(fs.Args(), " "))
	}
	hubSet := false
	fs.Visit(func(f *flag.Flag) { hubSet = hubSet || f.Name == "hub" })
	if !hubSet {
		return usageError(stderr, "the following arguments are required: --hub")
	}
	var now time.Time
	if *nowFlag == "" {
		now = clock().UTC()
	} else {
		t, err := frontmatter.ParseTime(*nowFlag)
		if err != nil {
			return usageError(stderr, "--now: "+err.Error())
		}
		now = t
	}
	log := func(s string) { fmt.Fprintln(stdout, s) }
	org, err := config.Org(getenv)
	if err == nil {
		var token string
		token, err = config.Token(*tokenEnv, getenv)
		if err == nil {
			base := *apiURL
			if base == "" {
				base = getenv("GITHUB_API_URL")
			}
			if base == "" {
				base = github.DefaultAPIURL
			}
			var code int
			code, err = Run(*hub, newClient(token, base, *dryRun), org, now, *dryRun, ignore, log)
			if err == nil {
				return code
			}
		}
	}
	log(fmt.Sprintf("%s: error: %v", prog, err))
	return 2
}
