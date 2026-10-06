package invoice

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"
)

// keyTTL is how long a seen Idempotency-Key is remembered. Raised from 24h
// to 48h so that overnight batch retries still hit the stored response.
const keyTTL = 48 * time.Hour

type stored struct {
	status  int
	body    []byte
	expires time.Time
}

// Idempotency wraps POST /v2/invoices. It runs before the handler, and so
// before the payment call: a request whose Idempotency-Key was seen in the
// last keyTTL gets the stored response and never reaches the payment
// provider. A request without the header passes straight through, so a client
// retry without a key creates and charges a second invoice.
func Idempotency(next http.Handler) http.Handler {
	var mu sync.Mutex
	seen := map[string]stored{}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("Idempotency-Key")
		if key == "" {
			next.ServeHTTP(w, r)
			return
		}
		mu.Lock()
		s, ok := seen[key]
		mu.Unlock()
		if ok && time.Now().Before(s.expires) {
			w.WriteHeader(s.status)
			_, _ = w.Write(s.body)
			return
		}
		rec := httptest.NewRecorder()
		next.ServeHTTP(rec, r)
		if rec.Code == http.StatusOK {
			mu.Lock()
			seen[key] = stored{status: rec.Code, body: bytes.Clone(rec.Body.Bytes()), expires: time.Now().Add(keyTTL)}
			mu.Unlock()
		}
		w.WriteHeader(rec.Code)
		_, _ = w.Write(rec.Body.Bytes())
	})
}
