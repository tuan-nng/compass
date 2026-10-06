package validate

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"
)

// The reference validator is Python. These helpers reproduce the Python
// string, regex and filesystem semantics its findings depend on.

// pySpace lists the characters Python's str.isspace() and the `\s` of a str
// regex accept (Go's \s is ASCII-only and has no \v).
const pySpace = "\t\n\x0b\f\r\x1c-\x1f \u0085\u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000"

// pyRegexp compiles a Python `re` pattern after translating the pieces whose
// meaning differs in RE2: `\d` (Unicode digits in Python), `\s`/`\S`
// (Unicode whitespace) and a trailing `$` (which in Python also matches just
// before a final newline). Only the constructs used in this package are
// handled; `\s` inside a character class must be written as `{S}`.
func pyRegexp(pattern string) *regexp.Regexp {
	r := strings.NewReplacer(
		`\d`, `\p{Nd}`,
		`{S}`, pySpace,
		`\s`, "["+pySpace+"]",
		`\S`, "[^"+pySpace+"]",
	).Replace(pattern)
	if strings.HasSuffix(r, "$") && !strings.HasSuffix(r, `\$`) {
		r = strings.TrimSuffix(r, "$") + `\n?\z`
	}
	return regexp.MustCompile(r)
}

func pyIsSpace(r rune) bool {
	return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f)
}

// pyStrip is Python's str.strip() with no argument.
func pyStrip(s string) string {
	return strings.TrimFunc(s, pyIsSpace)
}

func isPyLineBreak(r rune) bool {
	switch r {
	case '\n', '\r', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
		return true
	}
	return false
}

// pySplitLines is Python's str.splitlines(keepends).
func pySplitLines(s string, keepends bool) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if !isPyLineBreak(r) {
			i += size
			continue
		}
		end := i
		i += size
		if r == '\r' && i < len(s) && s[i] == '\n' {
			i++
		}
		if keepends {
			end = i
		}
		lines = append(lines, s[start:end])
		start = i
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// pyReadText is Path.read_text(encoding="utf-8"): strict UTF-8 decoding with
// universal newlines. The error text is the str() of the Python exception.
func pyReadText(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", errors.New(pyOSError(err, path))
	}
	if msg := pyUTF8Error(data); msg != "" {
		return "", errors.New(msg)
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	return strings.ReplaceAll(text, "\r", "\n"), nil
}

// pyOSError renders an OS error the way str(OSError) does:
// "[Errno 13] Permission denied: 'path'".
func pyOSError(err error, path string) string {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return err.Error()
	}
	text := errno.Error()
	if text != "" {
		text = strings.ToUpper(text[:1]) + text[1:]
	}
	return fmt.Sprintf("[Errno %d] %s: %s", int(errno), text, pyStrRepr(path))
}

// pyUTF8Error returns the str() of the UnicodeDecodeError CPython's UTF-8
// decoder raises for data, or "" when data is valid UTF-8.
func pyUTF8Error(b []byte) string {
	cont := func(c byte) bool { return c >= 0x80 && c < 0xC0 }
	fail := func(start, end int, reason string) string {
		if end-start == 1 {
			return fmt.Sprintf("'utf-8' codec can't decode byte 0x%02x in position %d: %s", b[start], start, reason)
		}
		return fmt.Sprintf("'utf-8' codec can't decode bytes in position %d-%d: %s", start, end-1, reason)
	}
	const (
		badStart = "invalid start byte"
		badCont  = "invalid continuation byte"
		eod      = "unexpected end of data"
	)
	for i := 0; i < len(b); {
		c := b[i]
		n := len(b) - i
		switch {
		case c < 0x80:
			i++
		case c < 0xC2 || c > 0xF4:
			return fail(i, i+1, badStart)
		case c < 0xE0:
			if n < 2 {
				return fail(i, len(b), eod)
			}
			if !cont(b[i+1]) {
				return fail(i, i+1, badCont)
			}
			i += 2
		case c < 0xF0:
			bad2 := func(c2 byte) bool {
				if !cont(c2) {
					return true
				}
				if c2 < 0xA0 {
					return c == 0xE0
				}
				return c == 0xED
			}
			if n < 3 {
				if n >= 2 && bad2(b[i+1]) {
					return fail(i, i+1, badCont)
				}
				return fail(i, len(b), eod)
			}
			if bad2(b[i+1]) {
				return fail(i, i+1, badCont)
			}
			if !cont(b[i+2]) {
				return fail(i, i+2, badCont)
			}
			i += 3
		default:
			bad2 := func(c2 byte) bool {
				if !cont(c2) {
					return true
				}
				if c2 < 0x90 {
					return c == 0xF0
				}
				return c == 0xF4
			}
			if n < 4 {
				if n >= 2 && bad2(b[i+1]) {
					return fail(i, i+1, badCont)
				}
				if n >= 3 && !cont(b[i+2]) {
					return fail(i, i+2, badCont)
				}
				return fail(i, len(b), eod)
			}
			if bad2(b[i+1]) {
				return fail(i, i+1, badCont)
			}
			if !cont(b[i+2]) {
				return fail(i, i+2, badCont)
			}
			if !cont(b[i+3]) {
				return fail(i, i+3, badCont)
			}
			i += 4
		}
	}
	return ""
}

// pyPath is str(pathlib.PurePosixPath(s)): repeated and trailing slashes and
// "." components are dropped, ".." is kept, "" becomes ".".
func pyPath(s string) string {
	root := ""
	switch {
	case strings.HasPrefix(s, "//") && !strings.HasPrefix(s, "///"):
		root = "//"
	case strings.HasPrefix(s, "/"):
		root = "/"
	}
	var parts []string
	for _, p := range strings.Split(s, "/") {
		if p != "" && p != "." {
			parts = append(parts, p)
		}
	}
	out := root + strings.Join(parts, "/")
	if out == "" {
		return "."
	}
	return out
}

// pyJoin is str(Path(base) / rel) for a normalized base and a relative rel.
func pyJoin(base, rel string) string {
	switch {
	case rel == "":
		return base
	case base == ".":
		return rel
	case strings.HasSuffix(base, "/"):
		return base + rel
	}
	return base + "/" + rel
}

// pyResolve is Path.resolve() (strict=False) on CPython 3.12: posixpath.realpath
// followed by the symlink-loop check that turns ELOOP into RuntimeError.
func pyResolve(path string) string {
	if strings.ContainsRune(path, 0) {
		panic(pyCrash("ValueError", "embedded null byte"))
	}
	resolved, _ := joinRealpath("", path, map[string]*string{})
	abs := pyAbspath(resolved)
	if _, err := os.Stat(abs); errors.Is(err, syscall.ELOOP) {
		panic(pyCrash("RuntimeError", "Symlink loop from "+pyStrRepr(abs)))
	}
	return abs
}

func joinRealpath(path, rest string, seen map[string]*string) (string, bool) {
	if strings.HasPrefix(rest, "/") {
		rest = rest[1:]
		path = "/"
	}
	for rest != "" {
		var name string
		name, rest, _ = strings.Cut(rest, "/")
		if name == "" || name == "." {
			continue
		}
		if name == ".." {
			if path != "" {
				var base string
				path, base = pySplit(path)
				if base == ".." {
					path = pyPosixJoin(path, "..", "..")
				}
			} else {
				path = ".."
			}
			continue
		}
		newpath := pyPosixJoin(path, name)
		st, err := os.Lstat(newpath)
		if err != nil || st.Mode()&fs.ModeSymlink == 0 {
			path = newpath
			continue
		}
		if p, ok := seen[newpath]; ok {
			if p != nil {
				path = *p
				continue
			}
			return pyPosixJoin(newpath, rest), false
		}
		seen[newpath] = nil
		target, err := os.Readlink(newpath)
		if err != nil {
			path = newpath
			continue
		}
		var ok bool
		path, ok = joinRealpath(path, target, seen)
		if !ok {
			return pyPosixJoin(path, rest), false
		}
		resolved := path
		seen[newpath] = &resolved
	}
	return path, true
}

// pySplit is posixpath.split.
func pySplit(p string) (string, string) {
	i := strings.LastIndex(p, "/") + 1
	head, tail := p[:i], p[i:]
	if head != "" && strings.Trim(head, "/") != "" {
		head = strings.TrimRight(head, "/")
	}
	return head, tail
}

// pyPosixJoin is posixpath.join.
func pyPosixJoin(a string, ps ...string) string {
	path := a
	for _, b := range ps {
		switch {
		case strings.HasPrefix(b, "/"):
			path = b
		case path == "" || strings.HasSuffix(path, "/"):
			path += b
		default:
			path += "/" + b
		}
	}
	return path
}

// pyAbspath is posixpath.abspath: join with the real cwd, then normpath.
func pyAbspath(p string) string {
	if !strings.HasPrefix(p, "/") {
		cwd, err := syscall.Getwd()
		if err != nil {
			panic(pyCrash("FileNotFoundError", pyOSError(err, ".")))
		}
		p = pyPosixJoin(cwd, p)
	}
	return pyNormpath(p)
}

// pyNormpath is posixpath.normpath for an absolute path.
func pyNormpath(p string) string {
	initial := 1
	if strings.HasPrefix(p, "//") && !strings.HasPrefix(p, "///") {
		initial = 2
	}
	var parts []string
	for _, c := range strings.Split(p, "/") {
		switch {
		case c == "" || c == ".":
		case c == "..":
			if len(parts) > 0 {
				parts = parts[:len(parts)-1]
			}
		default:
			parts = append(parts, c)
		}
	}
	return strings.Repeat("/", initial) + strings.Join(parts, "/")
}

// pyRelativeTo reports whether the resolved absolute path p is base or below
// it (PurePath.is_relative_to), and returns relative_to(base).as_posix().
func pyRelativeTo(p, base string) (string, bool) {
	if p == base {
		return ".", true
	}
	prefix := base
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	if strings.HasPrefix(p, prefix) {
		return p[len(prefix):], true
	}
	return "", false
}
