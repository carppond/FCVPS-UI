// Package main is the entry point for the shiguang-vps agent.
//
// Startup sequence (Tech Lead plan T-15):
//
//  1. Parse CLI flags (override-able by environment variables).
//  2. Build the slog logger (stderr by default — stdout is reserved for
//     scripted operators that pipe metrics into other tools).
//  3. Build the transport.Client.
//  4. ConnectWithBackoff (1s → 2s → 4s → … 60s ceiling).
//  5. Run the message pumps + heartbeat loop.
//  6. Trap SIGINT / SIGTERM → graceful close (sends bye to hub).
//
// On bye{version_unsupported} from the hub the agent exits with status 2
// so an operator's process supervisor (systemd, docker) surfaces the
// upgrade requirement instead of silently restart-looping.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"shiguang-vps/cmd/agent/internal/transport"
	"shiguang-vps/pkg/agentlib"
)

// Exit codes — kept distinct so supervisors can tell why the agent died.
const (
	exitOK                = 0
	exitConfig            = 1
	exitVersionUnsupported = 2
	exitRuntime           = 3
)

func main() {
	os.Exit(run())
}

// run is the testable entry point — main only handles process exit codes.
func run() int {
	cfg, err := parseFlags(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "agent: %v\n", err)
		return exitConfig
	}
	log := buildLogger(cfg.LogLevel, cfg.LogFormat)
	log.Info("shiguang-vps agent starting",
		slog.String("protocol_version", agentlib.ProtocolVersion),
		slog.String("agent_id", cfg.AgentID),
		slog.String("hub_url", cfg.HubURL),
		slog.Duration("interval", cfg.Interval),
	)

	client, err := transport.NewClient(transport.Config{
		HubURL:            cfg.HubURL,
		Token:             cfg.Token,
		AgentID:           cfg.AgentID,
		Version:           agentlib.ProtocolVersion,
		Tags:              cfg.Tags,
		HeartbeatInterval: cfg.Interval,
		Logger:            log,
	})
	if err != nil {
		log.Error("agent: build client failed", slog.String("err", err.Error()))
		return exitConfig
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// SIGINT / SIGTERM → graceful shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Info("agent: shutdown signal received", slog.String("signal", sig.String()))
		_ = client.Close()
		cancel()
	}()

	for {
		if err := client.ConnectWithBackoff(ctx); err != nil {
			if errors.Is(err, transport.ErrStopReconnect) {
				log.Error("agent: hub rejected agent; exiting (need upgrade?)")
				return exitVersionUnsupported
			}
			if errors.Is(err, context.Canceled) {
				return exitOK
			}
			log.Error("agent: connect failed", slog.String("err", err.Error()))
			return exitRuntime
		}
		if err := client.Run(ctx); err != nil {
			log.Warn("agent: session ended", slog.String("err", err.Error()))
		}
		if ctx.Err() != nil {
			return exitOK
		}
		// Reconnect on next iteration. The backoff state inside the client
		// resets on success — first reconnect attempt fires immediately
		// then escalates again on failure.
	}
}

// agentConfig is the parsed CLI / environment configuration.
type agentConfig struct {
	HubURL    string
	Token     string
	AgentID   string
	Tags      []string
	Interval  time.Duration
	LogLevel  string
	LogFormat string
}

// parseFlags resolves the precedence flag > env > default. Returns an error
// when a required field is still empty after both sources are consulted.
func parseFlags(args []string) (agentConfig, error) {
	fs := flag.NewFlagSet("agent", flag.ContinueOnError)
	hubURL := fs.String("hub-url", "", "hub WebSocket URL, e.g. wss://hub.example.com/api/agent/ws (env: SHIGUANG_HUB_URL)")
	token := fs.String("token", "", "agent token (env: SHIGUANG_AGENT_TOKEN)")
	agentID := fs.String("agent-id", "", "agent UUID (env: SHIGUANG_AGENT_ID)")
	tags := fs.String("tags", "", "comma-separated tags (env: SHIGUANG_AGENT_TAGS)")
	interval := fs.Duration("interval", 30*time.Second, "heartbeat / metrics interval; hub may override (env: SHIGUANG_AGENT_INTERVAL)")
	logLevel := fs.String("log-level", "info", "log level: debug|info|warn|error (env: SHIGUANG_LOG_LEVEL)")
	logFormat := fs.String("log-format", "text", "log format: text|json (env: SHIGUANG_LOG_FORMAT)")
	if err := fs.Parse(args); err != nil {
		return agentConfig{}, fmt.Errorf("parse flags: %w", err)
	}

	cfg := agentConfig{
		HubURL:    firstNonEmpty(*hubURL, os.Getenv("SHIGUANG_HUB_URL")),
		Token:     firstNonEmpty(*token, os.Getenv("SHIGUANG_AGENT_TOKEN")),
		AgentID:   firstNonEmpty(*agentID, os.Getenv("SHIGUANG_AGENT_ID")),
		LogLevel:  firstNonEmpty(*logLevel, os.Getenv("SHIGUANG_LOG_LEVEL")),
		LogFormat: firstNonEmpty(*logFormat, os.Getenv("SHIGUANG_LOG_FORMAT")),
		Interval:  *interval,
	}
	tagSrc := firstNonEmpty(*tags, os.Getenv("SHIGUANG_AGENT_TAGS"))
	if tagSrc != "" {
		for _, t := range strings.Split(tagSrc, ",") {
			if v := strings.TrimSpace(t); v != "" {
				cfg.Tags = append(cfg.Tags, v)
			}
		}
	}
	if cfg.HubURL == "" {
		return cfg, fmt.Errorf("hub URL is required (--hub-url or SHIGUANG_HUB_URL)")
	}
	if cfg.Token == "" {
		return cfg, fmt.Errorf("agent token is required (--token or SHIGUANG_AGENT_TOKEN)")
	}
	if cfg.AgentID == "" {
		return cfg, fmt.Errorf("agent ID is required (--agent-id or SHIGUANG_AGENT_ID)")
	}
	return cfg, nil
}

// firstNonEmpty returns the first non-empty string in the argument list.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// buildLogger returns a slog logger writing to stderr. JSON encoder is used
// when format == "json", text otherwise.
func buildLogger(level, format string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLevel(level)}
	var h slog.Handler
	switch strings.ToLower(format) {
	case "json":
		h = slog.NewJSONHandler(os.Stderr, opts)
	default:
		h = slog.NewTextHandler(os.Stderr, opts)
	}
	return slog.New(h)
}

// parseLevel maps the string to slog.Level (default info).
func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error", "err":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
