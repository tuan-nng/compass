package validate

// Differential test: the Go port against the Python reference validator
// (testdata/okf-skills/okf_validate.py, run under uv from its lock file as
// `--strict`, as the old okf-strict-validate wrapper did) on the fixture, the
// prototype hub set and a table of broken bundles.
//
// Required to be identical: the exit code, the JSON report (bundle, counts,
// conformant, passed, every error and warning in order) and the text output.
//
// Residual difference (plan decision 19, by design): the port parses YAML
// with gopkg.in/yaml.v3, not pyyaml, so the parser's own detail inside a
// "§11.1 frontmatter is not valid YAML: <detail>" finding differs. For those
// findings the test requires the same file, severity and prefix, and when
// the yaml.v3 detail names a line, that line must be one pyyaml's detail
// names too (or the one before it: yaml.v3 counts lines from zero in parser,
// as opposed to scanner, errors). Everything else, including which
// frontmatters are rejected, must match exactly.
//
// Where the Python validator crashes with an uncaught exception (exit 1, a
// traceback, no report), the port must exit 1 with no stdout and print the
// traceback's last line, "<Exception>: <message>", after "compass validate: ".

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

const yamlErrorMarker = "frontmatter is not valid YAML: "

var (
	pyYAMLLines = regexp.MustCompile(`line (\d+), column \d+`)
	goYAMLLine  = regexp.MustCompile(`^line (\d+): `)
)

// referenceScript is absolute: TestDifferentialArguments changes directory.
var referenceScript, _ = filepath.Abs(filepath.Join("testdata", "okf-skills", "okf_validate.py"))

type run struct {
	stdout, stderr string
	code           int
}

func runPython(t *testing.T, args ...string) run {
	t.Helper()
	cmd := exec.Command("uv", append([]string{"run", "--quiet", "--locked", "--script", referenceScript}, append(args, "--strict")...)...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	code := 0
	if err := cmd.Run(); err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("uv run: %v", err)
		}
		code = ee.ExitCode()
	}
	return run{out.String(), errb.String(), code}
}

func runGo(args ...string) run {
	var out, errb bytes.Buffer
	code := Main(args, &out, &errb)
	return run{out.String(), errb.String(), code}
}

func requireUV(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("uv"); err != nil {
		t.Skip("uv not installed: differential test skipped")
	}
}

// sameFinding compares one error or warning string.
func sameFinding(py, gov string) bool {
	if py == gov {
		return true
	}
	pi, gi := strings.Index(py, yamlErrorMarker), strings.Index(gov, yamlErrorMarker)
	if pi < 0 || gi < 0 || py[:pi] != gov[:gi] {
		return false
	}
	m := goYAMLLine.FindStringSubmatch(gov[gi+len(yamlErrorMarker):])
	if m == nil {
		return true
	}
	line, _ := strconv.Atoi(m[1])
	for _, pm := range pyYAMLLines.FindAllStringSubmatch(py[pi:], -1) {
		if pl, _ := strconv.Atoi(pm[1]); pl == line || pl == line+1 {
			return true
		}
	}
	return false
}

func sameFindings(py, gov []string) bool {
	if len(py) != len(gov) {
		return false
	}
	for i := range py {
		if !sameFinding(py[i], gov[i]) {
			return false
		}
	}
	return true
}

type jsonReport struct {
	Bundle     string         `json:"bundle"`
	Conformant bool           `json:"conformant"`
	Passed     bool           `json:"passed"`
	Counts     map[string]int `json:"counts"`
	Errors     []string       `json:"errors"`
	Warnings   []string       `json:"warnings"`
	Migrated   []string       `json:"migrated"`
}

// compare runs both validators on dir in text and JSON mode.
func compare(t *testing.T, dir string) {
	t.Helper()
	for _, mode := range [][]string{nil, {"--json"}} {
		args := append([]string{dir}, mode...)
		py, gov := runPython(t, args...), runGo(args...)
		if py.code != gov.code {
			t.Errorf("%v: exit code python %d, go %d\npython stdout:\n%s\npython stderr:\n%s\ngo stdout:\n%s\ngo stderr:\n%s",
				args, py.code, gov.code, py.stdout, py.stderr, gov.stdout, gov.stderr)
			continue
		}
		if strings.Contains(py.stderr, "Traceback (most recent call last):") {
			lines := strings.Split(strings.TrimSpace(py.stderr), "\n")
			want := "compass validate: " + lines[len(lines)-1] + "\n"
			if gov.stdout != "" || gov.stderr != want {
				t.Errorf("%v: python crashed with %q; go stdout %q, stderr %q", args, want, gov.stdout, gov.stderr)
			}
			continue
		}
		if py.stderr != gov.stderr {
			t.Errorf("%v: stderr python %q, go %q", args, py.stderr, gov.stderr)
		}
		if py.code == 2 {
			if py.stdout != gov.stdout {
				t.Errorf("%v: stdout python %q, go %q", args, py.stdout, gov.stdout)
			}
			continue
		}
		if mode == nil {
			compareText(t, args, py.stdout, gov.stdout)
			continue
		}
		var pr, gr jsonReport
		if err := json.Unmarshal([]byte(py.stdout), &pr); err != nil {
			t.Fatalf("%v: python JSON: %v\n%s", args, err, py.stdout)
		}
		if err := json.Unmarshal([]byte(gov.stdout), &gr); err != nil {
			t.Fatalf("%v: go JSON: %v\n%s", args, err, gov.stdout)
		}
		if !sameFindings(pr.Errors, gr.Errors) || !sameFindings(pr.Warnings, gr.Warnings) {
			t.Errorf("%v: findings differ\npython errors:   %q\ngo errors:       %q\npython warnings: %q\ngo warnings:     %q",
				args, pr.Errors, gr.Errors, pr.Warnings, gr.Warnings)
		}
		pr.Errors, pr.Warnings, gr.Errors, gr.Warnings = nil, nil, nil, nil
		if !reflect.DeepEqual(pr, gr) {
			t.Errorf("%v: JSON report differs\npython: %+v\ngo:     %+v", args, pr, gr)
		}
		if !strings.Contains(py.stdout, yamlErrorMarker) && py.stdout != gov.stdout {
			t.Errorf("%v: JSON text differs\npython:\n%s\ngo:\n%s", args, py.stdout, gov.stdout)
		}
	}
}

func compareText(t *testing.T, args []string, py, gov string) {
	t.Helper()
	pl, gl := strings.Split(py, "\n"), strings.Split(gov, "\n")
	same := len(pl) == len(gl)
	for i := 0; same && i < len(pl); i++ {
		same = sameFinding(pl[i], gl[i])
	}
	if !same {
		t.Errorf("%v: text output differs\npython:\n%s\ngo:\n%s", args, py, gov)
	}
}

func TestDifferentialRepoData(t *testing.T) {
	requireUV(t)
	dirs := []string{"../../testdata/fixture", "../../testdata/proto", "../../testdata/proto/hub"}
	repos, err := filepath.Glob("../../testdata/proto/*/okf")
	if err != nil || len(repos) == 0 {
		t.Fatalf("no prototype repo bundles: %v", err)
	}
	fixtureRepos, _ := filepath.Glob("../../testdata/fixture/*/okf")
	dirs = append(dirs, repos...)
	dirs = append(dirs, fixtureRepos...)
	dirs = append(dirs, "../../testdata/fixture/hub")
	for _, dir := range dirs {
		t.Run(dir, func(t *testing.T) {
			t.Parallel()
			compare(t, dir)
		})
	}
}

// fm wraps YAML lines into a concept file.
func fm(yaml string, body ...string) string {
	return "---\n" + yaml + "---\n" + strings.Join(body, "")
}

// ok is a frontmatter with every recommended field, so only the finding
// under test shows.
const ok = "type: Note\ntitle: T\ndescription: D\ntags: [a]\ngenerated: {by: human:dana, at: 2024-01-01T10:00:00Z}\n"

type bundleCase struct {
	name  string
	files map[string]string
	setup func(t *testing.T, dir string)
}

var bundleCases = []bundleCase{
	{name: "empty bundle"},
	{name: "clean concept", files: map[string]string{"a.md": fm(ok, "# A\n")}},
	{name: "missing frontmatter", files: map[string]string{"a.md": "# Title\n\ntext\n"}},
	{name: "unterminated frontmatter", files: map[string]string{"a.md": "---\ntype: Note\n"}},
	{name: "first line not a fence", files: map[string]string{"a.md": "--- x\ntype: Note\n---\n"}},
	{name: "fence with surrounding whitespace", files: map[string]string{"a.md": "--- \u00a0\n" + ok + " ---\t\nbody\n"}},
	{name: "only a fence line", files: map[string]string{"a.md": "---"}},
	{name: "yaml mapping values", files: map[string]string{"a.md": fm("type: Note\ntitle: Foo: bar\n")}},
	{name: "yaml mapping values line 1", files: map[string]string{"a.md": fm("title: Foo: bar\ntype: Note\n")}},
	{name: "yaml bad indentation", files: map[string]string{"a.md": fm("type: Note\n title: x\n")}},
	{name: "yaml unclosed quote", files: map[string]string{"a.md": fm("type: 'Note\n")}},
	{name: "yaml unclosed flow", files: map[string]string{"a.md": fm("type: Note\ntags: [a, b\n")}},
	{name: "yaml sequence then mapping", files: map[string]string{"a.md": fm("- a\ntype: Note\n")}},
	{name: "yaml reserved indicator", files: map[string]string{"a.md": fm("type: @note\n")}},
	{name: "yaml tab indentation", files: map[string]string{"a.md": fm("type: Note\ngenerated:\n\tby: x\n")}},
	{name: "yaml undefined alias", files: map[string]string{"a.md": fm("type: *nope\n")}},
	{name: "yaml duplicate anchor", files: map[string]string{"a.md": fm("type: &a Note\ntitle: &a T\n")}},
	{name: "yaml unknown tag", files: map[string]string{"a.md": fm("type: !note Note\n")}},
	{name: "yaml python tag", files: map[string]string{"a.md": fm("type: !!python/tuple [a]\n")}},
	{name: "yaml unhashable key", files: map[string]string{"a.md": fm("type: Note\n? [a, b]\n: c\n")}},
	{name: "yaml bad merge", files: map[string]string{"a.md": fm("type: Note\n<<: 5\n")}},
	{name: "yaml bad merge item", files: map[string]string{"a.md": fm("type: Note\n<<: [5]\n")}},
	{name: "yaml value key as value", files: map[string]string{"a.md": fm("type: Note\ntitle: =\n")}},
	{name: "yaml second document", files: map[string]string{"a.md": fm("type: Note\n--- !!str x\n")}},
	{name: "yaml control character", files: map[string]string{"a.md": fm("type: No\x01te\n")}},
	{name: "yaml str tag on mapping", files: map[string]string{"a.md": fm("type: !!str {a: b}\n")}},
	{name: "yaml bad base64", files: map[string]string{"a.md": fm("type: Note\ntitle: !!binary a\n")}},
	{name: "missing type", files: map[string]string{"a.md": fm("title: T\n")}},
	{name: "empty type", files: map[string]string{"a.md": fm("type: \"  \"\n")}},
	{name: "null type", files: map[string]string{"a.md": fm("type:\n")}},
	{name: "int type", files: map[string]string{"a.md": fm("type: 5\n")}},
	{name: "bool type", files: map[string]string{"a.md": fm("type: yes\n")}},
	{name: "list type", files: map[string]string{"a.md": fm("type: [Note]\n")}},
	{name: "quoted yes type", files: map[string]string{"a.md": fm("type: 'yes'\n")}},
	{name: "empty frontmatter", files: map[string]string{"a.md": "---\n---\nbody\n"}},
	{name: "comment-only frontmatter", files: map[string]string{"a.md": fm("# nothing\n")}},
	{name: "sequence frontmatter", files: map[string]string{"a.md": fm("- a\n- b\n")}},
	{name: "scalar frontmatter", files: map[string]string{"a.md": fm("just text\n")}},
	{name: "date frontmatter", files: map[string]string{"a.md": fm("2024-01-01\n")}},
	{name: "duplicate keys last wins", files: map[string]string{"a.md": fm("type: ''\n" + ok)}},
	{name: "merge keys", files: map[string]string{"a.md": fm("base: &b {type: Note, title: T}\n<<: *b\ndescription: D\n")}},
	{name: "merge list", files: map[string]string{"a.md": fm("<<: [{type: A}, {type: B, tags: []}]\ntitle: T\n")}},
	{name: "unknown keys", files: map[string]string{"a.md": fm(ok + "owner: team\nx-custom: [1, 2]\n")}},
	{name: "legacy timestamp", files: map[string]string{"a.md": fm("type: Note\ntitle: T\ndescription: D\ntags: []\ntimestamp: 2024-01-01\n")}},
	{name: "missing generated", files: map[string]string{"a.md": fm("type: Note\n")}},
	{name: "generated not mapping", files: map[string]string{"a.md": fm("type: Note\ngenerated: yesterday\n")}},
	{name: "generated without by", files: map[string]string{"a.md": fm("type: Note\ngenerated: {at: 2024-01-01}\n")}},
	{name: "generated null by", files: map[string]string{"a.md": fm("type: Note\ngenerated: {by: null}\n")}},
	{name: "actor near misses", files: map[string]string{
		"a.md": fm("type: Note\ngenerated: {by: Human:dana, at: 2024-01-01}\n"),
		"b.md": fm("type: Note\ngenerated: {by: human/dana, at: 2024-01-01}\n"),
		"c.md": fm("type: Note\ngenerated: {by: PROCESS-x, at: 2024-01-01}\n"),
		"d.md": fm("type: Note\ngenerated: {by: humanoid_agent/v1, at: 2024-01-01}\n"),
		"e.md": fm("type: Note\ngenerated: {by: humanſ, at: 2024-01-01}\n"),
		"f.md": fm("type: Note\ngenerated: {by: proceſs, at: 2024-01-01}\n"),
		"g.md": fm("type: Note\ngenerated: {by: human, at: 2024-01-01}\n"),
	}},
	{name: "actor shapes", files: map[string]string{
		"a.md": fm("type: Note\ngenerated: {by: ProcessBot, at: 2024-01-01}\n"),
		"b.md": fm("type: Note\ngenerated: {by: 'human: dana', at: 2024-01-01}\n"),
		"c.md": fm("type: Note\ngenerated: {by: claude/1.0, at: 2024-01-01}\n"),
		"d.md": fm("type: Note\ngenerated: {by: 'team:a b', at: 2024-01-01}\n"),
		"e.md": fm("type: Note\ngenerated: {by: 42, at: 2024-01-01}\n"),
		"f.md": fm("type: Note\ngenerated: {by: \"a:b\\u00a0c\", at: 2024-01-01}\n"),
	}},
	{name: "generated at values", files: map[string]string{
		"a.md": fm("type: Note\ngenerated: {by: human:a, at: yesterday}\n"),
		"b.md": fm("type: Note\ngenerated: {by: human:a, at: 12:30}\n"),
		"c.md": fm("type: Note\ngenerated: {by: human:a, at: 1.5}\n"),
		"d.md": fm("type: Note\ngenerated: {by: human:a, at: 2024-01-01 10:00:00.5 +05:30}\n"),
		"e.md": fm("type: Note\ngenerated: {by: human:a, at: 2024-01-01T10:00:00-05:00}\n"),
		"f.md": fm("type: Note\ngenerated: {by: human:a, at: '2024-01-01T10:00'}\n"),
		"g.md": fm("type: Note\ngenerated: {by: human:a, at: '２０２４-01-01'}\n"),
		"h.md": fm("type: Note\ngenerated: {by: human:a, at: [2024-01-01]}\n"),
		"i.md": fm("type: Note\ngenerated: {by: human:a, at: {d: 2024-01-01 10:00:00}}\n"),
	}},
	{name: "verified variants", files: map[string]string{
		"a.md": fm("type: Note\ngenerated: {by: human:a}\nverified: {by: human:b, at: 2024-01-01}\n"),
		"b.md": fm("type: Note\ngenerated: {by: human:a}\nverified: [{by: human:b}, {at: 2024-01-01}, x, {by: Human:c, at: soon}]\n"),
		"c.md": fm("type: Note\ngenerated: {by: human:a}\nverified: human:b\n"),
		"d.md": fm("type: Note\ngenerated: {by: human:a}\nverified: []\n"),
	}},
	{name: "lifecycle values", files: map[string]string{
		"a.md": fm("type: Note\nstatus: draft\nstale_after: 2024-01-01\n"),
		"b.md": fm("type: Note\nstatus: yes\nstale_after: next week\n"),
		"c.md": fm("type: Note\nstatus: 5\nstale_after: 2024-01-01 10:00:00\n"),
		"d.md": fm("type: Note\nstatus: 1.50\nstale_after: \"2024-01-01\\n\"\n"),
		"e.md": fm("type: Note\nstatus: 2024-01-01\nstale_after: 2024-01-01T10:00:00.123456789Z\n"),
		"f.md": fm("type: Note\nstatus: Stable\nstale_after: \" 2024-01-01\"\n"),
		"g.md": fm("type: Note\nstatus: !!binary aGVsbG8=\nstale_after: 20240101\n"),
		"h.md": fm("type: Note\nstatus: ~\nstale_after: [2024-01-01]\n"),
	}},
	{name: "number formatting", files: map[string]string{
		"a.md": fm("type: Note\nstatus: 1.0e+400\nstale_after: 100000000000000000.0\n"),
		"b.md": fm("type: Note\nstatus: 0.00001\nstale_after: 190:20:30.15\n"),
		"c.md": fm("type: Note\nstatus: 0x1F\nstale_after: 017\n"),
		"d.md": fm("type: Note\nstatus: 0b101\nstale_after: 1:30\n"),
		"e.md": fm("type: Note\nstatus: -.inf\nstale_after: .nan\n"),
		"f.md": fm("type: Note\nstatus: 123456789012345678901234567890\nstale_after: -0.0\n"),
		"g.md": fm("type: Note\nstatus: 1_000.5\nstale_after: +12_345\n"),
		"h.md": fm("type: Note\nstatus: 0.1\nstale_after: 1e5\n"),
		"i.md": fm("type: Note\nstatus: 08\nstale_after: 1.\n"),
		"j.md": fm("type: Note\nstatus: !!float 3\nstale_after: !!int '7'\n"),
	}},
	{name: "value formatting", files: map[string]string{
		"a.md": fm("type: Note\nstatus: \"it's\"\nstale_after: {a: 'it''s', b: \"x\\ty\", c: null, d: true}\n"),
		"b.md": fm("type: Note\nstatus: Off\nstale_after: !!omap [{a: 1}]\n"),
		"c.md": fm("type: Note\nstale_after: [2024-01-01, 2024-01-01 10:00:00, 2024-01-01T10:00:00+01:00, 2024-01-01 10:00:00.25Z]\n"),
		"d.md": fm("type: Note\nstale_after: [\"a'b\", 'a\"b', \"a'\\\"b\", \"é\\u0085\\u200b\\U0001F600\"]\n"),
		"e.md": fm("type: Note\nstale_after: !!binary \"J1x4\"\n"),
		"f.md": fm("type: Note\nstale_after: [!!set {}]\n"),
	}},
	{name: "crash unhashable status list", files: map[string]string{"a.md": fm("type: Note\nstatus: [a]\n")}},
	{name: "crash unhashable status dict", files: map[string]string{"a.md": fm("type: Note\nstatus: {a: 1}\n")}},
	{name: "status set", files: map[string]string{"a.md": fm("type: Note\nstatus: !!set {a}\n")}},
	{name: "crash unhashable status in tuple", files: map[string]string{"a.md": fm("type: Note\nstatus: !!pairs [{a: [1]}]\n")}},
	{name: "crash impossible month", files: map[string]string{"a.md": fm("type: Note\ngenerated: {by: human:a, at: 2024-13-01}\n")}},
	{name: "crash impossible day", files: map[string]string{"a.md": fm("type: Note\nstale_after: 2024-02-30\n")}},
	{name: "crash impossible hour", files: map[string]string{"a.md": fm("type: Note\nstale_after: 2024-01-01 25:00:00\n")}},
	{name: "crash impossible offset", files: map[string]string{"a.md": fm("type: Note\nstale_after: 2024-01-01T10:00:00+25:00\n")}},
	{name: "crash explicit int", files: map[string]string{"a.md": fm("type: Note\nstatus: !!int abc\n")}},
	{name: "crash explicit bool", files: map[string]string{"a.md": fm("type: Note\nstatus: !!bool maybe\n")}},
	{name: "crash explicit timestamp", files: map[string]string{"a.md": fm("type: Note\nstatus: !!timestamp soon\n")}},
	{name: "crash root index list", files: map[string]string{"index.md": fm("- okf_version\n")}},
	{name: "crash root index int", files: map[string]string{"index.md": fm("5\n")}},
	{name: "crash root index string", files: map[string]string{"index.md": fm("text\n")}},
	{name: "body citations", files: map[string]string{
		"a.md": fm(ok, "# A\n\n# Citations\n\n- [x](https://x)\n"),
		"b.md": fm(ok, "### Citations  \n"),
		"c.md": fm(ok, "#Citations\n####### Citations\n"),
	}},
	{name: "sources family", files: map[string]string{
		"a.md": fm(ok + "sources: x\n"),
		"b.md": fm(ok+"sources: [x, {}, {resource: ' '}, {resource: r, id: 1, author: Human:z}]\n", "Claim[^1] and[^2] and [^b] [^a] [^ä]\n"),
		"c.md": fm(ok + "usage_window: {from: 2024-01-01}\nsources:\n- {resource: r, usage_count: 3}\n- {resource: r, usage_count: 3, usage_window: null}\n- {resource: r, usage_window: x}\n- {resource: r, usage_window: {from: soon, to: 2024-01-01 10:00:00}}\n- {resource: r, last_modified: yesterday}\n- {resource: r, last_modified: 2024-01-01}\n"),
		"d.md": fm(ok+"sources: [{resource: r, id: a}]\n", "[^a][^ b]\n[^a]: def\n"),
	}},
	{name: "attested computation", files: map[string]string{
		"scripts/run.py": "print(1)\n",
		"a.md":           fm("type: Attested Computation\ncomputation: scripts/missing.py\nexecutor: {resource: /scripts/run.py}\nattester: {resource: /scripts/gone.py}\n"),
		"b.md":           fm("type: ' Attested Computation '\nruntime: null\ncomputation: https://x/y.py\nexecutor: {resource: mailto:a@b}\nattester: x\n"),
		"c.md":           fm("type: Attested Computation\nruntime: python\ncomputation: ../../outside.py\n"),
		"d.md":           fm("type: Attested Computation\nruntime: ' '\ncomputation: scripts/run.py\nexecutor: {resource: ''}\n"),
		"e.md":           fm("type: Attested Computation\nruntime: py\ncomputation: 5\nattester: {resource: /}\n"),
	}},
	{name: "links", files: map[string]string{
		"a.md": fm(ok,
			"[ok](b.md) [miss](missing.md) [frag](b.md#x) [frag only](#x) [dir](sub/)\n",
			"![img](img-missing.md) [titled](t-missing.md \"Title\") [abs](/b.md) [abs miss](/nope.md)\n",
			"[http](https://x/y.md) [mail](mailto:a@b.md) [up](../outside.md) [dot](./b.md) [deep](sub/../b.md)\n",
			"[two](c1.md)[three](c2.md) [a]](x.md) [nbsp](b\u00a0c.md) [notmd](b.txt) [[nested](n.md)\n",
			"```\n[fenced](fenced.md)\n```\n  ~~~\n[tilde](tilde.md)\n~~~\n[after](after.md)\n"),
		"b.md":       fm(ok, "[back](a.md) [sub](sub/c.md) [root abs](//a.md)\n"),
		"sub/c.md":   fm(ok, "[up](../b.md) [self](c.md) [missing](../../x.md) [abs](/sub/c.md)\n"),
		"index.md":   "# Index\n[a](a.md) [gone](gone.md)\n",
		"sub/log.md": "# Log\n[gone](gone.md)\n",
	}},
	{name: "unclosed fence", files: map[string]string{"a.md": fm(ok, "```\n[x](missing.md)\n")}},
	{name: "reserved files", files: map[string]string{
		"index.md":          fm("okf_version: \"0.2\"\nupkeep: {enforce: true}\n", "# Index\n"),
		"sub/index.md":      fm("title: nested\n", "# Sub\n"),
		"sub/deep/index.md": "# no frontmatter\n",
		"log.md":            fm("x: 1\n", "## 2024-01-01\n## Jan 1\n##  2024-01-02  \n### 2024\n## ２０２４-01-01\n## 2024-1-1\n"),
		"sub/log.md":        "## 2024-01-01 \u2028## bad\u2029\n",
	}},
	{name: "root index versions", files: map[string]string{"index.md": fm("okf_version: 0.1\n")}},
	{name: "root index float version", files: map[string]string{"index.md": fm("okf_version: 0.20\n")}},
	{name: "root index extra keys", files: map[string]string{"index.md": fm("zeta: 1\nalpha: 2\nupkeep: x\nokf_version: '0.2'\n")}},
	{name: "root index key equality", files: map[string]string{"index.md": fm("1: a\ntrue: b\n")}},
	{name: "root index date key", files: map[string]string{"index.md": fm("2024-01-01: x\n")}},
	{name: "root index odd key", files: map[string]string{"index.md": fm("\"a\\tb'c\": 1\n")}},
	{name: "root index null version", files: map[string]string{"index.md": fm("okf_version:\n")}},
	{name: "root index invalid yaml", files: map[string]string{"index.md": fm("okf_version: [\n")}},
	{name: "root index empty", files: map[string]string{"index.md": "---\n---\n# Index\n"}},
	{name: "root index falsy", files: map[string]string{"index.md": fm("0\n")}},
	{name: "file kinds", files: map[string]string{
		"notes.txt":     "no frontmatter\n",
		"README":        "no frontmatter\n",
		"upper.MD":      "no frontmatter\n",
		".hidden.md":    "no frontmatter\n",
		".git/x.md":     "no frontmatter\n",
		"dir.md/a.md":   fm(ok),
		"a/b/c/deep.md": "no frontmatter\n",
		"a-b/x.md":      fm(ok),
		"a/x.md":        fm(ok),
		"Z.md":          fm(ok),
		"é.md":          fm(ok),
	}},
	{name: "encodings", files: map[string]string{
		"bom.md":     "\ufeff\ufeff" + fm(ok),
		"crlf.md":    strings.ReplaceAll(fm(ok, "[x](missing.md)\n## 2024\n"), "\n", "\r\n"),
		"cr.md":      strings.ReplaceAll(fm(ok), "\n", "\r"),
		"ff.md":      "---\x0c" + ok + "---\n",
		"vt.md":      "---\n" + ok + "---\x0bbody\n",
		"bad1.md":    "\xff" + fm(ok),
		"bad2.md":    fm(ok) + "trailing \xe2\x82",
		"bad3.md":    fm(ok) + "\xe2\x28\xa1",
		"bad4.md":    fm(ok) + "\xf0\x9f\x98",
		"bad5.md":    fm(ok) + "\xed\xa0\x80",
		"bad6.md":    fm(ok) + "\xf0\x9f\x28\x80",
		"emoji.md":   fm(ok + "status: \"😀\"\n"),
		"index.md":   "\xc3\x28",
		"sub/log.md": "\x80",
	}},
	{name: "symlinks", files: map[string]string{
		"real/target.md": fm(ok, "[link](../alias.md) [loop](loopdir/x.md)\n"),
		"outside/x.md":   fm(ok),
	}, setup: func(t *testing.T, dir string) {
		must(t, os.Symlink("real/target.md", filepath.Join(dir, "alias.md")))
		must(t, os.Symlink("real", filepath.Join(dir, "linkdir")))
		must(t, os.Symlink("nowhere.md", filepath.Join(dir, "broken.md")))
		must(t, os.Symlink(filepath.Join(dir, "outside", "x.md"), filepath.Join(dir, "abs.md")))
		must(t, os.WriteFile(filepath.Join(dir, "via.md"), []byte(fm(ok, "[through](linkdir/target.md) [alias](alias.md) [dir](linkdir/../alias.md)\n")), 0o644))
	}},
	{name: "unreadable file", files: map[string]string{"a.md": fm(ok)}, setup: func(t *testing.T, dir string) {
		if os.Geteuid() == 0 {
			t.Skip("root reads mode 000 files")
		}
		must(t, os.WriteFile(filepath.Join(dir, "locked.md"), []byte(fm(ok)), 0o000))
		must(t, os.MkdirAll(filepath.Join(dir, "lockeddir"), 0o755))
		must(t, os.WriteFile(filepath.Join(dir, "lockeddir", "x.md"), []byte("no frontmatter\n"), 0o644))
		must(t, os.Chmod(filepath.Join(dir, "lockeddir"), 0o000))
		t.Cleanup(func() { os.Chmod(filepath.Join(dir, "lockeddir"), 0o755) })
	}},
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func writeBundle(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		must(t, os.MkdirAll(filepath.Dir(p), 0o755))
		must(t, os.WriteFile(p, []byte(content), 0o644))
	}
}

func TestDifferentialBrokenBundles(t *testing.T) {
	requireUV(t)
	for _, c := range bundleCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeBundle(t, dir, c.files)
			if c.setup != nil {
				c.setup(t, dir)
			}
			compare(t, dir)
		})
	}
}

// TestDifferentialArguments covers the bundle argument itself: path
// normalization in the report and a path that is not a directory.
func TestDifferentialArguments(t *testing.T) {
	requireUV(t)
	dir := t.TempDir()
	writeBundle(t, dir, map[string]string{"b/a.md": "no frontmatter\n", "file.md": "x"})
	wd, err := os.Getwd()
	must(t, err)
	must(t, os.Chdir(dir))
	defer func() { must(t, os.Chdir(wd)) }()
	for _, arg := range []string{"b/", "./b", "b//", ".", "", "file.md", "missing", "file.md/"} {
		t.Run(strconv.Quote(arg), func(t *testing.T) { compare(t, arg) })
	}
}
