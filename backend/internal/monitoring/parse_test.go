package monitoring

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/awbuilds/drovik/backend/internal/testssh"
)

func TestParseDefaultFixture(t *testing.T) {
	sample, err := parseSample(testssh.DefaultMetricsFixture, time.Unix(1_700_000_000, 0).UTC())
	if err != nil {
		t.Fatalf("parseSample: %v", err)
	}
	if sample.host.OS != "Linux" || sample.host.Kernel != "6.8.0-drovik-test" || sample.host.Architecture != "x86_64" {
		t.Fatalf("unexpected host: %+v", sample.host)
	}
	if sample.host.Hostname != "drovik-test" || sample.host.Distribution != "Drovik Test Linux" {
		t.Fatalf("unexpected identity: %+v", sample.host)
	}
	if sample.cores != 2 || len(sample.cpu.cores) != 2 {
		t.Fatalf("unexpected cores: %d %+v", sample.cores, sample.cpu.cores)
	}
	if sample.model != "Drovik Test CPU @ 2.40GHz" {
		t.Fatalf("unexpected model: %q", sample.model)
	}
	if sample.memory.TotalBytes != 2048000*1024 || sample.memory.AvailableBytes != 1536000*1024 {
		t.Fatalf("unexpected memory: %+v", sample.memory)
	}
	if sample.memory.SwapTotalBytes != 524288*1024 || sample.memory.SwapUsedBytes != 262144*1024 {
		t.Fatalf("unexpected swap: %+v", sample.memory)
	}
	if sample.load.One != 0.42 || sample.load.Fifteen != 0.25 || sample.load.Runnable != 2 || sample.load.Total != 128 {
		t.Fatalf("unexpected load: %+v", sample.load)
	}
	if sample.uptime != 123456.78 {
		t.Fatalf("unexpected uptime: %f", sample.uptime)
	}
	if len(sample.filesystems) != 2 {
		t.Fatalf("expected two real filesystems, got %+v", sample.filesystems)
	}
	if sample.filesystems[0].Mountpoint != "/" || sample.filesystems[0].FSType != "ext4" {
		t.Fatalf("unexpected root filesystem: %+v", sample.filesystems[0])
	}
	if sample.filesystems[1].Mountpoint != "/srv/data" || sample.filesystems[1].FSType != "xfs" {
		t.Fatalf("unexpected data filesystem: %+v", sample.filesystems[1])
	}
	if len(sample.network) != 2 || sample.network[1].name != "eth0" {
		t.Fatalf("unexpected interfaces: %+v", sample.network)
	}
}

func TestParseVariants(t *testing.T) {
	fixtures := []string{
		strings.Replace(testssh.DefaultMetricsFixture, "PRETTY_NAME=\"Drovik Test Linux\"", "NAME=\"RHEL\"\nVERSION_ID=\"9.4\"", 1),
		strings.Replace(testssh.DefaultMetricsFixture, "PRETTY_NAME=\"Drovik Test Linux\"", "", 1),
		strings.Replace(testssh.DefaultMetricsFixture, "model name\t: Drovik Test CPU @ 2.40GHz\n", "", 1),
	}
	for index, fixture := range fixtures {
		sample, err := parseSample(fixture, time.Now().UTC())
		if err != nil {
			t.Fatalf("fixture %d: %v", index, err)
		}
		if sample.host.OS != "Linux" || len(sample.cpu.cores) == 0 {
			t.Fatalf("fixture %d produced an incomplete sample: %+v", index, sample)
		}
	}
}

func TestComputeUsageAndRates(t *testing.T) {
	previous := rawSample{
		collectedAt:  time.Unix(1000, 0),
		cpuAvailable: true,
		cpu: rawCPU{
			total: 1000 + 20 + 300 + 8000 + 100 + 10 + 5,
			idle:  8000 + 100,
			cores: []rawCore{{index: 0, total: 4717, idle: 4050}, {index: 1, total: 4718, idle: 4050}},
		},
		networkAvailable: true,
		network:          []rawInterface{{name: "eth0", rx: 1000, tx: 2000}},
	}
	current := previous
	current.collectedAt = time.Unix(1010, 0)
	current.cpu.total += 500
	current.cpu.idle += 400
	current.cpu.cores = []rawCore{{index: 0, total: 4717 + 250, idle: 4050 + 200}, {index: 1, total: 4718 + 250, idle: 4050 + 200}}
	current.network = []rawInterface{{name: "eth0", rx: 3000, tx: 5000}}
	current.host = HostInfo{OS: "Linux"}
	current.memory.TotalBytes = 1000

	snapshot := computeSnapshot("server", &previous, current)
	if snapshot.CPU.UsagePercent == nil || *snapshot.CPU.UsagePercent != 20 {
		t.Fatalf("unexpected CPU usage: %+v", snapshot.CPU.UsagePercent)
	}
	if len(snapshot.CPU.Cores) != 2 || snapshot.CPU.Cores[0].UsagePercent == nil || *snapshot.CPU.Cores[0].UsagePercent != 20 {
		t.Fatalf("unexpected core usage: %+v", snapshot.CPU.Cores)
	}
	if snapshot.Network[0].ReceiveRateBps == nil || *snapshot.Network[0].ReceiveRateBps != 200 {
		t.Fatalf("unexpected receive rate: %+v", snapshot.Network[0].ReceiveRateBps)
	}
	if snapshot.Network[0].TransmitRateBps == nil || *snapshot.Network[0].TransmitRateBps != 300 {
		t.Fatalf("unexpected transmit rate: %+v", snapshot.Network[0].TransmitRateBps)
	}
}

func TestFirstSampleHasNoRates(t *testing.T) {
	previous, err := parseSample(testssh.DefaultMetricsFixture, time.Now().UTC())
	if err != nil {
		t.Fatalf("parseSample: %v", err)
	}
	fresh := computeSnapshot("server", nil, previous)
	if fresh.CPU.UsagePercent != nil {
		t.Fatalf("the first sample must not invent CPU usage: %+v", fresh.CPU.UsagePercent)
	}
	for _, item := range fresh.Network {
		if item.ReceiveRateBps != nil || item.TransmitRateBps != nil {
			t.Fatalf("the first sample must not invent network rates: %+v", item)
		}
	}
}

func TestCounterResets(t *testing.T) {
	previous := rawSample{
		collectedAt:      time.Unix(1000, 0),
		cpuAvailable:     true,
		cpu:              rawCPU{total: 5000, idle: 4000, cores: []rawCore{{index: 0, total: 5000, idle: 4000}}},
		networkAvailable: true,
		network:          []rawInterface{{name: "eth0", rx: 1000, tx: 1000}},
	}
	current := rawSample{
		collectedAt:      time.Unix(1010, 0),
		cpuAvailable:     true,
		cpu:              rawCPU{total: 100, idle: 90, cores: []rawCore{{index: 0, total: 100, idle: 90}}},
		networkAvailable: true,
		network:          []rawInterface{{name: "eth0", rx: 10, tx: 10}},
	}
	snapshot := computeSnapshot("server", &previous, current)
	if snapshot.CPU.UsagePercent != nil || snapshot.CPU.Cores[0].UsagePercent != nil {
		t.Fatalf("counter reset must produce unavailable CPU usage: %+v", snapshot.CPU)
	}
	if snapshot.Network[0].ReceiveRateBps != nil || snapshot.Network[0].TransmitRateBps != nil {
		t.Fatalf("counter reset must produce unavailable network rates: %+v", snapshot.Network[0])
	}
}

func TestMissingProcStat(t *testing.T) {
	fixture := "@@os-release@@\nPRETTY_NAME=\"Minimal\"\n@@stat@@\n@@end@@\n"
	_, err := parseSample(fixture, time.Now().UTC())
	if !errors.Is(err, ErrMetricsUnavailable) {
		t.Fatalf("expected ErrMetricsUnavailable, got %v", err)
	}
}

func TestFilesystemReadOnlyAndPercent(t *testing.T) {
	mounts := "/dev/sda1 / ext4 ro,relatime 0 0\n"
	df := "Filesystem 1024-blocks Used Available Capacity Mounted on\n/dev/sda1 1000 250 750 25% /\n"
	filesystems := parseFilesystems(df, mounts)
	if len(filesystems) != 1 {
		t.Fatalf("unexpected filesystems: %+v", filesystems)
	}
	if !filesystems[0].ReadOnly {
		t.Fatal("expected the mount to be read-only")
	}
	if filesystems[0].UsedPercent == nil || *filesystems[0].UsedPercent != 25 {
		t.Fatalf("unexpected used percent: %+v", filesystems[0].UsedPercent)
	}
}

func TestPartialMetricsRemainAvailable(t *testing.T) {
	fixture := "@@stat@@\ncpu  10 0 10 80 0 0 0 0\ncpu0 10 0 10 80 0 0 0 0\n@@meminfo@@\nMemTotal: 1024000 kB\nMemFree: 512000 kB\n@@end@@\n"
	sample, err := parseSample(fixture, time.Now().UTC())
	if err != nil {
		t.Fatalf("parseSample should tolerate missing sections: %v", err)
	}
	if !sample.cpuAvailable || !sample.memoryAvailable {
		t.Fatalf("expected CPU and memory to be available: %+v", sample)
	}
	if sample.loadAvailable || sample.uptimeAvailable || sample.filesystemsAvailable || sample.networkAvailable {
		t.Fatalf("missing sections must be marked unavailable: %+v", sample)
	}
}

func BenchmarkParseSample(b *testing.B) {
	for index := 0; index < b.N; index++ {
		if _, err := parseSample(testssh.DefaultMetricsFixture, time.Unix(1_700_000_000, 0)); err != nil {
			b.Fatalf("parseSample: %v", err)
		}
	}
}

func TestNonFiniteLoadAndUptimeAreUnavailable(t *testing.T) {
	fixture := strings.NewReplacer("0.42 0.31 0.25 2/128 4242", "inf nan 0.25 2/128 4242", "123456.78 987654.32", "inf 0").Replace(testssh.DefaultMetricsFixture)
	sample, err := parseSample(fixture, time.Now().UTC())
	if err != nil {
		t.Fatalf("parseSample: %v", err)
	}
	if sample.loadAvailable || sample.uptimeAvailable {
		t.Fatalf("non-finite values must be unavailable: load=%v uptime=%v", sample.loadAvailable, sample.uptimeAvailable)
	}
	snapshot := computeSnapshot("server", nil, sample)
	if snapshot.Load.Available || snapshot.Uptime.Available {
		t.Fatalf("snapshot must not expose non-finite values: %+v", snapshot)
	}
}
