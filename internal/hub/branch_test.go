package hub

import (
	"path/filepath"
	"testing"

	"compass/internal/config"
)

func TestBranchRefs(t *testing.T) {
	refs := []string{
		"refs/heads/main",
		"refs/heads/billing/feat",
		"refs/heads/billing/kept",
		"refs/remotes/origin/HEAD",
		"refs/remotes/origin/main",
		"refs/remotes/origin/billing/kept",
		"refs/remotes/origin/billing/remote",
		"refs/remotes/origin/billing/wip",
		"refs/remotes/origin/web",
		"refs/remotes/upstream/billing/feat/x",
	}
	for _, tc := range []struct {
		hb            string
		local, remote bool
		conflict      string
	}{
		{"billing/new", false, false, ""},
		{"billing/kept", true, true, ""},
		{"billing/remote", false, true, ""},
		{"billing/feat/x", false, false, "billing/feat"},
		{"billing/wip/x", false, false, "origin/billing/wip"},
		{"billing", false, false, "billing/feat"},
		{"web/feat", false, false, "origin/web"},
		{"HEAD/x", false, false, ""},
	} {
		local, remote, conflict := branchRefs(refs, tc.hb)
		if local != tc.local || remote != tc.remote || conflict != tc.conflict {
			t.Errorf("branchRefs(%q) = %v, %v, %q; want %v, %v, %q",
				tc.hb, local, remote, conflict, tc.local, tc.remote, tc.conflict)
		}
	}
}

func TestMatchRow(t *testing.T) {
	repos, err := config.ParseRepos([]byte("billing https://github.com/acme/billing-api\nweb https://github.com/acme/web.git\n"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		cands []string
		want  string
	}{
		{[]string{"Acme/Billing-API"}, "billing"},
		{[]string{"acme/other", "acme/web"}, "web"},
		{[]string{"acme/web", "acme/billing-api"}, "web"},
		{[]string{"acme/other"}, ""},
		{nil, ""},
	} {
		got := ""
		if r := matchRow(repos, tc.cands); r != nil {
			got = r.Name
		}
		if got != tc.want {
			t.Errorf("matchRow(%q) = %q, want %q", tc.cands, got, tc.want)
		}
	}
}

func TestWorktreePath(t *testing.T) {
	hub := filepath.FromSlash("/src/hub")
	want := filepath.FromSlash("/src/hub.worktrees/billing/feat/retry")
	for _, h := range []string{hub, hub + string(filepath.Separator)} {
		if got := worktreePath(h, "billing/feat/retry"); got != want {
			t.Errorf("worktreePath(%q) = %q, want %q", h, got, want)
		}
	}
}
