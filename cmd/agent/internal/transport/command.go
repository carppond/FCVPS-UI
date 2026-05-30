package transport

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"shiguang-vps/pkg/agentlib"
)

// CommandHandler dispatches inbound cmd payloads from the hub. v1 ships a
// single concrete implementation (DefaultCommandHandler); the interface lives
// here so tests can substitute their own.
type CommandHandler interface {
	Handle(ctx context.Context, cmd agentlib.CmdPayload) error
}

// DefaultCommandHandler is wired in by Client.dispatchInbound for every
// inbound cmd frame.
type DefaultCommandHandler struct {
	client *Client
}

// NewCommandHandler builds the default dispatcher.
func NewCommandHandler(c *Client) CommandHandler { return &DefaultCommandHandler{client: c} }

// Handle implements CommandHandler.
//
// v1 commands:
//
//   - refresh_subscription: agent does not own subscription state in v1
//     (subscriptions live in the hub); we log + ack OK so the hub's
//     book-keeping stays consistent.
//   - collect_now: trigger an immediate aggregator + emit metrics out of
//     band of the heartbeat tick.
//   - restart: not implemented in v1 (deferred to process supervisor) —
//     we log + ack with a clear error so the hub admin UI surfaces it.
//
// Unknown cmds return an error so cmd_ack reports a failure.
func (h *DefaultCommandHandler) Handle(ctx context.Context, cmd agentlib.CmdPayload) error {
	switch cmd.Cmd {
	case agentlib.CmdRefreshSubscription:
		h.client.cfg.Logger.Info("agent cmd: refresh_subscription received (no-op in v1)",
			slog.Any("args", cmd.Args))
		return nil
	case agentlib.CmdCollectNow:
		return h.collectNow(ctx)
	case agentlib.CmdUninstall:
		return h.uninstall()
	case agentlib.CmdRestart:
		h.client.cfg.Logger.Warn("agent cmd: restart received; defer to process supervisor")
		return fmt.Errorf("restart not implemented in v1")
	case agentlib.CmdShutdown:
		h.client.cfg.Logger.Warn("agent cmd: shutdown received; defer to process supervisor")
		return fmt.Errorf("shutdown not implemented in v1")
	default:
		return fmt.Errorf("unknown cmd %q", cmd.Cmd)
	}
}

// uninstall spawns a detached process that stops the systemd service and
// removes the unit + binary, then kills this agent. The agent IS the service,
// so it cannot stop itself inline (systemctl stop would kill the running
// command); the detached uninstaller (new session) survives that and tears the
// agent down. Best-effort — the ack reports any spawn failure to the hub.
func (h *DefaultCommandHandler) uninstall() error {
	h.client.cfg.Logger.Warn("agent cmd: uninstall received; self-removing")
	if err := spawnDetachedUninstall(os.Getpid()); err != nil {
		return fmt.Errorf("uninstall: %w", err)
	}
	return nil
}

// collectNow drives one immediate metrics frame.
func (h *DefaultCommandHandler) collectNow(ctx context.Context) error {
	payload, err := h.client.cfg.Aggregator.Collect(ctx)
	if err != nil {
		return fmt.Errorf("collect_now: %w", err)
	}
	if err := h.client.enqueue(agentlib.MsgMetrics, newMsgID("m-now"), payload); err != nil {
		return fmt.Errorf("collect_now: enqueue: %w", err)
	}
	return nil
}
