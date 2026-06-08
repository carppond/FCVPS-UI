package firewall

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// execFunc runs a base command (sudo-prefixed + timeout applied by the
// Service). It mirrors Service.exec so backends stay free of Service internals.
type execFunc func(ctx context.Context, base string, args ...string) (stdout, stderr string, err error)

// backend abstracts the concrete firewall frontend (ufw / firewalld). The
// Service owns all the cross-cutting safety (protected ports, listener
// enrichment, validation, mutex); a backend only knows how to talk to its CLI.
type backend interface {
	// name is the stable identifier surfaced in Status.Backend.
	name() string
	// detect reports whether this backend's CLI is present on the host.
	// exists is the injectable absolute-path probe (so tests don't depend on
	// the CI runner's installed firewall binaries).
	detect(lookPath func(string) (string, error), exists func(string) bool) bool
	// active reports whether the firewall is currently enforcing rules. raw is
	// the combined CLI output, used only for diagnostics on error.
	active(ctx context.Context, run execFunc) (active bool, raw string, err error)
	// listAllow returns the ALLOW rules (numeric port/proto) without the
	// listener/protected enrichment the Service layers on top.
	listAllow(ctx context.Context, run execFunc) ([]Rule, error)
	// allow adds an allow-in rule. port/proto are pre-validated by the Service.
	allow(ctx context.Context, run execFunc, port int, proto string) error
	// remove deletes the allow rule named by spec (pre-validated as a simple
	// numeric port or port/proto by the Service).
	remove(ctx context.Context, run execFunc, spec string) error
}

// findBinary resolves name via lookPath, falling back to common absolute paths
// (probed via exists) for sparse service-user PATHs.
func findBinary(lookPath func(string) (string, error), exists func(string) bool, name string, fallbacks ...string) bool {
	if lookPath != nil {
		if _, err := lookPath(name); err == nil {
			return true
		}
	}
	if exists != nil {
		for _, p := range fallbacks {
			if exists(p) {
				return true
			}
		}
	}
	return false
}

// allBackends is the detection priority order. ufw first preserves the prior
// default when both are installed.
func allBackends() []backend {
	return []backend{ufwBackend{}, firewalldBackend{}}
}

// ── ufw ───────────────────────────────────────────────────────────────────

type ufwBackend struct{}

func (ufwBackend) name() string { return "ufw" }

func (ufwBackend) detect(lookPath func(string) (string, error), exists func(string) bool) bool {
	return findBinary(lookPath, exists, "ufw", "/usr/sbin/ufw", "/sbin/ufw", "/usr/bin/ufw")
}

func (ufwBackend) active(ctx context.Context, run execFunc) (bool, string, error) {
	stdout, stderr, err := run(ctx, "ufw", "status")
	if err != nil {
		return false, stdout + stderr, err
	}
	active, _ := ParseUFWStatus(stdout)
	return active, stdout, nil
}

func (ufwBackend) listAllow(ctx context.Context, run execFunc) ([]Rule, error) {
	stdout, stderr, err := run(ctx, "ufw", "status")
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrUnavailable, firstLine(stdout+stderr))
	}
	_, rules := ParseUFWStatus(stdout)
	return rules, nil
}

func (ufwBackend) allow(ctx context.Context, run execFunc, port int, proto string) error {
	spec := strconv.Itoa(port) + "/" + proto
	stdout, stderr, err := run(ctx, "ufw", "allow", spec)
	if err != nil {
		return fmt.Errorf("ufw allow %s: %s", spec, firstLine(stdout+stderr))
	}
	return nil
}

func (ufwBackend) remove(ctx context.Context, run execFunc, spec string) error {
	stdout, stderr, err := run(ctx, "ufw", "delete", "allow", spec)
	if err != nil {
		return fmt.Errorf("ufw delete allow %s: %s", spec, firstLine(stdout+stderr))
	}
	return nil
}

// ── firewalld ───────────────────────────────────────────────────────────────

type firewalldBackend struct{}

func (firewalldBackend) name() string { return "firewalld" }

func (firewalldBackend) detect(lookPath func(string) (string, error), exists func(string) bool) bool {
	return findBinary(lookPath, exists, "firewall-cmd",
		"/usr/bin/firewall-cmd", "/usr/sbin/firewall-cmd", "/bin/firewall-cmd")
}

func (firewalldBackend) active(ctx context.Context, run execFunc) (bool, string, error) {
	// `firewall-cmd --state` prints "running" / "not running" and exits 0 only
	// when running. Treat a clean "running" as active; a non-zero exit means
	// the daemon is stopped (not an error we should surface as unavailable).
	stdout, stderr, err := run(ctx, "firewall-cmd", "--state")
	raw := stdout + stderr
	if strings.Contains(raw, "running") && !strings.Contains(raw, "not running") {
		return true, stdout, nil
	}
	if err != nil {
		// "not running" exits non-zero — that's a stopped (inactive) daemon,
		// still a manageable backend, not a hard failure.
		if strings.Contains(raw, "not running") {
			return false, stdout, nil
		}
		return false, raw, err
	}
	return false, stdout, nil
}

func (firewalldBackend) listAllow(ctx context.Context, run execFunc) ([]Rule, error) {
	// --list-ports prints the active zone's ports as "80/tcp 443/tcp 53/udp".
	// Service-based rules (e.g. the "ssh" service) are intentionally not shown;
	// the panel manages explicit ports, and SSH stays protected via the live
	// listener / protectedSet regardless.
	stdout, stderr, err := run(ctx, "firewall-cmd", "--list-ports")
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrUnavailable, firstLine(stdout+stderr))
	}
	return parseFirewalldPorts(stdout), nil
}

func (firewalldBackend) allow(ctx context.Context, run execFunc, port int, proto string) error {
	spec := strconv.Itoa(port) + "/" + proto
	// Apply to the running config AND persist, so the change takes effect
	// immediately without a `--reload` (reload can briefly disrupt connections).
	if stdout, stderr, err := run(ctx, "firewall-cmd", "--add-port="+spec); err != nil {
		return fmt.Errorf("firewall-cmd --add-port=%s: %s", spec, firstLine(stdout+stderr))
	}
	if stdout, stderr, err := run(ctx, "firewall-cmd", "--permanent", "--add-port="+spec); err != nil {
		return fmt.Errorf("firewall-cmd --permanent --add-port=%s: %s", spec, firstLine(stdout+stderr))
	}
	return nil
}

func (firewalldBackend) remove(ctx context.Context, run execFunc, spec string) error {
	port, proto := parseTarget(spec)
	if port == 0 {
		return ErrNotDeletable
	}
	// A bare port (no proto) maps to both tcp and udp; remove whichever exist.
	protos := []string{proto}
	if proto == "" {
		protos = []string{"tcp", "udp"}
	}
	var lastErr error
	removed := false
	for _, pr := range protos {
		ps := strconv.Itoa(port) + "/" + pr
		_, _, e1 := run(ctx, "firewall-cmd", "--remove-port="+ps)
		stdout, stderr, e2 := run(ctx, "firewall-cmd", "--permanent", "--remove-port="+ps)
		if e2 != nil {
			lastErr = fmt.Errorf("firewall-cmd --permanent --remove-port=%s: %s", ps, firstLine(stdout+stderr))
			continue
		}
		_ = e1 // runtime removal failure is non-fatal as long as permanent succeeds
		removed = true
	}
	if !removed && lastErr != nil {
		return lastErr
	}
	return nil
}
