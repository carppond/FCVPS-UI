package middleware

import (
	"bytes"
	"io"
)

// readAll reads as much of r as fits in buf, returning the byte count and any
// non-EOF error. Used by the audit middleware so we can keep buffer
// allocation bounded.
func readAll(r io.Reader, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := r.Read(buf[total:])
		total += n
		if err == io.EOF {
			return total, nil
		}
		if err != nil {
			return total, err
		}
		if n == 0 {
			break
		}
	}
	return total, nil
}

// replayBody is the http.Request.Body returned by capturePayload after the
// middleware has buffered the original body. It allows downstream handlers
// to re-read the (possibly truncated) payload without surprises.
type replayBody struct {
	*bytes.Reader
	truncated bool
}

func newReplayBody(b []byte, truncated bool) io.ReadCloser {
	return &replayBody{Reader: bytes.NewReader(b), truncated: truncated}
}

// Close satisfies io.Closer; the underlying buffer requires no cleanup.
func (rb *replayBody) Close() error { return nil }

// Truncated reports whether the audit middleware capped the captured body.
// Handlers do not normally need this; it exists for tests.
func (rb *replayBody) Truncated() bool { return rb.truncated }
