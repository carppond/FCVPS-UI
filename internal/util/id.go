package util

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"sync"
)

// UUIDv7Size is the canonical text length of a UUID v7 (36 chars incl. hyphens).
const UUIDv7Size = 36

// uuidMu protects against the highly unlikely case where two calls in the
// same millisecond would return identical values. We monotonically bump the
// random tail in that case.
var uuidMu sync.Mutex
var lastUUIDMs int64
var lastUUIDSeq uint16

// UUIDv7 returns a RFC 9562 §5.7 UUID v7 as a canonical lowercase string.
//
// Layout (16 bytes):
//
//	0-5   : Unix timestamp (ms, big-endian, 48 bit)
//	6     : version nibble (0x7) + 4 random bits
//	7     : 8 random bits
//	8     : variant (10xxxxxx) + 6 random bits
//	9-15  : 56 random bits
//
// Monotonicity within the same millisecond is preserved by incrementing an
// internal sequence counter encoded into the high 12 random bits at byte 6/7.
func UUIDv7() string {
	uuidMu.Lock()
	defer uuidMu.Unlock()

	ms := NowUnixMs()
	if ms <= lastUUIDMs {
		// Tight loop: same millisecond. Bump seq; if seq would overflow the
		// 12 bits we store, advance the encoded timestamp instead. Either way
		// the next UUID is strictly greater than the previous one.
		if lastUUIDSeq >= 0x0FFF {
			lastUUIDMs++
			lastUUIDSeq = 0
		} else {
			lastUUIDSeq++
		}
		ms = lastUUIDMs
	} else {
		lastUUIDMs = ms
		lastUUIDSeq = 0
	}
	seq := lastUUIDSeq

	var b [16]byte
	b[0] = byte(ms >> 40)
	b[1] = byte(ms >> 32)
	b[2] = byte(ms >> 24)
	b[3] = byte(ms >> 16)
	b[4] = byte(ms >> 8)
	b[5] = byte(ms)

	// 10 random bytes (positions 6..15) — we will overwrite the upper bits.
	if _, err := rand.Read(b[6:]); err != nil {
		// Should be impossible on supported platforms; degrade to zeros.
		for i := 6; i < 16; i++ {
			b[i] = 0
		}
	}

	// Encode sequence (12 bits) in the upper 12 bits of bytes 6..7.
	// Byte 6 layout: VVVV SSSS (version + top 4 seq bits)
	// Byte 7 layout: SSSSSSSS (lower 8 seq bits)
	b[6] = 0x70 | byte((seq>>8)&0x0F)
	b[7] = byte(seq & 0xFF)

	// Variant: 10xxxxxx in byte 8.
	b[8] = (b[8] & 0x3F) | 0x80

	const hexChars = "0123456789abcdef"
	out := make([]byte, 36)
	pos := 0
	for i, v := range b {
		out[pos] = hexChars[v>>4]
		out[pos+1] = hexChars[v&0x0F]
		pos += 2
		if i == 3 || i == 5 || i == 7 || i == 9 {
			out[pos] = '-'
			pos++
		}
	}
	return string(out)
}

// RandomHex32 returns a 32-character lowercase hex string (16 random bytes).
// Used for silent-mode URL prefixes, short_links.file_code/user_code pairs,
// agent token prefixes, etc.
func RandomHex32() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		// Should never happen; surface a panic instead of returning a
		// guessable token — callers always treat tokens as opaque.
		panic(fmt.Errorf("util.RandomHex32: read random: %w", err))
	}
	return hex.EncodeToString(buf[:])
}

// RandomBase64URL returns a URL-safe base64 string carrying n cryptographically
// random bytes, padding stripped. Used by the Nezha compat token mint flow
// (T-17) where we want shorter tokens than RandomHex32 — 16 bytes encodes to
// 22 characters base64url which mirrors the Nezha agent's own secret format.
func RandomBase64URL(n int) string {
	if n <= 0 {
		return ""
	}
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		panic(fmt.Errorf("util.RandomBase64URL(%d): read random: %w", n, err))
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

// RandomBytes returns n cryptographically secure random bytes. Returns an
// empty slice when n <= 0.
func RandomBytes(n int) []byte {
	if n <= 0 {
		return []byte{}
	}
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		panic(fmt.Errorf("util.RandomBytes(%d): read random: %w", n, err))
	}
	return buf
}
