package firewall

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// ── parser ──────────────────────────────────────────────────────────────────

func TestParseFirewalldPorts(t *testing.T) {
	rules := parseFirewalldPorts("80/tcp 443/tcp 53/udp 8081/tcp 80/tcp")
	bySpec := map[string]Rule{}
	for _, r := range rules {
		bySpec[r.Spec] = r
	}
	if len(bySpec) != 4 {
		t.Fatalf("want 4 unique rules, got %d (%v)", len(bySpec), rules)
	}
	if r := bySpec["443/tcp"]; r.Port != 443 || r.Proto != "tcp" {
		t.Errorf("443/tcp parsed wrong: %+v", r)
	}
	if r := bySpec["53/udp"]; r.Port != 53 || r.Proto != "udp" {
		t.Errorf("53/udp parsed wrong: %+v", r)
	}
}

func TestParseFirewalldPortsEmpty(t *testing.T) {
	if rules := parseFirewalldPorts("\n  \n"); len(rules) != 0 {
		t.Fatalf("empty output should yield no rules, got %v", rules)
	}
}

// ── fake runner scoped to firewalld ─────────────────────────────────────────

type fakeFirewalld struct {
	calls    [][]string
	portsOut string
	state    string // "running" / "not running"
}

func (f *fakeFirewalld) run(_ context.Context, name string, args ...string) (string, string, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	joined := strings.Join(append([]string{name}, args...), " ")
	switch {
	case strings.Contains(joined, "firewall-cmd --state"):
		if f.state == "" {
			f.state = "running"
		}
		if f.state == "not running" {
			return "not running\n", "", errors.New("exit 252")
		}
		return f.state + "\n", "", nil
	case strings.Contains(joined, "firewall-cmd --list-ports"):
		return f.portsOut, "", nil
	case strings.Contains(joined, "ss -ltnpH"):
		return ssTCPSample, "", nil
	default:
		return "", "", nil
	}
}

// newFirewalldService builds a Service whose backend resolves to firewalld:
// lookPath fails for ufw and succeeds for firewall-cmd.
func newFirewalldService(protected ...int) (*Service, *fakeFirewalld) {
	f := &fakeFirewalld{portsOut: "22/tcp 443/tcp 8081/tcp 53/udp"}
	svc := New(Config{
		run:            f.run,
		ProtectedPorts: protected,
		useSudo:        false,
		inContainer:    func() bool { return false },
		lookPath: func(name string) (string, error) {
			if name == "firewall-cmd" {
				return "/usr/bin/firewall-cmd", nil
			}
			return "", errors.New("not found")
		},
	})
	return svc, f
}

func TestFirewalldBackendSelected(t *testing.T) {
	svc, _ := newFirewalldService()
	if svc.backend == nil || svc.backend.name() != "firewalld" {
		t.Fatalf("expected firewalld backend, got %v", svc.backend)
	}
}

func TestFirewalldDetectStatusActive(t *testing.T) {
	svc, _ := newFirewalldService()
	st := svc.DetectStatus(context.Background())
	if !st.Available || !st.Active || !st.CanManage || st.Backend != "firewalld" {
		t.Errorf("active: got %+v", st)
	}
}

func TestFirewalldDetectStatusStopped(t *testing.T) {
	svc, f := newFirewalldService()
	f.state = "not running"
	st := svc.DetectStatus(context.Background())
	// Daemon stopped is still a manageable backend, just not currently active.
	if !st.Available || st.Active || !st.CanManage || st.Backend != "firewalld" {
		t.Errorf("stopped: got %+v", st)
	}
}

func TestFirewalldListRulesEnrichesAndProtects(t *testing.T) {
	svc, _ := newFirewalldService(443)
	rules, err := svc.ListRules(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	byPort := map[int]Rule{}
	for _, r := range rules {
		byPort[r.Port] = r
	}
	if r := byPort[22]; r.Process != "sshd" || !r.Protected {
		t.Errorf("port 22 = %+v, want sshd + protected", r)
	}
	if r := byPort[443]; r.Process != "nginx" || !r.Protected {
		t.Errorf("port 443 = %+v, want nginx + protected", r)
	}
	if r := byPort[8081]; r.Protected {
		t.Errorf("port 8081 should not be protected")
	}
}

func TestFirewalldAllowWritesRuntimeAndPermanent(t *testing.T) {
	svc, f := newFirewalldService()
	if err := svc.AllowPort(context.Background(), 8443, "tcp"); err != nil {
		t.Fatalf("AllowPort: %v", err)
	}
	var runtime, permanent bool
	for _, c := range f.calls {
		j := strings.Join(c, " ")
		if strings.Contains(j, "firewall-cmd --add-port=8443/tcp") {
			runtime = true
		}
		if strings.Contains(j, "firewall-cmd --permanent --add-port=8443/tcp") {
			permanent = true
		}
	}
	if !runtime || !permanent {
		t.Errorf("allow must write runtime AND permanent (runtime=%v permanent=%v): %v", runtime, permanent, f.calls)
	}
}

func TestFirewalldDeleteProtectedRefused(t *testing.T) {
	svc, f := newFirewalldService(443)
	if err := svc.DeletePort(context.Background(), "22/tcp"); !errors.Is(err, ErrProtectedPort) {
		t.Errorf("delete SSH: got %v want ErrProtectedPort", err)
	}
	if err := svc.DeletePort(context.Background(), "443/tcp"); !errors.Is(err, ErrProtectedPort) {
		t.Errorf("delete access port: got %v want ErrProtectedPort", err)
	}
	for _, c := range f.calls {
		if strings.Contains(strings.Join(c, " "), "remove-port") {
			t.Errorf("protected delete leaked a remove-port call: %v", c)
		}
	}
}

func TestFirewalldDeleteRemovesRuntimeAndPermanent(t *testing.T) {
	svc, f := newFirewalldService()
	if err := svc.DeletePort(context.Background(), "8081/tcp"); err != nil {
		t.Fatalf("DeletePort: %v", err)
	}
	var runtime, permanent bool
	for _, c := range f.calls {
		j := strings.Join(c, " ")
		if strings.Contains(j, "firewall-cmd --remove-port=8081/tcp") {
			runtime = true
		}
		if strings.Contains(j, "firewall-cmd --permanent --remove-port=8081/tcp") {
			permanent = true
		}
	}
	if !runtime || !permanent {
		t.Errorf("delete must remove runtime AND permanent: %v", f.calls)
	}
}

func TestFirewalldDeleteBarePortRemovesBothProtos(t *testing.T) {
	svc, f := newFirewalldService()
	if err := svc.DeletePort(context.Background(), "8081"); err != nil {
		t.Fatalf("DeletePort bare: %v", err)
	}
	var tcp, udp bool
	for _, c := range f.calls {
		j := strings.Join(c, " ")
		if strings.Contains(j, "--permanent --remove-port=8081/tcp") {
			tcp = true
		}
		if strings.Contains(j, "--permanent --remove-port=8081/udp") {
			udp = true
		}
	}
	if !tcp || !udp {
		t.Errorf("bare-port delete should remove both protos (tcp=%v udp=%v): %v", tcp, udp, f.calls)
	}
}
