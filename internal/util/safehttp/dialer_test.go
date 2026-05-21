package safehttp

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"testing"
)

func TestIsPrivateOrLoopback(t *testing.T) {
	cases := []struct {
		ip      string
		blocked bool
	}{
		{"127.0.0.1", true},      // loopback
		{"127.255.0.1", true},    // loopback /8
		{"10.0.0.1", true},       // RFC1918
		{"10.255.255.255", true}, // RFC1918
		{"172.16.0.1", true},     // RFC1918
		{"172.31.255.1", true},   // RFC1918
		{"192.168.1.1", true},    // RFC1918
		{"169.254.169.254", true}, // AWS metadata / link-local
		{"0.0.0.0", true},        // unspecified
		{"0.1.2.3", true},        // 0.0.0.0/8
		{"::1", true},            // IPv6 loopback
		{"fe80::1", true},        // IPv6 link-local
		{"fc00::1", true},        // IPv6 ULA
		{"fd12:3456::1", true},   // IPv6 ULA
		{"ff02::1", true},        // IPv6 multicast

		{"1.1.1.1", false},
		{"8.8.8.8", false},
		{"2606:4700:4700::1111", false},
	}
	for _, tc := range cases {
		ip := net.ParseIP(tc.ip)
		if got := IsPrivateOrLoopback(ip); got != tc.blocked {
			t.Errorf("IsPrivateOrLoopback(%s) = %v, want %v", tc.ip, got, tc.blocked)
		}
	}
}

func TestControlRejectBlocksPrivate(t *testing.T) {
	cases := []string{
		"127.0.0.1:80",
		"10.0.0.1:8080",
		"169.254.169.254:80",
		"[::1]:443",
		"[fc00::1]:443",
	}
	for _, addr := range cases {
		err := controlReject("tcp", addr, nil)
		if err == nil {
			t.Errorf("controlReject(%s) returned nil, expected ErrBlockedAddress", addr)
			continue
		}
		if !errors.Is(err, ErrBlockedAddress) {
			t.Errorf("controlReject(%s) err=%v, want ErrBlockedAddress", addr, err)
		}
	}
}

func TestControlRejectAllowsPublic(t *testing.T) {
	cases := []string{
		"1.1.1.1:443",
		"8.8.8.8:53",
		"[2606:4700:4700::1111]:443",
	}
	for _, addr := range cases {
		if err := controlReject("tcp", addr, nil); err != nil {
			t.Errorf("controlReject(%s) = %v, want nil", addr, err)
		}
	}
}

func TestNewDialerAllowPrivate(t *testing.T) {
	d := NewDialer(Config{AllowPrivate: true})
	if d.Control != nil {
		t.Fatalf("expected nil Control hook when AllowPrivate=true")
	}
}

func TestNewClientRejectsPrivate(t *testing.T) {
	client := NewClient(Config{}, 0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"http://10.0.0.1:8080/healthz", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	_, err = client.Do(req)
	if err == nil {
		t.Fatal("expected dial to be refused, got nil")
	}
	// The wrapping by net/http makes errors.Is unreliable across versions;
	// fall back to substring match on the safehttp sentinel text.
	if !strings.Contains(err.Error(), "safehttp") {
		t.Fatalf("unexpected error (want safehttp refusal): %v", err)
	}
}
