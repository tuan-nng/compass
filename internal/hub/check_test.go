package hub

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"compass/internal/okftest"
)

// templateHub copies the shipped hub template into a fresh git repo, adds
// the given files, runs `okf index .` at its root when index is set, and
// commits everything.
func templateHub(t *testing.T, files map[string]string, index bool) string {
	t.Helper()
	okf := okftest.Binary(t)
	dir := filepath.Join(t.TempDir(), "hub")
	if err := os.CopyFS(dir, os.DirFS("../../templates/hub")); err != nil {
		t.Fatal(err)
	}
	for name, text := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if index {
		run(t, dir, okf, "index", ".")
	}
	run(t, dir, "git", "init", "-q")
	commit(t, dir)
	return dir
}

func commit(t *testing.T, dir string) {
	t.Helper()
	run(t, dir, "git", "add", "-A")
	run(t, dir, "git", "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-qm", "hub")
}

func run(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, out)
	}
	return string(out)
}

const overview = `---
type: Overview
title: Billing API
description: What the billing API does.
tags: [overview]
status: stable
generated: { by: process:test, at: 2026-10-21T00:00:00Z }
---
# Billing API

Bills customers. See [the hub](/overview.md).
`

// check runs Check and fails the test if the folder changed.
func check(t *testing.T, dir string) []string {
	t.Helper()
	found, err := Check(dir)
	if err != nil {
		t.Fatal(err)
	}
	if st := run(t, dir, "git", "status", "--porcelain", "--untracked-files=all"); st != "" {
		t.Errorf("check changed the hub:\n%s", st)
	}
	var out []string
	for _, f := range found {
		out = append(out, f.String())
	}
	return out
}

func TestTemplateHubWithoutReposPasses(t *testing.T) {
	if got := check(t, templateHub(t, nil, false)); len(got) != 0 {
		t.Errorf("findings: %q", got)
	}
}

func TestRepoFolderPasses(t *testing.T) {
	dir := templateHub(t, map[string]string{"repos/billing-api/overview.md": overview}, true)
	if got := check(t, dir); len(got) != 0 {
		t.Errorf("findings: %q", got)
	}
}

// A stale index fails and names the file; nothing in the hub is rewritten.
func TestStaleIndexFails(t *testing.T) {
	for name, stale := range map[string]string{
		"repo folder": "repos/billing-api/index.md",
		"repos":       "repos/index.md",
	} {
		t.Run(name, func(t *testing.T) {
			dir := templateHub(t, map[string]string{"repos/billing-api/overview.md": overview}, true)
			p := filepath.Join(dir, stale)
			if err := os.WriteFile(p, []byte("# Index\n\nOld.\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			commit(t, dir)
			got := check(t, dir)
			if len(got) != 1 || !strings.HasPrefix(got[0], stale+": is stale") {
				t.Errorf("findings: %q, want %s stale", got, stale)
			}
		})
	}
}

func TestMissingIndexFails(t *testing.T) {
	dir := templateHub(t, map[string]string{"repos/billing-api/overview.md": overview}, false)
	got := check(t, dir)
	for _, want := range []string{"index.md: is stale", "repos/index.md: is missing", "repos/billing-api/index.md: is missing"} {
		if !strings.Contains(strings.Join(got, "\n"), want) {
			t.Errorf("findings %q lack %q", got, want)
		}
	}
}

// A repo folder with no concept files fails, as a folder okf skips would.
func TestEmptyRepoFolderFails(t *testing.T) {
	dir := templateHub(t, map[string]string{"repos/billing-api/overview.md": overview}, true)
	if err := os.Mkdir(filepath.Join(dir, "repos", "web-app"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := check(t, dir)
	if len(got) == 0 || !strings.Contains(strings.Join(got, "\n"), "repos/web-app: holds no concept files") {
		t.Errorf("findings: %q", got)
	}
}

// A symlink anywhere fails, and no other check reads through it.
func TestSymlinkFails(t *testing.T) {
	secret := filepath.Join(t.TempDir(), "secret.md")
	if err := os.WriteFile(secret, []byte("---\ntitle: s3cr3t\n---\ns3cr3t\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, link := range map[string]string{
		"inside repos":  "repos/billing-api/leak.md",
		"outside repos": "decisions/leak.md",
		"a folder":      "glossary",
	} {
		t.Run(name, func(t *testing.T) {
			dir := templateHub(t, map[string]string{"repos/billing-api/overview.md": overview}, true)
			target := secret
			if name == "a folder" {
				target = filepath.Dir(secret)
			}
			if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, link)), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, filepath.Join(dir, link)); err != nil {
				t.Fatal(err)
			}
			commit(t, dir)
			got := check(t, dir)
			if len(got) != 1 || got[0] != link+": is a symlink; a hub holds no links" {
				t.Errorf("findings: %q", got)
			}
		})
	}
}
