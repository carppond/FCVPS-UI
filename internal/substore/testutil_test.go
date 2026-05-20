package substore

import "encoding/base64"

// b64 is a tiny helper used by the parser tests to base64-encode literal
// JSON / payload strings inline.
func b64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

// b64url returns the URL-safe (without padding) base64 of s.
func b64url(s string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(s))
}
