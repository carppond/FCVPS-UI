package ota

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// DownloaderConfig wires the Downloader dependencies. All fields are optional.
type DownloaderConfig struct {
	// HTTPClient streams the binary asset. Defaults to a 5 minute timeout
	// http.Client when nil (large binaries on slow links need the headroom).
	HTTPClient *http.Client
}

// Downloader streams GitHub Release assets into a destination writer while
// emitting progress callbacks. The producer is goroutine-safe; concurrent
// Download calls use independent buffers.
type Downloader struct {
	httpClient *http.Client
}

// NewDownloader builds a Downloader with sensible defaults.
func NewDownloader(cfg DownloaderConfig) *Downloader {
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Minute}
	}
	return &Downloader{httpClient: client}
}

// ProgressFunc is invoked at most ~1× per 64KiB chunk with (bytes-so-far,
// total-bytes-if-known). `total` is -1 when the server returns no
// Content-Length header; the UI then renders an indeterminate bar.
type ProgressFunc func(downloaded, total int64)

// Download streams assetURL into w and emits progress every chunk. The caller
// owns w (typically an *os.File pointing at <bin>.new); the file is NOT closed
// here so a sha256 verification can immediately re-open it via a path-based
// helper without racing against Linux's "open with write fd" semantics.
//
// Returns the total bytes written and any transport / write error. A context
// cancellation is propagated through both the HTTP request and the io.Copy
// loop so an interrupted upgrade promptly aborts.
func (d *Downloader) Download(ctx context.Context, assetURL string, w io.Writer, progress ProgressFunc) (int64, error) {
	if d == nil {
		return 0, fmt.Errorf("ota: nil downloader")
	}
	if assetURL == "" {
		return 0, fmt.Errorf("ota: download: empty asset url")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, assetURL, nil)
	if err != nil {
		return 0, fmt.Errorf("ota: build download request: %w", err)
	}
	// GitHub's release downloads are public; explicit Accept anchors content
	// negotiation against a future "rich preview" content type rollout.
	req.Header.Set("Accept", "application/octet-stream")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("ota: download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return 0, fmt.Errorf("ota: download status %d", resp.StatusCode)
	}
	total := resp.ContentLength
	if total <= 0 {
		total = -1
	}
	pw := &progressWriter{w: w, total: total, cb: progress}
	n, err := io.Copy(pw, resp.Body)
	if err != nil {
		return n, fmt.Errorf("ota: copy: %w", err)
	}
	// Emit a final 100% tick — io.Copy may have skipped the last partial
	// chunk if it happened to align with the flush boundary.
	if progress != nil {
		progress(n, total)
	}
	return n, nil
}

// progressWriter is an io.Writer middleware that forwards bytes to w and
// fires the progress callback every 64 KiB so the UI updates smoothly without
// drowning the callback in calls.
type progressWriter struct {
	w          io.Writer
	total      int64
	written    int64
	lastTickAt int64
	cb         ProgressFunc
}

const progressTickBytes int64 = 64 * 1024

func (p *progressWriter) Write(b []byte) (int, error) {
	n, err := p.w.Write(b)
	p.written += int64(n)
	if p.cb != nil && p.written-p.lastTickAt >= progressTickBytes {
		p.cb(p.written, p.total)
		p.lastTickAt = p.written
	}
	return n, err
}

// VerifySHA256 streams the file at filePath through sha256.New() and returns
// nil if the resulting hex digest equals expectedHash (case-insensitive). Any
// IO error or mismatch yields a descriptive error so the applier can refuse
// to swap the binary into place.
//
// The expectedHash argument may be either the bare 64-char hex digest or the
// `<hex>  filename` form produced by `sha256sum` — the helper extracts the
// hex prefix on the caller's behalf.
func VerifySHA256(filePath, expectedHash string) error {
	if filePath == "" {
		return fmt.Errorf("ota: verify: empty file path")
	}
	expected := normaliseHash(expectedHash)
	if len(expected) != sha256.Size*2 {
		return fmt.Errorf("ota: verify: expected hash must be %d hex chars, got %d", sha256.Size*2, len(expected))
	}
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("ota: verify: open: %w", err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("ota: verify: hash: %w", err)
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, expected) {
		return fmt.Errorf("ota: sha256 mismatch: expected %s got %s", expected, got)
	}
	return nil
}

// FetchSHA256 issues a synchronous GET against the `.sha256` sidecar URL and
// returns the normalised hex digest. Used by the handler to chain
// CheckLatest → FetchSHA256 → Download → VerifySHA256 in one request.
func (d *Downloader) FetchSHA256(ctx context.Context, sha256URL string) (string, error) {
	if d == nil {
		return "", fmt.Errorf("ota: nil downloader")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sha256URL, nil)
	if err != nil {
		return "", fmt.Errorf("ota: build sha256 request: %w", err)
	}
	resp, err := d.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ota: sha256 fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("ota: sha256 fetch status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if err != nil {
		return "", fmt.Errorf("ota: sha256 read: %w", err)
	}
	hash := normaliseHash(string(body))
	if len(hash) != sha256.Size*2 {
		return "", fmt.Errorf("ota: sha256 sidecar malformed (got %d hex chars)", len(hash))
	}
	return hash, nil
}

// ErrSHA256Mismatch is the sentinel returned (wrapped) by VerifySHA256 when
// the on-disk digest disagrees with the published value. Handlers use
// errors.Is to map it onto a user-visible "checksum failed" message rather
// than a generic 500.
var ErrSHA256Mismatch = errors.New("ota: sha256 mismatch")

// normaliseHash trims whitespace and discards the sha256sum trailing filename
// (everything after the first whitespace).
func normaliseHash(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.IndexAny(s, " \t\n"); idx > 0 {
		s = s[:idx]
	}
	return strings.ToLower(strings.TrimSpace(s))
}
