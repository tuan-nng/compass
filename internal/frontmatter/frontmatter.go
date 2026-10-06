// Package frontmatter edits a concept's "verified" frontmatter field as text.
//
// Only the verified field is ever rewritten; every other byte of the file is
// kept. Entries from other verifiers (human: and other process: actors) keep
// their text byte-for-byte. Supported forms:
//
//   - one-line flow map:  verified: { by: X, at: Y }
//   - one-line flow list: verified: [{ by: X, at: Y }, ...]
//   - block list, items either "- by: X" + "  at: Y" or "- { by: X, at: Y }"
//
// Anything else returns an *Error and the caller leaves the file alone.
package frontmatter

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Error is an unsupported or malformed frontmatter form.
type Error struct{ msg string }

func (e *Error) Error() string { return e.msg }

func errorf(format string, a ...any) error { return &Error{fmt.Sprintf(format, a...)} }

// pyRepr quotes s the way Python's repr() does for the common cases the
// messages show (single quotes unless s holds a single quote and no double).
func pyRepr(s string) string {
	q := byte('\'')
	if strings.Contains(s, "'") && !strings.Contains(s, "\"") {
		q = '"'
	}
	var b strings.Builder
	b.WriteByte(q)
	for _, r := range s {
		switch {
		case r == '\\':
			b.WriteString(`\\`)
		case r == rune(q):
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\x%02x`, r)
		case r >= 0x80 && !unicode.IsPrint(r):
			switch {
			case r <= 0xff:
				fmt.Fprintf(&b, `\x%02x`, r)
			case r <= 0xffff:
				fmt.Fprintf(&b, `\u%04x`, r)
			default:
				fmt.Fprintf(&b, `\U%08x`, r)
			}
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte(q)
	return b.String()
}

// isSpace is Python's str.isspace for one rune.
func isSpace(r rune) bool { return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f) }

// strip is Python's str.strip() with no argument.
func strip(s string) string { return strings.TrimFunc(s, isSpace) }

func lstrip(s string) string { return strings.TrimLeftFunc(s, isSpace) }

func rstrip(s string) string { return strings.TrimRightFunc(s, isSpace) }

// rstripNL is Python's s.rstrip("\r\n").
func rstripNL(s string) string { return strings.TrimRight(s, "\r\n") }

// splitLines is Python's str.splitlines(keepends=True).
func splitLines(text string) []string {
	var out []string
	start := 0
	for i := 0; i < len(text); {
		r, size := rune(text[i]), 1
		if r >= 0x80 {
			r, size = utf8.DecodeRuneInString(text[i:])
		}
		end := -1
		switch r {
		case '\r':
			if i+1 < len(text) && text[i+1] == '\n' {
				end = i + 2
			} else {
				end = i + 1
			}
		case '\n', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
			end = i + size
		}
		if end >= 0 {
			out = append(out, text[start:end])
			start, i = end, end
			continue
		}
		i += size
	}
	if start < len(text) {
		out = append(out, text[start:])
	}
	return out
}

// ParseTime reads an RFC 3339 datetime (anything Python's
// datetime.fromisoformat accepts) or a YYYY-MM-DD date as a UTC time.
// A value without a zone is UTC.
func ParseTime(value string) (time.Time, error) {
	s := strip(value)
	bad := errorf("not a datetime: %s", pyRepr(value))
	if dateRe.MatchString(s) {
		t, err := time.Parse("2006-01-02", s)
		if err != nil || t.Year() < 1 {
			return time.Time{}, bad
		}
		return t, nil
	}
	if strings.HasSuffix(s, "Z") || strings.HasSuffix(s, "z") {
		s = s[:len(s)-1] + "+00:00"
	}
	t, ok := fromISOFormat(s)
	if !ok {
		return time.Time{}, bad
	}
	return t.UTC(), nil
}

var dateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// FormatTime renders t as YYYY-MM-DDTHH:MM:SSZ in UTC. Like Python's
// strftime("%Y") on glibc, years before 1000 are not zero-padded.
func FormatTime(t time.Time) string {
	t = t.UTC()
	return fmt.Sprintf("%d", t.Year()) + t.Format("-01-02T15:04:05Z")
}

func digits(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	n := 0
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
		n = n*10 + int(s[i]-'0')
	}
	return n, true
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

// fromISOFormat ports CPython's datetime.fromisoformat (3.11+).
func fromISOFormat(s string) (time.Time, bool) {
	if len(s) < 7 {
		return time.Time{}, false
	}
	sep, ok := isoSeparator(s)
	if !ok {
		return time.Time{}, false
	}
	if sep < len(s) && sep+1 == len(s) {
		return time.Time{}, false
	}
	dstr, tstr := s[:min(sep, len(s))], ""
	if sep+1 < len(s) {
		tstr = s[sep+1:]
	}
	y, mo, d, ok := isoDate(dstr)
	if !ok {
		return time.Time{}, false
	}
	var hh, mm, ss, us int
	var off time.Duration
	if tstr != "" {
		if hh, mm, ss, us, off, ok = isoTime(tstr); !ok {
			return time.Time{}, false
		}
	}
	if y < 1 || mo < 1 || mo > 12 || d < 1 || d > daysIn(y, mo) || hh > 23 || mm > 59 || ss > 59 {
		return time.Time{}, false
	}
	return time.Date(y, time.Month(mo), d, hh, mm, ss, us*1000, time.UTC).Add(-off), true
}

func daysIn(y, m int) int {
	return time.Date(y, time.Month(m)+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func isoSeparator(s string) (int, bool) {
	n := len(s)
	if n == 7 {
		return 7, true
	}
	if s[4] == '-' {
		if s[5] == 'W' {
			if n < 8 {
				return 0, false
			}
			if n > 8 && s[8] == '-' {
				if n == 9 {
					return 0, false
				}
				if n > 10 && isDigit(s[10]) {
					return 8, true
				}
				return 10, true
			}
			return 8, true
		}
		return 10, true
	}
	if s[4] == 'W' {
		idx := 7
		for idx < n && isDigit(s[idx]) {
			idx++
		}
		if idx < 9 {
			return idx, true
		}
		if idx%2 == 0 {
			return 7, true
		}
		return 8, true
	}
	return 8, true
}

func isoDate(s string) (y, m, d int, ok bool) {
	if len(s) != 7 && len(s) != 8 && len(s) != 10 {
		return 0, 0, 0, false
	}
	if y, ok = digits(s[0:4]); !ok {
		return
	}
	hasSep := s[4] == '-'
	pos := 4
	if hasSep {
		pos++
	}
	at := func(i int) string {
		if i < len(s) {
			return s[i : i+1]
		}
		return ""
	}
	if at(pos) == "W" {
		pos++
		if pos+2 > len(s) {
			return 0, 0, 0, false
		}
		week, ok1 := digits(s[pos : pos+2])
		if !ok1 {
			return 0, 0, 0, false
		}
		pos += 2
		day := 1
		if len(s) > pos {
			if (at(pos) == "-") != hasSep {
				return 0, 0, 0, false
			}
			if hasSep {
				pos++
			}
			if pos+1 > len(s) {
				return 0, 0, 0, false
			}
			if day, ok1 = digits(s[pos : pos+1]); !ok1 {
				return 0, 0, 0, false
			}
		}
		return isoWeek(y, week, day)
	}
	if pos+2 > len(s) {
		return 0, 0, 0, false
	}
	if m, ok = digits(s[pos : pos+2]); !ok {
		return
	}
	pos += 2
	if (at(pos) == "-") != hasSep {
		return 0, 0, 0, false
	}
	if hasSep {
		pos++
	}
	if pos+2 > len(s) {
		return 0, 0, 0, false
	}
	d, ok = digits(s[pos : pos+2])
	return
}

func isoWeek(y, week, day int) (int, int, int, bool) {
	if y < 1 || y > 9999 || week < 1 || day < 1 || day > 7 {
		return 0, 0, 0, false
	}
	if week == 53 {
		jan1 := time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC).Weekday()
		if !(jan1 == time.Thursday || (jan1 == time.Wednesday && daysIn(y, 2) == 29)) {
			return 0, 0, 0, false
		}
	} else if week > 53 {
		return 0, 0, 0, false
	}
	// Monday of ISO week 1 is the Monday of the week holding January 4.
	jan4 := time.Date(y, 1, 4, 0, 0, 0, 0, time.UTC)
	wd := int(jan4.Weekday()+6) % 7
	t := jan4.AddDate(0, 0, -wd+(week-1)*7+(day-1))
	return t.Year(), int(t.Month()), t.Day(), true
}

var fractionCorrection = []int{100000, 10000, 1000, 100, 10}

// hhmmssff ports CPython's C parse_hh_mm_ss_ff for s[p:end], reading
// HH[:?MM[:?SS[{.,}fff[fff]]]]. Like the C code it may look at bytes past
// end (the zone that follows); past the string it sees NUL. rv is negative
// on error, 1 when unparsed text is left, else 0.
func hhmmssff(s string, p, end int) (c [4]int, rv int) {
	at := func(i int) byte {
		if i < len(s) {
			return s[i]
		}
		return 0
	}
	digitsAt := func(i, n int) (int, bool) {
		v := 0
		for k := range n {
			b := at(i + k)
			if !isDigit(b) {
				return 0, false
			}
			v = v*10 + int(b-'0')
		}
		return v, true
	}
	hasSep := true
	for comp := range 3 {
		v, ok := digitsAt(p, 2)
		if !ok {
			return c, -3
		}
		c[comp] = v
		p += 2
		ch := at(p)
		p++
		if comp == 0 {
			hasSep = ch == ':'
		}
		if p >= end {
			if ch != 0 {
				return c, 1
			}
			return c, 0
		}
		if hasSep && ch == ':' {
			continue
		}
		if ch == '.' || ch == ',' {
			break
		}
		if !hasSep {
			p--
			continue
		}
		return c, -4
	}
	n := min(end-p, 6)
	v, ok := digitsAt(p, n)
	if !ok {
		return c, -3
	}
	c[3] = v
	if n < 6 {
		c[3] *= fractionCorrection[n-1]
	}
	p += n
	for isDigit(at(p)) {
		p++
	}
	if at(p) != 0 {
		return c, 1
	}
	return c, 0
}

func isoTime(s string) (hh, mm, ss, us int, off time.Duration, ok bool) {
	if len(s) < 2 {
		return
	}
	tz := strings.IndexAny(s, "+-Z")
	tzEnd := tz
	if tz < 0 {
		tzEnd = len(s)
	}
	c, rv := hhmmssff(s, 0, tzEnd)
	if rv < 0 || (tz < 0 && rv == 1) {
		return
	}
	hh, mm, ss, us = c[0], c[1], c[2], c[3]
	if tz >= 0 {
		if s[tz] == 'Z' {
			if tz != len(s)-1 {
				return
			}
		} else {
			tzstr := s[tz+1:]
			if l := len(tzstr); l == 0 || l == 1 || l == 3 {
				return
			}
			z, rv2 := hhmmssff(s, tz+1, len(s))
			if rv2 != 0 {
				return
			}
			d := time.Duration(z[0])*time.Hour + time.Duration(z[1])*time.Minute +
				time.Duration(z[2])*time.Second + time.Duration(z[3])*time.Microsecond
			if d >= 24*time.Hour {
				return
			}
			if s[tz] == '-' {
				d = -d
			}
			// CPython reads an offset whose hours, minutes and seconds are
			// all zero as UTC, dropping any fraction.
			if z[0] != 0 || z[1] != 0 || z[2] != 0 {
				off = d
			}
		}
	}
	ok = true
	return
}

// scalar reads one plain or simply quoted YAML scalar.
func scalar(raw string) (string, error) {
	s := strip(raw)
	if len(s) >= 2 && s[0] == s[len(s)-1] && (s[0] == '\'' || s[0] == '"') {
		inner := s[1 : len(s)-1]
		if s[0] == '"' && strings.Contains(inner, "\\") {
			return "", errorf("escaped string not supported: %s", pyRepr(raw))
		}
		if strings.IndexByte(inner, s[0]) >= 0 {
			return "", errorf("quoted string not supported: %s", pyRepr(raw))
		}
		return inner, nil
	}
	if s == "" || strings.IndexByte("{[&*!|>'\"%@`", s[0]) >= 0 || strings.Contains(s, " #") {
		return "", errorf("value not supported: %s", pyRepr(raw))
	}
	return s, nil
}

type span struct{ a, b int }

// splitTop splits s on commas outside quotes and brackets.
func splitTop(s string) ([]span, error) {
	var spans []span
	depth, start := 0, 0
	var quote byte
	for i := range len(s) {
		ch := s[i]
		switch {
		case quote != 0:
			if ch == quote {
				quote = 0
			}
		case ch == '\'' || ch == '"':
			quote = ch
		case ch == '{' || ch == '[':
			depth++
		case ch == '}' || ch == ']':
			depth--
			if depth < 0 {
				return nil, errorf("unbalanced brackets")
			}
		case ch == ',' && depth == 0:
			spans = append(spans, span{start, i})
			start = i + 1
		}
	}
	if quote != 0 || depth != 0 {
		return nil, errorf("unbalanced quotes or brackets")
	}
	return append(spans, span{start, len(s)}), nil
}

const ws = `[\t\n\v\f\r \x{1c}-\x{1f}\x{85}\p{Z}]`

var (
	word     = `[A-Za-z_][\p{L}\p{N}_-]*`
	flowKey  = regexp.MustCompile(`(?s)^(` + word + `)` + ws + `*:` + ws + `+(.*)$`)
	keyRe    = regexp.MustCompile(`^(` + word + `):(?:[ \t]+(.*?))?[ \t]*\r?\n?$`)
	genLine  = regexp.MustCompile(`^[ ]+(` + word + `):[ \t]+(.*?)[ \t]*$`)
	dashRe   = regexp.MustCompile(`^( *)- `)
	bodyKey  = regexp.MustCompile(`^(` + word + `):[ \t]+(.*)$`)
	pyFields = func(m map[string]string, order []string) string {
		parts := make([]string, len(order))
		for i, k := range order {
			parts[i] = pyRepr(k) + ": " + pyRepr(m[k])
		}
		return "{" + strings.Join(parts, ", ") + "}"
	}
)

// fields is an insertion-ordered string map, for Python-like messages.
type fields struct {
	m     map[string]string
	order []string
}

func newFields() *fields { return &fields{m: map[string]string{}} }

func (f *fields) set(k, v string) {
	if _, ok := f.m[k]; !ok {
		f.order = append(f.order, k)
	}
	f.m[k] = v
}

func (f *fields) has(k string) bool { _, ok := f.m[k]; return ok }

func (f *fields) String() string { return pyFields(f.m, f.order) }

// flowMap parses "{ key: value, ... }" with scalar values.
func flowMap(s string) (*fields, error) {
	s = strip(s)
	if !(strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")) {
		return nil, errorf("not a flow map: %s", pyRepr(s))
	}
	inner := ""
	if len(s) > 1 {
		inner = s[1 : len(s)-1]
	}
	if strings.ContainsAny(inner, "{[]}") {
		return nil, errorf("nested value not supported: %s", pyRepr(s))
	}
	spans, err := splitTop(inner)
	if err != nil {
		return nil, err
	}
	out := newFields()
	for _, sp := range spans {
		piece := strip(inner[sp.a:sp.b])
		m := flowKey.FindStringSubmatch(piece)
		if m == nil {
			return nil, errorf("bad flow map entry %s", pyRepr(piece))
		}
		if out.has(m[1]) {
			return nil, errorf("duplicate key %s", pyRepr(m[1]))
		}
		v, err := scalar(m[2])
		if err != nil {
			return nil, err
		}
		out.set(m[1], v)
	}
	return out, nil
}

type entry struct {
	by    string
	at    time.Time
	style string // "flow" or "block"
	first int    // line index (block list) or byte offset (flow list)
	last  int
}

type verified struct {
	kind       string // "absent", "flowmap", "flowlist", "blocklist"
	start, end int
	entries    []*entry
	dashIndent string
	listText   string
	listOffset int
}

func newEntry(f *fields, style string, first, last int) (*entry, error) {
	keys := append([]string(nil), f.order...)
	sort.Strings(keys)
	if len(keys) != 2 || keys[0] != "at" || keys[1] != "by" {
		return nil, errorf("verified entry must have exactly by and at: %s", f)
	}
	at, err := ParseTime(f.m["at"])
	if err != nil {
		return nil, err
	}
	return &entry{f.m["by"], at, style, first, last}, nil
}

// split returns the lines and the frontmatter bounds lines[a:b].
func split(text string) (lines []string, a, b int, err error) {
	lines = splitLines(text)
	if len(lines) == 0 || rstripNL(lines[0]) != "---" {
		return nil, 0, 0, errorf("no frontmatter")
	}
	for i := 1; i < len(lines); i++ {
		if rstripNL(lines[i]) == "---" {
			return lines, 1, i, nil
		}
	}
	return nil, 0, 0, errorf("frontmatter is not closed")
}

func topKeys(lines []string, a, b int, name string) []int {
	var out []int
	for i := a; i < b; i++ {
		if m := keyRe.FindStringSubmatch(lines[i]); m != nil && m[1] == name {
			out = append(out, i)
		}
	}
	return out
}

// blockEnd is the index after the indented block following the key on line i.
func blockEnd(lines []string, i, b int) int {
	j := i + 1
	for j < b && lines[j] != "" && strings.IndexByte(" \t-", lines[j][0]) >= 0 {
		j++
	}
	return j
}

func keyValue(line string) string {
	return strip(keyRe.FindStringSubmatch(line)[2])
}

// generatedAt is generated.at, or ok=false when there is no generated field.
func generatedAt(text string) (t time.Time, ok bool, err error) {
	lines, a, b, err := split(text)
	if err != nil {
		return t, false, err
	}
	found := topKeys(lines, a, b, "generated")
	if len(found) == 0 {
		return t, false, nil
	}
	if len(found) > 1 {
		return t, false, errorf("generated appears twice")
	}
	i := found[0]
	var f *fields
	if rest := keyValue(lines[i]); rest != "" {
		if f, err = flowMap(rest); err != nil {
			return t, false, err
		}
	} else {
		f = newFields()
		for _, line := range lines[i+1 : blockEnd(lines, i, b)] {
			m := genLine.FindStringSubmatch(rstripNL(line))
			if m == nil {
				return t, false, errorf("generated line not supported: %s", pyRepr(rstrip(line)))
			}
			v, err := scalar(m[2])
			if err != nil {
				return t, false, err
			}
			f.set(m[1], v)
		}
	}
	if !f.has("at") {
		return t, false, errorf("generated has no at")
	}
	t, err = ParseTime(f.m["at"])
	return t, err == nil, err
}

func parseVerified(text string) (*verified, error) {
	lines, a, b, err := split(text)
	if err != nil {
		return nil, err
	}
	found := topKeys(lines, a, b, "verified")
	if len(found) == 0 {
		return &verified{kind: "absent"}, nil
	}
	if len(found) > 1 {
		return nil, errorf("verified appears twice")
	}
	i := found[0]
	line := rstripNL(lines[i])
	rest := keyValue(lines[i])
	end := blockEnd(lines, i, b)
	if rest != "" {
		if end != i+1 {
			return nil, errorf("verified has a value and an indented block")
		}
		if strings.HasPrefix(rest, "{") {
			f, err := flowMap(rest)
			if err != nil {
				return nil, err
			}
			e, err := newEntry(f, "flow", 0, 0)
			if err != nil {
				return nil, err
			}
			return &verified{kind: "flowmap", start: i, end: i + 1, entries: []*entry{e}, dashIndent: "  "}, nil
		}
		if strings.HasPrefix(rest, "[") && strings.HasSuffix(rest, "]") {
			off := strings.IndexByte(line, '[') + 1
			inner := line[off:strings.LastIndexByte(line, ']')]
			var entries []*entry
			if strip(inner) != "" {
				spans, err := splitTop(inner)
				if err != nil {
					return nil, err
				}
				for _, sp := range spans {
					f, err := flowMap(inner[sp.a:sp.b])
					if err != nil {
						return nil, err
					}
					e, err := newEntry(f, "flow", sp.a, sp.b)
					if err != nil {
						return nil, err
					}
					entries = append(entries, e)
				}
			}
			return &verified{kind: "flowlist", start: i, end: i + 1, entries: entries, dashIndent: "  ", listText: inner, listOffset: off}, nil
		}
		return nil, errorf("verified form not supported: %s", pyRepr(line))
	}
	if end == i+1 {
		return nil, errorf("verified is empty")
	}
	m := dashRe.FindStringSubmatch(lines[i+1])
	if m == nil {
		return nil, errorf("verified form not supported: %s", pyRepr(rstrip(lines[i+1])))
	}
	dash := m[1]
	itemRe := regexp.MustCompile(`^` + regexp.QuoteMeta(dash) + `- (.*?)[ \t]*\r?\n?$`)
	contRe := regexp.MustCompile(`^` + regexp.QuoteMeta(dash) + `  (` + word + `):[ \t]+(.*?)[ \t]*\r?\n?$`)
	unsupported := func(j int) error {
		return errorf("verified line not supported: %s", pyRepr(rstrip(lines[j])))
	}
	var entries []*entry
	j := i + 1
	for j < end {
		im := itemRe.FindStringSubmatch(lines[j])
		if im == nil {
			return nil, unsupported(j)
		}
		body := im[1]
		if strings.HasPrefix(body, "{") {
			f, err := flowMap(body)
			if err != nil {
				return nil, err
			}
			e, err := newEntry(f, "flow", j, j)
			if err != nil {
				return nil, err
			}
			entries = append(entries, e)
			j++
			continue
		}
		km := bodyKey.FindStringSubmatch(body)
		if km == nil {
			return nil, unsupported(j)
		}
		v, err := scalar(km[2])
		if err != nil {
			return nil, err
		}
		f := newFields()
		f.set(km[1], v)
		start := j
		j++
		for j < end && !itemRe.MatchString(lines[j]) {
			cm := contRe.FindStringSubmatch(lines[j])
			if cm == nil || f.has(cm[1]) {
				return nil, unsupported(j)
			}
			v, err := scalar(cm[2])
			if err != nil {
				return nil, err
			}
			f.set(cm[1], v)
			j++
		}
		e, err := newEntry(f, "block", start, j-1)
		if err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return &verified{kind: "blocklist", start: i, end: end, entries: entries, dashIndent: dash}, nil
}

func renderItem(style, dash, actor, at, nl string) string {
	if style == "block" {
		return dash + "- by: " + actor + nl + dash + "  at: " + at + nl
	}
	return dash + "- { by: " + actor + ", at: " + at + " }" + nl
}

func newline(line string) string {
	if strings.HasSuffix(line, "\r\n") {
		return "\r\n"
	}
	return "\n"
}

// Outcomes of Stamp.
const (
	Added               = "added"
	Refreshed           = "refreshed"
	Current             = "current"
	GeneratedAfterCheck = "generated-after-check"
)

// Stamp adds or refreshes actor's verified entry, dated at.
//
// It returns the new text and the outcome; changed is false when nothing is
// due (outcome Current or GeneratedAfterCheck). An *Error means a form this
// package does not handle.
func Stamp(text, actor string, at time.Time) (newText, outcome string, changed bool, err error) {
	gen, hasGen, err := generatedAt(text)
	if err != nil {
		return "", "", false, err
	}
	v, err := parseVerified(text)
	if err != nil {
		return "", "", false, err
	}
	var own *entry
	for _, e := range v.entries {
		if e.by == actor {
			if own != nil {
				return "", "", false, errorf("%s has more than one verified entry", actor)
			}
			own = e
		}
	}
	if hasGen {
		if gen.After(at) {
			return "", GeneratedAfterCheck, false, nil
		}
		if own != nil && !own.at.Before(gen) {
			return "", Current, false, nil
		}
	} else if own != nil {
		return "", Current, false, nil
	}

	ats := FormatTime(at)
	lines, a, b, err := split(text)
	if err != nil {
		return "", "", false, err
	}
	outcome = Added
	if own != nil {
		outcome = Refreshed
	}
	flow := "{ by: " + actor + ", at: " + ats + " }"
	switch v.kind {
	case "absent":
		pos := b
		if g := topKeys(lines, a, b, "generated"); len(g) > 0 {
			pos = blockEnd(lines, g[0], b)
		}
		nl := newline(lines[pos-1])
		lines = insert(lines, pos, "verified: "+flow+nl)
	case "flowmap":
		line := lines[v.start]
		nl := newline(line)
		if own != nil {
			lines[v.start] = "verified: " + flow + nl
		} else {
			old := strip(rstripNL(line)[len("verified:"):])
			lines[v.start] = "verified:" + nl + "  - " + old + nl + "  - " + flow + nl
		}
	case "flowlist":
		line := lines[v.start]
		inner := v.listText
		switch {
		case own != nil:
			s, e := own.first, own.last
			piece := inner[s:e]
			lead := piece[:len(piece)-len(lstrip(piece))]
			trail := piece[len(rstrip(piece)):]
			inner = inner[:s] + lead + flow + trail + inner[e:]
		case len(v.entries) > 0:
			e := v.entries[len(v.entries)-1].last
			body := rstrip(inner[:e])
			inner = body + ", " + flow + inner[len(body):]
		default:
			inner = flow
		}
		closeAt := strings.LastIndexByte(line, ']')
		lines[v.start] = line[:v.listOffset] + inner + line[closeAt:]
	default: // blocklist
		nl := newline(lines[v.start])
		if own != nil {
			item := renderItem(own.style, v.dashIndent, actor, ats, nl)
			lines = append(lines[:own.first], append([]string{item}, lines[own.last+1:]...)...)
		} else {
			last := v.entries[len(v.entries)-1]
			lines = insert(lines, last.last+1, renderItem(last.style, v.dashIndent, actor, ats, nl))
		}
	}
	return strings.Join(lines, ""), outcome, true, nil
}

func insert(lines []string, i int, s string) []string {
	lines = append(lines, "")
	copy(lines[i+1:], lines[i:])
	lines[i] = s
	return lines
}

// IsError reports whether err is a frontmatter *Error.
func IsError(err error) bool {
	_, ok := err.(*Error)
	return ok
}
