package notify

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// defaultHTTPClient is shared by the channels so connection pooling kicks in
// across repeated sends. Timeout is per-request and bounded — the manager's
// own 30s timeout via context wraps the entire attempt sequence.
var defaultHTTPClient = &http.Client{
	Timeout: 15 * time.Second,
}

// HTTPClient is the minimal surface the channels need. Exposed so tests can
// swap it via Channel constructors that accept an override (factories use
// defaultHTTPClient).
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// retryableHTTPPost issues a POST with up to maxAttempts attempts, applying
// exponential backoff (1s / 2s / 4s) on HTTP 429 / 5xx. The body is buffered
// in memory so the retry can re-serialise it; only call this for small JSON
// payloads (the channels here send < 2 KB).
//
// Returns the final response (already drained) or the last error. The
// returned response Body is closed before returning so callers must NOT
// read it again — only the status code / header is preserved on the returned
// pointer.
func retryableHTTPPost(ctx context.Context, client HTTPClient, url string, headers map[string]string, body []byte, maxAttempts int) (int, []byte, error) {
	if client == nil {
		client = defaultHTTPClient
	}
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return 0, nil, fmt.Errorf("build request: %w", err)
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			if !shouldRetry(ctx, attempt, maxAttempts, 0, "") {
				return 0, nil, fmt.Errorf("http post: %w", err)
			}
			sleepBackoff(ctx, attempt, 0)
			continue
		}
		respBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp.StatusCode, respBody, nil
		}
		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
		if !shouldRetry(ctx, attempt, maxAttempts, resp.StatusCode, "") {
			return resp.StatusCode, respBody, fmt.Errorf("http %d: %s", resp.StatusCode, truncate(string(respBody), 200))
		}
		lastErr = fmt.Errorf("http %d: %s", resp.StatusCode, truncate(string(respBody), 200))
		sleepBackoff(ctx, attempt, retryAfter)
	}
	if lastErr == nil {
		lastErr = errors.New("http post: exhausted retries")
	}
	return 0, nil, lastErr
}

// shouldRetry decides whether attempt + status is retryable. 429 / 5xx are
// retryable; everything else is terminal. Network errors (status == 0) are
// retryable up to maxAttempts.
func shouldRetry(ctx context.Context, attempt, maxAttempts, status int, _ string) bool {
	if ctx.Err() != nil {
		return false
	}
	if attempt >= maxAttempts {
		return false
	}
	if status == 0 {
		return true // network error
	}
	if status == http.StatusTooManyRequests {
		return true
	}
	return status >= 500 && status < 600
}

// sleepBackoff blocks for the longer of (exponential schedule, Retry-After).
// Honours ctx cancellation.
func sleepBackoff(ctx context.Context, attempt int, retryAfter time.Duration) {
	delay := time.Duration(1<<uint(attempt-1)) * time.Second
	if retryAfter > delay {
		delay = retryAfter
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

// parseRetryAfter accepts the integer-seconds form. The HTTP-date form is
// rare for our channels and falls back to 0.
func parseRetryAfter(header string) time.Duration {
	if header == "" {
		return 0
	}
	if secs, err := strconv.Atoi(header); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	return 0
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
