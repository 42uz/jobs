package source

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

// Fetcher is a resilient HTTP client shared by all adapters. It provides:
//   - a bounded, retrying request pipeline (exponential backoff + jitter),
//   - per-host concurrency limiting and pacing so we never hammer a single ATS
//     host (many companies share e.g. boards-api.greenhouse.io),
//   - a response size cap to bound memory,
//   - context propagation so an interrupted crawl stops promptly.
//
// A single Fetcher is safe for concurrent use by many goroutines.
type Fetcher struct {
	client      *http.Client
	ua          string
	timeout     time.Duration // per-request deadline, applied via context
	maxAttempts int
	maxBytes    int64
	baseBackoff time.Duration
	maxBackoff  time.Duration
	gate        *hostGate
	logf        func(format string, args ...any)
}

// FetcherOptions configures a Fetcher. Zero values fall back to sane defaults.
type FetcherOptions struct {
	UserAgent     string
	Timeout       time.Duration // per-request timeout
	MaxAttempts   int
	MaxBytes      int64
	BaseBackoff   time.Duration
	MaxBackoff    time.Duration
	PerHostConc   int           // max concurrent requests to a single host
	PerHostMinGap time.Duration // minimum spacing between requests to a host
	Log           func(format string, args ...any)
}

// NewFetcher builds a Fetcher from the given options.
func NewFetcher(o FetcherOptions) *Fetcher {
	if o.UserAgent == "" {
		o.UserAgent = "FaangJobs/1.0 (+https://faangjobs; job aggregator)"
	}
	if o.Timeout <= 0 {
		o.Timeout = 25 * time.Second
	}
	if o.MaxAttempts <= 0 {
		o.MaxAttempts = 4
	}
	if o.MaxBytes <= 0 {
		o.MaxBytes = 96 << 20 // 96 MiB
	}
	if o.BaseBackoff <= 0 {
		o.BaseBackoff = 600 * time.Millisecond
	}
	if o.MaxBackoff <= 0 {
		o.MaxBackoff = 20 * time.Second
	}
	if o.PerHostConc <= 0 {
		o.PerHostConc = 4
	}
	if o.PerHostMinGap < 0 {
		o.PerHostMinGap = 0
	}
	if o.Log == nil {
		o.Log = func(string, ...any) {}
	}
	transport := &http.Transport{
		MaxIdleConns:        200,
		MaxIdleConnsPerHost: 8,
		IdleConnTimeout:     90 * time.Second,
		ForceAttemptHTTP2:   true,
	}
	return &Fetcher{
		// No client-level Timeout: the deadline is applied per request (see
		// attempt) so a single slow source can be given more time without
		// slowing every other request down.
		client:      &http.Client{Transport: transport},
		ua:          o.UserAgent,
		timeout:     o.Timeout,
		maxAttempts: o.MaxAttempts,
		maxBytes:    o.MaxBytes,
		baseBackoff: o.BaseBackoff,
		maxBackoff:  o.MaxBackoff,
		gate:        newHostGate(o.PerHostConc, o.PerHostMinGap),
		logf:        o.Log,
	}
}

// GetJSON performs a GET and decodes the JSON body into out.
func (f *Fetcher) GetJSON(ctx context.Context, rawURL string, headers map[string]string, out any) error {
	body, err := f.Do(ctx, http.MethodGet, rawURL, nil, headers)
	if err != nil {
		return err
	}
	return decodeJSON(body, out)
}

// PostJSON performs a POST with a JSON body and decodes the JSON response.
func (f *Fetcher) PostJSON(ctx context.Context, rawURL string, reqBody any, headers map[string]string, out any) error {
	var payload []byte
	switch b := reqBody.(type) {
	case nil:
		payload = nil
	case []byte:
		payload = b
	case string:
		payload = []byte(b)
	default:
		var err error
		payload, err = json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
	}
	if headers == nil {
		headers = map[string]string{}
	}
	if _, ok := headers["Content-Type"]; !ok {
		headers["Content-Type"] = "application/json"
	}
	body, err := f.Do(ctx, http.MethodPost, rawURL, payload, headers)
	if err != nil {
		return err
	}
	return decodeJSON(body, out)
}

// retryableError marks transport-level errors that are worth retrying.
type httpStatusError struct {
	status int
	url    string
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("unexpected status %d for %s", e.status, e.url)
}

// Do executes an HTTP request with retries, per-host pacing, and a size cap. It
// returns the (fully read) response body on a 2xx status.
func (f *Fetcher) Do(ctx context.Context, method, rawURL string, body []byte, headers map[string]string) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid url %q: %w", rawURL, err)
	}
	host := u.Host

	var lastErr error
	attemptsMade := 0
	for attempt := 1; attempt <= f.maxAttempts; attempt++ {
		attemptsMade = attempt
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		// Pace requests to this host.
		release, err := f.gate.acquire(ctx, host)
		if err != nil {
			return nil, err
		}
		respBody, retryAfter, err := f.attempt(ctx, method, rawURL, body, headers)
		release()

		if err == nil {
			return respBody, nil
		}
		lastErr = err
		if !isRetryable(err) || attempt == f.maxAttempts {
			break
		}
		wait := f.backoff(attempt, retryAfter)
		f.logf("retry %d/%d for %s in %s: %v", attempt, f.maxAttempts, rawURL, wait.Round(time.Millisecond), err)
		if err := sleep(ctx, wait); err != nil {
			return nil, err
		}
	}
	return nil, fmt.Errorf("after %d attempt(s): %w", attemptsMade, lastErr)
}

// WithTimeout returns a view of the Fetcher that uses a different per-request
// deadline. The underlying client, connection pool and per-host gate are
// shared, so per-company overrides stay polite to the host.
func (f *Fetcher) WithTimeout(d time.Duration) *Fetcher {
	if d <= 0 || d == f.timeout {
		return f
	}
	clone := *f
	clone.timeout = d
	return &clone
}

// attempt performs a single HTTP round-trip. It returns the body on success, or
// an error plus an optional Retry-After hint.
func (f *Fetcher) attempt(ctx context.Context, method, rawURL string, body []byte, headers map[string]string) ([]byte, time.Duration, error) {
	if f.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, f.timeout)
		defer cancel()
	}
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, reader)
	if err != nil {
		return nil, 0, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", f.ua)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		// Network/timeout errors are retryable.
		return nil, 0, &retryable{err}
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, f.maxBytes+1)
	data, readErr := io.ReadAll(limited)
	if readErr != nil {
		return nil, 0, &retryable{fmt.Errorf("read body: %w", readErr)}
	}
	if int64(len(data)) > f.maxBytes {
		return nil, 0, fmt.Errorf("response from %s exceeds %d byte cap", rawURL, f.maxBytes)
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return data, 0, nil
	}

	retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
	statusErr := &httpStatusError{status: resp.StatusCode, url: rawURL}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, retryAfter, &retryable{statusErr}
	}
	return nil, 0, statusErr // 4xx (except 429): permanent, do not retry
}

func (f *Fetcher) backoff(attempt int, retryAfter time.Duration) time.Duration {
	if retryAfter > 0 {
		if retryAfter > f.maxBackoff {
			return f.maxBackoff
		}
		return retryAfter
	}
	// Exponential backoff with full jitter. Clamp the shift so a large attempt
	// count can never overflow the exponent into a negative value.
	shift := attempt - 1
	if shift > 30 {
		shift = 30
	}
	exp := float64(f.baseBackoff) * float64(int64(1)<<uint(shift))
	if exp > float64(f.maxBackoff) {
		exp = float64(f.maxBackoff)
	}
	return time.Duration(rand.Int63n(int64(exp) + 1))
}

// --- error classification ---

type retryable struct{ err error }

func (r *retryable) Error() string { return r.err.Error() }
func (r *retryable) Unwrap() error { return r.err }

func isRetryable(err error) bool {
	var r *retryable
	return errors.As(err, &r)
}

func parseRetryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

func decodeJSON(data []byte, out any) error {
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		snippet := data
		if len(snippet) > 180 {
			snippet = snippet[:180]
		}
		return fmt.Errorf("decode json: %w (body starts: %q)", err, snippet)
	}
	return nil
}

func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// --- per-host gate ---

type hostGate struct {
	conc   int
	minGap time.Duration
	mu     sync.Mutex
	hosts  map[string]*hostState
}

type hostState struct {
	sem  chan struct{}
	mu   sync.Mutex
	next time.Time
}

func newHostGate(conc int, minGap time.Duration) *hostGate {
	return &hostGate{conc: conc, minGap: minGap, hosts: map[string]*hostState{}}
}

func (g *hostGate) state(host string) *hostState {
	g.mu.Lock()
	defer g.mu.Unlock()
	hs := g.hosts[host]
	if hs == nil {
		hs = &hostState{sem: make(chan struct{}, g.conc)}
		g.hosts[host] = hs
	}
	return hs
}

// acquire blocks until a slot for host is available and the minimum gap since
// the previous request has elapsed. The returned function releases the slot.
func (g *hostGate) acquire(ctx context.Context, host string) (func(), error) {
	hs := g.state(host)
	select {
	case hs.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if g.minGap > 0 {
		hs.mu.Lock()
		now := time.Now()
		wait := time.Until(hs.next)
		if hs.next.Before(now) {
			hs.next = now.Add(g.minGap)
			wait = 0
		} else {
			hs.next = hs.next.Add(g.minGap)
		}
		hs.mu.Unlock()
		if wait > 0 {
			if err := sleep(ctx, wait); err != nil {
				<-hs.sem
				return nil, err
			}
		}
	}
	return func() { <-hs.sem }, nil
}
