package collector

import (
	"context"
	"testing"
	"time"
)

// TestCPU is a smoke test — gopsutil sampling depends on /proc on Linux and
// sysctl on macOS, both of which are present in CI. The only assertion is
// that the returned value lies in the protocol-mandated [0, 100] range.
func TestCPU(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	v, err := CPU(ctx)
	if err != nil {
		t.Fatalf("CPU: unexpected error: %v", err)
	}
	if v < 0 || v > 100 {
		t.Fatalf("CPU: value %.2f out of [0, 100]", v)
	}
}

// TestCPUNilContext exercises the nil-ctx fallback.
func TestCPUNilContext(t *testing.T) {
	v, err := CPU(nil) //nolint:staticcheck // intentional nil ctx
	if err != nil {
		t.Fatalf("CPU(nil): unexpected error: %v", err)
	}
	if v < 0 || v > 100 {
		t.Fatalf("CPU(nil): value %.2f out of range", v)
	}
}
