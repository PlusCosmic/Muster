package retry

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type recorder struct {
	waits []time.Duration
	whys  []string
}

func newTransport(rec *recorder) *Transport {
	return &Transport{
		Sleep: func(ctx context.Context, d time.Duration) error {
			rec.waits = append(rec.waits, d)
			return ctx.Err()
		},
		OnWait: func(_ string, _ time.Duration, why string) { rec.whys = append(rec.whys, why) },
	}
}

func get(t *testing.T, tr *Transport, url string) (*http.Response, error) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	resp, err := tr.RoundTrip(req)
	if err == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
	return resp, err
}

func TestWaitsOutA429ThenSucceeds(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			w.Header().Set("Retry-After", "7")
			w.WriteHeader(429)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()
	rec := &recorder{}
	resp, err := get(t, newTransport(rec), srv.URL)
	if err != nil || resp.StatusCode != 200 || hits.Load() != 2 {
		t.Fatalf("%v %v hits=%d", resp, err, hits.Load())
	}
	if len(rec.waits) != 1 || rec.waits[0] != 7*time.Second || rec.whys[0] != "rate limit" {
		t.Fatalf("%v %v", rec.waits, rec.whys)
	}
}

// flakyBase fails the first calls with a connection error, then delegates.
type flakyBase struct {
	fail  int
	calls int
	base  http.RoundTripper
}

func (f *flakyBase) RoundTrip(r *http.Request) (*http.Response, error) {
	f.calls++
	if f.calls <= f.fail {
		return nil, errors.New("connection reset by peer")
	}
	return f.base.RoundTrip(r)
}

func TestBacksOffOn5xxAndConnectionErrors(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			w.WriteHeader(503)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()
	rec := &recorder{}
	tr := newTransport(rec)
	// One connection error before the server is even reached, then a 503,
	// then success: three attempts, two waits, each longer than the last.
	tr.Base = &flakyBase{fail: 1, base: http.DefaultTransport}
	resp, err := get(t, tr, srv.URL)
	if err != nil || resp.StatusCode != 200 || hits.Load() != 2 {
		t.Fatalf("%v %v hits=%d", resp, err, hits.Load())
	}
	if len(rec.waits) != 2 || rec.waits[0] > rec.waits[1]*2 || rec.whys[0] != "retry" || rec.whys[1] != "retry" {
		t.Fatalf("%v %v", rec.waits, rec.whys)
	}
	if rec.waits[0] < backoffBase/2 || rec.waits[1] > 2*backoffBase {
		t.Fatalf("backoff out of range: %v", rec.waits)
	}
}

func TestGivesUpAfterMaxAttemptsAndReturnsLastResponse(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(502)
	}))
	defer srv.Close()
	tr := newTransport(&recorder{})
	tr.MaxAttempts = 3
	resp, err := get(t, tr, srv.URL)
	if err != nil || resp.StatusCode != 502 || hits.Load() != 3 {
		t.Fatalf("%v %v hits=%d", resp, err, hits.Load())
	}
}

func TestWaitBudgetIsNotExceeded(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Retry-After", "600")
		w.WriteHeader(429)
	}))
	defer srv.Close()
	rec := &recorder{}
	resp, err := get(t, newTransport(rec), srv.URL)
	if err != nil || resp.StatusCode != 429 || hits.Load() != 1 || len(rec.waits) != 0 {
		t.Fatalf("a 10-minute wait must not be taken: %v %v hits=%d waits=%v", resp, err, hits.Load(), rec.waits)
	}
}

func TestPausesHostWhenRateLimitNearlySpent(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("X-Ratelimit-Limit", "300")
		w.Header().Set("X-Ratelimit-Remaining", "2")
		w.Header().Set("X-Ratelimit-Reset", "30")
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()
	rec := &recorder{}
	now := time.Now()
	tr := newTransport(rec)
	tr.Now = func() time.Time { return now }
	if _, err := get(t, tr, srv.URL); err != nil || len(rec.waits) != 0 {
		t.Fatalf("first request should go straight through: %v %v", err, rec.waits)
	}
	// Second request: paused until the reset, then sent.
	if _, err := get(t, tr, srv.URL); err != nil || len(rec.waits) != 1 || rec.waits[0] != 30*time.Second || rec.whys[0] != "rate limit" {
		t.Fatalf("%v %v %v", err, rec.waits, rec.whys)
	}
	// Once the clock passes the reset the hold is gone: no new wait.
	now = now.Add(31 * time.Second)
	if _, err := get(t, tr, srv.URL); err != nil || len(rec.waits) != 1 {
		t.Fatalf("%v %v", err, rec.waits)
	}
	if hits.Load() != 3 {
		t.Fatalf("hits %d", hits.Load())
	}
}

func TestPostIsNeverRetried(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(503)
	}))
	defer srv.Close()
	req, _ := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader("x"))
	resp, err := newTransport(&recorder{}).RoundTrip(req)
	if err != nil || resp.StatusCode != 503 || hits.Load() != 1 {
		t.Fatalf("%v %v hits=%d", resp, err, hits.Load())
	}
	resp.Body.Close()
}

func TestCancelledContextStopsWaiting(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "5")
		w.WriteHeader(429)
	}))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	tr := &Transport{Sleep: func(ctx context.Context, d time.Duration) error {
		cancel()
		return ctx.Err()
	}}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	resp, err := tr.RoundTrip(req)
	// Either the 429 comes back (waiting was abandoned) or a context error;
	// what must not happen is a second attempt.
	if err == nil {
		if resp.StatusCode != 429 {
			t.Fatalf("%v", resp)
		}
		resp.Body.Close()
	} else if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestRetryAfterParsing(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	h := http.Header{}
	h.Set("Retry-After", "2.5")
	if d := retryAfter(h, now); d != 2500*time.Millisecond {
		t.Fatal(d)
	}
	h.Set("Retry-After", now.Add(90*time.Second).UTC().Format(http.TimeFormat))
	if d := retryAfter(h, now); d != 90*time.Second {
		t.Fatal(d)
	}
	h.Del("Retry-After")
	h.Set("X-Ratelimit-Reset", "12")
	if d := retryAfter(h, now); d != 12*time.Second {
		t.Fatal(d)
	}
	if d := retryAfter(http.Header{}, now); d != 0 {
		t.Fatal(d)
	}
}
