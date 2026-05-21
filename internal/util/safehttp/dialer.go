// Package safehttp builds *http.Client values that refuse outbound dials to
// RFC1918 / loopback / link-local / IPv6 ULA destinations. The goal is to
// shrink the SSRF surface area for any user-supplied URL the hub fetches:
//
//   - subscription source_url fetch (substore.SyncService.fetchURL)
//   - notification webhooks (ch_webhook / ch_discord / ch_slack / …)
//
// Both call paths take an arbitrary URL from a (possibly non-admin) user and
// follow it server-side, so without this guard a logged-in attacker could
// probe the cloud metadata service, container-internal admin ports, or
// other tenants' SSRF-reachable services.
//
// The implementation operates at the dial layer so:
//
//   - DNS rebinding is defeated (the same Dial that resolves the name also
//     vets the resolved IP; the http client never sees a hostname that
//     resolves to two different IPs across the lookup and the connect).
//   - Custom http.Client.Transport.DialContext gets the per-connection veto
//     hook for free.
//
// AllowPrivate=true is provided so an admin who needs to fetch
// http://10.x.y.z/some/internal/service can flip a single system_settings
// row (allow_private_networks=true) instead of disabling the protection
// entirely.
package safehttp

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"syscall"
	"time"
)

// ErrBlockedAddress is the sentinel returned when a dial is refused. Wrap
// callers check it with errors.Is to surface a friendlier "private network
// not allowed" message.
var ErrBlockedAddress = errors.New("safehttp: dial to blocked address refused")

// Config is the optional knob set for NewDialer / NewClient.
type Config struct {
	// AllowPrivate, when true, disables the RFC1918 / loopback / link-local
	// / ULA filter. Reserved for admin-controlled deployments where the
	// hub legitimately fetches an internal HTTP service.
	AllowPrivate bool

	// DialTimeout caps how long a single dial attempt may take. 0 falls
	// back to the package default (10s) which is roomy enough for slow
	// upstreams while still bounding the worst case.
	DialTimeout time.Duration

	// KeepAlive sets the TCP keepalive interval. 0 falls back to 30s.
	KeepAlive time.Duration
}

// NewDialer returns a *net.Dialer whose DialContext rejects connections to
// addresses that resolve into the disallowed private / loopback ranges.
// When cfg.AllowPrivate is true the dialer behaves like a vanilla
// net.Dialer.
func NewDialer(cfg Config) *net.Dialer {
	if cfg.DialTimeout <= 0 {
		cfg.DialTimeout = 10 * time.Second
	}
	if cfg.KeepAlive <= 0 {
		cfg.KeepAlive = 30 * time.Second
	}
	d := &net.Dialer{Timeout: cfg.DialTimeout, KeepAlive: cfg.KeepAlive}
	if cfg.AllowPrivate {
		return d
	}
	// Install a Control hook that vetoes the resolved address. Control runs
	// AFTER name resolution but BEFORE connect — perfect for SSRF guarding.
	d.Control = controlReject
	return d
}

// NewTransport wraps a *http.Transport with safe defaults plus the dialer
// from NewDialer. Callers attach the result to their own *http.Client.
func NewTransport(cfg Config) *http.Transport {
	d := NewDialer(cfg)
	t := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           d.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          50,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return t
}

// NewClient returns a *http.Client configured with the safe transport. The
// caller-supplied timeout applies to the entire round-trip; passing 0 falls
// back to 30s (the same default the subscription fetcher / webhook
// channels use).
func NewClient(cfg Config, timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &http.Client{
		Transport: NewTransport(cfg),
		Timeout:   timeout,
	}
}

// controlReject is the net.Dialer.Control hook. It blocks the dial when the
// resolved address falls into a disallowed range. Returning a non-nil
// error here aborts the connection before any TCP handshake.
func controlReject(network, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		// Some callers pass the raw IP without port (e.g. ICMP). Tolerate
		// the parse failure and fall back to treating `address` as host.
		host = address
	}
	ip := net.ParseIP(host)
	if ip == nil {
		// A literal IP is expected here because net.Dialer resolved the name
		// before calling Control. If we somehow got a hostname, treat it as
		// a programming error and refuse the dial to fail closed.
		return fmt.Errorf("%w: %s (unresolved)", ErrBlockedAddress, address)
	}
	if IsPrivateOrLoopback(ip) {
		return fmt.Errorf("%w: %s", ErrBlockedAddress, ip.String())
	}
	return nil
}

// IsPrivateOrLoopback reports whether ip is in a range the dialer refuses by
// default: RFC1918, loopback, link-local, multicast, the v4 0.0.0.0/8, the
// v6 ULA fc00::/7 + link-local fe80::/10. Exposed for tests + admin
// settings handler ("private network reachability").
func IsPrivateOrLoopback(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified() ||
		ip.IsInterfaceLocalMulticast() {
		return true
	}
	// IPv4 0.0.0.0/8 (RFC 1122 §3.2.1.3) — IsUnspecified only catches
	// 0.0.0.0 exactly, so check the leading octet explicitly.
	if v4 := ip.To4(); v4 != nil {
		if v4[0] == 0 {
			return true
		}
		// AWS metadata: 169.254.169.254 is the canonical EC2 IMDS address
		// but is already covered by IsLinkLocalUnicast (169.254.0.0/16).
		return false
	}
	return false
}

// Default returns a client built with default settings (AllowPrivate=false,
// 30s timeout). Convenience constructor for the substore.SyncService.
func Default() *http.Client {
	return NewClient(Config{}, 0)
}
