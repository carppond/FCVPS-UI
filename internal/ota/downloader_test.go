package ota_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"shiguang-vps/internal/ota"
)

func TestDownloader_Download_WritesBytesAndCallsProgress(t *testing.T) {
	t.Parallel()
	payload := bytes.Repeat([]byte("ABCD"), 50*1024) // 200 KiB triggers > 1 tick
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "204800")
		_, _ = w.Write(payload)
	}))
	t.Cleanup(srv.Close)

	d := ota.NewDownloader(ota.DownloaderConfig{HTTPClient: srv.Client()})
	var buf bytes.Buffer
	ticks := 0
	var lastTotal int64
	n, err := d.Download(context.Background(), srv.URL, &buf, func(downloaded, total int64) {
		ticks++
		lastTotal = total
	})
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if n != int64(len(payload)) {
		t.Fatalf("bytes mismatch: got %d want %d", n, len(payload))
	}
	if !bytes.Equal(buf.Bytes(), payload) {
		t.Fatalf("body mismatch")
	}
	if ticks < 2 {
		t.Fatalf("expected multiple progress ticks, got %d", ticks)
	}
	if lastTotal != int64(len(payload)) {
		t.Fatalf("final total %d != payload len %d", lastTotal, len(payload))
	}
}

func TestDownloader_Download_PropagatesNon2xx(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "gone", http.StatusGone)
	}))
	t.Cleanup(srv.Close)
	d := ota.NewDownloader(ota.DownloaderConfig{HTTPClient: srv.Client()})
	_, err := d.Download(context.Background(), srv.URL, &bytes.Buffer{}, nil)
	if err == nil {
		t.Fatalf("expected error for 410")
	}
}

func TestVerifySHA256_MatchAndMismatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "blob.bin")
	body := []byte("hello-ota-test")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	sum := sha256.Sum256(body)
	hex := hex.EncodeToString(sum[:])
	if err := ota.VerifySHA256(path, hex); err != nil {
		t.Fatalf("verify matches: %v", err)
	}
	// sha256sum-style format
	if err := ota.VerifySHA256(path, hex+"  blob.bin\n"); err != nil {
		t.Fatalf("verify accepts sha256sum format: %v", err)
	}
	if err := ota.VerifySHA256(path, "00"); err == nil {
		t.Fatalf("expected error for wrong-length hash")
	}
	wrong := "0000000000000000000000000000000000000000000000000000000000000000"
	if err := ota.VerifySHA256(path, wrong); err == nil {
		t.Fatalf("expected mismatch error")
	}
}

func TestDownloader_FetchSHA256(t *testing.T) {
	t.Parallel()
	good := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	mux := http.NewServeMux()
	mux.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(good + "  shiguang-vps-linux-amd64\n"))
	})
	mux.HandleFunc("/bad", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not-a-hash"))
	})
	mux.HandleFunc("/404", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	d := ota.NewDownloader(ota.DownloaderConfig{HTTPClient: srv.Client()})

	if hash, err := d.FetchSHA256(context.Background(), srv.URL+"/ok"); err != nil || hash != good {
		t.Fatalf("good sidecar: hash=%q err=%v", hash, err)
	}
	if _, err := d.FetchSHA256(context.Background(), srv.URL+"/bad"); err == nil {
		t.Fatalf("expected error for malformed sidecar")
	}
	if _, err := d.FetchSHA256(context.Background(), srv.URL+"/404"); err == nil {
		t.Fatalf("expected error for 404 sidecar")
	}
}
