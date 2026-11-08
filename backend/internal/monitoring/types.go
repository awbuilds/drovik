// Package monitoring collects bounded, agentless host metrics over SSH.
package monitoring

import (
	"time"
)

// MetricsCommand is a fixed, enumerated remote command. It only reads kernel
// pseudo-files and runs df; it never interpolates caller input.
const MetricsCommand = `printf '@@os-release@@\n'
cat /etc/os-release 2>/dev/null || true
printf '@@uname@@\n'
uname -srm 2>/dev/null || true
printf '@@hostname@@\n'
hostname 2>/dev/null || true
printf '@@uptime@@\n'
cat /proc/uptime 2>/dev/null || true
printf '@@loadavg@@\n'
cat /proc/loadavg 2>/dev/null || true
printf '@@stat@@\n'
cat /proc/stat 2>/dev/null || true
printf '@@meminfo@@\n'
cat /proc/meminfo 2>/dev/null || true
printf '@@netdev@@\n'
cat /proc/net/dev 2>/dev/null || true
printf '@@cpuinfo@@\n'
cat /proc/cpuinfo 2>/dev/null || true
printf '@@mounts@@\n'
cat /proc/mounts 2>/dev/null || true
printf '@@df@@\n'
df -Pk 2>/dev/null || true
printf '@@posix-boottime@@\n'
sysctl -n kern.boottime 2>/dev/null || true
printf '@@posix-load@@\n'
sysctl -n vm.loadavg 2>/dev/null || true
printf '@@posix-ncpu@@\n'
sysctl -n hw.ncpu 2>/dev/null || true
printf '@@posix-memsize@@\n'
sysctl -n hw.memsize 2>/dev/null || true
printf '@@posix-physmem@@\n'
sysctl -n hw.physmem 2>/dev/null || true
printf '@@posix-pagesize@@\n'
sysctl -n hw.pagesize 2>/dev/null || true
printf '@@posix-brand@@\n'
sysctl -n machdep.cpu.brand_string 2>/dev/null || true
printf '@@posix-model@@\n'
sysctl -n hw.model 2>/dev/null || true
printf '@@vmstat@@\n'
vm_stat 2>/dev/null || true
printf '@@sw-vers@@\n'
sw_vers 2>/dev/null || true
printf '@@top@@\n'
top -l 1 -n 0 2>/dev/null || true
printf '@@mount@@\n'
mount 2>/dev/null || true
printf '@@netstat@@\n'
netstat -ibn 2>/dev/null || true
printf '@@end@@\n'`

type Snapshot struct {
	ServerID             string             `json:"server_id"`
	CollectedAt          time.Time          `json:"collected_at"`
	Cached               bool               `json:"cached"`
	Stale                bool               `json:"stale"`
	Warning              string             `json:"warning,omitempty"`
	Host                 HostInfo           `json:"host"`
	CPU                  CPUMetrics         `json:"cpu"`
	Load                 LoadAverage        `json:"load"`
	Memory               MemoryMetrics      `json:"memory"`
	Uptime               UptimeMetrics      `json:"uptime"`
	Filesystems          []Filesystem       `json:"filesystems"`
	FilesystemsAvailable bool               `json:"filesystems_available"`
	Network              []NetworkInterface `json:"network"`
	NetworkAvailable     bool               `json:"network_available"`
	FilesystemsTruncated bool               `json:"filesystems_truncated"`
	NetworkTruncated     bool               `json:"network_truncated"`
}

type HostInfo struct {
	Hostname     string `json:"hostname"`
	OS           string `json:"os"`
	Distribution string `json:"distribution"`
	Kernel       string `json:"kernel"`
	Architecture string `json:"architecture"`
}

type CPUMetrics struct {
	Available    bool        `json:"available"`
	UsagePercent *float64    `json:"usage_percent"`
	Model        string      `json:"model"`
	Cores        []CoreUsage `json:"cores"`
}

type CoreUsage struct {
	Index        int      `json:"index"`
	UsagePercent *float64 `json:"usage_percent"`
}

type LoadAverage struct {
	Available bool    `json:"available"`
	One       float64 `json:"one"`
	Five      float64 `json:"five"`
	Fifteen   float64 `json:"fifteen"`
	Runnable  int     `json:"runnable"`
	Total     int     `json:"total"`
}

type MemoryMetrics struct {
	Available      bool     `json:"available"`
	TotalBytes     int64    `json:"total_bytes"`
	AvailableBytes int64    `json:"available_bytes"`
	UsedBytes      int64    `json:"used_bytes"`
	FreeBytes      int64    `json:"free_bytes"`
	BuffersBytes   int64    `json:"buffers_bytes"`
	CachedBytes    int64    `json:"cached_bytes"`
	SwapTotalBytes int64    `json:"swap_total_bytes"`
	SwapUsedBytes  int64    `json:"swap_used_bytes"`
	UsedPercent    *float64 `json:"used_percent"`
}

type UptimeMetrics struct {
	Available bool      `json:"available"`
	Seconds   float64   `json:"seconds"`
	BootTime  time.Time `json:"boot_time"`
}

type Filesystem struct {
	Device      string   `json:"device"`
	Mountpoint  string   `json:"mountpoint"`
	FSType      string   `json:"fstype"`
	TotalBytes  int64    `json:"total_bytes"`
	UsedBytes   int64    `json:"used_bytes"`
	FreeBytes   int64    `json:"free_bytes"`
	UsedPercent *float64 `json:"used_percent"`
	ReadOnly    bool     `json:"read_only"`
}

type NetworkInterface struct {
	Name             string   `json:"name"`
	ReceivedBytes    int64    `json:"received_bytes"`
	TransmittedBytes int64    `json:"transmitted_bytes"`
	ReceiveRateBps   *float64 `json:"receive_rate_bps"`
	TransmitRateBps  *float64 `json:"transmit_rate_bps"`
}
