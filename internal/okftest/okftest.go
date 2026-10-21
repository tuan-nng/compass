// Package okftest finds the pinned okf for Go tests that run it.
package okftest

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"compass"
	"compass/internal/config"
)

// Binary returns $OKF, else the pinned okf that `make test` installs into
// .cache/pinned/<tag>/okf, and sets $OKF to it for the test. It fails the
// test, naming `make test`, when neither exists; it never skips.
func Binary(t testing.TB) string {
	t.Helper()
	if p := os.Getenv("OKF"); p != "" {
		return p
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("okftest: cannot locate the compass root")
	}
	root := filepath.Join(filepath.Dir(file), "..", "..")
	tag := config.EnvValue(compass.PinsEnv, "OKF_TAG")
	p := filepath.Join(root, ".cache", "pinned", tag, "okf")
	if st, err := os.Stat(p); err != nil || st.Mode()&0o111 == 0 {
		t.Fatalf("okftest: no okf: $OKF is empty and %s is missing; run `make test` once to install it", p)
	}
	t.Setenv("OKF", p)
	return p
}
