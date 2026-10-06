package frontmatter

// The verified-field editor: every supported form, and refusal of the rest.

import (
	"strings"
	"testing"
	"time"
)

const (
	actor = "process:contract-test"
	newE  = "{ by: process:contract-test, at: 2026-10-05T09:12:34Z }"
)

var at = mustTime("2026-10-05T09:12:34Z")

func mustTime(s string) time.Time {
	t, err := ParseTime(s)
	if err != nil {
		panic(err)
	}
	return t
}

const defaultGenerated = "generated: { by: a/b, at: 2026-06-01T00:00:00Z }\n"

func doc(verified string) string { return docGen(verified, defaultGenerated) }

func docGen(verified, generated string) string {
	return "---\ntype: Gotcha\ntitle: T\n" + generated + verified + "stale_after: 2026-12-01T00:00:00Z\n---\n# T\n"
}

func mustStamp(t *testing.T, text string) (string, string) {
	t.Helper()
	out, how, changed, err := Stamp(text, actor, at)
	if err != nil {
		t.Fatalf("Stamp: %v", err)
	}
	if !changed {
		t.Fatalf("Stamp: nothing changed (%s)", how)
	}
	return out, how
}

func eq(t *testing.T, got, want any) {
	t.Helper()
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestBlockListAppendsInItemStyleAndKeepsHumanBytes(t *testing.T) {
	v := "verified:\n  - by: human:alice   \n    at: '2026-06-12T16:00:00Z'\n"
	out, how := mustStamp(t, doc(v))
	eq(t, how, "added")
	eq(t, out, doc(v+"  - by: process:contract-test\n    at: 2026-10-05T09:12:34Z\n"))
}

func TestBlockListRefreshesOwnEntryInPlace(t *testing.T) {
	v := "verified:\n- { by: process:contract-test, at: 2026-05-01T00:00:00Z }\n" +
		"- by: human:bob\n  at: 2026-06-02T00:00:00Z\n"
	out, how := mustStamp(t, doc(v))
	eq(t, how, "refreshed")
	eq(t, out, doc("verified:\n- "+newE+"\n- by: human:bob\n  at: 2026-06-02T00:00:00Z\n"))
}

func TestFlowListAppendAndRefresh(t *testing.T) {
	human := "{by: human:alice,at: 2026-06-12T16:00:00Z}"
	out, _ := mustStamp(t, doc("verified: ["+human+"]\n"))
	eq(t, out, doc("verified: ["+human+", "+newE+"]\n"))
	old := "verified: [ " + human + " , { by: process:contract-test, at: 2026-01-01 } ]\n"
	out, how := mustStamp(t, doc(old))
	eq(t, how, "refreshed")
	eq(t, out, doc("verified: [ "+human+" , "+newE+" ]\n"))
}

func TestCurrentEntryIsNotRewritten(t *testing.T) {
	v := "verified: { by: process:contract-test, at: 2026-06-01T00:00:00Z }\n"
	out, how, changed, err := Stamp(doc(v), actor, at)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, changed, false)
	eq(t, out, "")
	eq(t, how, "current")
}

func TestBlockGeneratedAndCRLFAreKept(t *testing.T) {
	text := strings.ReplaceAll(docGen("", "generated:\n  by: a/b\n  at: 2026-06-01T00:00:00Z\n"), "\n", "\r\n")
	out, _ := mustStamp(t, text)
	eq(t, out, strings.ReplaceAll(text, "2026-06-01T00:00:00Z\r\n", "2026-06-01T00:00:00Z\r\nverified: "+newE+"\r\n"))
}

func TestUnsupportedFormsRaise(t *testing.T) {
	bad := []string{
		"verified:\n  by: human:alice\n  at: 2026-06-12T16:00:00Z\n",                  // block map
		"verified: human:alice\n",                                                     // scalar
		"verified: { by: human:alice }\n",                                             // no at
		"verified: { by: human:alice, at: soon }\n",                                   // bad date
		"verified: { by: human:alice, at: 2026-06-12, note: x }\n",                    // extra key
		"verified:\n  - by: human:alice\n    at: 2026-06-12\n    extra:\n      - x\n", // nested
		"verified: { by: human:alice, at: 2026-06-12 }  # reviewed\n",                 // comment
		"verified:\n", // empty
		"verified: [{ by: a, at: 2026-01-01 }, { by: process:contract-test, at: 2026-01-01 }," +
			" { by: process:contract-test, at: 2026-02-01 }]\n", // actor twice
	}
	for _, v := range bad {
		t.Run(v, func(t *testing.T) {
			if _, _, _, err := Stamp(doc(v), actor, at); !IsError(err) {
				t.Errorf("want *Error, got %v", err)
			}
		})
	}
	if _, _, _, err := Stamp("# no frontmatter\n", actor, at); !IsError(err) {
		t.Errorf("no frontmatter: want *Error, got %v", err)
	}
}
