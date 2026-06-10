// Package sshrelay opens an interactive SSH session (PTY + shell) to a target
// host and exposes it as an io.ReadWriteCloser plus a Resize hook. The hub's
// WebSocket handler bridges a browser/mobile xterm to this session, so SSH
// credentials never leave the server and both clients share one code path.
//
// This package deliberately does NOT import any WebSocket library: the
// transport is the caller's concern. That keeps the SSH wiring unit-testable
// against an in-process ssh.Server (see relay_test.go).
package sshrelay

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"time"

	"golang.org/x/crypto/ssh"
)

// Defaults for dial / window sizing.
const (
	DefaultDialTimeout = 15 * time.Second
	defaultCols        = 80
	defaultRows        = 24
	// termType is what we advertise to the remote; xterm-256color matches the
	// xterm.js frontend so colour + key handling line up.
	termType = "xterm-256color"
)

// ErrNoCredentials is returned when the target has neither a password nor a
// private key configured.
var ErrNoCredentials = errors.New("sshrelay: target has no password or private key")

// Target describes the host to connect to. Credentials are read server-side
// from the VPS asset record; exactly one of Password / PrivateKey is required
// (PrivateKey wins when both are set).
type Target struct {
	Host       string
	Port       int
	User       string
	Password   string
	PrivateKey string
	// Passphrase decrypts PrivateKey when it is encrypted; empty otherwise.
	Passphrase string
}

// Config tunes the relay. All fields are optional.
type Config struct {
	DialTimeout time.Duration
	Logger      *slog.Logger
}

// Session is one live SSH shell. Read pulls PTY output, Write pushes keystrokes
// to the remote stdin, Resize updates the window, and Close tears everything
// down. It is safe to call Resize / Write / Close from a goroutine other than
// the one calling Read (gorilla-style read/write split).
type Session struct {
	client  *ssh.Client
	session *ssh.Session
	stdin   io.WriteCloser
	stdout  io.Reader
}

// Dial connects to target, authenticates, requests a PTY + interactive shell,
// and returns a ready Session. The caller owns Close.
//
// HostKeyCallback is InsecureIgnoreHostKey: this tool connects to user-owned
// VPS assets addressed by IP (often freshly provisioned, no known_hosts entry),
// matching the prior mobile direct-connect behaviour. Host-key TOFU is a
// future hardening (store the fingerprint on the asset on first connect).
func Dial(ctx context.Context, target Target, cfg Config) (*Session, error) {
	if cfg.DialTimeout <= 0 {
		cfg.DialTimeout = DefaultDialTimeout
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if target.Port == 0 {
		target.Port = 22
	}
	auth, err := authMethods(target)
	if err != nil {
		return nil, err
	}

	clientCfg := &ssh.ClientConfig{
		User:            target.User,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // see doc comment
		Timeout:         cfg.DialTimeout,
	}

	addr := net.JoinHostPort(target.Host, strconv.Itoa(target.Port))
	// Dial through a context-aware net.Dialer so a slow/blocked host honours
	// the caller's cancellation instead of hanging for the full Timeout.
	d := net.Dialer{Timeout: cfg.DialTimeout}
	netConn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("sshrelay: dial %s: %w", addr, err)
	}
	sshConn, chans, reqs, err := ssh.NewClientConn(netConn, addr, clientCfg)
	if err != nil {
		_ = netConn.Close()
		return nil, fmt.Errorf("sshrelay: handshake: %w", err)
	}
	client := ssh.NewClient(sshConn, chans, reqs)

	sess, err := client.NewSession()
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("sshrelay: new session: %w", err)
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := sess.RequestPty(termType, defaultRows, defaultCols, modes); err != nil {
		_ = sess.Close()
		_ = client.Close()
		return nil, fmt.Errorf("sshrelay: request pty: %w", err)
	}
	// With a PTY allocated the remote merges stderr into stdout, so a single
	// stdout pipe carries everything the terminal should render.
	stdin, err := sess.StdinPipe()
	if err != nil {
		_ = sess.Close()
		_ = client.Close()
		return nil, fmt.Errorf("sshrelay: stdin pipe: %w", err)
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		_ = sess.Close()
		_ = client.Close()
		return nil, fmt.Errorf("sshrelay: stdout pipe: %w", err)
	}
	if err := sess.Shell(); err != nil {
		_ = sess.Close()
		_ = client.Close()
		return nil, fmt.Errorf("sshrelay: start shell: %w", err)
	}

	return &Session{client: client, session: sess, stdin: stdin, stdout: stdout}, nil
}

// Read returns PTY output. Returns io.EOF when the remote shell exits.
func (s *Session) Read(p []byte) (int, error) { return s.stdout.Read(p) }

// Write forwards keystrokes to the remote shell's stdin.
func (s *Session) Write(p []byte) (int, error) { return s.stdin.Write(p) }

// Resize updates the remote PTY window. Out-of-range values are clamped to a
// sane minimum so a transient 0×0 from the client never wedges the terminal.
func (s *Session) Resize(cols, rows int) error {
	if cols <= 0 {
		cols = defaultCols
	}
	if rows <= 0 {
		rows = defaultRows
	}
	return s.session.WindowChange(rows, cols)
}

// Close tears down the shell and the underlying connection. Idempotent enough
// for defer + an explicit call.
func (s *Session) Close() error {
	if s.session != nil {
		_ = s.session.Close()
	}
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}

// authMethods builds the ssh auth chain from the target's credentials.
func authMethods(t Target) ([]ssh.AuthMethod, error) {
	if t.PrivateKey != "" {
		var signer ssh.Signer
		var err error
		if t.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(t.PrivateKey), []byte(t.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey([]byte(t.PrivateKey))
		}
		if err != nil {
			return nil, fmt.Errorf("sshrelay: parse private key: %w", err)
		}
		return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil
	}
	if t.Password != "" {
		return []ssh.AuthMethod{ssh.Password(t.Password)}, nil
	}
	return nil, ErrNoCredentials
}
