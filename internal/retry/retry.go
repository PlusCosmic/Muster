// Package retry is an http.RoundTripper that waits instead of failing: a
// 429 sleeps for as long as the server asks, a 5xx or a connection error
// backs off and tries again, and a host that announces it is nearly out of
// rate limit (X-Ratelimit-Remaining / X-Ratelimit-Reset, as Modrinth's API
// sends) is paused until the limit resets, before the 429 ever happens.
//
// Only GET and HEAD are retried: they carry no body and are safe to repeat.
// A failure after the response headers have arrived (a download cut off
// halfway) is the caller's to retry; see packwiz.Client.download.
package retry

import (
	"context"
	"errors"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Transport retries requests. The zero value is usable.
type Transport struct {
	// Base does the actual round trips; nil means http.DefaultTransport.
	Base http.RoundTripper
	// MaxAttempts bounds tries per request (default 5).
	MaxAttempts int
	// MaxWait bounds the total time one request spends waiting between
	// attempts and on rate-limit pauses (default 90 s). A wait that would
	// exceed it is not taken: the last response or error is returned.
	MaxWait time.Duration
	// Reserve is how many requests to leave in a host's rate-limit window:
	// when X-Ratelimit-Remaining drops to it, the host is paused until
	// X-Ratelimit-Reset (default 3; negative disables).
	Reserve int
	// OnWait is told about every pause: the host, how long, and why
	// ("rate limit" or "retry"). Used to keep the UI honest.
	OnWait func(host string, d time.Duration, why string)
	// Sleep waits for d or until ctx ends; nil means the real thing.
	// Tests inject one that records instead.
	Sleep func(ctx context.Context, d time.Duration) error
	// Now is the clock; nil means time.Now.
	Now func() time.Time

	mu    sync.Mutex
	holds map[string]time.Time // host -> do not send before
}

const (
	defaultAttempts = 5
	defaultMaxWait  = 90 * time.Second
	defaultReserve  = 3
	backoffBase     = 500 * time.Millisecond
	backoffCap      = 15 * time.Second
)

func (t *Transport) base() http.RoundTripper {
	if t.Base != nil {
		return t.Base
	}
	return http.DefaultTransport
}

func (t *Transport) attempts() int {
	if t.MaxAttempts > 0 {
		return t.MaxAttempts
	}
	return defaultAttempts
}

func (t *Transport) maxWait() time.Duration {
	if t.MaxWait > 0 {
		return t.MaxWait
	}
	return defaultMaxWait
}

func (t *Transport) reserve() int {
	if t.Reserve != 0 {
		return t.Reserve
	}
	return defaultReserve
}

func (t *Transport) now() time.Time {
	if t.Now != nil {
		return t.Now()
	}
	return time.Now()
}

func (t *Transport) sleep(ctx context.Context, d time.Duration) error {
	if t.Sleep != nil {
		return t.Sleep(ctx, d)
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// holdFor is how long the host is paused for, or 0.
func (t *Transport) holdFor(host string) time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	until, ok := t.holds[host]
	if !ok {
		return 0
	}
	d := until.Sub(t.now())
	if d <= 0 {
		delete(t.holds, host)
		return 0
	}
	return d
}

func (t *Transport) hold(host string, d time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.holds == nil {
		t.holds = map[string]time.Time{}
	}
	until := t.now().Add(d)
	if until.After(t.holds[host]) {
		t.holds[host] = until
	}
}

// Retryable reports whether a status is worth another try.
func Retryable(status int) bool {
	switch status {
	case http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}

// retryAfter reads how long a 429/503 asks us to wait: Retry-After as
// seconds or an HTTP date, else X-Ratelimit-Reset as seconds. 0 = unsaid.
func retryAfter(h http.Header, now time.Time) time.Duration {
	if v := strings.TrimSpace(h.Get("Retry-After")); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil && n >= 0 {
			return time.Duration(n * float64(time.Second))
		}
		if at, err := http.ParseTime(v); err == nil {
			if d := at.Sub(now); d > 0 {
				return d
			}
		}
	}
	if v := strings.TrimSpace(h.Get("X-Ratelimit-Reset")); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil && n > 0 {
			return time.Duration(n * float64(time.Second))
		}
	}
	return 0
}

// backoff is the wait before attempt n (0-based): exponential with full
// jitter, capped.
func backoff(n int) time.Duration {
	d := backoffBase << uint(n)
	if d > backoffCap || d <= 0 {
		d = backoffCap
	}
	return time.Duration(rand.Int64N(int64(d)/2)) + d/2
}

// RoundTrip implements http.RoundTripper.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		return t.base().RoundTrip(req)
	}
	ctx := req.Context()
	host := req.URL.Host
	var waited time.Duration
	// wait pauses for d if the budget allows; false means give up waiting.
	wait := func(d time.Duration, why string) bool {
		if d <= 0 {
			return true
		}
		if waited+d > t.maxWait() {
			return false
		}
		if t.OnWait != nil {
			t.OnWait(host, d, why)
		}
		if err := t.sleep(ctx, d); err != nil {
			return false
		}
		waited += d
		return true
	}

	if d := t.holdFor(host); d > 0 {
		if !wait(d, "rate limit") && ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}

	var lastResp *http.Response
	var lastErr error
	for attempt := 0; attempt < t.attempts(); attempt++ {
		if attempt > 0 {
			d := backoff(attempt - 1)
			why := "retry"
			if lastResp != nil {
				if ra := retryAfter(lastResp.Header, t.now()); ra > 0 {
					d = ra
				}
				if lastResp.StatusCode == http.StatusTooManyRequests {
					why = "rate limit"
				}
			}
			if !wait(d, why) {
				break // lastResp goes back to the caller, body intact
			}
			if lastResp != nil {
				lastResp.Body.Close()
			}
			lastResp, lastErr = nil, nil
		}
		resp, err := t.base().RoundTrip(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, err
			}
			lastErr = err
			continue
		}
		t.noteLimit(host, resp.Header)
		if Retryable(resp.StatusCode) {
			lastResp = resp
			continue
		}
		return resp, nil
	}
	if lastResp != nil {
		return lastResp, nil
	}
	if lastErr == nil {
		lastErr = errors.New("retry: no attempts")
	}
	return nil, lastErr
}

// noteLimit pauses the host when its window is nearly spent.
func (t *Transport) noteLimit(host string, h http.Header) {
	if t.reserve() < 0 {
		return
	}
	rem := strings.TrimSpace(h.Get("X-Ratelimit-Remaining"))
	if rem == "" {
		return
	}
	n, err := strconv.Atoi(rem)
	if err != nil || n > t.reserve() {
		return
	}
	reset, err := strconv.ParseFloat(strings.TrimSpace(h.Get("X-Ratelimit-Reset")), 64)
	if err != nil || reset <= 0 {
		return
	}
	t.hold(host, time.Duration(reset*float64(time.Second)))
}
