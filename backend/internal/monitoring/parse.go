package monitoring

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

var ErrMetricsUnavailable = errors.New("host metrics are unavailable")

type rawSample struct {
	collectedAt          time.Time
	host                 HostInfo
	load                 LoadAverage
	uptime               float64
	cpu                  rawCPU
	memory               MemoryMetrics
	filesystems          []Filesystem
	network              []rawInterface
	cores                int
	model                string
	cpuAvailable         bool
	loadAvailable        bool
	memoryAvailable      bool
	uptimeAvailable      bool
	filesystemsAvailable bool
	networkAvailable     bool
}

type rawCPU struct {
	total uint64
	idle  uint64
	cores []rawCore
}

type rawCore struct {
	index int
	total uint64
	idle  uint64
}

type rawInterface struct {
	name string
	rx   int64
	tx   int64
}

var pseudoFilesystems = map[string]bool{
	"tmpfs":           true,
	"ramfs":           true,
	"squashfs":        true,
	"proc":            true,
	"sysfs":           true,
	"devtmpfs":        true,
	"devpts":          true,
	"cgroup":          true,
	"cgroup2":         true,
	"securityfs":      true,
	"debugfs":         true,
	"tracefs":         true,
	"configfs":        true,
	"fusectl":         true,
	"pstore":          true,
	"bpf":             true,
	"mqueue":          true,
	"hugetlbfs":       true,
	"autofs":          true,
	"binfmt_misc":     true,
	"rpc_pipefs":      true,
	"nsfs":            true,
	"fuse.gvfsd-fuse": true,
}

func parseSample(output string, now time.Time) (rawSample, error) {
	sections := parseSections(output)
	if len(sections) == 0 {
		return rawSample{}, fmt.Errorf("%w: empty collector output", ErrMetricsUnavailable)
	}
	sample := rawSample{collectedAt: now}
	sample.host = parseHost(sections["os-release"], sections["uname"], sections["hostname"])
	if strings.TrimSpace(sections["stat"]) != "" {
		cpu, cores, ok := parseCPUStat(sections["stat"])
		if ok {
			sample.cpu = cpu
			sample.cores = cores
			sample.cpuAvailable = true
		}
	}
	if strings.TrimSpace(sections["meminfo"]) != "" {
		sample.memory = parseMeminfo(sections["meminfo"])
		sample.memoryAvailable = sample.memory.TotalBytes > 0
		sample.memory.Available = sample.memoryAvailable
	}
	sample.load, sample.loadAvailable = parseLoad(sections["loadavg"])
	sample.uptime, sample.uptimeAvailable = parseUptime(sections["uptime"])
	if strings.TrimSpace(sections["df"]) != "" {
		sample.filesystems = parseFilesystems(sections["df"], sections["mounts"])
		sample.filesystemsAvailable = true
	}
	if strings.TrimSpace(sections["netdev"]) != "" {
		sample.network = parseNetwork(sections["netdev"])
		sample.networkAvailable = true
	}
	if sample.host.OS == "" {
		sample.host.OS = "Linux"
	}
	if sample.host.Architecture == "" {
		sample.host.Architecture = parseCPUArchitecture(sections["cpuinfo"])
	}
	if sample.cores == 0 {
		sample.cores = parseCPUCount(sections["cpuinfo"])
	}
	sample.model = parseCPUModel(sections["cpuinfo"])
	if !sample.cpuAvailable && !sample.memoryAvailable && !sample.loadAvailable && !sample.uptimeAvailable && !sample.filesystemsAvailable && !sample.networkAvailable {
		return rawSample{}, fmt.Errorf("%w: no readable metric sources", ErrMetricsUnavailable)
	}
	return sample, nil
}

func parseSections(output string) map[string]string {
	sections := make(map[string]string, 12)
	current := ""
	var builder strings.Builder
	flush := func() {
		if current != "" {
			sections[current] = builder.String()
		}
		builder.Reset()
	}
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "@@") && strings.HasSuffix(trimmed, "@@") && len(trimmed) > 4 {
			flush()
			current = strings.Trim(trimmed, "@")
			continue
		}
		if current == "" {
			continue
		}
		builder.WriteString(line)
		builder.WriteByte('\n')
	}
	flush()
	return sections
}

func parseHost(osRelease, uname, hostname string) HostInfo {
	info := HostInfo{Hostname: strings.TrimSpace(hostname)}
	for _, line := range strings.Split(osRelease, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		switch key {
		case "PRETTY_NAME", "NAME":
			if info.Distribution == "" || key == "PRETTY_NAME" {
				info.Distribution = value
			}
		case "ID":
			if info.Distribution == "" {
				info.Distribution = value
			}
		}
	}
	fields := strings.Fields(uname)
	if len(fields) >= 1 {
		info.OS = fields[0]
	}
	if len(fields) >= 2 {
		info.Kernel = fields[1]
	}
	if len(fields) >= 3 {
		info.Architecture = fields[2]
	}
	return info
}

func parseCPUStat(raw string) (rawCPU, int, bool) {
	var cpu rawCPU
	cores := 0
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 || !strings.HasPrefix(fields[0], "cpu") {
			continue
		}
		values := make([]uint64, 0, 8)
		for _, field := range fields[1:] {
			value, err := strconv.ParseUint(field, 10, 64)
			if err != nil {
				values = nil
				break
			}
			values = append(values, value)
			if len(values) == 8 {
				break
			}
		}
		if len(values) < 4 {
			continue
		}
		var total uint64
		for _, value := range values {
			total += value
		}
		idle := values[3]
		if len(values) > 4 {
			idle += values[4]
		}
		if fields[0] == "cpu" {
			cpu.total = total
			cpu.idle = idle
			continue
		}
		index, err := strconv.Atoi(strings.TrimPrefix(fields[0], "cpu"))
		if err != nil {
			continue
		}
		cpu.cores = append(cpu.cores, rawCore{index: index, total: total, idle: idle})
		cores++
	}
	if cpu.total == 0 {
		return rawCPU{}, 0, false
	}
	sort.SliceStable(cpu.cores, func(i, j int) bool { return cpu.cores[i].index < cpu.cores[j].index })
	return cpu, cores, true
}

func parseMeminfo(raw string) MemoryMetrics {
	var memory MemoryMetrics
	var swapFree int64
	for _, line := range strings.Split(raw, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok {
			continue
		}
		fields := strings.Fields(value)
		if len(fields) == 0 {
			continue
		}
		amount, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			continue
		}
		bytes := amount * 1024
		switch key {
		case "MemTotal":
			memory.TotalBytes = bytes
		case "MemAvailable":
			memory.AvailableBytes = bytes
		case "MemFree":
			memory.FreeBytes = bytes
		case "Buffers":
			memory.BuffersBytes = bytes
		case "Cached":
			memory.CachedBytes = bytes
		case "SwapTotal":
			memory.SwapTotalBytes = bytes
		case "SwapFree":
			swapFree = bytes
		}
	}
	if memory.AvailableBytes == 0 {
		memory.AvailableBytes = memory.FreeBytes + memory.BuffersBytes + memory.CachedBytes
	}
	if memory.TotalBytes > 0 {
		memory.UsedBytes = max(memory.TotalBytes-memory.AvailableBytes, 0)
		percent := float64(memory.UsedBytes) / float64(memory.TotalBytes) * 100
		memory.UsedPercent = &percent
	}
	if memory.SwapTotalBytes > 0 {
		memory.SwapUsedBytes = max(memory.SwapTotalBytes-swapFree, 0)
	}
	return memory
}

func parseLoad(raw string) (LoadAverage, bool) {
	var load LoadAverage
	fields := strings.Fields(raw)
	if len(fields) < 3 {
		return load, false
	}
	one, errOne := strconv.ParseFloat(fields[0], 64)
	five, errFive := strconv.ParseFloat(fields[1], 64)
	fifteen, errFifteen := strconv.ParseFloat(fields[2], 64)
	if errOne != nil || errFive != nil || errFifteen != nil ||
		math.IsInf(one, 0) || math.IsNaN(one) ||
		math.IsInf(five, 0) || math.IsNaN(five) ||
		math.IsInf(fifteen, 0) || math.IsNaN(fifteen) {
		return load, false
	}
	load.Available = true
	load.One = one
	load.Five = five
	load.Fifteen = fifteen
	if len(fields) >= 4 {
		if running, total, ok := strings.Cut(fields[3], "/"); ok {
			load.Runnable, _ = strconv.Atoi(running)
			load.Total, _ = strconv.Atoi(total)
		}
	}
	return load, true
}

func parseUptime(raw string) (float64, bool) {
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return 0, false
	}
	value, err := strconv.ParseFloat(fields[0], 64)
	if err != nil || value < 0 || math.IsInf(value, 0) || math.IsNaN(value) {
		return 0, false
	}
	return value, true
}

func parseFilesystems(df, mounts string) []Filesystem {
	mountInfo := make(map[string]struct {
		fstype string
		device string
		ro     bool
	}, 32)
	for _, line := range strings.Split(mounts, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		options := fields[3]
		mountInfo[fields[1]] = struct {
			fstype string
			device string
			ro     bool
		}{fstype: fields[2], device: fields[0], ro: strings.Contains(","+options+",", ",ro,")}
	}

	filesystems := make([]Filesystem, 0, 16)
	for _, line := range strings.Split(df, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 6 || fields[0] == "Filesystem" {
			continue
		}
		total, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			continue
		}
		used, errUsed := strconv.ParseInt(fields[2], 10, 64)
		free, errFree := strconv.ParseInt(fields[3], 10, 64)
		if errUsed != nil || errFree != nil {
			continue
		}
		mountpoint := strings.Join(fields[5:], " ")
		info, known := mountInfo[mountpoint]
		fstype := ""
		readOnly := false
		device := fields[0]
		if known {
			fstype = info.fstype
			readOnly = info.ro
			if device == "" || strings.HasPrefix(device, "none") {
				device = info.device
			}
		}
		if pseudoFilesystems[fstype] {
			continue
		}
		item := Filesystem{
			Device:      device,
			Mountpoint:  mountpoint,
			FSType:      fstype,
			TotalBytes:  total * 1024,
			UsedBytes:   used * 1024,
			FreeBytes:   free * 1024,
			ReadOnly:    readOnly,
			UsedPercent: percent(used, total),
		}
		filesystems = append(filesystems, item)
	}
	sort.SliceStable(filesystems, func(i, j int) bool {
		return filesystems[i].Mountpoint < filesystems[j].Mountpoint
	})
	return filesystems
}

func parseNetwork(raw string) []rawInterface {
	interfaces := make([]rawInterface, 0, 8)
	for _, line := range strings.Split(raw, "\n") {
		name, stats, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		if name == "" || name == "Inter-|" || strings.Contains(name, "|") {
			continue
		}
		fields := strings.Fields(stats)
		if len(fields) < 16 {
			continue
		}
		rx, errRx := strconv.ParseInt(fields[0], 10, 64)
		tx, errTx := strconv.ParseInt(fields[8], 10, 64)
		if errRx != nil || errTx != nil {
			continue
		}
		interfaces = append(interfaces, rawInterface{name: name, rx: rx, tx: tx})
	}
	return interfaces
}

func parseCPUArchitecture(raw string) string {
	for _, line := range strings.Split(raw, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if strings.TrimSpace(key) == "architecture" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func parseCPUCount(raw string) int {
	count := 0
	for _, line := range strings.Split(raw, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "processor") {
			count++
		}
	}
	return count
}

func parseCPUModel(raw string) string {
	for _, line := range strings.Split(raw, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "model name" || key == "Hardware" || key == "Processor" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func percent(used, total int64) *float64 {
	if total <= 0 {
		return nil
	}
	value := float64(used) / float64(total) * 100
	if value < 0 {
		value = 0
	}
	if value > 100 {
		value = 100
	}
	return &value
}

func usagePercent(previous, current rawCPU) *float64 {
	if current.total < previous.total || current.idle < previous.idle {
		return nil
	}
	totalDelta := current.total - previous.total
	idleDelta := current.idle - previous.idle
	if totalDelta == 0 || idleDelta > totalDelta {
		return nil
	}
	value := float64(totalDelta-idleDelta) / float64(totalDelta) * 100
	return &value
}

func computeSnapshot(serverID string, previous *rawSample, current rawSample) Snapshot {
	snapshot := Snapshot{
		ServerID:             serverID,
		CollectedAt:          current.collectedAt,
		Host:                 current.host,
		Load:                 current.load,
		Memory:               current.memory,
		Filesystems:          current.filesystems,
		FilesystemsAvailable: current.filesystemsAvailable,
		NetworkAvailable:     current.networkAvailable,
		Uptime: UptimeMetrics{
			Available: current.uptimeAvailable,
			Seconds:   current.uptime,
			BootTime:  current.collectedAt.Add(-time.Duration(current.uptime * float64(time.Second))),
		},
		CPU: CPUMetrics{
			Available: current.cpuAvailable,
			Model:     current.model,
			Cores:     make([]CoreUsage, 0, len(current.cpu.cores)),
		},
		Network: make([]NetworkInterface, 0, len(current.network)),
	}

	if previous != nil && previous.cpuAvailable && current.cpuAvailable {
		snapshot.CPU.UsagePercent = usagePercent(previous.cpu, current.cpu)
		previousCores := make(map[int]rawCPU, len(previous.cpu.cores))
		for _, core := range previous.cpu.cores {
			previousCores[core.index] = rawCPU{total: core.total, idle: core.idle}
		}
		for _, core := range current.cpu.cores {
			usage := CoreUsage{Index: core.index}
			if prev, ok := previousCores[core.index]; ok {
				usage.UsagePercent = usagePercent(prev, rawCPU{total: core.total, idle: core.idle})
			}
			snapshot.CPU.Cores = append(snapshot.CPU.Cores, usage)
		}
	} else {
		for _, core := range current.cpu.cores {
			snapshot.CPU.Cores = append(snapshot.CPU.Cores, CoreUsage{Index: core.index})
		}
	}

	previousInterfaces := make(map[string]rawInterface, 0)
	if previous != nil && previous.networkAvailable {
		for _, item := range previous.network {
			previousInterfaces[item.name] = item
		}
	}
	elapsed := 0.0
	if previous != nil {
		elapsed = current.collectedAt.Sub(previous.collectedAt).Seconds()
	}
	for _, item := range current.network {
		entry := NetworkInterface{
			Name:             item.name,
			ReceivedBytes:    item.rx,
			TransmittedBytes: item.tx,
		}
		if prev, ok := previousInterfaces[item.name]; ok && elapsed > 0 {
			if item.rx >= prev.rx {
				rate := float64(item.rx-prev.rx) / elapsed
				entry.ReceiveRateBps = &rate
			}
			if item.tx >= prev.tx {
				rate := float64(item.tx-prev.tx) / elapsed
				entry.TransmitRateBps = &rate
			}
		}
		snapshot.Network = append(snapshot.Network, entry)
	}
	return snapshot
}
