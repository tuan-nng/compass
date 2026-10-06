// Package github is a minimal GitHub REST client: one HTTP transport,
// pagination, no retries.
//
// Every request goes through Client.HTTP, which tests replace with a fake
// http.RoundTripper. Any HTTP error, network error or rate limit returns an
// *HTTPError; the caller logs it, names the repo, and exits non-zero so the
// next scheduled run retries (plan Failure modes).
package github

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// DefaultAPIURL is the public GitHub REST API base.
const DefaultAPIURL = "https://api.github.com"

// HTTPError is a non-2xx response, a network failure (Status 0), or a rate
// limit.
type HTTPError struct {
	Method  string
	URL     string
	Status  int
	Message string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("%s %s: HTTP %d: %s", e.Method, e.URL, e.Status, e.Message)
}

// ErrDryRunWrite is returned when a write call is attempted in dry-run mode.
var ErrDryRunWrite = errors.New("dry-run: write refused")

// IsDryRunWrite reports whether err came from a refused dry-run write.
func IsDryRunWrite(err error) bool { return errors.Is(err, ErrDryRunWrite) }

// Client calls the REST API with one token.
type Client struct {
	Token   string
	BaseURL string
	HTTP    *http.Client
	DryRun  bool
}

// New returns a client for baseURL (DefaultAPIURL when empty).
func New(token, baseURL string, dryRun bool) *Client {
	if baseURL == "" {
		baseURL = DefaultAPIURL
	}
	return &Client{
		Token:   token,
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTP:    &http.Client{Timeout: 30 * time.Second},
		DryRun:  dryRun,
	}
}

func isWrite(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete:
		return true
	}
	return false
}

// QuotePath percent-encodes a path piece such as a branch name, keeping its
// slashes. Only ASCII letters, digits, "_.-~" and "/" stay as they are.
func QuotePath(s string) string {
	var b strings.Builder
	for i := range len(s) {
		c := s[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.IndexByte("_.-~/", c) >= 0 {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

func (c *Client) url(path string, params url.Values) string {
	u := path
	if !strings.HasPrefix(path, "http://") && !strings.HasPrefix(path, "https://") {
		u = c.BaseURL + path
	}
	if len(params) > 0 {
		sep := "?"
		if strings.Contains(u, "?") {
			sep = "&"
		}
		u += sep + params.Encode()
	}
	return u
}

func header(h http.Header, name string) (string, bool) {
	v, ok := h[http.CanonicalHeaderKey(name)]
	if !ok || len(v) == 0 {
		return "", false
	}
	return v[0], true
}

// Do sends one request. A non-nil body is sent as JSON. When out is non-nil
// and the response has a body, it is decoded into out. A status listed in
// okStatuses is returned instead of an error.
func (c *Client) Do(method, path string, params url.Values, body, out any, okStatuses ...int) (int, http.Header, error) {
	if isWrite(method) && c.DryRun {
		return 0, nil, fmt.Errorf("%w: %s %s", ErrDryRunWrite, method, path)
	}
	u := c.url(path, params)
	if !strings.HasPrefix(u, c.BaseURL+"/") {
		return 0, nil, &HTTPError{method, u, 0, "refusing a URL outside the API base"}
	}
	var rd io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		rd = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, u, rd)
	if err != nil {
		return 0, nil, &HTTPError{method, u, 0, err.Error()}
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("User-Agent", "compass")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, nil, &HTTPError{method, u, 0, "network error: " + err.Error()}
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, &HTTPError{method, u, 0, "network error: " + err.Error()}
	}
	ok := resp.StatusCode >= 200 && resp.StatusCode < 300
	for _, s := range okStatuses {
		ok = ok || resp.StatusCode == s
	}
	if ok {
		if out != nil && len(raw) > 0 {
			if err := json.Unmarshal(raw, out); err != nil {
				return resp.StatusCode, resp.Header, &HTTPError{method, u, resp.StatusCode, "bad JSON: " + err.Error()}
			}
		}
		return resp.StatusCode, resp.Header, nil
	}
	var msg struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(raw, &msg)
	message := msg.Message
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		remaining, _ := header(resp.Header, "x-ratelimit-remaining")
		_, retry := header(resp.Header, "retry-after")
		if remaining == "0" || retry || strings.Contains(strings.ToLower(message), "rate limit") {
			message = strings.TrimRight("rate limited: "+message, ": ")
		}
	}
	if message == "" {
		message = "request failed"
	}
	return resp.StatusCode, resp.Header, &HTTPError{method, u, resp.StatusCode, message}
}

// Get fetches one resource into out.
func (c *Client) Get(path string, params url.Values, out any) error {
	_, _, err := c.Do(http.MethodGet, path, params, nil, out)
	return err
}

// GetOptional fetches one resource into out; a 404 returns found=false.
func (c *Client) GetOptional(path string, params url.Values, out any) (found bool, err error) {
	status, _, err := c.Do(http.MethodGet, path, params, nil, out, http.StatusNotFound)
	if err != nil {
		return false, err
	}
	return status != http.StatusNotFound, nil
}

// Post sends a POST with a JSON body.
func (c *Client) Post(path string, body, out any, okStatuses ...int) (int, error) {
	s, _, err := c.Do(http.MethodPost, path, nil, body, out, okStatuses...)
	return s, err
}

// Patch sends a PATCH with a JSON body.
func (c *Client) Patch(path string, body, out any, okStatuses ...int) (int, error) {
	s, _, err := c.Do(http.MethodPatch, path, nil, body, out, okStatuses...)
	return s, err
}

// Put sends a PUT with a JSON body.
func (c *Client) Put(path string, body, out any, okStatuses ...int) (int, error) {
	s, _, err := c.Do(http.MethodPut, path, nil, body, out, okStatuses...)
	return s, err
}

// Delete sends a DELETE.
func (c *Client) Delete(path string, okStatuses ...int) (int, error) {
	s, _, err := c.Do(http.MethodDelete, path, nil, nil, nil, okStatuses...)
	return s, err
}

var nextLink = regexp.MustCompile(`<([^>]+)>\s*;\s*rel="next"`)

// Paginate GETs every page, following Link rel="next", and decodes each item
// as T. A non-empty key picks the list out of wrapped responses such as
// {"workflow_runs": [...]}. per_page defaults to 100.
func Paginate[T any](c *Client, path string, params url.Values, key string) ([]T, error) {
	p := url.Values{}
	for k, v := range params {
		p[k] = v
	}
	if p.Get("per_page") == "" {
		p.Set("per_page", "100")
	}
	next := c.url(path, p)
	var items []T
	for next != "" {
		var raw json.RawMessage
		_, h, err := c.Do(http.MethodGet, next, nil, nil, &raw)
		if err != nil {
			return nil, err
		}
		page := raw
		if key != "" {
			var wrapped map[string]json.RawMessage
			if err := json.Unmarshal(raw, &wrapped); err != nil {
				return nil, &HTTPError{"GET", next, 0, "expected a list response"}
			}
			page = wrapped[key]
		}
		var got []T
		if len(page) == 0 || json.Unmarshal(page, &got) != nil || bytes.Equal(bytes.TrimSpace(page), []byte("null")) {
			if key != "" && len(page) == 0 {
				got = nil // a wrapped response without the key: no items
			} else {
				return nil, &HTTPError{"GET", next, 0, "expected a list response"}
			}
		}
		items = append(items, got...)
		next = ""
		if link, ok := header(h, "link"); ok {
			if m := nextLink.FindStringSubmatch(link); m != nil {
				next = m[1]
			}
		}
	}
	return items, nil
}
