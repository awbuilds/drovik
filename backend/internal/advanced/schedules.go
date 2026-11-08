package advanced

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type CronEntry struct {
	Source string `json:"source"`
	Line   string `json:"line"`
}

type TimerEntry struct {
	Unit      string `json:"unit"`
	Next      string `json:"next"`
	Left      string `json:"left"`
	Last      string `json:"last"`
	Passed    string `json:"passed"`
	Activates string `json:"activates"`
}

const (
	maxCron   = 256
	maxTimers = 256
)

type ScheduleOverview struct {
	Available   bool         `json:"available"`
	Cron        []CronEntry  `json:"cron"`
	Timers      []TimerEntry `json:"timers"`
	CollectedAt time.Time    `json:"collected_at"`
}

const scheduleCommand = `printf '@@crontab@@\n'
crontab -l 2>/dev/null || true
printf '@@etc-crontab@@\n'
cat /etc/crontab 2>/dev/null || true
printf '@@cron-d@@\n'
for f in /etc/cron.d/*; do
  if [ -f "$f" ]; then
    printf '### %s\n' "$f"
    cat "$f" 2>/dev/null || true
  fi
done
printf '@@timers@@\n'
systemctl list-timers --all --no-legend --no-pager 2>/dev/null || true`

func (s *Service) Schedules(ctx context.Context, ownerID, serverID string) (*ScheduleOverview, error) {
	client, _, err := s.servers.Connect(ctx, ownerID, serverID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()
	output, stderr, err := s.run(ctx, client, scheduleCommand)
	if err != nil && output == "" {
		return nil, classify(stderr, err)
	}
	sections := splitSections(output)
	result := &ScheduleOverview{
		Available:   strings.TrimSpace(output) != "",
		Cron:        make([]CronEntry, 0, 16),
		Timers:      make([]TimerEntry, 0, 16),
		CollectedAt: time.Now().UTC(),
	}
	result.Cron = appendCron(result.Cron, "user crontab", sections["crontab"])
	result.Cron = appendCron(result.Cron, "/etc/crontab", sections["etc-crontab"])
	source := ""
	for _, line := range strings.Split(sections["cron-d"], "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "### ") {
			source = strings.TrimPrefix(trimmed, "### ")
			continue
		}
		if len(result.Cron) >= maxCron {
			break
		}
		if parsed, ok := cronLine(source, line); ok {
			result.Cron = append(result.Cron, parsed)
		}
	}
	for _, line := range strings.Split(sections["timers"], "\n") {
		if len(result.Timers) >= maxTimers {
			break
		}
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		result.Timers = append(result.Timers, TimerEntry{
			Next: fields[0], Left: fields[1], Last: fields[2], Passed: fields[3],
			Unit: fields[4], Activates: strings.Join(fields[5:], " "),
		})
	}
	if !result.Available {
		return nil, fmt.Errorf("%w: cron and systemd timers are unavailable", ErrUnavailable)
	}
	return result, nil
}

func appendCron(entries []CronEntry, source, raw string) []CronEntry {
	for _, line := range strings.Split(raw, "\n") {
		if entry, ok := cronLine(source, line); ok {
			entries = append(entries, entry)
		}
	}
	return entries
}

func cronLine(source, line string) (CronEntry, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "SHELL=") ||
		strings.HasPrefix(trimmed, "PATH=") || strings.HasPrefix(trimmed, "MAILTO=") ||
		strings.HasPrefix(trimmed, "###") {
		return CronEntry{}, false
	}
	fields := strings.Fields(trimmed)
	if len(fields) < 6 {
		return CronEntry{}, false
	}
	return CronEntry{Source: source, Line: trimmed}, true
}
