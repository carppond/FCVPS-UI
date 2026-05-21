package transport

import (
	"context"
	"log/slog"
	"time"

	"github.com/gorilla/websocket"

	"shiguang-vps/pkg/agentlib"
)

// runReadPump consumes inbound frames until the connection is closed or ctx
// is cancelled. It enforces the idle deadline via SetReadDeadline + the
// pong handler so any traffic keeps the socket alive.
//
// Unknown / unparseable frames are logged at debug and discarded; the
// transport never crashes on a malformed hub message.
func (c *Client) runReadPump(ctx context.Context, conn *websocket.Conn) {
	idleTimeout := 3 * c.HeartbeatInterval()
	resetDeadline := func() {
		_ = conn.SetReadDeadline(c.cfg.Now().Add(idleTimeout))
	}
	resetDeadline()
	conn.SetPongHandler(func(string) error { resetDeadline(); return nil })

	for {
		if err := ctx.Err(); err != nil {
			return
		}
		_, data, err := conn.ReadMessage()
		if err != nil {
			c.cfg.Logger.Info("agent transport: read pump terminated",
				slog.String("err", err.Error()))
			return
		}
		resetDeadline()
		env, err := agentlib.UnmarshalEnvelope(data)
		if err != nil {
			c.cfg.Logger.Warn("agent transport: invalid envelope",
				slog.String("err", err.Error()))
			continue
		}
		c.dispatchInbound(ctx, env, conn)
	}
}

// dispatchInbound routes one inbound envelope. Only cmd + bye need handling
// in v1; hello_ack is consumed by Connect() and should never re-appear.
func (c *Client) dispatchInbound(ctx context.Context, env *agentlib.Envelope, conn *websocket.Conn) {
	switch env.Type {
	case agentlib.MsgCmd:
		cmd, err := agentlib.UnmarshalCmd(env.Payload)
		if err != nil {
			c.cfg.Logger.Warn("agent transport: cmd parse",
				slog.String("err", err.Error()))
			_ = c.ackCmd(env.ID, false, "invalid cmd payload")
			return
		}
		handler := NewCommandHandler(c)
		if err := handler.Handle(ctx, *cmd); err != nil {
			c.cfg.Logger.Warn("agent transport: cmd handler",
				slog.String("cmd", string(cmd.Cmd)),
				slog.String("err", err.Error()))
			_ = c.ackCmd(env.ID, false, err.Error())
			return
		}
		_ = c.ackCmd(env.ID, true, "")
	case agentlib.MsgBye:
		c.handleBye(env, conn)
	case agentlib.MsgHelloAck:
		// Late ack — ignore; Connect already consumed the negotiated one.
		c.cfg.Logger.Debug("agent transport: duplicate hello_ack ignored")
	default:
		c.cfg.Logger.Debug("agent transport: unknown inbound type",
			slog.String("type", string(env.Type)))
	}
}

// runWritePump drains the send channel + emits keepalive pings at half the
// heartbeat cadence. Exits when send is closed, ctx is cancelled, or the
// connection errors.
func (c *Client) runWritePump(ctx context.Context, conn *websocket.Conn, send chan []byte) {
	pingPeriod := c.HeartbeatInterval() / 2
	if pingPeriod <= 0 {
		pingPeriod = 15 * time.Second
	}
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-send:
			if !ok {
				return
			}
			_ = conn.SetWriteDeadline(c.cfg.Now().Add(c.cfg.WriteTimeout))
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				c.cfg.Logger.Info("agent transport: write pump terminated",
					slog.String("err", err.Error()))
				return
			}
		case <-ticker.C:
			deadline := c.cfg.Now().Add(c.cfg.WriteTimeout)
			if err := conn.WriteControl(websocket.PingMessage, nil, deadline); err != nil {
				c.cfg.Logger.Debug("agent transport: ping failed",
					slog.String("err", err.Error()))
				return
			}
		}
	}
}

// ackCmd is the standard cmd → cmd_ack reply.
func (c *Client) ackCmd(cmdID string, ok bool, errMsg string) error {
	return c.enqueue(agentlib.MsgCmdAck, newMsgID("ack"), agentlib.CmdAckPayload{
		CmdID: cmdID,
		OK:    ok,
		Error: errMsg,
	})
}
