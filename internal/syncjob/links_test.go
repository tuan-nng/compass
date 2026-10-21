package syncjob

import "testing"

// Whether a hub pull request and its code pull request link each other
// decides whether the hub one may merge. A reference names the target's repo,
// which is the other pull request's (plan decision 5); a short #N never
// counts. Digits read as the original Python links() read them, with \d and
// int(): any Unicode decimal digit, leading zeros in base 10.
func TestLinksReadsReferencesAsPythonDoes(t *testing.T) {
	target := &pull{Number: 10, HTMLURL: "https://github.com/acme/web-app/pull/10"}
	for body, want := range map[string]bool{
		"#10":                  false,
		"acme/web-app#10":      true,
		"ACME/Web-App#10":      true,
		"acme/web-app#010":     true,
		"acme/web-app#１０":      true,
		"acme/web-app#𝟏𝟎":      true,
		"acme/web-app#100":     false,
		"acme/web-app#10５":     false,
		"fixes acme/web-app#8": false,
		"acme/hub#10":          false,
		"other/x#10":           false,
		"a#10":                 false,
		"xacme/web-app#10":     false,
		"(acme/web-app#10)":    true,
		"https://github.com/acme/web-app/pull/10":  true,
		"https://github.com/acme/web-app/pull/105": false,
		"https://github.com/acme/web-app/pull/10５": false,
		"https://github.com/acme/hub/pull/10":      false,
	} {
		if got := links(&pull{Body: body}, target, "acme/web-app"); got != want {
			t.Errorf("links(%q) = %v, want %v", body, got, want)
		}
	}
}

func TestPathRule(t *testing.T) {
	for path, want := range map[string]bool{
		"repos/a/x.md":             true,
		"repos/a/deep/x.md":        true,
		"repos/index.md":           true,
		"index.md":                 true,
		"decisions/x.md":           true,
		"repos/a-b/x.md":           false,
		"repos/b/x.md":             false,
		"repos/x.md":               false,
		"repos":                    false,
		"repos.txt":                false,
		"checks.txt":               false,
		"CODEOWNERS":               false,
		"docs/CODEOWNERS":          false,
		"repos/a/.gitattributes":   false,
		".gitattributes":           false,
		"repos/a/.gitmodules":      false,
		".gitignore":               false,
		".github/workflows/ci.yml": false,
		"Repos/a/x.md":             false,
		"REPOS/index.md":           false,
		"repos/A/x.md":             false,
		"Repos.txt":                false,
		"CHECKS.TXT":               false,
		".GitIgnore":               false,
		".GITHUB/workflows/ci.yml": false,
		"docs/repos.txt":           true,
	} {
		if got := pathAllowed("a", path); got != want {
			t.Errorf("pathAllowed(a, %q) = %v, want %v", path, got, want)
		}
	}
}
