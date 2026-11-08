//go:build darwin

package monitoring

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

// TestLiveDarwinCollector runs the read-only collector against the live macOS
// host and verifies the parser against real command output. It is compiled only
// on Darwin and never runs in Linux CI.
func TestLiveDarwinCollector(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "sh", "-c", MetricsCommand).Output()
	if err != nil {
		t.Fatalf("collector: %v", err)
	}
	sample, err := parseSample(string(output), time.Now().UTC())
	if err != nil {
		t.Fatalf("parseSample: %v", err)
	}
	if sample.host.OS != "Darwin" {
		t.Fatalf("expected Darwin, got %+v", sample.host)
	}
	if !sample.memoryAvailable || sample.memory.TotalBytes <= 0 {
		t.Fatalf("live macOS memory was not parsed: %+v", sample.memory)
	}
	if !sample.filesystemsAvailable || len(sample.filesystems) == 0 {
		t.Fatalf("live macOS filesystems were not parsed: %+v", sample.filesystems)
	}
	if !sample.loadAvailable || !sample.uptimeAvailable {
		t.Fatalf("live macOS load/uptime were not parsed: %+v %f", sample.load, sample.uptime)
	}
	if sample.cores <= 0 {
		t.Fatalf("live macOS CPU count was not parsed: %d", sample.cores)
	}
}
