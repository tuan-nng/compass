package validate

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"unicode"
)

// Frontmatter is decoded into the Python objects pyyaml's SafeLoader builds,
// because the findings print them with str()/repr() and branch on their type:
//
//	None → nil            bool → bool          int → *big.Int
//	float → float64       str → string         bytes → pyBytes
//	datetime.date → pyDate                     datetime.datetime → pyDateTime
//	list → pyList         tuple → pyTuple      dict → *pyDict    set → *pySet
type (
	pyBytes []byte
	pyList  []any
	pyTuple []any
)

type pyDate struct{ year, month, day int }

type pyDateTime struct {
	pyDate
	hour, minute, second, micro int
	aware                       bool
	offset                      int // seconds east of UTC when aware
}

// pyDict keeps insertion order. Keys compare with Python equality, so 1, 1.0
// and True are one key: a repeated key keeps its first position and object
// and takes the last value, as `mapping[key] = value` does.
type pyDict struct {
	keys, vals []any
}

func (d *pyDict) index(key any) int {
	for i, k := range d.keys {
		if pyEq(k, key) {
			return i
		}
	}
	return -1
}

func (d *pyDict) set(key, val any) {
	if i := d.index(key); i >= 0 {
		d.vals[i] = val
		return
	}
	d.keys = append(d.keys, key)
	d.vals = append(d.vals, val)
}

// get is dict.get(key): nil when absent.
func (d *pyDict) get(key string) any {
	v, _ := d.lookup(key)
	return v
}

func (d *pyDict) lookup(key string) (any, bool) {
	if i := d.index(key); i >= 0 {
		return d.vals[i], true
	}
	return nil, false
}

func (d *pyDict) has(key string) bool {
	return d.index(key) >= 0
}

// pySet keeps insertion order; CPython iterates a set in hash order, which is
// randomized for str, so a printed set can only match by luck.
type pySet struct{ items []any }

func (s *pySet) add(v any) {
	for _, it := range s.items {
		if pyEq(it, v) {
			return
		}
	}
	s.items = append(s.items, v)
}

// pyCrashError is an uncaught Python exception: the reference validator dies
// with a traceback and exit code 1, before printing any report.
type pyCrashError struct{ typ, msg string }

func (e pyCrashError) Error() string { return e.typ + ": " + e.msg }

func pyCrash(typ, msg string) pyCrashError { return pyCrashError{typ, msg} }

func pyTypeName(v any) string {
	switch v.(type) {
	case nil:
		return "NoneType"
	case bool:
		return "bool"
	case *big.Int:
		return "int"
	case float64:
		return "float"
	case string:
		return "str"
	case pyBytes:
		return "bytes"
	case pyDate:
		return "datetime.date"
	case pyDateTime:
		return "datetime.datetime"
	case pyList:
		return "list"
	case pyTuple:
		return "tuple"
	case *pyDict:
		return "dict"
	case *pySet:
		return "set"
	}
	panic(fmt.Sprintf("validate: unexpected value %T", v))
}

// pyUnhashable returns the type name of the first unhashable object in v.
func pyUnhashable(v any) string {
	switch t := v.(type) {
	case pyList, *pyDict, *pySet:
		return pyTypeName(v)
	case pyTuple:
		for _, e := range t {
			if n := pyUnhashable(e); n != "" {
				return n
			}
		}
	}
	return ""
}

// pyNumber returns a numeric value as a big.Float for bool, int and float.
func pyNumber(v any) (*big.Float, bool) {
	switch t := v.(type) {
	case bool:
		if t {
			return big.NewFloat(1), true
		}
		return big.NewFloat(0), true
	case *big.Int:
		return new(big.Float).SetPrec(0).SetInt(t), true
	case float64:
		if math.IsNaN(t) {
			return nil, true
		}
		return big.NewFloat(t), true
	}
	return nil, false
}

// pyEq is Python == for the hashable values a YAML key or a set item can be.
func pyEq(a, b any) bool {
	if na, ok := pyNumber(a); ok {
		nb, ok := pyNumber(b)
		if !ok || na == nil || nb == nil {
			return false
		}
		if na.IsInf() || nb.IsInf() {
			fa, _ := na.Float64()
			fb, _ := nb.Float64()
			return fa == fb
		}
		return na.Cmp(nb) == 0
	}
	switch x := a.(type) {
	case nil:
		return b == nil
	case string:
		y, ok := b.(string)
		return ok && x == y
	case pyBytes:
		y, ok := b.(pyBytes)
		return ok && string(x) == string(y)
	case pyDate:
		y, ok := b.(pyDate)
		return ok && x == y
	case pyDateTime:
		y, ok := b.(pyDateTime)
		if !ok || x.aware != y.aware {
			return false
		}
		if !x.aware {
			return x == y
		}
		return x.utcMicros() == y.utcMicros()
	case pyTuple:
		y, ok := b.(pyTuple)
		if !ok || len(x) != len(y) {
			return false
		}
		for i := range x {
			if !pyEq(x[i], y[i]) {
				return false
			}
		}
		return true
	}
	return false
}

func (d pyDate) ordinal() int64 {
	t := int64(d.year-1)*365 + int64((d.year-1)/4-(d.year-1)/100+(d.year-1)/400)
	days := []int{0, 31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	for m := 1; m < d.month; m++ {
		t += int64(days[m])
		if m == 2 && isLeap(d.year) {
			t++
		}
	}
	return t + int64(d.day)
}

func (d pyDateTime) utcMicros() int64 {
	secs := d.ordinal()*86400 + int64(d.hour*3600+d.minute*60+d.second) - int64(d.offset)
	return secs*1_000_000 + int64(d.micro)
}

func (d pyDateTime) localMicros() int64 {
	secs := d.ordinal()*86400 + int64(d.hour*3600+d.minute*60+d.second)
	return secs*1_000_000 + int64(d.micro)
}

func isLeap(y int) bool { return y%4 == 0 && (y%100 != 0 || y%400 == 0) }

// pyLess is Python's `a < b` as sorted() uses it, raising TypeError where
// Python does.
func pyLess(a, b any) bool {
	notSupported := func() {
		panic(pyCrash("TypeError", fmt.Sprintf("'<' not supported between instances of '%s' and '%s'",
			pyShortTypeName(a), pyShortTypeName(b))))
	}
	if na, ok := pyNumber(a); ok {
		nb, ok := pyNumber(b)
		if !ok {
			notSupported()
		}
		if na == nil || nb == nil {
			return false
		}
		return na.Cmp(nb) < 0
	}
	switch x := a.(type) {
	case string:
		y, ok := b.(string)
		if !ok {
			notSupported()
		}
		return x < y
	case pyBytes:
		y, ok := b.(pyBytes)
		if !ok {
			notSupported()
		}
		return string(x) < string(y)
	case pyDateTime:
		y, ok := b.(pyDateTime)
		if !ok {
			if _, isDate := b.(pyDate); isDate {
				panic(pyCrash("TypeError", "can't compare datetime.datetime to datetime.date"))
			}
			notSupported()
		}
		if x.aware != y.aware {
			panic(pyCrash("TypeError", "can't compare offset-naive and offset-aware datetimes"))
		}
		if x.aware {
			return x.utcMicros() < y.utcMicros()
		}
		return x.localMicros() < y.localMicros()
	case pyDate:
		y, ok := b.(pyDate)
		if !ok {
			if _, isDT := b.(pyDateTime); isDT {
				panic(pyCrash("TypeError", "can't compare datetime.date to datetime.datetime"))
			}
			notSupported()
		}
		return x.ordinal() < y.ordinal()
	case pyTuple:
		y, ok := b.(pyTuple)
		if !ok {
			notSupported()
		}
		for i := range min(len(x), len(y)) {
			if !pyEq(x[i], y[i]) {
				return pyLess(x[i], y[i])
			}
		}
		return len(x) < len(y)
	}
	notSupported()
	return false
}

// pyShortTypeName is the type name CPython prints in comparison errors.
func pyShortTypeName(v any) string {
	switch v.(type) {
	case pyDate:
		return "datetime.date"
	case pyDateTime:
		return "datetime.datetime"
	}
	return pyTypeName(v)
}

// pySorted is sorted(items); a stable insertion sort is enough for the handful
// of keys a frontmatter carries.
func pySorted(items []any) []any {
	out := append([]any(nil), items...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && pyLess(out[j], out[j-1]); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// pyStr is str(v).
func pyStr(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case pyDate:
		return fmt.Sprintf("%04d-%02d-%02d", t.year, t.month, t.day)
	case pyDateTime:
		s := fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d", t.year, t.month, t.day, t.hour, t.minute, t.second)
		if t.micro != 0 {
			s += fmt.Sprintf(".%06d", t.micro)
		}
		if t.aware {
			off, sign := t.offset, "+"
			if off < 0 {
				off, sign = -off, "-"
			}
			s += fmt.Sprintf("%s%02d:%02d", sign, off/3600, off/60%60)
			if off%60 != 0 {
				s += fmt.Sprintf(":%02d", off%60)
			}
		}
		return s
	}
	return pyRepr(v)
}

// pyRepr is repr(v).
func pyRepr(v any) string {
	switch t := v.(type) {
	case nil:
		return "None"
	case bool:
		if t {
			return "True"
		}
		return "False"
	case *big.Int:
		return t.String()
	case float64:
		return pyFloatRepr(t)
	case string:
		return pyStrRepr(t)
	case pyBytes:
		return pyBytesRepr(t)
	case pyDate:
		return fmt.Sprintf("datetime.date(%d, %d, %d)", t.year, t.month, t.day)
	case pyDateTime:
		s := fmt.Sprintf("datetime.datetime(%d, %d, %d, %d, %d", t.year, t.month, t.day, t.hour, t.minute)
		switch {
		case t.micro != 0:
			s += fmt.Sprintf(", %d, %d", t.second, t.micro)
		case t.second != 0:
			s += fmt.Sprintf(", %d", t.second)
		}
		if t.aware {
			s += ", tzinfo=" + pyTimezoneRepr(t.offset)
		}
		return s + ")"
	case pyList:
		return "[" + pyJoinRepr(t) + "]"
	case pyTuple:
		if len(t) == 1 {
			return "(" + pyRepr(t[0]) + ",)"
		}
		return "(" + pyJoinRepr(t) + ")"
	case *pyDict:
		parts := make([]string, len(t.keys))
		for i := range t.keys {
			parts[i] = pyRepr(t.keys[i]) + ": " + pyRepr(t.vals[i])
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case *pySet:
		if len(t.items) == 0 {
			return "set()"
		}
		return "{" + pyJoinRepr(t.items) + "}"
	}
	panic(fmt.Sprintf("validate: unexpected value %T", v))
}

func pyJoinRepr(items []any) string {
	parts := make([]string, len(items))
	for i, it := range items {
		parts[i] = pyRepr(it)
	}
	return strings.Join(parts, ", ")
}

func pyTimezoneRepr(offset int) string {
	if offset == 0 {
		return "datetime.timezone.utc"
	}
	days, secs := offset/86400, offset%86400
	if secs < 0 {
		days, secs = days-1, secs+86400
	}
	var parts []string
	if days != 0 {
		parts = append(parts, fmt.Sprintf("days=%d", days))
	}
	if secs != 0 {
		parts = append(parts, fmt.Sprintf("seconds=%d", secs))
	}
	return "datetime.timezone(datetime.timedelta(" + strings.Join(parts, ", ") + "))"
}

// pyFloatRepr is repr(float): the shortest round-tripping digits, in fixed
// notation when the decimal exponent is in [-4, 16), else scientific.
func pyFloatRepr(f float64) string {
	switch {
	case math.IsNaN(f):
		return "nan"
	case math.IsInf(f, 1):
		return "inf"
	case math.IsInf(f, -1):
		return "-inf"
	}
	e := strconv.FormatFloat(f, 'e', -1, 64) // [-]d[.ddd]e±XX
	sign := ""
	if strings.HasPrefix(e, "-") {
		sign, e = "-", e[1:]
	}
	mant, exp, _ := strings.Cut(e, "e")
	digits := strings.Replace(mant, ".", "", 1)
	x, _ := strconv.Atoi(exp)
	decpt := x + 1
	if decpt <= -4 || decpt > 16 {
		m := digits[:1]
		if len(digits) > 1 {
			m += "." + digits[1:]
		}
		es := "+"
		if x < 0 {
			es, x = "-", -x
		}
		return fmt.Sprintf("%s%se%s%02d", sign, m, es, x)
	}
	switch {
	case decpt <= 0:
		return sign + "0." + strings.Repeat("0", -decpt) + digits
	case decpt >= len(digits):
		return sign + digits + strings.Repeat("0", decpt-len(digits)) + ".0"
	}
	return sign + digits[:decpt] + "." + digits[decpt:]
}

func pyQuote(hasSingle, hasDouble bool) byte {
	if hasSingle && !hasDouble {
		return '"'
	}
	return '\''
}

// pyStrRepr is repr(str).
func pyStrRepr(s string) string {
	q := pyQuote(strings.ContainsRune(s, '\''), strings.ContainsRune(s, '"'))
	var b strings.Builder
	b.WriteByte(q)
	for _, r := range s {
		switch {
		case r == rune(q) || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r < ' ' || r == 0x7f:
			fmt.Fprintf(&b, `\x%02x`, r)
		case r < 0x7f || unicode.IsPrint(r):
			b.WriteRune(r)
		case r <= 0xff:
			fmt.Fprintf(&b, `\x%02x`, r)
		case r <= 0xffff:
			fmt.Fprintf(&b, `\u%04x`, r)
		default:
			fmt.Fprintf(&b, `\U%08x`, r)
		}
	}
	b.WriteByte(q)
	return b.String()
}

// pyBytesRepr is repr(bytes).
func pyBytesRepr(s pyBytes) string {
	q := pyQuote(strings.ContainsRune(string(s), '\''), strings.ContainsRune(string(s), '"'))
	var b strings.Builder
	b.WriteString("b")
	b.WriteByte(q)
	for _, c := range s {
		switch {
		case c == q || c == '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		case c == '\t':
			b.WriteString(`\t`)
		case c == '\n':
			b.WriteString(`\n`)
		case c == '\r':
			b.WriteString(`\r`)
		case c < ' ' || c >= 0x7f:
			fmt.Fprintf(&b, `\x%02x`, c)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte(q)
	return b.String()
}
