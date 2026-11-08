package monitoring

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// parsePosixSample handles macOS and BSD hosts. The collector emits the same
// markers on every host; Linux ignores the POSIX sections because /proc is
// present. Every field is optional: an unsupported source simply leaves that
// metric unavailable instead of failing the whole snapshot.
func parsePosixSample(sections map[string]string, now time.Time) (rawSample, error) {
	sample := rawSample{collectedAt: now}
	sample.host = parseHost(sections["os-release"], sections["uname"], sections["hostname"])
	if version := parseSwVers(sections["sw-vers"]); version != "" {
		sample.host.Distribution = version
	}
	if sample.host.Distribution == "" {
		if sample.host.OS == "Darwin" {
			sample.host.Distribution = "macOS"
		} else {
			sample.host.Distribution = sample.host.OS
		}
	}

	sample.load, sample.loadAvailable = parsePosixLoad(sections["posix-load"])
	sample.uptime, sample.uptimeAvailable = parseBootTime(sections["posix-boottime"], now)
	sample.memory, sample.memoryAvailable = parsePosixMemory(
		sections["posix-memsize"], sections["posix-physmem"],
		sections["posix-pagesize"], sections["vmstat"],
	)

	if strings.TrimSpace(sections["df"]) != "" {
		sample.filesystems = parseFilesystems(sections["df"], posixMountTable(sections["mounts"], sections["mount"]))
		sample.filesystemsAvailable = true
	}
	if network := parseNetstat(sections["netstat"]); len(network) > 0 {
		sample.network = network
		sample.networkAvailable = true
	}

	cores := parseSysctlInt(sections["posix-ncpu"])
	if cores > 0 {
		sample.cores = cores
		sample.cpu.cores = make([]rawCore, 0, cores)
		for index := 0; index < cores; index++ {
			sample.cpu.cores = append(sample.cpu.cores, rawCore{index: index})
		}
	}
	sample.model = firstNonEmpty(sections["posix-brand"], sections["posix-model"])
	sample.cpuUsage = parseTopCPU(sections["top"])
	sample.cpuAvailable = sample.cpuUsage != nil

	if !sample.cpuAvailable && !sample.memoryAvailable && !sample.loadAvailable && !sample.uptimeAvailable &&
		!sample.filesystemsAvailable && !sample.networkAvailable {
		return rawSample{}, fmt.Errorf("%w: no readable non-Linux metric sources", ErrMetricsUnavailable)
	}
	return sample, nil
}

func parseSwVers(raw string) string {
	name, version := "", ""
	for _, line := range strings.Split(raw, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "ProductName":
			name = strings.TrimSpace(value)
		case "ProductVersion":
			version = strings.TrimSpace(value)
		}
	}
	if name == "" {
		return ""
	}
	if version == "" {
		return name
	}
	return name + " " + version
}

func parseSysctlInt(raw string) int {
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || value <= 0 || value > 1_000_000 {
		return 0
	}
	return int(value)
}

func parsePosixLoad(raw string) (LoadAverage, bool) {
	trimmed := strings.Trim(strings.TrimSpace(raw), "{}")
	fields := strings.Fields(trimmed)
	if len(fields) < 3 {
		return LoadAverage{}, false
	}
	values := make([]float64, 0, 3)
	for _, field := range fields[:3] {
		value, err := strconv.ParseFloat(field, 64)
		if err != nil || math.IsInf(value, 0) || math.IsNaN(value) || value < 0 {
			return LoadAverage{}, false
		}
		values = append(values, value)
	}
	return LoadAverage{
		Available: true,
		One:       values[0],
		Five:      values[1],
		Fifteen:   values[2],
	}, true
}

func parseBootTime(raw string, now time.Time) (float64, bool) {
	index := strings.Index(raw, "sec =")
	if index < 0 {
		return 0, false
	}
	rest := strings.TrimSpace(raw[index+len("sec ="):])
	end := 0
	for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0, false
	}
	seconds, err := strconv.ParseInt(rest[:end], 10, 64)
	if err != nil || seconds <= 0 {
		return 0, false
	}
	uptime := float64(now.Unix() - seconds)
	if uptime < 0 || uptime > 100*365*24*3600 {
		return 0, false
	}
	return uptime, true
}

func parsePosixMemory(memsize, physmem, pagesize, vmstat string) (MemoryMetrics, bool) {
	total, err := strconv.ParseInt(strings.TrimSpace(firstNonEmpty(memsize, physmem)), 10, 64)
	if err != nil || total <= 0 {
		return MemoryMetrics{}, false
	}
	pageSize, err := strconv.ParseInt(strings.TrimSpace(pagesize), 10, 64)
	if err != nil || pageSize <= 0 {
		if start := strings.Index(vmstat, "page size of "); start >= 0 {
			rest := vmstat[start+len("page size of "):]
			end := 0
			for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
				end++
			}
			if end > 0 {
				pageSize, _ = strconv.ParseInt(rest[:end], 10, 64)
			}
		}
	}
	if pageSize <= 0 {
		return MemoryMetrics{}, false
	}

	pages := make(map[string]int64, 8)
	for _, line := range strings.Split(vmstat, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok {
			continue
		}
		value = strings.TrimSuffix(strings.TrimSpace(value), ".")
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil || parsed < 0 {
			continue
		}
		pages[strings.TrimSpace(key)] = parsed
	}
	if len(pages) == 0 {
		return MemoryMetrics{}, false
	}

	freePages := pages["Pages free"] + pages["Pages speculative"]
	inactivePages := pages["Pages inactive"]
	usedPages := pages["Pages active"] + pages["Pages wired down"] + pages["Pages occupied by compressor"]
	free := freePages * pageSize
	available := (freePages + inactivePages) * pageSize
	used := usedPages * pageSize
	if available > total {
		available = total
	}
	if used > total-available {
		used = total - available
	}
	if used < 0 {
		used = 0
	}
	memory := MemoryMetrics{
		Available:      true,
		TotalBytes:     total,
		AvailableBytes: available,
		UsedBytes:      used,
		FreeBytes:      free,
		CachedBytes:    inactivePages * pageSize,
		UsedPercent:    percent(used, total),
	}
	return memory, true
}

func parseTopCPU(raw string) *float64 {
	for _, line := range strings.Split(raw, "\n") {
		index := strings.Index(line, "CPU usage:")
		if index < 0 {
			continue
		}
		fields := strings.Fields(line[index:])
		for position, field := range fields {
			if field != "idle" || position == 0 {
				continue
			}
			value := strings.TrimSuffix(strings.TrimSuffix(fields[position-1], ","), "%")
			idle, err := strconv.ParseFloat(value, 64)
			if err != nil || math.IsInf(idle, 0) || math.IsNaN(idle) || idle < 0 || idle > 100 {
				return nil
			}
			usage := 100 - idle
			return &usage
		}
	}
	return nil
}

// posixMountTable converts BSD/macOS `mount` output into the /proc/mounts
// shape parseFilesystems already understands.
func posixMountTable(procMounts, mountOutput string) string {
	var builder strings.Builder
	builder.WriteString(procMounts)
	for _, line := range strings.Split(mountOutput, "\n") {
		device, rest, ok := strings.Cut(strings.TrimSpace(line), " on ")
		if !ok {
			continue
		}
		optionsStart := strings.LastIndex(rest, " (")
		if optionsStart < 0 || !strings.HasSuffix(rest, ")") {
			continue
		}
		mountpoint := strings.TrimSpace(rest[:optionsStart])
		options := strings.TrimSuffix(rest[optionsStart+2:], ")")
		parts := strings.Split(options, ",")
		if mountpoint == "" || len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
			continue
		}
		normalized := make([]string, 0, len(parts))
		for _, part := range parts {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				normalized = append(normalized, trimmed)
			}
		}
		builder.WriteString(device + " " + mountpoint + " " + strings.TrimSpace(parts[0]) + " " + strings.Join(normalized, ",") + " 0 0\n")
	}
	return builder.String()
}

func parseNetstat(raw string) []rawInterface {
	lines := make([]string, 0, 16)
	for _, line := range strings.Split(raw, "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) < 2 {
		return nil
	}
	header := strings.Fields(lines[0])
	columns := make(map[string]int, len(header))
	for index, name := range header {
		columns[name] = index
	}
	addressIndex, hasAddress := columns["Address"]
	_, hasNetwork := columns["Network"]
	nameIndex, hasName := columns["Name"]
	if !hasName || !hasNetwork || !hasAddress {
		return nil
	}
	seen := make(map[string]bool, 8)
	interfaces := make([]rawInterface, 0, 8)
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		// Empty Address cells collapse out of the whitespace-separated output,
		// so shift later columns left by one for those rows.
		shift := 0
		if len(fields) == len(header)-1 {
			shift = 1
		}
		column := func(name string) (int, bool) {
			index, ok := columns[name]
			if !ok {
				return 0, false
			}
			if shift == 1 && index > addressIndex {
				index--
			}
			if index >= len(fields) {
				return 0, false
			}
			return index, true
		}
		nameIdx, ok := column("Name")
		if !ok || nameIdx != nameIndex {
			continue
		}
		if hasNetwork {
			networkIdx, _ := columns["Network"]
			if shift == 1 && networkIdx > addressIndex {
				networkIdx--
			}
			if networkIdx >= len(fields) || !strings.HasPrefix(fields[networkIdx], "<Link#") {
				continue
			}
		}
		name := strings.TrimSuffix(fields[nameIdx], "*")
		if name == "" || seen[name] {
			continue
		}
		ibytesIdx, okBytes := column("Ibytes")
		obytesIdx, okOut := column("Obytes")
		if !okBytes || !okOut {
			continue
		}
		rx, errRx := strconv.ParseInt(fields[ibytesIdx], 10, 64)
		tx, errTx := strconv.ParseInt(fields[obytesIdx], 10, 64)
		if errRx != nil || errTx != nil || rx < 0 || tx < 0 {
			continue
		}
		seen[name] = true
		interfaces = append(interfaces, rawInterface{name: name, rx: rx, tx: tx})
	}
	sort.SliceStable(interfaces, func(i, j int) bool { return interfaces[i].name < interfaces[j].name })
	return interfaces
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
