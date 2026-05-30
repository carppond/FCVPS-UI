package transport

import (
	"context"
	"testing"

	"shiguang-vps/pkg/agentlib"
)

// TestCommandHandler_Uninstall verifies CmdUninstall dispatches to the detached
// uninstaller. spawnDetachedUninstall is stubbed — the real one rm's the binary
// and kills the PID, which must never run inside a test.
func TestCommandHandler_Uninstall(t *testing.T) {
	orig := spawnDetachedUninstall
	t.Cleanup(func() { spawnDetachedUninstall = orig })
	called := 0
	spawnDetachedUninstall = func(pid int) error { called++; return nil }

	h := &DefaultCommandHandler{client: &Client{cfg: Config{Logger: silentLogger()}}}
	if err := h.Handle(context.Background(), agentlib.CmdPayload{Cmd: agentlib.CmdUninstall}); err != nil {
		t.Fatalf("uninstall handle: %v", err)
	}
	if called != 1 {
		t.Fatalf("expected spawnDetachedUninstall called once, got %d", called)
	}
}

func TestCommandHandler_UnknownCmd(t *testing.T) {
	h := &DefaultCommandHandler{client: &Client{cfg: Config{Logger: silentLogger()}}}
	if err := h.Handle(context.Background(), agentlib.CmdPayload{Cmd: "bogus"}); err == nil {
		t.Fatal("expected error for unknown cmd")
	}
}
