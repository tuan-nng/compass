// Package app implements `compass app create`: create the reader or writer
// GitHub App for an OKF hub (plan decision 14).
package app

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"os/exec"
	"strings"
)

const help = `Usage:
  compass app create --role reader|writer --hub OWNER/HUB
                     [--owner OWNER] [--name NAME] [--port 8765]

Create the reader or writer GitHub App for an OKF hub (plan decision 14).

It serves one local page that posts an app manifest to GitHub. You review and
click "Create GitHub App" in the browser; GitHub redirects back here with a
one-time code, which this command converts into the app. The private key goes
straight from that response into ` + "`gh secret set`" + ` on the hub repo and is never
printed or written to disk. The client id is stored as a variable.

  reader: contents:read. Hub CI (variable OKF_READER_CLIENT_ID, secret
          OKF_READER_PRIVATE_KEY on the repo).
  writer: contents:write, pull_requests:write, actions:read, checks:read. Sync
          job and stamper. The hub's okf-write environment is created first,
          limited to the hub's default branch; the variable and secret
          (OKF_WRITER_CLIENT_ID, OKF_WRITER_PRIVATE_KEY) live only there.

Afterwards, install the app on the repos it needs; the command prints the URL.
Needs: gh (logged in as an owner/admin of OWNER and the hub), a browser.

Flags:
`

// permissions per role; encoding/json sorts map keys, as the page shows them.
var permissions = map[string]map[string]string{
	"reader": {"contents": "read", "metadata": "read"},
	"writer": {
		"contents":      "write",
		"pull_requests": "write",
		"actions":       "read",
		"checks":        "read",
		"metadata":      "read",
	},
}

const environment = "okf-write"

// exitError carries a message for stderr and exit code 1.
type exitError struct{ msg string }

func (e *exitError) Error() string { return e.msg }

// gh runs gh with stdin; returns stdout or an error carrying gh's message.
func gh(stdin string, args ...string) (string, error) {
	c := exec.Command("gh", args...)
	if stdin != "" {
		c.Stdin = strings.NewReader(stdin)
	}
	var out, errb bytes.Buffer
	c.Stdout, c.Stderr = &out, &errb
	if err := c.Run(); err != nil {
		shown := args
		if len(shown) > 3 {
			shown = shown[:3]
		}
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", &exitError{fmt.Sprintf("app create: gh %s failed: %s", strings.Join(shown, " "), msg)}
	}
	return out.String(), nil
}

type options struct {
	role, hub, owner, name string
	port                   int
}

// Main runs `compass app create`.
func Main(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("compass app create", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var o options
	fs.StringVar(&o.role, "role", "", "reader or writer (required)")
	fs.StringVar(&o.hub, "hub", "", "OWNER/REPO of the knowledge hub (required)")
	fs.StringVar(&o.owner, "owner", "", "account that owns the app (default: the hub's owner)")
	fs.StringVar(&o.name, "name", "", "app name (default: <owner>-okf-<role>)")
	fs.IntVar(&o.port, "port", 8765, "local port for the manifest page")
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), help)
		fs.PrintDefaults()
	}
	for _, a := range args {
		if a == "-h" || a == "--help" || a == "-help" {
			fs.SetOutput(stdout)
			fs.Usage()
			return 0
		}
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	switch {
	case fs.NArg() > 0:
		fmt.Fprintf(stderr, "compass app create: unexpected argument: %s\n", fs.Arg(0))
		return 2
	case permissions[o.role] == nil:
		fmt.Fprintln(stderr, "compass app create: --role must be reader or writer")
		return 2
	case o.hub == "":
		fmt.Fprintln(stderr, "compass app create: --hub is required")
		return 2
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", o.port))
	if err != nil {
		fmt.Fprintf(stderr, "app create: %v\n", err)
		return 1
	}
	if err := create(o, ln, stdout); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

// appResult is the part of the manifest conversion response we use.
type appResult struct {
	ID       int64  `json:"id"`
	Slug     string `json:"slug"`
	ClientID string `json:"client_id"`
	PEM      string `json:"pem"`
}

// create runs the flow on ln, which it closes.
func create(o options, ln net.Listener, stdout io.Writer) error {
	defer ln.Close()
	owner := o.owner
	if owner == "" {
		owner = strings.SplitN(o.hub, "/", 2)[0]
	}
	name := o.name
	if name == "" {
		name = owner + "-okf-" + o.role
	}
	if r := []rune(name); len(r) > 34 {
		name = string(r[:34])
	}
	kindOut, err := gh("", "api", "users/"+owner, "-q", ".type")
	if err != nil {
		return err
	}
	kind := strings.TrimSpace(kindOut)
	if o.role == "writer" {
		if err := ensureEnvironment(o.hub, stdout); err != nil {
			return err
		}
	}

	state := newState()
	port := ln.Addr().(*net.TCPAddr).Port
	manifest := manifestJSON(o.role, name, o.hub, fmt.Sprintf("http://127.0.0.1:%d/callback", port))
	base := "https://github.com/settings/apps/new"
	if kind == "Organization" {
		base = "https://github.com/organizations/" + owner + "/settings/apps/new"
	}
	action := base + "?state=" + state
	perms, _ := json.Marshal(permissions[o.role])

	type outcome struct {
		app appResult
		err error
	}
	done := make(chan outcome, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			reply(w, "<!doctype html><title>Create OKF app</title>"+
				"<p>Creating <b>"+html.EscapeString(name)+"</b> owned by <b>"+html.EscapeString(owner)+"</b>"+
				" with permissions "+html.EscapeString(string(perms))+".</p>"+
				"<form method=post action='"+html.EscapeString(action)+"'>"+
				"<input type=hidden name=manifest value='"+html.EscapeString(manifest)+"'>"+
				"<button>Continue to GitHub</button></form>")
		case "/callback":
			q := r.URL.Query()
			code := q.Get("code")
			if q.Get("state") != state || !q.Has("code") {
				http.Error(w, "state mismatch or no code", http.StatusBadRequest)
				return
			}
			app, err := convert(code)
			if err == nil {
				err = store(o.hub, o.role, app, stdout)
			}
			if err != nil {
				http.Error(w, "app creation failed; see the terminal", http.StatusInternalServerError)
				select {
				case done <- outcome{err: err}:
				default:
				}
				return
			}
			install := "https://github.com/apps/" + app.Slug + "/installations/new"
			reply(w, "<p>Created "+html.EscapeString(app.Slug)+". "+
				"<a href='"+html.EscapeString(install)+"'>Install it on the repos it needs</a>.</p>")
			select {
			case done <- outcome{app: app}:
			default:
			}
		default:
			http.NotFound(w, r)
		}
	})
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln)
	fmt.Fprintf(stdout, "Open http://127.0.0.1:%d/ in a browser logged in to GitHub as an admin of %s.\n", port, owner)
	res := <-done
	srv.Close()
	if res.err != nil {
		return res.err
	}
	a := res.app
	fmt.Fprintf(stdout, "app: %s (id %d, client id %s)\n", a.Slug, a.ID, a.ClientID)
	fmt.Fprintf(stdout, "install: https://github.com/apps/%s/installations/new\n", a.Slug)
	if o.role == "reader" {
		fmt.Fprintln(stdout, "Install it only on the repos listed in repos.txt.")
	} else {
		fmt.Fprintln(stdout, "Install it on the pilot repos, then add it to their rulesets' bypass list.")
	}
	return nil
}

func reply(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	io.WriteString(w, body)
}

// newState is 16 random bytes, URL-safe base64 without padding.
func newState() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// manifestJSON is the app manifest GitHub receives.
func manifestJSON(role, name, hub, redirect string) string {
	m := map[string]any{
		"name":                name,
		"url":                 "https://github.com/" + hub,
		"hook_attributes":     map[string]any{"url": "https://github.com/" + hub, "active": false},
		"redirect_url":        redirect,
		"public":              false,
		"default_permissions": permissions[role],
		"default_events":      []string{},
	}
	b, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// convert exchanges the one-time code for the app, its client id and key.
func convert(code string) (appResult, error) {
	var app appResult
	out, err := gh("", "api", "-X", "POST", "app-manifests/"+code+"/conversions")
	if err != nil {
		return app, err
	}
	if err := json.Unmarshal([]byte(out), &app); err != nil {
		return app, &exitError{"app create: cannot read the app-manifests conversion response: " + err.Error()}
	}
	if app.Slug == "" || app.ClientID == "" || app.PEM == "" {
		return app, errors.New("app create: the app-manifests conversion response lacks slug, client_id or pem")
	}
	return app, nil
}

// ensureEnvironment creates okf-write, usable only from the hub's default branch.
func ensureEnvironment(hub string, stdout io.Writer) error {
	out, err := gh("", "api", "repos/"+hub, "-q", ".default_branch")
	if err != nil {
		return err
	}
	def := strings.TrimSpace(out)
	env := "repos/" + hub + "/environments/" + environment
	if _, err := gh(`{"deployment_branch_policy": {"protected_branches": false, "custom_branch_policies": true}}`,
		"api", "-X", "PUT", env, "--input", "-"); err != nil {
		return err
	}
	out, err = gh("", "api", env+"/deployment-branch-policies")
	if err != nil {
		return err
	}
	var existing struct {
		BranchPolicies []struct {
			Name string `json:"name"`
		} `json:"branch_policies"`
	}
	if err := json.Unmarshal([]byte(out), &existing); err != nil {
		return &exitError{"app create: cannot read the deployment branch policies: " + err.Error()}
	}
	found := false
	for _, p := range existing.BranchPolicies {
		found = found || p.Name == def
	}
	if !found {
		if _, err := gh("", "api", "-X", "POST", env+"/deployment-branch-policies",
			"-f", "name="+def, "-f", "type=branch"); err != nil {
			return err
		}
	}
	fmt.Fprintf(stdout, "environment %s: only branch %s may deploy\n", environment, def)
	return nil
}

// store keeps the client id as a variable and pipes the key into a secret.
func store(hub, role string, app appResult, stdout io.Writer) error {
	prefix := "OKF_" + strings.ToUpper(role)
	var scope []string
	where := "repository"
	if role == "writer" {
		scope = []string{"--env", environment}
		where = "environment " + environment
	}
	if _, err := gh("", append(append([]string{"variable", "set", prefix + "_CLIENT_ID", "-R", hub}, scope...),
		"--body", app.ClientID)...); err != nil {
		return err
	}
	if _, err := gh(app.PEM, append([]string{"secret", "set", prefix + "_PRIVATE_KEY", "-R", hub}, scope...)...); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "stored %s_CLIENT_ID (variable) and %s_PRIVATE_KEY (secret) on %s, %s\n", prefix, prefix, hub, where)
	return nil
}
