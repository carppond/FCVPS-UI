// Package nezha_emitter is reserved for the "reverse compatibility" path
// where this agent emits metrics to a Nezha-protocol hub instead of the
// native shiguang hub.
//
// v1 status: placeholder only. The forward direction (Nezha agent → shiguang
// hub) is owned by T-17 / internal/nezha. The reverse direction (shiguang
// agent → Nezha hub) is rarely useful in practice and is parked for P2.
//
// When this gets wired, the package will expose:
//
//   Emit(ctx context.Context, cfg Config, payload *agentlib.MetricsPayload) error
//
// which translates the protocol-level metrics struct into Nezha v2
// gRPC/HTTP payloads. See docs/03-architecture.md §3.8 for the contract.
package nezha_emitter
