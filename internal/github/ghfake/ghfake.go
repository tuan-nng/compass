// Package ghfake is a fake GitHub transport for the stamp and sync tests,
// replaying recorded API responses.
//
// Routes are keyed by method and path; an optional query must be a subset of
// the request's query. Every request is recorded, so tests assert the exact
// write calls. An unrouted request fails the test.
package ghfake

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"compass/internal/github"
)

// API is the fake API base; Token is the only token it accepts.
const (
	API   = "https://api.test"
	Token = "test-token"
)

// Env is the environment the jobs run with in tests.
var Env = map[string]string{"GITHUB_TOKEN": Token, "OKF_ORG": "acme"}

// Getenv looks names up in Env.
func Getenv(name string) string { return Env[name] }

// Call is one recorded request. Body is the decoded JSON body, or nil.
type Call struct {
	Method string
	Path   string
	Query  map[string]string
	Body   any
}

func (c Call) String() string { return fmt.Sprintf("%s %s %v %v", c.Method, c.Path, c.Query, c.Body) }

type route struct {
	method, path string
	query        map[string]string
	status       int
	body         any
	headers      map[string]string
	times        int // -1: unlimited
}

// Option changes one route.
type Option func(*route)

// Status sets the response status (default 200).
func Status(s int) Option { return func(r *route) { r.status = s } }

// Query requires these query parameters on the request.
func Query(q map[string]string) Option { return func(r *route) { r.query = q } }

// Header adds a response header.
func Header(k, v string) Option { return func(r *route) { r.headers[k] = v } }

// Times limits how often the route answers.
func Times(n int) Option { return func(r *route) { r.times = n } }

// Fake is an http.RoundTripper that answers from registered routes.
type Fake struct {
	t      testing.TB
	mu     sync.Mutex
	routes []*route
	Calls  []Call
}

// New returns an empty fake bound to t.
func New(t testing.TB) *Fake { return &Fake{t: t} }

// Add registers a response. Later routes win. A nil body sends no body.
func (f *Fake) Add(method, path string, body any, opts ...Option) {
	r := &route{method: method, path: path, status: 200, body: body, headers: map[string]string{}, times: -1}
	for _, o := range opts {
		o(r)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.routes = append([]*route{r}, f.routes...)
}

// Client returns a client that talks to this fake.
func (f *Fake) Client(dryRun bool) *github.Client {
	c := github.New(Token, API, dryRun)
	c.HTTP = &http.Client{Transport: f}
	return c
}

// RoundTrip implements http.RoundTripper.
func (f *Fake) RoundTrip(req *http.Request) (*http.Response, error) {
	f.t.Helper()
	if base := req.URL.Scheme + "://" + req.URL.Host; base != API {
		f.t.Errorf("request outside the fake API: %s", req.URL)
		return nil, fmt.Errorf("outside API: %s", req.URL)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer "+Token {
		f.t.Errorf("bad Authorization header %q", got)
	}
	query := map[string]string{}
	for k, v := range req.URL.Query() {
		if k != "per_page" && len(v) > 0 {
			query[k] = v[0]
		}
	}
	var body any
	if req.Body != nil {
		raw, _ := io.ReadAll(req.Body)
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &body); err != nil {
				f.t.Errorf("request body is not JSON: %s", raw)
			}
		}
	}
	path, _ := url.PathUnescape(req.URL.EscapedPath())
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, Call{req.Method, path, query, body})
	for _, r := range f.routes {
		if r.method != req.Method || r.path != path || !subset(r.query, query) {
			continue
		}
		if r.times == 0 {
			continue
		}
		if r.times > 0 {
			r.times--
		}
		var raw []byte
		if r.body != nil {
			raw, _ = json.Marshal(r.body)
		}
		h := http.Header{}
		for k, v := range r.headers {
			h.Set(k, v)
		}
		return &http.Response{
			StatusCode: r.status,
			Header:     h,
			Body:       io.NopCloser(bytes.NewReader(raw)),
			Request:    req,
		}, nil
	}
	f.t.Errorf("unrouted request: %s %s %v", req.Method, path, query)
	return nil, fmt.Errorf("unrouted request: %s %s", req.Method, path)
}

func subset(want, got map[string]string) bool {
	for k, v := range want {
		if got[k] != v {
			return false
		}
	}
	return true
}

// Writes returns the recorded POST, PATCH, PUT and DELETE calls.
func (f *Fake) Writes() []Call {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Call
	for _, c := range f.Calls {
		switch c.Method {
		case http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete:
			out = append(out, c)
		}
	}
	return out
}

// Fixture decodes <dir>/fixtures/<name> as generic JSON.
func Fixture(t testing.TB, dir, name string) any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "fixtures", name))
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return v
}

// With deep-copies obj (a JSON object) with top-level keys replaced.
func With(obj any, changes map[string]any) map[string]any {
	raw, _ := json.Marshal(obj)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	for k, v := range changes {
		out[k] = v
	}
	return out
}

// Contents is a contents-API file response holding text.
func Contents(text string) map[string]any {
	return map[string]any{"type": "file", "encoding": "base64", "content": base64.StdEncoding.EncodeToString([]byte(text))}
}

// WriteHub writes repos.txt and checks.txt into dir and returns dir.
func WriteHub(t testing.TB, dir, repos, checks string) string {
	t.Helper()
	for name, text := range map[string]string{"repos.txt": repos, "checks.txt": checks} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}
