package config

import (
	"bytes"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// WithoutUserinfo drops the user and password from a URL-form remote, so a
// token in it is neither matched on nor printed. scp-form remotes
// (git@host:path) carry no password and come back unchanged. A URL that does
// not parse still loses everything between "://" and the last "@" before the
// path.
func WithoutUserinfo(remote string) string {
	if u, err := url.Parse(remote); err == nil && u.Scheme != "" && u.User != nil {
		u.User = nil
		return u.String()
	} else if err != nil {
		return userinfoRe.ReplaceAllString(remote, "$1")
	}
	return remote
}

var userinfoRe = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9+.-]*://)[^/]*@`)

var originRe = regexp.MustCompile(`^(?:https://github\.com/|git@github\.com:|ssh://github\.com/)([^/]+/[^/]+?)(?:\.git)?/?$`)

// GitHubRepo returns OWNER/REPO for a GitHub https or ssh remote URL without
// userinfo, else "".
func GitHubRepo(remote string) string {
	if m := originRe.FindStringSubmatch(remote); m != nil {
		return m[1]
	}
	return ""
}

// OriginRepos returns OWNER/REPO for each of the clone's origin URLs that
// names a GitHub repo: the configured URL first, before any
// url.<base>.insteadOf rewrite, so a user who maps https://github.com/ to ssh
// still matches; then the rewritten one, for host aliases that map to GitHub.
// origin is the first URL found, without userinfo, for messages; "" when the
// clone has no origin.
func OriginRepos(dir string) (repos []string, origin string) {
	for _, args := range [][]string{{"config", "--get", "remote.origin.url"}, {"remote", "get-url", "origin"}} {
		out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
		if err != nil {
			continue
		}
		remote := WithoutUserinfo(strings.TrimSpace(string(out)))
		if r := GitHubRepo(remote); r != "" {
			repos = append(repos, r)
		}
		if origin == "" {
			origin = remote
		}
	}
	return repos, origin
}

// HubRepo returns the GitHub repo a hub clone's origin names, so `--hub DIR`
// and the recorded clone find the same hub (plan decision 7).
func HubRepo(dir string) (owner, repo string, err error) {
	repos, origin := OriginRepos(dir)
	if len(repos) == 0 {
		if origin == "" {
			origin = "no origin"
		}
		return "", "", errorf("hub folder %s: %s is not a GitHub repo", dir, origin)
	}
	owner, repo, _ = strings.Cut(repos[0], "/")
	return owner, repo, nil
}

// git runs git in dir with prompts off and returns its trimmed stdout.
func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(errOut.String()); msg != "" {
			return "", errorf("git %s: %s", strings.Join(args, " "), msg)
		}
		return "", errorf("git %s: %v", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(out.String()), nil
}

// RemoteDefault returns the default branch of a clone's origin: origin/HEAD
// when the clone recorded it, else what `git ls-remote --symref origin HEAD`
// reports.
func RemoteDefault(dir string) (string, error) {
	if ref, err := git(dir, "symbolic-ref", "-q", "refs/remotes/origin/HEAD"); err == nil {
		if b, ok := strings.CutPrefix(ref, "refs/remotes/origin/"); ok && b != "" {
			return b, nil
		}
	}
	out, err := git(dir, "ls-remote", "--symref", "origin", "HEAD")
	if err != nil {
		return "", err
	}
	for _, l := range strings.Split(out, "\n") {
		if rest, ok := strings.CutPrefix(l, "ref: refs/heads/"); ok {
			if b, _, ok := strings.Cut(rest, "\tHEAD"); ok && b != "" {
				return b, nil
			}
		}
	}
	return "", errorf("%s: origin reports no default branch", dir)
}

// ControlFile returns name as committed on the hub clone's fetched default
// branch, refs/remotes/origin/<branch>, never the working tree, so a stale,
// dirty or switched checkout can't change what the writers read (plan
// decision 7).
func ControlFile(dir, branch, name string) ([]byte, error) {
	cmd := exec.Command("git", "-C", dir, "show", "refs/remotes/origin/"+branch+":"+name)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errOut.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, errorf("cannot read %s from origin/%s in %s: %s", name, branch, dir, msg)
	}
	return out.Bytes(), nil
}

// HubRepos parses repos.txt from the hub clone's fetched default branch.
func HubRepos(dir, branch string) (*Repos, error) {
	data, err := ControlFile(dir, branch, "repos.txt")
	if err != nil {
		return nil, err
	}
	return ParseRepos(data)
}

// Fetch updates the clone's remote-tracking branches from origin, pruning
// branches gone from the remote. Prompts are off and no hook runs, so a hook
// in the clone can't act during `compass sync` or `compass stamp`. The
// writers call it before reading control files (plan decision 7).
func Fetch(dir string) error {
	_, err := git(dir, "-c", "core.hooksPath="+os.DevNull, "fetch", "--quiet", "--prune", "--no-tags", "origin")
	return err
}
