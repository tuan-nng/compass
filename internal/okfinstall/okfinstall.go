// Package okfinstall installs the pinned okf release binary into a folder
// (plan decision 5: the pinned, patched fork). It refuses a checksum
// mismatch and then installs nothing.
package okfinstall

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"compass"
	"compass/internal/config"
)

const help = `Install the pinned okf binary into a folder. Refuses a checksum mismatch.

Usage: compass okf install --dest DIR [--pins FILE]
  --dest DIR    folder to install okf into (created if missing)
  --pins FILE   pin file to use instead of the embedded pins/okf.env (for tests)
`

// releaseBase is the host release assets are downloaded from; tests point it
// at a local server.
var releaseBase = "https://github.com"

// retryDelay is the wait before the first retry; it doubles per retry, as
// curl --retry does.
var retryDelay = time.Second

// failure is a fatal install error; its message follows the "okf install: "
// prefix.
type failure string

func (f failure) Error() string { return string(f) }

func fail(format string, a ...any) error { return failure(fmt.Sprintf(format, a...)) }

// Main runs `compass okf install`.
func Main(args []string, stdout, stderr io.Writer) int {
	pinsPath, dest := "", ""
	for len(args) > 0 {
		switch args[0] {
		case "--dest":
			if len(args) < 2 {
				return die(stderr, fail("--dest needs a folder"))
			}
			dest, args = args[1], args[2:]
		case "--pins":
			if len(args) < 2 {
				return die(stderr, fail("--pins needs a file"))
			}
			pinsPath, args = args[1], args[2:]
		case "-h", "--help":
			fmt.Fprint(stdout, help)
			return 0
		default:
			return die(stderr, fail("unknown argument: %s", args[0]))
		}
	}
	if dest == "" {
		return die(stderr, fail("--dest is required"))
	}
	pinsName, pins := "pins/okf.env", compass.PinsEnv
	if pinsPath != "" {
		raw, err := os.ReadFile(pinsPath)
		if err != nil {
			if st, serr := os.Stat(pinsPath); serr != nil || !st.Mode().IsRegular() {
				return die(stderr, fail("pin file not found: %s", pinsPath))
			}
			return die(stderr, fail("%v", err))
		}
		pinsName, pins = pinsPath, string(raw)
	}
	msg, err := install(dest, pinsName, pins, runtime.GOOS, runtime.GOARCH, os.Getenv, stderr)
	if err != nil {
		return die(stderr, err)
	}
	fmt.Fprintln(stdout, msg)
	return 0
}

func die(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "okf install: %v\n", err)
	return 1
}

// Fetch downloads the pinned okf for this machine from the embedded pins,
// checks the archive's sha256, and returns the okf binary and a label,
// "okf <version> (<repo>@<tag>)". Download details go to stderr.
func Fetch(getenv func(string) string, stderr io.Writer) ([]byte, string, error) {
	return fetch("pins/okf.env", compass.PinsEnv, runtime.GOOS, runtime.GOARCH, getenv, stderr)
}

// install downloads, verifies and installs okf for goos/goarch, and returns
// the success line.
func install(dest, pinsName, pins, goos, goarch string, getenv func(string) string, stderr io.Writer) (string, error) {
	bin, label, err := fetch(pinsName, pins, goos, goarch, getenv, stderr)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dest, 0o777); err != nil {
		return "", err
	}
	target := filepath.Join(dest, "okf")
	tmp := fmt.Sprintf("%s.tmp.%d", target, os.Getpid())
	if err := writeExecutable(tmp, bin); err != nil {
		os.Remove(tmp)
		return "", err
	}
	if err := os.Rename(tmp, target); err != nil {
		os.Remove(tmp)
		return "", err
	}
	return fmt.Sprintf("okf install: installed %s to %s/okf", label, dest), nil
}

// fetch is Fetch for a given pin text and platform. A pin missing from the
// pin text falls back to the environment, as sourcing the pin file into the
// shell did.
func fetch(pinsName, pins, goos, goarch string, getenv func(string) string, stderr io.Writer) ([]byte, string, error) {
	pin := func(key string) string {
		if v := config.EnvValue(pins, key); v != "" {
			return v
		}
		return getenv(key)
	}
	repo := pin("OKF_RELEASE_REPO")
	if repo == "" {
		return nil, "", fail("OKF_RELEASE_REPO: missing in %s", pinsName)
	}
	tag, version := pin("OKF_TAG"), pin("OKF_VERSION")
	if tag == "" {
		return nil, "", fail("OKF_TAG: missing in %s", pinsName)
	}
	if version == "" {
		return nil, "", fail("OKF_VERSION: missing in %s", pinsName)
	}

	var osName, arch string
	switch goos {
	case "linux", "darwin":
		osName = goos
	default:
		return nil, "", fail("unsupported OS: %s", goos)
	}
	switch goarch {
	case "amd64", "arm64":
		arch = goarch
	default:
		return nil, "", fail("unsupported CPU: %s", goarch)
	}
	want := pin("OKF_SHA256_" + osName + "_" + arch)
	if want == "" {
		return nil, "", fail("no pinned checksum for %s/%s in %s", osName, arch, pinsName)
	}

	asset := fmt.Sprintf("okf_%s_%s_%s.tar.gz", version, osName, arch)
	url := fmt.Sprintf("%s/%s/releases/download/%s/%s", releaseBase, repo, tag, asset)
	data, err := download(url)
	if err != nil {
		fmt.Fprintf(stderr, "okf install: %v\n", err)
		return nil, "", fail("download failed: %s", url)
	}

	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != want {
		return nil, "", fail("checksum mismatch for %s: got %s, pinned %s; nothing installed", asset, got, want)
	}

	bin, err := extractOKF(data)
	if err != nil {
		return nil, "", fail("archive has no okf binary: %s", asset)
	}
	return bin, fmt.Sprintf("okf %s (%s@%s)", version, repo, tag), nil
}

// download fetches url, failing on HTTP errors and retrying transient
// failures up to 3 times, as curl -fsSL --retry 3 does.
func download(url string) ([]byte, error) {
	delay := retryDelay
	var err error
	for attempt := 0; ; attempt++ {
		var data []byte
		var retry bool
		data, retry, err = get(url)
		if err == nil {
			return data, nil
		}
		if !retry || attempt == 3 {
			return nil, err
		}
		time.Sleep(delay)
		delay *= 2
	}
}

// get makes one download attempt and reports whether a failure is transient
// in curl's sense: a timeout, or HTTP 408, 429, 500, 502, 503 or 504.
func get(url string) ([]byte, bool, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, isTimeout(err), err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		err := fmt.Errorf("the requested URL returned error: %d", resp.StatusCode)
		switch resp.StatusCode {
		case 408, 429, 500, 502, 503, 504:
			return nil, true, err
		}
		return nil, false, err
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, isTimeout(err), err
	}
	return data, false, nil
}

func isTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

// extractOKF returns the regular file named exactly okf in a tar.gz archive;
// a later member of that name replaces an earlier one, as tar -x does.
func extractOKF(archive []byte) ([]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	tr := tar.NewReader(zr)
	var bin []byte
	found := false
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if h.Name != "okf" || h.Typeflag != tar.TypeReg {
			continue
		}
		if bin, err = io.ReadAll(tr); err != nil {
			return nil, err
		}
		found = true
	}
	if !found {
		return nil, errors.New("no okf member")
	}
	return bin, nil
}

// writeExecutable writes data to path with mode 0755 regardless of umask, as
// install -m 0755 does.
func writeExecutable(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Chmod(0o755); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
