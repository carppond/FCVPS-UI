package firewall

import (
	"context"
	"errors"
	"strings"
	"testing"
)

const ufwStatusSample = `Status: active

To                         Action      From
--                         ------      ----
22/tcp                     ALLOW       Anywhere
443/tcp                    ALLOW       Anywhere
8081                       ALLOW       Anywhere
Nginx Full                 ALLOW       Anywhere
9000/tcp                   DENY        Anywhere
22/tcp (v6)                ALLOW       Anywhere (v6)
443/tcp (v6)               ALLOW       Anywhere (v6)
`

func TestParseUFWStatus(t *testing.T) {
	active, rules := ParseUFWStatus(ufwStatusSample)
	if !active {
		t.Fatal("expected active")
	}
	// Expect 22/tcp, 443/tcp, 8081 — v6 dupes, the named profile, and the DENY
	// rule are all excluded.
	want := map[string]int{"22/tcp": 22, "443/tcp": 443, "8081": 8081}
	if len(rules) != len(want) {
		t.Fatalf("got %d rules %+v, want %d", len(rules), rules, len(want))
	}
	for _, r := range rules {
		p, ok := want[r.Spec]
		if !ok {
			t.Errorf("unexpected rule %q", r.Spec)
		}
		if r.Port != p {
			t.Errorf("rule %q: port=%d want %d", r.Spec, r.Port, p)
		}
	}
}

func TestParseUFWStatusInactive(t *testing.T) {
	active, _ := ParseUFWStatus("Status: inactive\n")
	if active {
		t.Fatal("expected inactive")
	}
}

func TestParseTargetProto(t *testing.T) {
	cases := []struct {
		in    string
		port  int
		proto string
	}{
		{"22/tcp", 22, "tcp"},
		{"53/udp", 53, "udp"},
		{"8081", 8081, ""},
		{"8000:8100/tcp", 0, "tcp"}, // range → not a simple port
		{"OpenSSH", 0, ""},
	}
	for _, c := range cases {
		p, pr := parseTarget(c.in)
		if p != c.port || pr != c.proto {
			t.Errorf("parseTarget(%q)=(%d,%q), want (%d,%q)", c.in, p, pr, c.port, c.proto)
		}
	}
}

func TestIsSimplePortSpec(t *testing.T) {
	ok := []string{"22", "8081/tcp", "53/udp"}
	bad := []string{"8000:8100/tcp", "OpenSSH", "22/sctp", "", "abc"}
	for _, s := range ok {
		if !IsSimplePortSpec(s) {
			t.Errorf("%q should be deletable", s)
		}
	}
	for _, s := range bad {
		if IsSimplePortSpec(s) {
			t.Errorf("%q should NOT be deletable", s)
		}
	}
}

const ssTCPSample = `LISTEN 0      4096         0.0.0.0:22         0.0.0.0:*    users:(("sshd",pid=700,fd=3))
LISTEN 0      511          0.0.0.0:443        0.0.0.0:*    users:(("nginx",pid=1200,fd=6))
LISTEN 0      128          127.0.0.1:8080     0.0.0.0:*    users:(("hub",pid=1500,fd=7))
LISTEN 0      511          [::]:443           [::]:*       users:(("nginx",pid=1200,fd=8))
`

func TestParseSSListeners(t *testing.T) {
	m := ParseSSListeners(ssTCPSample)
	if l, ok := m[22]; !ok || l.Process != "sshd" || l.PID != 700 {
		t.Errorf("port 22 = %+v, want sshd/700", l)
	}
	if l, ok := m[443]; !ok || l.Process != "nginx" {
		t.Errorf("port 443 = %+v, want nginx", l)
	}
	if l, ok := m[8080]; !ok || l.Process != "hub" || l.PID != 1500 {
		t.Errorf("port 8080 = %+v, want hub/1500", l)
	}
	if _, ok := m[80]; ok {
		t.Error("port 80 should not be present")
	}
}

// fakeRunner records invocations and returns scripted output keyed by the
// joined command. SS commands return the tcp listener sample.
type fakeRunner struct {
	calls     [][]string
	statusOut string
}

func (f *fakeRunner) run(_ context.Context, name string, args ...string) (string, string, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	joined := strings.Join(append([]string{name}, args...), " ")
	switch {
	case strings.Contains(joined, "ufw status"):
		return f.statusOut, "", nil
	case strings.Contains(joined, "ss -ltnpH"):
		return ssTCPSample, "", nil
	case strings.Contains(joined, "ss -lunpH"):
		return "", "", nil
	default:
		return "", "", nil
	}
}

func newTestService(protected ...int) (*Service, *fakeRunner) {
	f := &fakeRunner{statusOut: ufwStatusSample}
	svc := New(Config{
		run:            f.run,
		ProtectedPorts: protected,
		useSudo:        false,
		inContainer:    func() bool { return false },
		lookPath:       func(string) (string, error) { return "/usr/sbin/ufw", nil },
	})
	return svc, f
}

func TestAllowPortValidation(t *testing.T) {
	svc, _ := newTestService()
	if err := svc.AllowPort(context.Background(), 0, "tcp"); !errors.Is(err, ErrInvalidPort) {
		t.Errorf("port 0: got %v want ErrInvalidPort", err)
	}
	if err := svc.AllowPort(context.Background(), 8081, "sctp"); !errors.Is(err, ErrInvalidProto) {
		t.Errorf("bad proto: got %v want ErrInvalidProto", err)
	}
	if err := svc.AllowPort(context.Background(), 8081, "tcp"); err != nil {
		t.Errorf("valid allow: %v", err)
	}
}

func TestDeleteProtectedRefused(t *testing.T) {
	// 443 is the panel access port → protected.
	svc, f := newTestService(443)
	if err := svc.DeletePort(context.Background(), "22/tcp"); !errors.Is(err, ErrProtectedPort) {
		t.Errorf("delete SSH: got %v want ErrProtectedPort", err)
	}
	if err := svc.DeletePort(context.Background(), "443/tcp"); !errors.Is(err, ErrProtectedPort) {
		t.Errorf("delete access port: got %v want ErrProtectedPort", err)
	}
	// Ensure no `ufw delete` ever ran for the protected attempts.
	for _, c := range f.calls {
		if len(c) >= 2 && c[0] == "ufw" && c[1] == "delete" {
			t.Errorf("protected delete leaked to exec: %v", c)
		}
	}
	// A non-protected port deletes fine.
	if err := svc.DeletePort(context.Background(), "8081/tcp"); err != nil {
		t.Errorf("delete 8081: %v", err)
	}
}

func TestDeleteNamedProfileRejected(t *testing.T) {
	svc, _ := newTestService()
	if err := svc.DeletePort(context.Background(), "OpenSSH"); !errors.Is(err, ErrNotDeletable) {
		t.Errorf("named profile: got %v want ErrNotDeletable", err)
	}
}

func TestListRulesEnrichesAndProtects(t *testing.T) {
	svc, _ := newTestService(443)
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

func TestDetectStatusContainer(t *testing.T) {
	svc := New(Config{
		run:         func(context.Context, string, ...string) (string, string, error) { return "", "", nil },
		inContainer: func() bool { return true },
		lookPath:    func(string) (string, error) { return "/usr/sbin/ufw", nil },
	})
	st := svc.DetectStatus(context.Background())
	if st.CanManage || st.Reason == "" {
		t.Errorf("container: got %+v, want CanManage=false + reason", st)
	}
}

func TestDetectStatusActive(t *testing.T) {
	svc, _ := newTestService()
	st := svc.DetectStatus(context.Background())
	if !st.Available || !st.Active || !st.CanManage || st.Backend != "ufw" {
		t.Errorf("active: got %+v", st)
	}
}
