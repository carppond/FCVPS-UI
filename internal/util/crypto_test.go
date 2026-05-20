package util_test

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"shiguang-vps/internal/util"
)

func TestSHA256HexKnownVector(t *testing.T) {
	// RFC 6234 / NIST test vector: sha256("abc")
	got := util.SHA256Hex("abc")
	want := "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got != want {
		t.Fatalf("SHA256Hex(\"abc\") = %s, want %s", got, want)
	}
}

func TestSHA256HexEmpty(t *testing.T) {
	got := util.SHA256Hex("")
	sum := sha256.Sum256(nil)
	if got != hex.EncodeToString(sum[:]) {
		t.Fatalf("SHA256Hex empty mismatch: %s", got)
	}
}

func TestHashPasswordVerifyRoundtrip(t *testing.T) {
	plain := "S0me-Sup3r-Strong-Pass!"
	hashed, err := util.HashPassword(plain)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hashed == plain {
		t.Fatal("HashPassword returned plaintext")
	}
	if !util.VerifyPassword(plain, hashed) {
		t.Fatal("VerifyPassword should accept correct password")
	}
	if util.VerifyPassword("wrong", hashed) {
		t.Fatal("VerifyPassword accepted wrong password")
	}
}

func TestHashPasswordDifferentSalt(t *testing.T) {
	plain := "same-password"
	first, err := util.HashPassword(plain)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := util.HashPassword(plain)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if first == second {
		t.Fatal("HashPassword produced identical hashes for same plaintext (salt missing?)")
	}
}

func TestHashPasswordRejectsEmpty(t *testing.T) {
	if _, err := util.HashPassword(""); err == nil {
		t.Fatal("HashPassword(\"\") should error")
	}
	if util.VerifyPassword("", "anything") {
		t.Fatal("VerifyPassword should reject empty plaintext")
	}
}

func TestBase64URLRoundtrip(t *testing.T) {
	in := []byte{0xff, 0xee, 0xdd, 0xcc, 0xbb, 0xaa, 0x00, 0x11}
	encoded := util.Base64URL(in)
	decoded, err := util.UnBase64URL(encoded)
	if err != nil {
		t.Fatalf("UnBase64URL: %v", err)
	}
	if len(decoded) != len(in) {
		t.Fatalf("decoded len = %d, want %d", len(decoded), len(in))
	}
	for i := range in {
		if decoded[i] != in[i] {
			t.Fatalf("byte %d: got 0x%02x want 0x%02x", i, decoded[i], in[i])
		}
	}
}

func TestUnBase64URLRejectsInvalid(t *testing.T) {
	if _, err := util.UnBase64URL("!!! not base64"); err == nil {
		t.Fatal("UnBase64URL should error on invalid input")
	}
}
