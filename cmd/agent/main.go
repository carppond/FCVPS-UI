// Package main is the entry point for the shiguang-vps agent.
package main

import (
	"fmt"
	"log/slog"
	"os"

	"shiguang-vps/pkg/agentlib"
)

func main() {
	slog.New(slog.NewJSONHandler(os.Stdout, nil)).Info("shiguang-vps agent starting",
		"protocol_version", agentlib.ProtocolVersion,
	)
	fmt.Printf("shiguang-vps agent starting (protocol %s)\n", agentlib.ProtocolVersion)
	// TODO(T-10): connect to hub via WebSocket and start metric collection loop.
	os.Exit(0)
}
