package notify

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"shiguang-vps/internal/storage"
	"shiguang-vps/pkg/agentlib"
)

// formatBytes renders a byte count in human-readable units. We avoid the
// CJK-specific "兆字节" string and stick to neutral suffixes so the same
// helper is usable across the four supported locales.
func formatBytes(n int64) string {
	const (
		kib = 1024
		mib = kib * 1024
		gib = mib * 1024
		tib = gib * 1024
	)
	switch {
	case n >= tib:
		return fmt.Sprintf("%.2f TiB", float64(n)/float64(tib))
	case n >= gib:
		return fmt.Sprintf("%.2f GiB", float64(n)/float64(gib))
	case n >= mib:
		return fmt.Sprintf("%.2f MiB", float64(n)/float64(mib))
	case n >= kib:
		return fmt.Sprintf("%.2f KiB", float64(n)/float64(kib))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

// ---------------------------------------------------------------------------
// Repos surface — every command depends on a narrow slice of the storage
// layer. Defining interfaces keeps the bot tests free of SQLite setup.
// ---------------------------------------------------------------------------

// TGNodeRepo is the slice of NodeRepo the /nodes handler reaches into.
type TGNodeRepo interface {
	ListByUser(ctx context.Context, userID string, opts storage.NodeListOptions) ([]storage.NodeRecord, int64, error)
}

// TGSubscriptionRepo is the slice the /refresh handler reaches into.
type TGSubscriptionRepo interface {
	GetByID(ctx context.Context, id, userID string) (*storage.SubscriptionRecord, error)
	List(ctx context.Context, userID string, opts storage.SubscriptionListOptions) ([]storage.SubscriptionRecord, int64, error)
}

// TGAgentRepo is the slice used by /agent_restart.
type TGAgentRepo interface {
	GetByID(ctx context.Context, id, userID string) (*storage.AgentRecord, error)
}

// TGTrafficRepo is the slice used by /traffic.
type TGTrafficRepo interface {
	GetMonthSummary(ctx context.Context, userID string, year int, month time.Month, monthlyLimit int64) (*storage.TrafficSummary, error)
}

// TGChannelRepo is the slice used by /start (chat_id binding).
type TGChannelRepo interface {
	GetByID(ctx context.Context, id, userID string) (*storage.NotificationChannelRecord, error)
	Update(ctx context.Context, rec storage.NotificationChannelRecord) error
	List(ctx context.Context, userID string, opts storage.NotificationChannelListOptions) ([]storage.NotificationChannelRecord, int64, error)
}

// TGUserRepo is the slice used by /start to resolve a verification token to
// the owning user.
type TGUserRepo interface {
	GetByID(ctx context.Context, id string) (*storage.UserRecord, error)
}

// TGSettingsRepo is the slice used by /silent (Toggle the silent_mode
// setting). Reads + writes go through Get/Set.
type TGSettingsRepo interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
}

// TGSubSyncer triggers a one-shot sync of a single subscription. The
// substore.SyncService satisfies this interface; we abstract it so tests do
// not need to pull in the full sync pipeline.
type TGSubSyncer interface {
	SyncOne(ctx context.Context, sub *storage.SubscriptionRecord) (any, error)
}

// TGAgentHub is the slice used by /agent_restart to push the restart cmd
// over the live WS connection.
type TGAgentHub interface {
	IsOnline(agentID string) bool
	SendCommand(ctx context.Context, agentID, cmdID string, payload agentlib.CmdPayload) error
}

// CommandsConfig wires the per-command business logic onto the bot.
//
// Every dependency is optional — when nil the corresponding command replies
// with "service unavailable" so the bot stays useful even in partial-config
// deployments. This mirrors the rest of the project's nil-safety policy
// (see handler/router.go Deps comments).
type CommandsConfig struct {
	Nodes         TGNodeRepo
	Subscriptions TGSubscriptionRepo
	Agents        TGAgentRepo
	Traffic       TGTrafficRepo
	Channels      TGChannelRepo
	Users         TGUserRepo
	Settings      TGSettingsRepo
	Hub           TGAgentHub
	Syncer        TGSubSyncer

	// AdminCheck reports whether the resolved user is an admin. Used to
	// gate /agent_restart + /silent. nil treats every user as non-admin.
	AdminCheck func(ctx context.Context, userID string) bool

	// Now returns the current time. nil defaults to time.Now.
	Now func() time.Time
}

// RegisterCommands wires every command into router using the deps in cfg.
// The router gains 6 handlers: /start /help /nodes /refresh /agent_restart
// /traffic /silent.
//
// The function is idempotent — re-invocation overwrites prior handlers,
// which is what tests rely on for fixture rebuilds.
func RegisterCommands(router *CommandRouter, cfg CommandsConfig) {
	if router == nil {
		return
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	router.RegisterCommand("start", cfg.cmdStart)
	router.RegisterCommand("help", cfg.cmdHelp)
	router.RegisterCommand("nodes", cfg.cmdNodes)
	router.RegisterCommand("refresh", cfg.cmdRefresh)
	router.RegisterCommand("agent_restart", cfg.cmdAgentRestart)
	router.RegisterCommand("traffic", cfg.cmdTraffic)
	router.RegisterCommand("silent", cfg.cmdSilent)
}

// ---------------------------------------------------------------------------
// /start <token>
// ---------------------------------------------------------------------------
//
// The token is a per-channel verification string — typically the channel ID
// itself (we keep the token equal to the channel.id so the operator only has
// to copy a value already visible on the settings page). When the token
// resolves to a telegram channel and the chat_id is not yet bound, we add it
// to the channel.config.chat_ids array and persist.
//
// On success the bot replies with a short confirmation; on failure the user
// sees a generic "invalid token" message so a brute-force scan cannot
// enumerate channel IDs.

func (c CommandsConfig) cmdStart(ctx context.Context, chat *TGChatContext, args string) (string, *TGInlineKeyboard, error) {
	if c.Channels == nil || c.Users == nil {
		return "Telegram bot is not fully configured.", nil, nil
	}
	token := strings.TrimSpace(args)
	if token == "" {
		return "Usage: `/start <token>`\nGet the token from the web UI: Settings → Notifications → Telegram.", nil, nil
	}
	// The token format is "<userID>:<channelID>". We split, look up both,
	// and bind on success. Backwards-compat: a plain channelID is accepted
	// when chat is already authorised (rebind flow).
	var userID, channelID string
	if i := strings.IndexByte(token, ':'); i >= 0 {
		userID = token[:i]
		channelID = token[i+1:]
	} else {
		channelID = token
		userID = chat.UserID
	}
	if userID == "" || channelID == "" {
		return "Invalid token.", nil, nil
	}
	user, err := c.Users.GetByID(ctx, userID)
	if err != nil || user == nil {
		return "Invalid token.", nil, nil
	}
	rec, err := c.Channels.GetByID(ctx, channelID, userID)
	if err != nil || rec == nil || rec.Kind != "telegram" {
		return "Invalid token.", nil, nil
	}
	// Append chat_id to config.chat_ids (deduping). The Telegram channel
	// stores a JSON object; we operate on a decoded map and re-encode.
	cfg, err := decodeChannelConfig(rec.ConfigJSON)
	if err != nil {
		return "Channel config could not be parsed.", nil, nil
	}
	if !appendChatID(cfg, chat.ChatID) {
		return fmt.Sprintf("Chat `%d` is already bound.", chat.ChatID), nil, nil
	}
	newJSON, err := encodeChannelConfig(cfg)
	if err != nil {
		return "Failed to encode channel config.", nil, nil
	}
	upd := *rec
	upd.ConfigJSON = newJSON
	if err := c.Channels.Update(ctx, upd); err != nil {
		return "Failed to persist binding.", nil, nil
	}
	return fmt.Sprintf(
		"✅ Chat `%d` bound to channel `%s`\\.\nUser: `%s`",
		chat.ChatID,
		tgEscapeMarkdownV2(rec.Name),
		tgEscapeMarkdownV2(user.Username),
	), nil, nil
}

// ---------------------------------------------------------------------------
// /help
// ---------------------------------------------------------------------------

func (c CommandsConfig) cmdHelp(ctx context.Context, chat *TGChatContext, _ string) (string, *TGInlineKeyboard, error) {
	lines := []string{
		"*shiguang\\-vps Bot*",
		"",
		"`/start <token>` — bind this chat to a notification channel",
		"`/nodes [subID]` — list nodes \\(optionally filtered by subscription\\)",
		"`/refresh <subID>` — trigger a subscription sync",
		"`/agent_restart <agentID>` — restart an agent \\(admin only\\)",
		"`/traffic` — show this month's traffic usage",
		"`/silent on|off` — toggle silent mode \\(admin only\\)",
		"`/help` — show this help",
	}
	return strings.Join(lines, "\n"), nil, nil
}

// ---------------------------------------------------------------------------
// /nodes [subID]
// ---------------------------------------------------------------------------

func (c CommandsConfig) cmdNodes(ctx context.Context, chat *TGChatContext, args string) (string, *TGInlineKeyboard, error) {
	if c.Nodes == nil {
		return "Node service unavailable.", nil, nil
	}
	subID := strings.TrimSpace(args)
	opts := storage.NodeListOptions{Page: 1, PageSize: 20, Sort: "latency_asc"}
	if subID != "" {
		opts.SubscriptionID = subID
	}
	recs, total, err := c.Nodes.ListByUser(ctx, chat.UserID, opts)
	if err != nil {
		return "", nil, err
	}
	if len(recs) == 0 {
		return "No nodes found\\.", nil, nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "*Nodes* \\(showing %d of %d\\)\n", len(recs), total)
	for _, n := range recs {
		latency := "—"
		if n.LastLatencyMs != nil {
			if *n.LastLatencyMs < 0 {
				latency = "✗"
			} else {
				latency = fmt.Sprintf("%dms", *n.LastLatencyMs)
			}
		}
		fmt.Fprintf(&b, "• `%s` _\\[%s\\]_ — %s — `%s:%d`\n",
			tgEscapeMarkdownV2(truncate(n.Tag, 32)),
			tgEscapeMarkdownV2(n.Protocol),
			tgEscapeMarkdownV2(latency),
			tgEscapeMarkdownV2(n.Server),
			n.Port,
		)
	}
	return b.String(), nil, nil
}

// ---------------------------------------------------------------------------
// /refresh <subID>
// ---------------------------------------------------------------------------

func (c CommandsConfig) cmdRefresh(ctx context.Context, chat *TGChatContext, args string) (string, *TGInlineKeyboard, error) {
	if c.Subscriptions == nil {
		return "Subscription service unavailable.", nil, nil
	}
	subID := strings.TrimSpace(args)
	if subID == "" {
		return "Usage: `/refresh <subID>`", nil, nil
	}
	sub, err := c.Subscriptions.GetByID(ctx, subID, chat.UserID)
	if err != nil {
		if errors.Is(err, storage.ErrSubscriptionNotFound) {
			return "Subscription not found\\.", nil, nil
		}
		return "", nil, err
	}
	if c.Syncer == nil {
		return "Sync service unavailable\\.", nil, nil
	}
	if _, err := c.Syncer.SyncOne(ctx, sub); err != nil {
		return fmt.Sprintf("Refresh failed: `%s`", tgEscapeMarkdownV2(truncate(err.Error(), 120))), nil, nil
	}
	return fmt.Sprintf("✅ Refresh queued for `%s`\\.", tgEscapeMarkdownV2(sub.Name)), nil, nil
}

// ---------------------------------------------------------------------------
// /agent_restart <agentID>
// ---------------------------------------------------------------------------

func (c CommandsConfig) cmdAgentRestart(ctx context.Context, chat *TGChatContext, args string) (string, *TGInlineKeyboard, error) {
	if c.AdminCheck == nil || !c.AdminCheck(ctx, chat.UserID) {
		return "Admin only\\.", nil, nil
	}
	if c.Agents == nil || c.Hub == nil {
		return "Agent service unavailable\\.", nil, nil
	}
	agentID := strings.TrimSpace(args)
	if agentID == "" {
		return "Usage: `/agent_restart <agentID>`", nil, nil
	}
	rec, err := c.Agents.GetByID(ctx, agentID, chat.UserID)
	if err != nil {
		if errors.Is(err, storage.ErrAgentNotFound) {
			return "Agent not found\\.", nil, nil
		}
		return "", nil, err
	}
	if !c.Hub.IsOnline(rec.ID) {
		return fmt.Sprintf("Agent `%s` is offline\\.", tgEscapeMarkdownV2(rec.Name)), nil, nil
	}
	cmdID := fmt.Sprintf("tg-%d", c.Now().UnixMilli())
	if err := c.Hub.SendCommand(ctx, rec.ID, cmdID, agentlib.CmdPayload{Cmd: agentlib.CmdRestart}); err != nil {
		return fmt.Sprintf("Send failed: `%s`", tgEscapeMarkdownV2(truncate(err.Error(), 120))), nil, nil
	}
	return fmt.Sprintf("✅ Restart dispatched to `%s`\\.", tgEscapeMarkdownV2(rec.Name)), nil, nil
}

// ---------------------------------------------------------------------------
// /traffic
// ---------------------------------------------------------------------------

func (c CommandsConfig) cmdTraffic(ctx context.Context, chat *TGChatContext, _ string) (string, *TGInlineKeyboard, error) {
	if c.Traffic == nil {
		return "Traffic service unavailable\\.", nil, nil
	}
	now := c.Now().UTC()
	summary, err := c.Traffic.GetMonthSummary(ctx, chat.UserID, now.Year(), now.Month(), 0)
	if err != nil {
		return "", nil, err
	}
	pct := "—"
	if summary.TotalLimit > 0 {
		pct = fmt.Sprintf("%.1f%%", float64(summary.TotalUsed)/float64(summary.TotalLimit)*100)
	}
	limit := "—"
	if summary.TotalLimit > 0 {
		limit = formatBytes(summary.TotalLimit)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "*Traffic* `%s` → `%s`\n",
		tgEscapeMarkdownV2(summary.PeriodStart), tgEscapeMarkdownV2(summary.PeriodEnd))
	fmt.Fprintf(&b, "• Used: `%s` \\(%s of `%s`\\)\n",
		tgEscapeMarkdownV2(formatBytes(summary.TotalUsed)),
		tgEscapeMarkdownV2(pct),
		tgEscapeMarkdownV2(limit),
	)
	fmt.Fprintf(&b, "• In: `%s`\n", tgEscapeMarkdownV2(formatBytes(summary.TotalIn)))
	fmt.Fprintf(&b, "• Out: `%s`\n", tgEscapeMarkdownV2(formatBytes(summary.TotalOut)))
	if len(summary.Agents) > 0 {
		// Stable order by usage desc for the per-agent list.
		agents := append([]storage.TrafficAgentBreakdown(nil), summary.Agents...)
		sort.Slice(agents, func(i, j int) bool { return agents[i].TotalUsed > agents[j].TotalUsed })
		fmt.Fprintf(&b, "\n*Per agent:*\n")
		for i, a := range agents {
			if i >= 10 {
				break
			}
			label := a.AgentID
			if label == "" {
				label = "—"
			}
			fmt.Fprintf(&b, "• `%s` — `%s`\n",
				tgEscapeMarkdownV2(truncate(label, 32)),
				tgEscapeMarkdownV2(formatBytes(a.TotalUsed)),
			)
		}
	}
	return b.String(), nil, nil
}

// ---------------------------------------------------------------------------
// /silent on|off
// ---------------------------------------------------------------------------

func (c CommandsConfig) cmdSilent(ctx context.Context, chat *TGChatContext, args string) (string, *TGInlineKeyboard, error) {
	if c.AdminCheck == nil || !c.AdminCheck(ctx, chat.UserID) {
		return "Admin only\\.", nil, nil
	}
	if c.Settings == nil {
		return "Settings service unavailable\\.", nil, nil
	}
	arg := strings.ToLower(strings.TrimSpace(args))
	switch arg {
	case "":
		cur, _ := c.Settings.Get(ctx, storage.SettingSilentModeEnabled)
		state := "off"
		if cur == "1" || strings.EqualFold(cur, "true") {
			state = "on"
		}
		return fmt.Sprintf("Silent mode is `%s`\\. Use `/silent on` or `/silent off`\\.", state), nil, nil
	case "on":
		if err := c.Settings.Set(ctx, storage.SettingSilentModeEnabled, "1"); err != nil {
			return "", nil, err
		}
		return "✅ Silent mode enabled\\.", nil, nil
	case "off":
		if err := c.Settings.Set(ctx, storage.SettingSilentModeEnabled, "0"); err != nil {
			return "", nil, err
		}
		return "✅ Silent mode disabled\\.", nil, nil
	default:
		return "Usage: `/silent on|off`", nil, nil
	}
}
