// Package firewall manages the local host's ufw firewall on behalf of the
// admin panel. It is deliberately scoped to the machine the hub runs on (not
// remote VPS assets) and only to ufw — the friendly frontend that persists
// rules across reboots. Raw iptables / firewalld are detected but not managed
// (see DetectStatus).
//
// Safety model:
//   - Mutations require role=admin (enforced at the HTTP layer) and are
//     serialised by a mutex (ufw rule numbers shift on every change).
//   - SSH and the panel's own access port are "protected": the service
//     refuses to delete their allow-rules so an operator cannot lock
//     themselves out from the very panel they are using.
//   - The service never enables/disables ufw — flipping ufw on without the
//     right allow-rules is an instant lockout, so that stays a manual,
//     documented SSH operation.
//   - All commands run via exec with an argument slice (never a shell
//     string); ports are validated as integers and protocols against a
//     whitelist, so neither shell nor ufw argument injection is possible.
package firewall

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Errors returned by the service. Handlers map these onto API error codes.
var (
	// ErrUnavailable means ufw is not installed / not manageable in this
	// environment (container, missing binary, no privilege).
	ErrUnavailable = errors.New("firewall: ufw not available")
	// ErrProtectedPort means the caller tried to delete a rule for SSH or the
	// panel access port.
	ErrProtectedPort = errors.New("firewall: port is protected")
	// ErrInvalidPort / ErrInvalidProto are validation failures.
	ErrInvalidPort  = errors.New("firewall: invalid port")
	ErrInvalidProto = errors.New("firewall: invalid protocol")
	// ErrNotDeletable means the rule spec is not a simple numeric port rule
	// (e.g. a named app profile) and is not deletable through the UI.
	ErrNotDeletable = errors.New("firewall: rule not deletable via panel")
)

// commandTimeout caps any single ufw / ss invocation. These are local,
// near-instant commands; a multi-second hang means something is wrong.
const commandTimeout = 8 * time.Second

// runner executes a command and returns combined behaviour needed by the
// service. Abstracted for tests. stdout is returned even on non-nil err so
// callers can inspect partial output (ufw prints status to stdout).
type runner func(ctx context.Context, name string, args ...string) (stdout, stderr string, err error)

// Status is the environment probe surfaced to the UI so it can render the
// feature as usable, read-only, or disabled-with-reason.
type Status struct {
	Available   bool   `json:"available"`    // ufw binary present and queryable
	Active      bool   `json:"active"`       // ufw is enforcing rules
	CanManage   bool   `json:"can_manage"`   // hub may add/delete rules
	InContainer bool   `json:"in_container"` // running inside a container
	Backend     string `json:"backend"`      // "ufw" | "firewalld" | "iptables" | "none"
	Reason      string `json:"reason,omitempty"`
}

// Config wires the service.
type Config struct {
	Logger *slog.Logger
	// ProtectedPorts are always-protected ports beyond the auto-detected SSH
	// port — typically the panel's external access port, injected by the
	// deploy/systemd env. Deletion of these rules is refused.
	ProtectedPorts []int
	// run is the command runner; nil uses the real exec runner.
	run runner
	// useSudo forces sudo prefixing; when the Config is built via New it is
	// derived from the effective uid (root → false, otherwise true).
	useSudo bool
	// inContainerOverride / lookPath let tests bypass host probes. nil → real.
	inContainer func() bool
	lookPath    func(string) (string, error)
}

// Service is safe for concurrent use; mutations are serialised.
type Service struct {
	mu        sync.Mutex
	cfg       Config
	run       runner
	useSudo   bool
	protected []int
}

// New constructs a Service. It derives sudo usage from the effective uid:
// root runs ufw directly, anyone else is prefixed with `sudo -n` (the deploy
// is expected to install a NOPASSWD sudoers entry for ufw/ss).
func New(cfg Config) *Service {
	run := cfg.run
	if run == nil {
		run = realRunner
	}
	inContainer := cfg.inContainer
	if inContainer == nil {
		inContainer = detectContainer
	}
	cfg.inContainer = inContainer
	if cfg.lookPath == nil {
		cfg.lookPath = exec.LookPath
	}
	useSudo := cfg.useSudo
	if !useSudo && os.Geteuid() != 0 {
		useSudo = true
	}
	return &Service{cfg: cfg, run: run, useSudo: useSudo, protected: cfg.ProtectedPorts}
}

func realRunner(ctx context.Context, name string, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	return out.String(), errb.String(), err
}

// detectContainer reports whether we appear to be inside a container, where
// managing the host firewall is impossible.
func detectContainer() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		s := string(data)
		if strings.Contains(s, "docker") || strings.Contains(s, "containerd") || strings.Contains(s, "kubepods") {
			return true
		}
	}
	return false
}

// ufwArgs builds the argv for a ufw invocation, prefixing sudo when needed.
func (s *Service) cmd(base string, args ...string) (string, []string) {
	full := append([]string{base}, args...)
	if s.useSudo {
		return "sudo", append([]string{"-n", "--"}, full...)
	}
	return full[0], full[1:]
}

func (s *Service) exec(ctx context.Context, base string, args ...string) (string, string, error) {
	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	name, argv := s.cmd(base, args...)
	return s.run(ctx, name, argv...)
}

// ufwPath locates the ufw binary, tolerating a sparse PATH for non-login
// service users (ufw commonly lives in /usr/sbin).
func (s *Service) ufwPath() (string, bool) {
	if p, err := s.cfg.lookPath("ufw"); err == nil {
		return p, true
	}
	for _, p := range []string{"/usr/sbin/ufw", "/sbin/ufw", "/usr/bin/ufw"} {
		if _, err := os.Stat(p); err == nil {
			return p, true
		}
	}
	return "", false
}

// DetectStatus probes the environment. It never errors — every failure mode
// maps onto a Status with CanManage=false and a human-readable Reason.
func (s *Service) DetectStatus(ctx context.Context) Status {
	st := Status{InContainer: s.cfg.inContainer(), Backend: "none"}
	if st.InContainer {
		st.Reason = "运行在容器内，无法管理宿主机防火墙；请在宿主机用 -p 端口映射"
		return st
	}
	ufw, ok := s.ufwPath()
	if !ok {
		// ufw missing — note other backends if present so the UI can hint.
		if _, err := s.cfg.lookPath("firewall-cmd"); err == nil {
			st.Backend = "firewalld"
			st.Reason = "检测到 firewalld（暂不支持面板管理），请手动配置或改用 ufw"
		} else if _, err := s.cfg.lookPath("iptables"); err == nil {
			st.Backend = "iptables"
			st.Reason = "检测到裸 iptables（暂不支持面板管理），建议安装 ufw 统一管理"
		} else {
			st.Reason = "未安装 ufw"
		}
		return st
	}
	_ = ufw
	st.Backend = "ufw"
	stdout, stderr, err := s.exec(ctx, "ufw", "status")
	if err != nil {
		// Distinguish "no privilege" from other failures for a useful message.
		combined := stdout + stderr
		if strings.Contains(combined, "ERROR: You need to be root") ||
			strings.Contains(stderr, "sudo: a password is required") ||
			strings.Contains(stderr, "Permission denied") {
			st.Available = true
			st.Reason = "hub 无权限执行 ufw（需以 root 运行或配置 sudo 白名单：hub ALL=(root) NOPASSWD: /usr/sbin/ufw, /usr/bin/ss）"
			return st
		}
		st.Reason = "查询 ufw 状态失败：" + firstLine(combined)
		return st
	}
	st.Available = true
	st.Active, _ = ParseUFWStatus(stdout)
	st.CanManage = true
	return st
}

// ListRules returns the current ALLOW rules enriched with the live listener
// for each numeric port and a Protected flag. Requires a manageable ufw.
func (s *Service) ListRules(ctx context.Context) ([]Rule, error) {
	stdout, stderr, err := s.exec(ctx, "ufw", "status")
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrUnavailable, firstLine(stdout+stderr))
	}
	_, rules := ParseUFWStatus(stdout)
	protected := s.protectedSet(ctx)
	listeners := s.listeners(ctx)
	for i := range rules {
		if rules[i].Port > 0 {
			if l, ok := listeners[rules[i].Port]; ok {
				rules[i].Process = l.Process
				rules[i].PID = l.PID
			}
			rules[i].Protected = protected[rules[i].Port]
		}
	}
	return rules, nil
}

// Listener returns the process currently bound to port (any proto). Used by
// the delete-confirmation UI so the operator sees what they may cut off.
func (s *Service) Listener(ctx context.Context, port int) (Listener, bool) {
	l, ok := s.listeners(ctx)[port]
	return l, ok
}

// AllowPort adds an allow-in rule for port/proto. Idempotent (ufw skips an
// existing rule). proto must be tcp or udp.
func (s *Service) AllowPort(ctx context.Context, port int, proto string) error {
	if port < 1 || port > 65535 {
		return ErrInvalidPort
	}
	if proto != "tcp" && proto != "udp" {
		return ErrInvalidProto
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	spec := strconv.Itoa(port) + "/" + proto
	stdout, stderr, err := s.exec(ctx, "ufw", "allow", spec)
	if err != nil {
		return fmt.Errorf("ufw allow %s: %s", spec, firstLine(stdout+stderr))
	}
	return nil
}

// DeletePort removes an allow rule. spec must be a simple port or port/proto
// (named profiles are rejected). The port is refused if protected.
func (s *Service) DeletePort(ctx context.Context, spec string) error {
	spec = strings.TrimSpace(spec)
	if !IsSimplePortSpec(spec) {
		return ErrNotDeletable
	}
	port, _ := parseTarget(spec)
	if port == 0 {
		return ErrNotDeletable
	}
	if s.protectedSet(ctx)[port] {
		return ErrProtectedPort
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stdout, stderr, err := s.exec(ctx, "ufw", "delete", "allow", spec)
	if err != nil {
		return fmt.Errorf("ufw delete allow %s: %s", spec, firstLine(stdout+stderr))
	}
	return nil
}

// protectedSet is the union of configured protected ports, the always-on SSH
// fallback (22), and any port a live sshd is bound to.
func (s *Service) protectedSet(ctx context.Context) map[int]bool {
	set := map[int]bool{22: true}
	for _, p := range s.protected {
		if p > 0 {
			set[p] = true
		}
	}
	for port, l := range s.listeners(ctx) {
		if strings.Contains(l.Process, "sshd") {
			set[port] = true
		}
	}
	return set
}

// listeners reads tcp + udp listeners via ss and merges them into one map.
// Failures degrade to an empty map (the feature still works, just without
// process annotations).
func (s *Service) listeners(ctx context.Context) map[int]Listener {
	out := make(map[int]Listener)
	for _, flag := range []string{"-ltnpH", "-lunpH"} {
		stdout, _, err := s.exec(ctx, "ss", flag)
		if err != nil {
			continue
		}
		for port, l := range ParseSSListeners(stdout) {
			if _, dup := out[port]; !dup {
				out[port] = l
			}
		}
	}
	return out
}

// ProtectedPorts returns the sorted protected port list (for display/tests).
func (s *Service) ProtectedPorts(ctx context.Context) []int {
	set := s.protectedSet(ctx)
	ports := make([]int, 0, len(set))
	for p := range set {
		ports = append(ports, p)
	}
	sort.Ints(ports)
	return ports
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	if s == "" {
		return "unknown error"
	}
	return s
}
