package sshrelay

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// newEchoSSHServer starts an in-process SSH server on 127.0.0.1 that accepts
// password auth for (user, pass), grants a PTY + shell, and echoes stdin back
// to stdout. Returns host, port and a cleanup func.
func newEchoSSHServer(t *testing.T, user, pass string) (string, int) {
	t.Helper()

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("gen host key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	cfg := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, p []byte) (*ssh.Permissions, error) {
			if c.User() == user && string(p) == pass {
				return nil, nil
			}
			return nil, errors.New("denied")
		},
	}
	cfg.AddHostKey(signer)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			nConn, err := ln.Accept()
			if err != nil {
				return
			}
			go serveEcho(nConn, cfg)
		}
	}()

	host, portStr, _ := net.SplitHostPort(ln.Addr().String())
	port, _ := strconv.Atoi(portStr)
	return host, port
}

func serveEcho(nConn net.Conn, cfg *ssh.ServerConfig) {
	conn, chans, reqs, err := ssh.NewServerConn(nConn, cfg)
	if err != nil {
		return
	}
	defer conn.Close()
	go ssh.DiscardRequests(reqs)
	for newCh := range chans {
		if newCh.ChannelType() != "session" {
			_ = newCh.Reject(ssh.UnknownChannelType, "only session")
			continue
		}
		ch, chReqs, err := newCh.Accept()
		if err != nil {
			return
		}
		go func() {
			for req := range chReqs {
				switch req.Type {
				case "pty-req", "shell", "window-change":
					_ = req.Reply(true, nil)
				default:
					_ = req.Reply(false, nil)
				}
			}
		}()
		go func() {
			_, _ = io.Copy(ch, ch) // echo stdin → stdout
			_ = ch.Close()
		}()
	}
}

func TestDialEchoRoundTrip(t *testing.T) {
	host, port := newEchoSSHServer(t, "alice", "s3cret")
	sess, err := Dial(context.Background(), Target{
		Host: host, Port: port, User: "alice", Password: "s3cret",
	}, Config{DialTimeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer sess.Close()

	if _, err := sess.Write([]byte("ping\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	// Resize must not error against a live PTY.
	if err := sess.Resize(120, 40); err != nil {
		t.Errorf("Resize: %v", err)
	}

	// The echo server replies immediately, so this blocking Read returns
	// promptly; a misbehaving server would trip the test runner's timeout.
	buf := make([]byte, 64)
	n, err := sess.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("Read: %v", err)
	}
	if !strings.Contains(string(buf[:n]), "ping") {
		t.Errorf("echo not received, got %q", string(buf[:n]))
	}
}

func TestDialWrongPasswordFails(t *testing.T) {
	host, port := newEchoSSHServer(t, "alice", "s3cret")
	_, err := Dial(context.Background(), Target{
		Host: host, Port: port, User: "alice", Password: "wrong",
	}, Config{DialTimeout: 5 * time.Second})
	if err == nil {
		t.Fatal("expected auth failure")
	}
}

func TestAuthMethodsNoCredentials(t *testing.T) {
	if _, err := authMethods(Target{User: "x"}); !errors.Is(err, ErrNoCredentials) {
		t.Fatalf("want ErrNoCredentials, got %v", err)
	}
}

func TestAuthMethodsBadKey(t *testing.T) {
	if _, err := authMethods(Target{PrivateKey: "not a key"}); err == nil {
		t.Fatal("expected parse error for malformed key")
	}
}

func TestAuthMethodsPasswordSelected(t *testing.T) {
	m, err := authMethods(Target{Password: "p"})
	if err != nil || len(m) != 1 {
		t.Fatalf("password auth: got %v err %v", m, err)
	}
}
