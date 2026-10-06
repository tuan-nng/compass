// Package validate is the strict OKF v0.2 conformance checker behind
// `compass validate` (plan decision 19). It is a port of the okf-skills
// validator (okf_validate.py, MIT; reference copy in testdata/okf-skills),
// always run as `--strict`: every warning fails the bundle. Findings, text
// and JSON output and exit codes match the Python; differential_test.go
// holds the two side by side.
package validate

import (
	"fmt"
	"io"
	"math/big"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

const okfVersion = "0.2"

var recommended = []string{"title", "description", "tags"}

var statusValues = map[string]bool{"draft": true, "stable": true, "deprecated": true}

var (
	isoDate = pyRegexp(`^\d{4}-\d{2}-\d{2}$`)
	// RFC 3339 or a bare date; str() of a PyYAML datetime separates with a space.
	rfc3339 = pyRegexp(`^\d{4}-\d{2}-\d{2}` +
		`(?:[Tt ]\d{2}:\d{2}(?::\d{2}(?:\.\d+)?)?(?:[Zz]|[+-]\d{2}:?\d{2})?)?$`)
	actorShape = pyRegexp(`^(?:[^{S}:/]+:\S+|\S+/\S+)$`)
	// ACTOR_HUMANISH without its lookahead, which actorHumanish applies.
	actorHumanishPrefix = regexp.MustCompile(`(?i)^(?:human|process)`)
	// LINK without its `(?<!\!)` lookbehind, which linkTargets applies.
	link      = pyRegexp(`^\[[^\]]*\]\(([^){S}]+)(?:\s+"[^"]*")?\)`)
	citations = regexp.MustCompile(`(?m)^#{1,6}[ \t]+Citations[ \t]*$`)
	footnote  = pyRegexp(`\[\^([^\]{S}]+)\]`)
	urlScheme = regexp.MustCompile(`^[a-z][a-z0-9+.-]*://`)
)

// report collects findings as "<rel>: <message>" strings.
type report struct {
	errors, warnings        []string
	concepts, indexes, logs int
}

func (r *report) err(rel, msg string)  { r.errors = append(r.errors, rel+": "+msg) }
func (r *report) warn(rel, msg string) { r.warnings = append(r.warnings, rel+": "+msg) }

// splitFrontmatter returns the raw YAML between a leading `---` line and the
// next `---` line, and the body after it; ok is false when there is no
// terminated block.
func splitFrontmatter(text string) (raw, body string, ok bool) {
	if !strings.HasPrefix(text, "---") {
		return "", text, false
	}
	lines := pySplitLines(text, true)
	if pyStrip(lines[0]) != "---" {
		return "", text, false
	}
	for i := 1; i < len(lines); i++ {
		if pyStrip(lines[i]) == "---" {
			return strings.Join(lines[1:i], ""), strings.Join(lines[i+1:], ""), true
		}
	}
	return "", text, false
}

// readText reads a concept, index or log file, reporting an unreadable or
// non-UTF-8 file as a per-file error.
func (r *report) readText(file, rel string) (string, bool) {
	text, err := pyReadText(file)
	if err != nil {
		r.err(rel, "cannot read file: "+err.Error())
		return "", false
	}
	return strings.TrimLeft(text, "\ufeff"), true
}

// getOr is dict.get(key, def).
func (d *pyDict) getOr(key string, def any) any {
	if v, ok := d.lookup(key); ok {
		return v
	}
	return def
}

type validator struct {
	bundle string // str(Path(arg))
	r      *report
}

// validate checks every `.md` file under the bundle. It panics with a
// pyCrashError where the Python validator raises an uncaught exception.
func validate(bundle string) *report {
	v := &validator{bundle: bundle, r: &report{}}
	files := mdFiles(bundle)
	for _, rel := range files {
		file := pyJoin(bundle, rel)
		switch path.Base(rel) {
		case "index.md":
			v.checkIndex(file, rel, rel == "index.md")
		case "log.md":
			v.checkLog(file, rel)
		default:
			v.checkConcept(file, rel)
		}
	}
	v.checkLinks(files)
	return v.r
}

// mdFiles is sorted(p for p in bundle.rglob("*.md") if p.is_file()) on
// CPython 3.12: symlinked directories are not descended, symlinked files
// count, and paths sort component by component.
func mdFiles(bundle string) []string {
	var files []string
	var walk func(dir string)
	walk = func(dir string) {
		entries, err := os.ReadDir(pyJoin(bundle, dir))
		if err != nil {
			return
		}
		for _, e := range entries {
			rel := e.Name()
			if dir != "" {
				rel = dir + "/" + rel
			}
			if strings.HasSuffix(e.Name(), ".md") {
				if st, err := os.Stat(pyJoin(bundle, rel)); err == nil && st.Mode().IsRegular() {
					files = append(files, rel)
				}
			}
			if e.IsDir() {
				walk(rel)
			}
		}
	}
	walk("")
	sort.Slice(files, func(i, j int) bool {
		a, b := strings.Split(files[i], "/"), strings.Split(files[j], "/")
		for k := range min(len(a), len(b)) {
			if a[k] != b[k] {
				return a[k] < b[k]
			}
		}
		return len(a) < len(b)
	})
	return files
}

func parentDir(rel string) string {
	if i := strings.LastIndex(rel, "/"); i >= 0 {
		return rel[:i]
	}
	return ""
}

func (v *validator) checkConcept(file, rel string) {
	r := v.r
	r.concepts++
	text, ok := r.readText(file, rel)
	if !ok {
		return
	}
	raw, body, ok := splitFrontmatter(text)
	if !ok {
		r.err(rel, "§11.1 no parseable YAML frontmatter block "+
			"(every non-reserved `.md` in a bundle is a concept: add "+
			"frontmatter with a `type` field, or move the file outside "+
			"the bundle)")
		return
	}
	loaded, err := loadYAML(raw)
	if err != nil {
		r.err(rel, strings.ReplaceAll("§11.1 frontmatter is not valid YAML: "+err.Error(), "\n", " "))
		return
	}
	meta, ok := loaded.(*pyDict)
	if !ok {
		r.err(rel, "§11.1 frontmatter must be a YAML mapping")
		return
	}
	typeVal, isStr := meta.get("type").(string)
	if !isStr || pyStrip(typeVal) == "" {
		r.err(rel, "§11.2 missing or empty required `type` field")
	}
	for _, key := range recommended {
		if !meta.has(key) {
			r.warn(rel, fmt.Sprintf("recommended field `%s` is absent (§4.1)", key))
		}
	}
	v.checkTrust(meta, rel)
	v.checkLifecycle(meta, rel)
	v.checkSources(meta, body, rel)
	if isStr && pyStrip(typeVal) == "Attested Computation" {
		v.checkComputation(meta, file, rel)
	}
}

// checkComputation is §10.2: an Attested Computation needs `runtime`, and its
// bundle-relative `computation`, `executor.resource` and `attester.resource`
// paths must exist.
func (v *validator) checkComputation(meta *pyDict, file, rel string) {
	r := v.r
	if pyStrip(pyStr(meta.getOr("runtime", ""))) == "" {
		r.warn(rel, "§10.2 an Attested Computation concept requires `runtime`")
	}
	type field struct {
		where  string
		target any
	}
	fields := []field{{"computation", meta.get("computation")}}
	for _, key := range []string{"executor", "attester"} {
		if block, ok := meta.get(key).(*pyDict); ok {
			fields = append(fields, field{key + ".resource", block.get("resource")})
		}
	}
	for _, f := range fields {
		if f.target == nil {
			continue
		}
		t := pyStrip(pyStr(f.target))
		if t == "" || urlScheme.MatchString(t) || strings.HasPrefix(t, "mailto:") {
			continue
		}
		var candidate string
		if strings.HasPrefix(t, "/") {
			candidate = pyJoin(v.bundle, strings.TrimLeft(t, "/"))
		} else {
			candidate = pyJoin(pyJoin(v.bundle, parentDir(rel)), t)
		}
		_, inside := pyRelativeTo(pyResolve(candidate), pyResolve(v.bundle))
		if _, err := os.Stat(candidate); inside && err != nil {
			r.warn(rel, fmt.Sprintf("§6.2 `%s` points at `%s`, which does not exist", f.where, t))
		}
	}
}

// actorHumanish is ACTOR_HUMANISH: human/process in any case, not followed
// by a character `[A-Za-z0-9_]` matches under re.IGNORECASE.
func actorHumanish(actor string) bool {
	loc := actorHumanishPrefix.FindStringIndex(actor)
	if loc == nil {
		return false
	}
	next, _ := utf8.DecodeRuneInString(actor[loc[1]:])
	switch {
	case loc[1] == len(actor):
		return true
	case next < 0x80:
		return !(next >= 'a' && next <= 'z' || next >= 'A' && next <= 'Z' || next >= '0' && next <= '9' || next == '_')
	}
	// Non-ASCII letters whose case mapping is an ASCII letter.
	return next != 'İ' && next != 'ı' && next != 'ſ' && next != 'K'
}

// checkActor is §7, shared by `generated.by`, `verified[].by` and
// `sources[].author`.
func (v *validator) checkActor(value any, where, rel string) {
	actor := pyStrip(pyStr(value))
	if actor == "" {
		return
	}
	if actorHumanish(actor) && !strings.HasPrefix(actor, "human:") && !strings.HasPrefix(actor, "process:") {
		v.r.warn(rel, fmt.Sprintf("§7 `%s` `%s` is a near-miss of "+
			"`human:`/`process:` — §5.3 keys trust tiers off that exact "+
			"lowercase prefix, so this silently reads as an agent", where, actor))
	} else if !actorShape.MatchString(actor) {
		v.r.warn(rel, fmt.Sprintf("§7 `%s` `%s` matches no actor shape "+
			"(`human:<id>`, `process:<id>`, or `<producer>/<version>`)", where, actor))
	}
}

// checkInstant is §5.2: `at` values are RFC 3339; a bare date is tolerated.
func (v *validator) checkInstant(value any, where, rel string) {
	if value != nil && !rfc3339.MatchString(pyStrip(pyStr(value))) {
		v.r.warn(rel, fmt.Sprintf("§5.2 `%s` `%s` is not an RFC 3339 timestamp", where, pyStr(value)))
	}
}

// checkTrust is §5.2: `generated` and `verified`, with the v0.1 `timestamp`
// fallback.
func (v *validator) checkTrust(meta *pyDict, rel string) {
	r := v.r
	switch gen := meta.get("generated").(type) {
	case nil:
		if meta.has("timestamp") {
			r.warn(rel, "legacy v0.1 `timestamp`: v0.2 records the last content "+
				"change as `generated: {by, at}` (§13.1)")
		} else {
			r.warn(rel, "recommended field `generated` is absent (§5.2)")
		}
	case *pyDict:
		if pyStrip(pyStr(gen.getOr("by", ""))) == "" {
			r.warn(rel, "§5.2 `generated.by` is required within `generated`")
		} else {
			v.checkActor(gen.get("by"), "generated.by", rel)
			v.checkInstant(gen.get("at"), "generated.at", rel)
		}
	default:
		r.warn(rel, "§5.2 `generated` must be a mapping with `by` and `at`")
	}

	var entries pyList
	switch ver := meta.get("verified").(type) {
	case nil:
		return
	case *pyDict:
		// A bare mapping is one verification event (§5.2).
		entries = pyList{ver}
	case pyList:
		entries = ver
	default:
		r.warn(rel, "§5.2 `verified` must be a `{by, at}` mapping or a list of them")
		return
	}
	for i, entry := range entries {
		e, ok := entry.(*pyDict)
		if !ok || pyStrip(pyStr(e.getOr("by", ""))) == "" {
			r.warn(rel, "§5.2 every `verified` entry needs a `by` actor")
			continue
		}
		v.checkActor(e.get("by"), fmt.Sprintf("verified[%d].by", i), rel)
		v.checkInstant(e.get("at"), fmt.Sprintf("verified[%d].at", i), rel)
	}
}

// checkLifecycle is §5.4 / §5.5: `status` and `stale_after`.
func (v *validator) checkLifecycle(meta *pyDict, rel string) {
	if status := meta.get("status"); status != nil {
		// `in` a set hashes the value; a set argument is frozen first.
		if _, isSet := status.(*pySet); !isSet {
			if name := pyUnhashable(status); name != "" {
				panic(pyCrash("TypeError", fmt.Sprintf("unhashable type: '%s'", name)))
			}
		}
		if s, ok := status.(string); !ok || !statusValues[s] {
			v.r.warn(rel, fmt.Sprintf("§5.4 unknown `status` `%s` (expected deprecated|draft|stable)", pyStr(status)))
		}
	}
	if stale := meta.get("stale_after"); stale != nil && !rfc3339.MatchString(pyStr(stale)) {
		v.r.warn(rel, fmt.Sprintf("§5.5 `stale_after` `%s` is not an ISO 8601 datetime "+
			"(a bare date is tolerated)", pyStr(stale)))
	}
}

// checkWindow is §5.1: a `usage_window` is a `{from, to}` pair.
func (v *validator) checkWindow(window any, where, rel string) {
	if window == nil {
		return
	}
	w, ok := window.(*pyDict)
	if !ok {
		v.r.warn(rel, fmt.Sprintf("§5.1 `%s` `usage_window` must be a `{from, to}` mapping", where))
		return
	}
	for _, bound := range []string{"from", "to"} {
		value := w.get(bound)
		if value == nil {
			v.r.warn(rel, fmt.Sprintf("§5.1 `%s` `usage_window` is missing `%s`", where, bound))
		} else if !rfc3339.MatchString(pyStr(value)) {
			v.r.warn(rel, fmt.Sprintf("§5.1 `%s` `usage_window.%s` `%s` "+
				"is not an ISO 8601 datetime (a bare date is tolerated)", where, bound, pyStr(value)))
		}
	}
}

// checkSources is §5.1: the `sources` family and its footnote join keys.
func (v *validator) checkSources(meta *pyDict, body, rel string) {
	r := v.r
	if citations.MatchString(body) {
		r.warn(rel, "legacy v0.1 `# Citations` body list: v0.2 records provenance "+
			"in the `sources` frontmatter (§13.1)")
	}
	raw := meta.get("sources")
	if raw == nil {
		return
	}
	srcs, ok := raw.(pyList)
	if !ok {
		r.warn(rel, "§5.1 `sources` must be a list of entries")
		return
	}
	ids := map[string]bool{}
	for i, item := range srcs {
		src, ok := item.(*pyDict)
		if !ok {
			r.warn(rel, fmt.Sprintf("§5.1 `sources[%d]` must be a mapping", i))
			continue
		}
		if pyStrip(pyStr(src.getOr("resource", ""))) == "" {
			r.warn(rel, fmt.Sprintf("§5.1 `sources[%d]` is missing the required `resource`", i))
		}
		if id, ok := src.lookup("id"); ok {
			ids[pyStr(id)] = true
		}
		if author := src.get("author"); author != nil {
			v.checkActor(author, fmt.Sprintf("sources[%d].author", i), rel)
		}
		window := src.getOr("usage_window", meta.get("usage_window"))
		if src.get("usage_count") != nil && window == nil {
			r.warn(rel, fmt.Sprintf("§5.1 `sources[%d].usage_count` has no `usage_window` "+
				"framing it (a sibling of `sources`, or on the entry)", i))
		}
		v.checkWindow(window, fmt.Sprintf("sources[%d]", i), rel)
		if lastMod := src.get("last_modified"); lastMod != nil && !rfc3339.MatchString(pyStr(lastMod)) {
			r.warn(rel, fmt.Sprintf("§5.1 `sources[%d].last_modified` `%s` is not an "+
				"ISO 8601 datetime (a bare date is tolerated)", i, pyStr(lastMod)))
		}
	}
	// Attribution joins on the label, not on position (§5.1).
	labels := map[string]bool{}
	for _, m := range footnote.FindAllStringSubmatch(body, -1) {
		if !ids[m[1]] {
			labels[m[1]] = true
		}
	}
	sorted := make([]string, 0, len(labels))
	for l := range labels {
		sorted = append(sorted, l)
	}
	sort.Strings(sorted)
	for _, label := range sorted {
		r.warn(rel, fmt.Sprintf("§5.1 footnote `[^%s]` matches no `sources[].id`", label))
	}
}

func (v *validator) checkIndex(file, rel string, isRoot bool) {
	r := v.r
	r.indexes++
	text, ok := r.readText(file, rel)
	if !ok {
		return
	}
	raw, _, ok := splitFrontmatter(text)
	if !ok {
		return
	}
	if !isRoot {
		r.warn(rel, "§8 index.md should contain no frontmatter")
		return
	}
	meta, err := loadYAML(raw)
	if err != nil {
		r.warn(rel, "§12 root index.md frontmatter is not valid YAML")
		meta = &pyDict{}
	} else if !pyTruthy(meta) {
		meta = &pyDict{}
	}
	// `upkeep` rides along: the plugin's Stop hook reads its opt-in flag here.
	extra := &pySet{}
	for _, k := range pyIter(meta) {
		if !pyEq(k, "okf_version") && !pyEq(k, "upkeep") {
			extra.add(k)
		}
	}
	if len(extra.items) > 0 {
		r.warn(rel, fmt.Sprintf("§12 root index.md frontmatter may only carry `okf_version` (found %s)",
			pyRepr(pyList(pySorted(extra.items)))))
	}
	d, ok := meta.(*pyDict)
	if !ok {
		panic(pyCrash("AttributeError", fmt.Sprintf("'%s' object has no attribute 'get'", pyTypeName(meta))))
	}
	if declared := d.get("okf_version"); declared != nil && pyStr(declared) != okfVersion {
		r.warn(rel, fmt.Sprintf("§12 bundle declares `okf_version: \"%s\"`; checked against v%s",
			pyStr(declared), okfVersion))
	}
}

// pyTruthy is bool(v).
func pyTruthy(v any) bool {
	switch t := v.(type) {
	case nil:
		return false
	case bool:
		return t
	case float64:
		return t != 0
	case string:
		return t != ""
	case pyBytes:
		return len(t) > 0
	case pyList:
		return len(t) > 0
	case pyTuple:
		return len(t) > 0
	case *pyDict:
		return len(t.keys) > 0
	case *pySet:
		return len(t.items) > 0
	case pyDate, pyDateTime:
		return true
	}
	n, _ := pyNumber(v)
	return n.Sign() != 0
}

// pyIter is set(v)'s iteration of v, raising TypeError where Python does.
func pyIter(v any) []any {
	var items []any
	switch t := v.(type) {
	case *pyDict:
		return t.keys
	case *pySet:
		return t.items
	case pyList:
		items = t
	case pyTuple:
		items = t
	case string:
		for _, c := range t {
			items = append(items, string(c))
		}
		return items
	case pyBytes:
		for _, c := range t {
			items = append(items, big.NewInt(int64(c)))
		}
		return items
	default:
		panic(pyCrash("TypeError", fmt.Sprintf("'%s' object is not iterable", pyTypeName(v))))
	}
	for _, it := range items {
		if name := pyUnhashable(it); name != "" {
			panic(pyCrash("TypeError", fmt.Sprintf("unhashable type: '%s'", name)))
		}
	}
	return items
}

func (v *validator) checkLog(file, rel string) {
	r := v.r
	r.logs++
	text, ok := r.readText(file, rel)
	if !ok {
		return
	}
	if _, _, ok := splitFrontmatter(text); ok {
		r.warn(rel, "§9 log.md should contain no frontmatter")
	}
	for _, line := range pySplitLines(text, false) {
		if strings.HasPrefix(line, "## ") {
			heading := pyStrip(line[3:])
			if !isoDate.MatchString(heading) {
				r.warn(rel, fmt.Sprintf("§9 date heading `%s` is not ISO 8601 YYYY-MM-DD", heading))
			}
		}
	}
}

// linkTargets returns the markdown link targets outside code fences. An
// unreadable file was already reported by its per-file check.
func linkTargets(file string) []string {
	text, err := pyReadText(file)
	if err != nil {
		return nil
	}
	var targets []string
	inFence := false
	for _, line := range pySplitLines(text, false) {
		if s := pyStrip(line); strings.HasPrefix(s, "```") || strings.HasPrefix(s, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		for i := 0; i < len(line); {
			if line[i] == '[' && (i == 0 || line[i-1] != '!') {
				if m := link.FindStringSubmatchIndex(line[i:]); m != nil {
					targets = append(targets, line[i+m[2]:i+m[3]])
					i += m[1]
					continue
				}
			}
			i++
		}
	}
	return targets
}

// checkLinks reports broken bundle-internal `.md` links; §6.1 makes them
// warnings only.
func (v *validator) checkLinks(files []string) {
	existing := make(map[string]bool, len(files))
	for _, rel := range files {
		existing[rel] = true
	}
	for _, rel := range files {
		for _, target := range linkTargets(pyJoin(v.bundle, rel)) {
			t, _, _ := strings.Cut(target, "#")
			if t == "" || strings.HasSuffix(t, "/") {
				continue
			}
			if urlScheme.MatchString(t) || strings.HasPrefix(t, "mailto:") {
				continue
			}
			if !strings.HasSuffix(t, ".md") {
				continue
			}
			resolved := t
			if strings.HasPrefix(t, "/") {
				resolved = strings.TrimLeft(t, "/")
			} else {
				candidate := pyJoin(pyJoin(v.bundle, parentDir(rel)), t)
				if inner, ok := pyRelativeTo(pyResolve(candidate), pyResolve(v.bundle)); ok {
					resolved = inner
				}
			}
			if !existing[resolved] {
				v.r.warn(rel, fmt.Sprintf("cross-link target not found: `%s` (tolerated under §6.1)", target))
			}
		}
	}
}

const usage = "usage: compass validate <bundle> [--json]"

// Main is `compass validate <bundle> [--json]`: the strict validator. Exit 0
// when the bundle has no errors and no warnings, 1 when it has any, 2 on a
// usage error or a bundle that is not a directory.
func Main(args []string, stdout, stderr io.Writer) int {
	jsonOut := false
	var positional []string
	for i, a := range args {
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		switch {
		case a == "-h" || a == "--help":
			fmt.Fprintf(stdout, "%s\n\nValidate an OKF v%s bundle in strict mode: every warning fails it.\n\n"+
				"  --json  emit a JSON report\n", usage, okfVersion)
			return 0
		case a == "--json":
			jsonOut = true
		case a == "--strict":
			// Always strict; accepted for callers of the old wrapper.
		case strings.HasPrefix(a, "-") && a != "-":
			fmt.Fprintf(stderr, "compass validate: unknown option %s\n%s\n", a, usage)
			return 2
		default:
			positional = append(positional, a)
		}
	}
	if len(positional) != 1 {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	bundle := pyPath(positional[0])
	if st, err := os.Stat(bundle); err != nil || !st.IsDir() {
		fmt.Fprintf(stderr, "error: %s is not a directory\n", bundle)
		return 2
	}

	r, crash := runValidate(bundle)
	if crash != nil {
		fmt.Fprintf(stderr, "compass validate: %s\n", crash)
		return 1
	}
	failed := len(r.errors) > 0 || len(r.warnings) > 0
	exit := 0
	if failed {
		exit = 1
	}
	if jsonOut {
		writeJSON(stdout, bundle, r, !failed)
		return exit
	}
	writeText(stdout, bundle, r)
	return exit
}

// runValidate turns the panic for an uncaught Python exception into an error.
func runValidate(bundle string) (r *report, crash error) {
	defer func() {
		if p := recover(); p != nil {
			c, ok := p.(pyCrashError)
			if !ok {
				panic(p)
			}
			crash = c
		}
	}()
	return validate(bundle), nil
}

func writeText(w io.Writer, bundle string, r *report) {
	fmt.Fprintf(w, "OKF v%s conformance — %s\n", okfVersion, bundle)
	fmt.Fprintf(w, "  concepts: %d   index.md: %d   log.md: %d\n", r.concepts, r.indexes, r.logs)
	for _, e := range r.errors {
		fmt.Fprintf(w, "  \033[31m✗ ERROR\033[0m  %s\n", e)
	}
	for _, wn := range r.warnings {
		fmt.Fprintf(w, "  \033[33m! warn \033[0m  %s\n", wn)
	}
	switch {
	case len(r.errors) == 0 && len(r.warnings) == 0:
		fmt.Fprintln(w, "  \033[32m✓ conformant — no issues\033[0m")
	case len(r.errors) == 0:
		fmt.Fprintf(w, "  \033[32m✓ conformant\033[0m (%d warning(s))\n", len(r.warnings))
	default:
		fmt.Fprintf(w, "  \033[31m✗ non-conformant\033[0m (%d error(s))\n", len(r.errors))
	}
}

// writeJSON prints the report as Python's json.dumps(..., indent=2) does,
// including its ASCII-only escaping.
func writeJSON(w io.Writer, bundle string, r *report, passed bool) {
	list := func(items []string) string {
		if len(items) == 0 {
			return "[]"
		}
		quoted := make([]string, len(items))
		for i, s := range items {
			quoted[i] = "    " + pyJSONString(s)
		}
		return "[\n" + strings.Join(quoted, ",\n") + "\n  ]"
	}
	fmt.Fprintf(w, "{\n  \"bundle\": %s,\n  \"conformant\": %t,\n  \"passed\": %t,\n"+
		"  \"counts\": {\n    \"concepts\": %d,\n    \"indexes\": %d,\n    \"logs\": %d\n  },\n"+
		"  \"errors\": %s,\n  \"warnings\": %s,\n  \"migrated\": []\n}\n",
		pyJSONString(bundle), len(r.errors) == 0, passed,
		r.concepts, r.indexes, r.logs, list(r.errors), list(r.warnings))
}

func pyJSONString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, c := range s {
		switch {
		case c == '"':
			b.WriteString(`\"`)
		case c == '\\':
			b.WriteString(`\\`)
		case c == '\n':
			b.WriteString(`\n`)
		case c == '\r':
			b.WriteString(`\r`)
		case c == '\t':
			b.WriteString(`\t`)
		case c == '\b':
			b.WriteString(`\b`)
		case c == '\f':
			b.WriteString(`\f`)
		case c >= 0x20 && c <= 0x7e:
			b.WriteRune(c)
		case c > 0xffff:
			c -= 0x10000
			fmt.Fprintf(&b, `\u%04x\u%04x`, 0xd800+(c>>10), 0xdc00+(c&0x3ff))
		default:
			fmt.Fprintf(&b, `\u%04x`, c)
		}
	}
	b.WriteByte('"')
	return b.String()
}
