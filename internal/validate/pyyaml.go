package validate

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// loadYAML is the reference validator's `yaml.safe_load(raw)`: yaml.v3 parses
// the text and the node tree is then resolved and constructed with pyyaml's
// SafeLoader rules (YAML 1.1 implicit types, merge keys, last duplicate key
// wins, dates and datetimes), so the validator sees the same Python objects.
//
// A *yamlError is a YAMLError: the frontmatter is not valid YAML. Its text is
// yaml.v3's ("line N: problem") or, for errors pyyaml raises after parsing,
// this package's; the wording differs from pyyaml's, the finding does not.
// Inputs on which pyyaml raises a non-YAML exception (an impossible date, an
// explicit `!!int` that is not a number) panic with a pyCrashError.
func loadYAML(raw string) (any, error) {
	dec := yaml.NewDecoder(strings.NewReader(raw))
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		return nil, newYAMLError(err)
	}
	var next yaml.Node
	switch err := dec.Decode(&next); {
	case errors.Is(err, io.EOF):
	case err != nil:
		return nil, newYAMLError(err)
	default:
		return nil, yamlErrorAt(&next, "expected a single document in the stream, but found another document")
	}
	if len(doc.Content) == 0 {
		return nil, nil
	}
	root := doc.Content[0]
	if err := checkAnchors(root, map[string]bool{}); err != nil {
		return nil, err
	}
	c := &constructor{built: map[*yaml.Node]any{}, building: map[*yaml.Node]bool{}}
	return c.object(root)
}

type yamlError struct{ msg string }

func (e *yamlError) Error() string { return e.msg }

func newYAMLError(err error) error {
	return &yamlError{strings.TrimPrefix(err.Error(), "yaml: ")}
}

func yamlErrorAt(n *yaml.Node, format string, args ...any) error {
	return &yamlError{fmt.Sprintf("line %d: ", n.Line) + fmt.Sprintf(format, args...)}
}

// checkAnchors rejects a redefined anchor, which pyyaml's composer refuses
// and yaml.v3 accepts.
func checkAnchors(n *yaml.Node, seen map[string]bool) error {
	if n.Kind == yaml.AliasNode {
		return nil
	}
	if n.Anchor != "" {
		if seen[n.Anchor] {
			return yamlErrorAt(n, "found duplicate anchor %s", pyStrRepr(n.Anchor))
		}
		seen[n.Anchor] = true
	}
	for _, c := range n.Content {
		if err := checkAnchors(c, seen); err != nil {
			return err
		}
	}
	return nil
}

// pyyaml's implicit resolvers (yaml/resolver.py), tried in registration order
// for the first character of a plain scalar.
var (
	resolveBool  = regexp.MustCompile(`^(?:yes|Yes|YES|no|No|NO|true|True|TRUE|false|False|FALSE|on|On|ON|off|Off|OFF)\z`)
	resolveFloat = regexp.MustCompile(`^(?:[-+]?(?:[0-9][0-9_]*)\.[0-9_]*(?:[eE][-+][0-9]+)?` +
		`|\.[0-9][0-9_]*(?:[eE][-+][0-9]+)?` +
		`|[-+]?[0-9][0-9_]*(?::[0-5]?[0-9])+\.[0-9_]*` +
		`|[-+]?\.(?:inf|Inf|INF)` +
		`|\.(?:nan|NaN|NAN))\z`)
	resolveInt = regexp.MustCompile(`^(?:[-+]?0b[0-1_]+` +
		`|[-+]?0[0-7_]+` +
		`|[-+]?(?:0|[1-9][0-9_]*)` +
		`|[-+]?0x[0-9a-fA-F_]+` +
		`|[-+]?[1-9][0-9_]*(?::[0-5]?[0-9])+)\z`)
	resolveNull      = regexp.MustCompile(`^(?:~|null|Null|NULL|)\z`)
	resolveTimestamp = regexp.MustCompile(`^(?:[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]` +
		`|[0-9][0-9][0-9][0-9]-[0-9][0-9]?-[0-9][0-9]?` +
		`(?:[Tt]|[ \t]+)[0-9][0-9]?` +
		`:[0-9][0-9]:[0-9][0-9](?:\.[0-9]*)?` +
		`(?:[ \t]*(?:Z|[-+][0-9][0-9]?(?::[0-9][0-9])?))?)\z`)
	// SafeConstructor.timestamp_regexp
	timestampParts = regexp.MustCompile(`^([0-9][0-9][0-9][0-9])-([0-9][0-9]?)-([0-9][0-9]?)` +
		`(?:(?:[Tt]|[ \t]+)([0-9][0-9]?):([0-9][0-9]):([0-9][0-9])(?:\.([0-9]*))?` +
		`(?:[ \t]*(Z|([-+])([0-9][0-9]?)(?::([0-9][0-9]))?))?)?\n?\z`)
)

func resolveImplicit(v string) string {
	type resolver struct {
		tag   string
		re    *regexp.Regexp
		first string
	}
	resolvers := []resolver{
		{"!!bool", resolveBool, "yYnNtTfFoO"},
		{"!!float", resolveFloat, "-+0123456789."},
		{"!!int", resolveInt, "-+0123456789"},
		{"!!merge", nil, "<"},
		{"!!null", resolveNull, "~nN"},
		{"!!timestamp", resolveTimestamp, "0123456789"},
		{"!!value", nil, "="},
	}
	if v == "" {
		return "!!null"
	}
	for _, r := range resolvers {
		if !strings.Contains(r.first, v[:1]) {
			continue
		}
		switch {
		case r.tag == "!!merge" && v == "<<", r.tag == "!!value" && v == "=":
			return r.tag
		case r.re != nil && r.re.MatchString(v):
			return r.tag
		}
	}
	return "!!str"
}

const (
	quotedStyles = yaml.DoubleQuotedStyle | yaml.SingleQuotedStyle | yaml.LiteralStyle | yaml.FoldedStyle
)

// nodeTag is the tag pyyaml's composer gives the node.
func nodeTag(n *yaml.Node) string {
	if n.Style&yaml.TaggedStyle != 0 {
		return n.ShortTag()
	}
	switch n.Kind {
	case yaml.MappingNode:
		return "!!map"
	case yaml.SequenceNode:
		return "!!seq"
	}
	if n.Style&quotedStyles != 0 {
		return "!!str"
	}
	return resolveImplicit(n.Value)
}

func deref(n *yaml.Node) *yaml.Node {
	for n.Kind == yaml.AliasNode && n.Alias != nil {
		n = n.Alias
	}
	return n
}

func nodeID(n *yaml.Node) string {
	switch n.Kind {
	case yaml.MappingNode:
		return "mapping"
	case yaml.SequenceNode:
		return "sequence"
	}
	return "scalar"
}

type constructor struct {
	built    map[*yaml.Node]any
	building map[*yaml.Node]bool
}

func (c *constructor) object(n *yaml.Node) (any, error) {
	n = deref(n)
	if v, ok := c.built[n]; ok {
		return v, nil
	}
	if c.building[n] {
		return nil, yamlErrorAt(n, "found unconstructable recursive node")
	}
	c.building[n] = true
	defer delete(c.building, n)
	v, err := c.construct(n, nodeTag(n))
	if err != nil {
		return nil, err
	}
	c.built[n] = v
	return v, nil
}

func (c *constructor) construct(n *yaml.Node, tag string) (any, error) {
	switch tag {
	case "!!null":
		if _, err := c.scalar(n); err != nil {
			return nil, err
		}
		return nil, nil
	case "!!bool":
		s, err := c.scalar(n)
		if err != nil {
			return nil, err
		}
		switch strings.ToLower(s) {
		case "yes", "true", "on":
			return true, nil
		case "no", "false", "off":
			return false, nil
		}
		panic(pyCrash("KeyError", pyStrRepr(strings.ToLower(s))))
	case "!!int":
		s, err := c.scalar(n)
		if err != nil {
			return nil, err
		}
		return constructInt(s), nil
	case "!!float":
		s, err := c.scalar(n)
		if err != nil {
			return nil, err
		}
		return constructFloat(s), nil
	case "!!binary":
		s, err := c.scalar(n)
		if err != nil {
			return nil, err
		}
		return constructBinary(n, s)
	case "!!timestamp":
		if _, err := c.scalar(n); err != nil {
			return nil, err
		}
		return constructTimestamp(n.Value), nil
	case "!!str":
		return c.scalar(n)
	case "!!seq":
		if n.Kind != yaml.SequenceNode {
			return nil, yamlErrorAt(n, "expected a sequence node, but found %s", nodeID(n))
		}
		list := make(pyList, 0, len(n.Content))
		for _, item := range n.Content {
			v, err := c.object(item)
			if err != nil {
				return nil, err
			}
			list = append(list, v)
		}
		return list, nil
	case "!!map":
		return c.mapping(n)
	case "!!set":
		d, err := c.mapping(n)
		if err != nil {
			return nil, err
		}
		s := &pySet{}
		for _, k := range d.keys {
			s.add(k)
		}
		return s, nil
	case "!!omap", "!!pairs":
		if n.Kind != yaml.SequenceNode {
			return nil, yamlErrorAt(n, "expected a sequence, but found %s", nodeID(n))
		}
		list := pyList{}
		for _, item := range n.Content {
			sub := deref(item)
			if sub.Kind != yaml.MappingNode {
				return nil, yamlErrorAt(sub, "expected a mapping of length 1, but found %s", nodeID(sub))
			}
			if len(sub.Content) != 2 {
				return nil, yamlErrorAt(sub, "expected a single mapping item, but found %d items", len(sub.Content)/2)
			}
			k, err := c.object(sub.Content[0])
			if err != nil {
				return nil, err
			}
			v, err := c.object(sub.Content[1])
			if err != nil {
				return nil, err
			}
			list = append(list, pyTuple{k, v})
		}
		return list, nil
	}
	return nil, yamlErrorAt(n, "could not determine a constructor for the tag %s", pyStrRepr(tag))
}

// scalar is SafeConstructor.construct_scalar.
func (c *constructor) scalar(n *yaml.Node) (string, error) {
	if n.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(n.Content); i += 2 {
			if nodeTag(deref(n.Content[i])) == "!!value" {
				return c.scalar(deref(n.Content[i+1]))
			}
		}
	}
	if n.Kind != yaml.ScalarNode {
		return "", yamlErrorAt(n, "expected a scalar node, but found %s", nodeID(n))
	}
	return n.Value, nil
}

type mappingPair struct {
	key, value *yaml.Node
	keyTag     string
}

// flatten is SafeConstructor.flatten_mapping: merge keys are expanded in
// front of the mapping's own pairs, and a `=` key becomes a plain string.
func (c *constructor) flatten(n *yaml.Node) ([]mappingPair, error) {
	var merge, own []mappingPair
	for i := 0; i+1 < len(n.Content); i += 2 {
		key, value := deref(n.Content[i]), deref(n.Content[i+1])
		switch tag := nodeTag(key); tag {
		case "!!merge":
			switch value.Kind {
			case yaml.MappingNode:
				sub, err := c.flatten(value)
				if err != nil {
					return nil, err
				}
				merge = append(merge, sub...)
			case yaml.SequenceNode:
				var subs [][]mappingPair
				for _, item := range value.Content {
					item = deref(item)
					if item.Kind != yaml.MappingNode {
						return nil, yamlErrorAt(item, "expected a mapping for merging, but found %s", nodeID(item))
					}
					sub, err := c.flatten(item)
					if err != nil {
						return nil, err
					}
					subs = append(subs, sub)
				}
				for j := len(subs) - 1; j >= 0; j-- {
					merge = append(merge, subs[j]...)
				}
			default:
				return nil, yamlErrorAt(value, "expected a mapping or list of mappings for merging, but found %s", nodeID(value))
			}
		case "!!value":
			own = append(own, mappingPair{key, value, "!!str"})
		default:
			own = append(own, mappingPair{key, value, tag})
		}
	}
	return append(merge, own...), nil
}

func (c *constructor) mapping(n *yaml.Node) (*pyDict, error) {
	if n.Kind != yaml.MappingNode {
		return nil, yamlErrorAt(n, "expected a mapping node, but found %s", nodeID(n))
	}
	pairs, err := c.flatten(n)
	if err != nil {
		return nil, err
	}
	d := &pyDict{}
	for _, p := range pairs {
		var key any
		if p.keyTag == nodeTag(p.key) {
			key, err = c.object(p.key)
		} else {
			key, err = c.construct(p.key, p.keyTag)
		}
		if err != nil {
			return nil, err
		}
		if pyUnhashable(key) != "" {
			return nil, yamlErrorAt(p.key, "found unhashable key")
		}
		value, err := c.object(p.value)
		if err != nil {
			return nil, err
		}
		d.set(key, value)
	}
	return d, nil
}

// pyInt is int(s, base) for an underscore-free s, raising ValueError the way
// CPython does.
func pyInt(s string, base int) *big.Int {
	v, ok := new(big.Int).SetString(strings.TrimFunc(s, pyIsSpace), base)
	if !ok {
		panic(pyCrash("ValueError", fmt.Sprintf("invalid literal for int() with base %d: %s", base, pyStrRepr(s))))
	}
	return v
}

// constructInt is SafeConstructor.construct_yaml_int.
func constructInt(s string) *big.Int {
	value := strings.ReplaceAll(s, "_", "")
	if value == "" {
		panic(pyCrash("IndexError", "string index out of range"))
	}
	sign := big.NewInt(1)
	if value[0] == '-' {
		sign = big.NewInt(-1)
	}
	if value[0] == '+' || value[0] == '-' {
		value = value[1:]
	}
	var out *big.Int
	switch {
	case value == "0":
		return big.NewInt(0)
	case strings.HasPrefix(value, "0b"):
		out = pyInt(value[2:], 2)
	case strings.HasPrefix(value, "0x"):
		out = pyInt(value[2:], 16)
	case strings.HasPrefix(value, "0"):
		out = pyInt(value, 8)
	case strings.Contains(value, ":"):
		parts := strings.Split(value, ":")
		digits := make([]*big.Int, len(parts))
		for i, part := range parts {
			digits[i] = pyInt(part, 10)
		}
		out = new(big.Int)
		base := big.NewInt(1)
		for i := len(digits) - 1; i >= 0; i-- {
			out.Add(out, new(big.Int).Mul(digits[i], base))
			base.Mul(base, big.NewInt(60))
		}
	default:
		out = pyInt(value, 10)
	}
	return out.Mul(out, sign)
}

// pyFloat is float(s) for the strings construct_yaml_float passes it.
func pyFloat(s string) float64 {
	t := strings.TrimFunc(s, pyIsSpace)
	switch strings.ToLower(strings.TrimLeft(t, "+-")) {
	case "inf", "infinity", "nan":
		f, _ := strconv.ParseFloat(t, 64)
		return f
	}
	valid := t != "" && !strings.ContainsAny(t, "_xXpP")
	f, err := strconv.ParseFloat(t, 64)
	if err != nil && !errors.Is(err, strconv.ErrRange) || !valid {
		panic(pyCrash("ValueError", "could not convert string to float: "+pyStrRepr(s)))
	}
	return f
}

// constructFloat is SafeConstructor.construct_yaml_float.
func constructFloat(s string) float64 {
	value := strings.ToLower(strings.ReplaceAll(s, "_", ""))
	if value == "" {
		panic(pyCrash("IndexError", "string index out of range"))
	}
	sign := 1.0
	if value[0] == '-' {
		sign = -1
	}
	if value[0] == '+' || value[0] == '-' {
		value = value[1:]
	}
	switch {
	case value == ".inf":
		return sign * math.Inf(1)
	case value == ".nan":
		return math.NaN()
	case strings.Contains(value, ":"):
		parts := strings.Split(value, ":")
		digits := make([]float64, len(parts))
		for i, part := range parts {
			digits[i] = pyFloat(part)
		}
		out, base := 0.0, 1.0
		for i := len(digits) - 1; i >= 0; i-- {
			out += digits[i] * base
			base *= 60
		}
		return sign * out
	}
	return sign * pyFloat(value)
}

// constructBinary is SafeConstructor.construct_yaml_binary;
// base64.decodebytes skips characters outside the alphabet.
func constructBinary(n *yaml.Node, s string) (any, error) {
	for _, r := range s {
		if r > 0x7f {
			return nil, yamlErrorAt(n, "failed to convert base64 data into ascii")
		}
	}
	var clean strings.Builder
	for i := range len(s) {
		c := s[i]
		if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '+' || c == '/' || c == '=' {
			clean.WriteByte(c)
		}
	}
	data, err := base64.StdEncoding.DecodeString(clean.String())
	if err != nil {
		return nil, yamlErrorAt(n, "failed to decode base64 data: %v", err)
	}
	return pyBytes(data), nil
}

// constructTimestamp is SafeConstructor.construct_yaml_timestamp, including
// the ValueError datetime raises for an impossible date or time.
func constructTimestamp(value string) any {
	m := timestampParts.FindStringSubmatch(value)
	if m == nil {
		panic(pyCrash("AttributeError", "'NoneType' object has no attribute 'groupdict'"))
	}
	atoi := func(s string) int { v, _ := strconv.Atoi(s); return v }
	d := pyDate{atoi(m[1]), atoi(m[2]), atoi(m[3])}
	checkDate := func() {
		switch {
		case d.year < 1:
			panic(pyCrash("ValueError", fmt.Sprintf("year %d is out of range", d.year)))
		case d.month < 1 || d.month > 12:
			panic(pyCrash("ValueError", "month must be in 1..12"))
		case d.day < 1 || d.day > daysIn(d.year, d.month):
			panic(pyCrash("ValueError", "day is out of range for month"))
		}
	}
	if m[4] == "" {
		checkDate()
		return d
	}
	t := pyDateTime{pyDate: d, hour: atoi(m[4]), minute: atoi(m[5]), second: atoi(m[6])}
	if m[7] != "" {
		frac := m[7]
		if len(frac) > 6 {
			frac = frac[:6]
		}
		t.micro = atoi(frac + strings.Repeat("0", 6-len(frac)))
	}
	// datetime.timezone(delta) is built before datetime() validates the rest.
	if m[9] != "" {
		t.offset = atoi(m[10])*3600 + atoi(m[11])*60
		if m[9] == "-" {
			t.offset = -t.offset
		}
		if t.offset <= -86400 || t.offset >= 86400 {
			panic(pyCrash("ValueError", "offset must be a timedelta strictly between -timedelta(hours=24) and timedelta(hours=24), not "+pyTimedeltaRepr(t.offset)+"."))
		}
		t.aware = true
	} else if m[8] != "" {
		t.aware = true
	}
	checkDate()
	switch {
	case t.hour > 23:
		panic(pyCrash("ValueError", "hour must be in 0..23"))
	case t.minute > 59:
		panic(pyCrash("ValueError", "minute must be in 0..59"))
	case t.second > 59:
		panic(pyCrash("ValueError", "second must be in 0..59"))
	}
	return t
}

func pyTimedeltaRepr(secs int) string {
	tz := pyTimezoneRepr(secs)
	return strings.TrimSuffix(strings.TrimPrefix(tz, "datetime.timezone("), ")")
}

func daysIn(year, month int) int {
	switch month {
	case 2:
		if isLeap(year) {
			return 29
		}
		return 28
	case 4, 6, 9, 11:
		return 30
	}
	return 31
}
