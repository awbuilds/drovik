package advanced

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type BlockDevice struct {
	Name       string        `json:"name"`
	Type       string        `json:"type"`
	SizeBytes  int64         `json:"size_bytes"`
	FSType     string        `json:"fstype,omitempty"`
	Mountpoint string        `json:"mountpoint,omitempty"`
	ReadOnly   bool          `json:"read_only"`
	Model      string        `json:"model,omitempty"`
	Transport  string        `json:"transport,omitempty"`
	Children   []BlockDevice `json:"children,omitempty"`
}

type FilesystemUsage struct {
	Device       string  `json:"device"`
	Mountpoint   string  `json:"mountpoint"`
	FSType       string  `json:"fstype"`
	ReadOnly     bool    `json:"read_only"`
	TotalBytes   int64   `json:"total_bytes"`
	UsedBytes    int64   `json:"used_bytes"`
	FreeBytes    int64   `json:"free_bytes"`
	Inodes       int64   `json:"inodes"`
	InodesUsed   int64   `json:"inodes_used"`
	InodesFree   int64   `json:"inodes_free"`
	InodePercent float64 `json:"inode_percent"`
}

type StorageOverview struct {
	Available    bool              `json:"available"`
	BlockDevices []BlockDevice     `json:"block_devices"`
	Filesystems  []FilesystemUsage `json:"filesystems"`
	Truncated    bool              `json:"truncated"`
	CollectedAt  time.Time         `json:"collected_at"`
}

const storageCommand = `printf '@@lsblk@@\n'
lsblk -J -b -o NAME,TYPE,SIZE,FSTYPE,MOUNTPOINT,RO,MODEL,TRAN 2>/dev/null || true
printf '@@df@@\n'
df -Pki 2>/dev/null || true
printf '@@df-bytes@@\n'
df -Pk 2>/dev/null || true
printf '@@mounts@@\n'
cat /proc/mounts 2>/dev/null || true`

func (s *Service) Storage(ctx context.Context, ownerID, serverID string) (*StorageOverview, error) {
	client, _, err := s.servers.Connect(ctx, ownerID, serverID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()
	output, stderr, err := s.run(ctx, client, storageCommand)
	if err != nil && output == "" {
		return nil, classify(stderr, err)
	}
	sections := splitSections(output)
	overview := &StorageOverview{
		Available:    strings.TrimSpace(sections["lsblk"]) != "" || strings.TrimSpace(sections["df"]) != "",
		BlockDevices: parseBlockDevices(sections["lsblk"]),
		Filesystems:  parseFilesystems(sections["df"], sections["df-bytes"], sections["mounts"], s.cfg.MonitorMaxFilesystems),
		CollectedAt:  time.Now().UTC(),
	}
	if !overview.Available {
		return nil, fmt.Errorf("%w: lsblk and df are unavailable", ErrUnavailable)
	}
	return overview, nil
}

func splitSections(output string) map[string]string {
	sections := make(map[string]string, 4)
	current := ""
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "@@") && strings.HasSuffix(trimmed, "@@") {
			current = strings.Trim(trimmed, "@")
			continue
		}
		if current != "" {
			sections[current] += line + "\n"
		}
	}
	return sections
}

func parseBlockDevices(raw string) []BlockDevice {
	var payload struct {
		BlockDevices []map[string]any `json:"blockdevices"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &payload); err != nil {
		return []BlockDevice{}
	}
	result := make([]BlockDevice, 0, len(payload.BlockDevices))
	for _, item := range payload.BlockDevices {
		result = append(result, blockDevice(item, 0))
	}
	return result
}

// maxBlockDepth bounds recursion over attacker-controlled lsblk JSON so a
// malformed tree cannot exhaust the stack.
const maxBlockDepth = 8

func blockDevice(item map[string]any, depth int) BlockDevice {
	device := BlockDevice{
		Name:       anyString(item["name"]),
		Type:       anyString(item["type"]),
		FSType:     anyString(item["fstype"]),
		Mountpoint: anyString(item["mountpoint"]),
		Model:      anyString(item["model"]),
		Transport:  anyString(item["tran"]),
		ReadOnly:   anyBool(item["ro"]),
	}
	device.SizeBytes, _ = strconv.ParseInt(anyString(item["size"]), 10, 64)
	if children, ok := item["children"].([]any); ok && depth < maxBlockDepth {
		for _, child := range children {
			if values, ok := child.(map[string]any); ok {
				device.Children = append(device.Children, blockDevice(values, depth+1))
			}
		}
	}
	return device
}

func parseFilesystems(df, dfBytes, mounts string, max int) []FilesystemUsage {
	bytesByMount := make(map[string][3]int64, 16)
	for _, line := range strings.Split(dfBytes, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 6 || fields[0] == "Filesystem" {
			continue
		}
		total, e1 := strconv.ParseInt(fields[1], 10, 64)
		used, e2 := strconv.ParseInt(fields[2], 10, 64)
		free, e3 := strconv.ParseInt(fields[3], 10, 64)
		if e1 == nil && e2 == nil && e3 == nil {
			bytesByMount[strings.Join(fields[5:], " ")] = [3]int64{total * 1024, used * 1024, free * 1024}
		}
	}
	mountInfo := make(map[string]struct {
		fstype   string
		device   string
		readOnly bool
	}, 32)
	for _, line := range strings.Split(mounts, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		mountInfo[fields[1]] = struct {
			fstype   string
			device   string
			readOnly bool
		}{fstype: fields[2], device: fields[0], readOnly: strings.Contains(","+fields[3]+",", ",ro,")}
	}
	result := make([]FilesystemUsage, 0, 16)
	for _, line := range strings.Split(df, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 6 || fields[0] == "Filesystem" {
			continue
		}
		inodes, err1 := strconv.ParseInt(fields[1], 10, 64)
		used, err2 := strconv.ParseInt(fields[2], 10, 64)
		free, err3 := strconv.ParseInt(fields[3], 10, 64)
		if err1 != nil || err2 != nil || err3 != nil {
			continue
		}
		mountpoint := strings.Join(fields[5:], " ")
		info := mountInfo[mountpoint]
		entry := FilesystemUsage{
			Device: fields[0], Mountpoint: mountpoint, FSType: info.fstype,
			ReadOnly: info.readOnly, Inodes: inodes, InodesUsed: used, InodesFree: free,
		}
		if bytes, ok := bytesByMount[mountpoint]; ok {
			entry.TotalBytes, entry.UsedBytes, entry.FreeBytes = bytes[0], bytes[1], bytes[2]
		}
		if inodes > 0 {
			entry.InodePercent = float64(used) / float64(inodes) * 100
		}
		result = append(result, entry)
		if len(result) >= max {
			break
		}
	}
	return result
}

func anyString(value any) string {
	text, _ := value.(string)
	return text
}

func anyBool(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return typed == "1" || strings.EqualFold(typed, "true")
	default:
		return false
	}
}
