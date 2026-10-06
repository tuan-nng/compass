package okfinstall

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

const assetPath = "/acme/okf/releases/download/v1/okf_1.0_linux_amd64.tar.gz"

// tarGz builds a tar.gz holding the given name -> content regular files.
func tarGz(t *testing.T, files ...[2]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	for _, f := range files {
		if err := tw.WriteHeader(&tar.Header{Name: f[0], Mode: 0o644, Size: int64(len(f[1])), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(f[1])); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sum(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// serve points releaseBase at a server that runs handler for the asset path.
func serve(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != assetPath {
			http.NotFound(w, r)
			return
		}
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	oldBase, oldDelay := releaseBase, retryDelay
	releaseBase, retryDelay = srv.URL, 0
	t.Cleanup(func() { releaseBase, retryDelay = oldBase, oldDelay })
}

func pins(sha string) string {
	return "OKF_RELEASE_REPO=acme/okf\nOKF_TAG=v1\nOKF_VERSION=1.0\nOKF_SHA256_linux_amd64=" + sha + "\n"
}

func noEnv(string) string { return "" }

func run(t *testing.T, dest, pinText string) (string, error) {
	t.Helper()
	return install(dest, "test.env", pinText, "linux", "amd64", noEnv, io.Discard)
}

func TestInstallsVerifiedBinary(t *testing.T) {
	archive := tarGz(t, [2]string{"README.md", "readme"}, [2]string{"okf", "old"}, [2]string{"okf", "#!/bin/sh\necho okf\n"})
	serve(t, func(w http.ResponseWriter, r *http.Request) { w.Write(archive) })
	dest := filepath.Join(t.TempDir(), "bin")
	msg, err := run(t, dest, pins(sum(archive)))
	if err != nil {
		t.Fatal(err)
	}
	if want := "okf install: installed okf 1.0 (acme/okf@v1) to " + dest + "/okf"; msg != want {
		t.Errorf("message %q, want %q", msg, want)
	}
	got, err := os.ReadFile(filepath.Join(dest, "okf"))
	if err != nil || string(got) != "#!/bin/sh\necho okf\n" {
		t.Errorf("installed %q, %v; want the last okf member", got, err)
	}
	st, _ := os.Stat(filepath.Join(dest, "okf"))
	if st.Mode().Perm() != 0o755 {
		t.Errorf("mode %v, want 0755", st.Mode().Perm())
	}
	if ents, _ := os.ReadDir(dest); len(ents) != 1 {
		t.Errorf("dest holds %d entries, want only okf", len(ents))
	}
}

func TestChecksumMismatchInstallsNothing(t *testing.T) {
	archive := tarGz(t, [2]string{"okf", "binary"})
	serve(t, func(w http.ResponseWriter, r *http.Request) { w.Write(archive) })
	dest := filepath.Join(t.TempDir(), "bin")
	pinned := strings.Repeat("0", 64)
	_, err := run(t, dest, pins(pinned))
	want := "checksum mismatch for okf_1.0_linux_amd64.tar.gz: got " + sum(archive) + ", pinned " + pinned + "; nothing installed"
	if err == nil || err.Error() != want {
		t.Fatalf("error %v, want %q", err, want)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Errorf("dest exists after a mismatch: %v", err)
	}
}

func TestArchiveWithoutOKF(t *testing.T) {
	for name, archive := range map[string][]byte{
		"no okf member":  tarGz(t, [2]string{"bin/okf", "x"}, [2]string{"./okf", "x"}),
		"not a gzip tar": []byte("not an archive"),
	} {
		t.Run(name, func(t *testing.T) {
			serve(t, func(w http.ResponseWriter, r *http.Request) { w.Write(archive) })
			dest := filepath.Join(t.TempDir(), "bin")
			_, err := run(t, dest, pins(sum(archive)))
			if err == nil || err.Error() != "archive has no okf binary: okf_1.0_linux_amd64.tar.gz" {
				t.Fatalf("error %v", err)
			}
			if _, err := os.Stat(dest); !os.IsNotExist(err) {
				t.Errorf("dest exists: %v", err)
			}
		})
	}
}

func TestDownloadRetries(t *testing.T) {
	archive := tarGz(t, [2]string{"okf", "binary"})
	for _, tc := range []struct {
		name      string
		status    int
		failures  int32
		wantCalls int32
		wantErr   bool
	}{
		{"transient 503 then success", 503, 2, 3, false},
		{"transient 503 gives up after 3 retries", 503, 10, 4, true},
		{"404 is not retried", 404, 10, 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			serve(t, func(w http.ResponseWriter, r *http.Request) {
				if calls.Add(1) <= tc.failures {
					w.WriteHeader(tc.status)
					return
				}
				w.Write(archive)
			})
			_, err := run(t, filepath.Join(t.TempDir(), "bin"), pins(sum(archive)))
			if (err != nil) != tc.wantErr {
				t.Fatalf("error %v, wantErr %v", err, tc.wantErr)
			}
			if err != nil && !strings.HasPrefix(err.Error(), "download failed: http://") {
				t.Errorf("error %q", err)
			}
			if calls.Load() != tc.wantCalls {
				t.Errorf("%d requests, want %d", calls.Load(), tc.wantCalls)
			}
		})
	}
}

func TestPinsAndPlatform(t *testing.T) {
	for _, tc := range []struct {
		name, pins, goos, goarch, want string
		getenv                         func(string) string
	}{
		{"missing tag", "OKF_RELEASE_REPO=a/okf\nOKF_VERSION=1\n", "linux", "amd64", "OKF_TAG: missing in test.env", noEnv},
		{"unsupported OS", pins("x"), "windows", "amd64", "unsupported OS: windows", noEnv},
		{"unsupported CPU", pins("x"), "linux", "386", "unsupported CPU: 386", noEnv},
		{"no checksum", pins("x"), "darwin", "arm64", "no pinned checksum for darwin/arm64 in test.env", noEnv},
		{"bad org", "OKF_TAG=v1\nOKF_VERSION=1\n", "linux", "amd64", `OKF_ORG is not a GitHub account name: "bad org"`,
			func(k string) string {
				if k == "OKF_ORG" {
					return "bad org"
				}
				return ""
			}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := install(t.TempDir(), "test.env", tc.pins, tc.goos, tc.goarch, tc.getenv, io.Discard)
			if err == nil || err.Error() != tc.want {
				t.Fatalf("error %v, want %q", err, tc.want)
			}
		})
	}
}
