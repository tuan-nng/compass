package validate

import (
	"bytes"
	"strings"
	"testing"
)

// The argument grammar is the only part that intentionally differs from the
// Python script (argparse, behind the okf-strict-validate wrapper), so the
// differential test does not cover it.
func TestMainArguments(t *testing.T) {
	cases := []struct {
		args        []string
		code        int
		stdout, err string
	}{
		{nil, 2, "", usage + "\n"},
		{[]string{"a", "b"}, 2, "", usage + "\n"},
		{[]string{"--json"}, 2, "", usage + "\n"},
		{[]string{"--migrate", "x"}, 2, "", "compass validate: unknown option --migrate\n" + usage + "\n"},
		{[]string{"--max-warnings", "3", "x"}, 2, "", "compass validate: unknown option --max-warnings\n" + usage + "\n"},
		{[]string{"-h"}, 0, usage, ""},
		{[]string{"x", "--help"}, 0, usage, ""},
		{[]string{"../../testdata/proto/billing-api/okf", "--strict"}, 0, "✓ conformant — no issues", ""},
		{[]string{"--", "../../testdata/proto/web-app/okf/"}, 0, "OKF v0.2 conformance — ../../testdata/proto/web-app/okf\n", ""},
	}
	for _, c := range cases {
		var out, errb bytes.Buffer
		code := Main(c.args, &out, &errb)
		if code != c.code || !strings.Contains(out.String(), c.stdout) || errb.String() != c.err {
			t.Errorf("Main(%q) = %d, stdout %q, stderr %q; want %d, stdout containing %q, stderr %q",
				c.args, code, out.String(), errb.String(), c.code, c.stdout, c.err)
		}
	}
}

// test/check-fixture.sh counts the three nested-index findings on the
// prototype hub and needs every prototype bundle to pass; this pins both
// without uv.
func TestFixtureExpectations(t *testing.T) {
	var out bytes.Buffer
	if code := Main([]string{"../../testdata/fixture"}, &out, &bytes.Buffer{}); code != 1 {
		t.Fatalf("fixture: exit %d, want 1 (strict: warnings fail)", code)
	}
	if n := strings.Count(out.String(), "index.md should contain no frontmatter"); n != 4 {
		t.Errorf("fixture: %d nested-index findings, want 4:\n%s", n, out.String())
	}
	for _, b := range []string{"billing-api", "shared-auth", "web-app"} {
		if code := Main([]string{"../../testdata/proto/" + b + "/okf"}, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
			t.Errorf("proto %s: exit %d, want 0", b, code)
		}
	}
}
