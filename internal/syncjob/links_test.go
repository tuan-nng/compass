package syncjob

import (
	"testing"

	"compass/internal/config"
)

// Whether a knowledge PR links its code PR decides whether it may merge.
// Expected values come from the original Python links(), which reads digits
// with \d and int(): any Unicode decimal digit, leading zeros in base 10.
func TestLinksReadsReferencesAsPythonDoes(t *testing.T) {
	repo := &config.Repo{Owner: "acme", Repo: "web-app"}
	target := &pull{Number: 10, HTMLURL: "https://github.com/acme/web-app/pull/10"}
	for body, want := range map[string]bool{
		"#10":             true,
		"#010":            true,
		"#１０":             true,
		"#𝟏𝟎":             true,
		"#100":            false,
		"#10５":            false,
		"fixes #8":        false,
		"acme/web-app#10": true,
		"other/x#10":      false,
		"a#10":            false,
		"https://github.com/acme/web-app/pull/10":  true,
		"https://github.com/acme/web-app/pull/105": false,
		"https://github.com/acme/web-app/pull/10５": false,
	} {
		if got := links(&pull{Body: body}, target, repo); got != want {
			t.Errorf("links(%q) = %v, want %v", body, got, want)
		}
	}
}
