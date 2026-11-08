package advanced

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/awbuilds/drovik/backend/internal/monitoring"
	"github.com/awbuilds/drovik/backend/internal/secrets"
	"github.com/awbuilds/drovik/backend/internal/storage"
	"github.com/awbuilds/drovik/backend/internal/testenv"
	"github.com/awbuilds/drovik/backend/internal/testssh"
)

func newAdvancedEnv(t *testing.T, hook func(string) (string, string, int, bool)) (*testenv.Env, *Service) {
	t.Helper()
	sshServer, err := testssh.Start(t.TempDir())
	if err != nil {
		t.Fatalf("testssh.Start: %v", err)
	}
	t.Cleanup(sshServer.Close)
	sshServer.CommandHook = hook
	env, err := testenv.New(t.TempDir(), sshServer)
	if err != nil {
		t.Fatalf("testenv.New: %v", err)
	}
	t.Cleanup(func() { _ = env.Close() })
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cipher, err := secrets.New([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("secrets.New: %v", err)
	}
	service := NewService(
		env.Cfg, env.ServerService,
		storage.NewSnippetRepository(env.DB),
		storage.NewMetricRepository(env.DB),
		storage.NewAlertRepository(env.DB),
		cipher,
		env.Audit, logger,
	)
	return env, service
}

const dockerFixture = `{"ID":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","Names":"/web","Image":"nginx:latest","State":"running","Status":"Up 2 hours","Ports":"0.0.0.0:80->80/tcp","CreatedAt":"2026-11-08 01:00:00 +0000 UTC","RunningFor":"2 hours ago"}
`
const imageFixture = `{"ID":"sha256:bbbb","Repository":"nginx","Tag":"latest","Size":"187MB","CreatedSince":"2 weeks ago"}
`

func TestDockerInspectionAndAction(t *testing.T) {
	acted := ""
	env, service := newAdvancedEnv(t, func(command string) (string, string, int, bool) {
		switch {
		case strings.Contains(command, "docker ps -a"):
			return dockerFixture, "", 0, true
		case strings.Contains(command, "docker images"):
			return imageFixture, "", 0, true
		case strings.Contains(command, "docker start "):
			acted = command
			return "", "", 0, true
		case strings.Contains(command, "docker logs"):
			return "container output\n", "", 0, true
		}
		return "", "", 0, false
	})
	result, err := service.Docker(context.Background(), env.User.ID, env.Server.ID)
	if err != nil {
		t.Fatalf("Docker: %v", err)
	}
	if len(result.Containers) != 1 || result.Containers[0].Name != "web" || len(result.Images) != 1 {
		t.Fatalf("unexpected docker result: %+v", result)
	}
	if err := service.DockerAction(context.Background(), env.User.ID, env.Server.ID, result.Containers[0].ID, "start"); err != nil {
		t.Fatalf("DockerAction: %v", err)
	}
	if !strings.HasPrefix(acted, "docker start ") {
		t.Fatalf("unexpected action command: %q", acted)
	}
	if err := service.DockerAction(context.Background(), env.User.ID, env.Server.ID, "bad; rm -rf /", "start"); err != ErrInvalidTarget {
		t.Fatalf("expected invalid target, got %v", err)
	}
	logs, err := service.DockerLogs(context.Background(), env.User.ID, env.Server.ID, result.Containers[0].ID, 50)
	if err != nil || !strings.Contains(logs.Output, "container output") {
		t.Fatalf("DockerLogs: %v %+v", err, logs)
	}
}

func TestStorageNetworkPackagesSchedules(t *testing.T) {
	env, service := newAdvancedEnv(t, func(command string) (string, string, int, bool) {
		switch {
		case strings.Contains(command, "@@lsblk@@"):
			return `@@lsblk@@
{"blockdevices":[{"name":"sda","type":"disk","size":"1000000","ro":false,"model":"Test Disk","tran":"sata","children":[{"name":"sda1","type":"part","size":"900000","fstype":"ext4","mountpoint":"/"}]}]}
@@df@@
Filesystem Inodes IUsed IFree IUse% Mounted on
/dev/sda1 100000 25000 75000 25% /
@@df-bytes@@
Filesystem 1024-blocks Used Available Capacity Mounted on
/dev/sda1 1000000 250000 750000 25% /
@@mounts@@
/dev/sda1 / ext4 rw,relatime 0 0
`, "", 0, true
		case strings.Contains(command, "@@addr@@"):
			return `@@addr@@
[{"ifname":"eth0","operstate":"UP","mtu":1500,"address":"aa:bb:cc:dd:ee:ff","addr_info":[{"family":"inet","local":"192.0.2.10","prefixlen":24}]}]
@@route@@
[{"dst":"default","gateway":"192.0.2.1","dev":"eth0","metric":100}]
@@listen@@
tcp LISTEN 0 128 0.0.0.0:22 0.0.0.0:* users:(("sshd",pid=1))
`, "", 0, true
		case strings.Contains(command, "dpkg-query"):
			return "dpkg\ncurl\t8.0\nopenssh-server\t9.6\n", "", 0, true
		case strings.Contains(command, "@@crontab@@"):
			return `@@crontab@@
0 3 * * * /usr/local/bin/backup
@@etc-crontab@@
@@cron-d@@
### /etc/cron.d/test
*/5 * * * * root /usr/bin/true
@@timers@@
Mon 2026-11-09 00:00:00 UTC 1h left Sun 2026-11-08 00:00:00 UTC 23h ago logrotate.timer logrotate.service
`, "", 0, true
		}
		return "", "", 0, false
	})

	storageOverview, err := service.Storage(context.Background(), env.User.ID, env.Server.ID)
	if err != nil {
		t.Fatalf("Storage: %v", err)
	}
	if len(storageOverview.BlockDevices) != 1 || len(storageOverview.BlockDevices[0].Children) != 1 {
		t.Fatalf("unexpected storage: %+v", storageOverview)
	}
	if storageOverview.Filesystems[0].TotalBytes != 1024000000 || storageOverview.Filesystems[0].Inodes != 100000 {
		t.Fatalf("unexpected filesystem: %+v", storageOverview.Filesystems[0])
	}

	networkOverview, err := service.Network(context.Background(), env.User.ID, env.Server.ID)
	if err != nil {
		t.Fatalf("Network: %v", err)
	}
	if len(networkOverview.Interfaces) != 1 || len(networkOverview.Routes) != 1 || len(networkOverview.Listeners) != 1 {
		t.Fatalf("unexpected network: %+v", networkOverview)
	}

	packages, err := service.Packages(context.Background(), env.User.ID, env.Server.ID)
	if err != nil {
		t.Fatalf("Packages: %v", err)
	}
	if packages.Manager != "dpkg" || len(packages.Packages) != 2 {
		t.Fatalf("unexpected packages: %+v", packages)
	}

	schedules, err := service.Schedules(context.Background(), env.User.ID, env.Server.ID)
	if err != nil {
		t.Fatalf("Schedules: %v", err)
	}
	if len(schedules.Cron) != 2 || len(schedules.Timers) != 1 {
		t.Fatalf("unexpected schedules: %+v", schedules)
	}
}

func TestSnippetPreviewRunAndAlertRule(t *testing.T) {
	env, service := newAdvancedEnv(t, func(command string) (string, string, int, bool) {
		if strings.Contains(command, "echo") {
			return "ran\n", "", 0, true
		}
		return "", "", 0, false
	})
	snippet, err := service.CreateSnippet(context.Background(), env.User.ID, SnippetInput{
		Name: "Echo value", Command: "echo {{value}}",
		Variables: []SnippetVariable{{Name: "value", Default: "hello"}},
	})
	if err != nil {
		t.Fatalf("CreateSnippet: %v", err)
	}
	preview, err := service.PreviewSnippet(context.Background(), env.User.ID, snippet.ID, map[string]string{"value": "it's ok"})
	if err != nil {
		t.Fatalf("PreviewSnippet: %v", err)
	}
	if preview != "echo 'it'\"'\"'s ok'" {
		t.Fatalf("unexpected preview: %q", preview)
	}
	if _, err := service.RunSnippet(context.Background(), env.User.ID, env.Server.ID, snippet.ID, map[string]string{}, false); err == nil {
		t.Fatal("expected confirmation requirement")
	}
	result, err := service.RunSnippet(context.Background(), env.User.ID, env.Server.ID, snippet.ID, map[string]string{"value": "x"}, true)
	if err != nil {
		t.Fatalf("RunSnippet: %v", err)
	}
	if !strings.Contains(result.Output, "ran") {
		t.Fatalf("unexpected run result: %+v", result)
	}

	snapshot := &monitoring.Snapshot{
		ServerID: env.Server.ID, CollectedAt: time.Now().UTC(),
		CPU: monitoring.CPUMetrics{Available: true}, Memory: monitoring.MemoryMetrics{Available: true},
		Load:        monitoring.LoadAverage{Available: true, One: 0.5},
		Filesystems: []monitoring.Filesystem{{UsedPercent: ptr(40)}}, FilesystemsAvailable: true,
		Network: []monitoring.NetworkInterface{{ReceiveRateBps: ptr(10), TransmitRateBps: ptr(20)}}, NetworkAvailable: true,
	}
	service.RecordSnapshot(context.Background(), snapshot)
	history, err := service.History(context.Background(), env.User.ID, env.Server.ID, "1h")
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(history.Samples) != 1 {
		t.Fatalf("expected one history sample: %+v", history)
	}
	rule, err := service.SaveAlertRule(context.Background(), env.User.ID, env.Server.ID, "", AlertRuleInput{
		Metric: "disk_percent", Comparator: "gte", Threshold: 30, Enabled: true,
	})
	if err != nil || rule == nil {
		t.Fatalf("SaveAlertRule: %v %+v", err, rule)
	}
	history, err = service.History(context.Background(), env.User.ID, env.Server.ID, "1h")
	if err != nil || len(history.Alerts) != 1 || !history.Alerts[0].Active {
		t.Fatalf("expected active alert: %v %+v", err, history)
	}
}

func ptr(value float64) *float64 { return &value }

func TestParseListenersRejectsShortLines(t *testing.T) {
	// Exactly five fields previously indexed fields[5] and could panic on
	// malformed or truncated remote output.
	short := "tcp LISTEN 0 128 0.0.0.0:22"
	valid := "tcp LISTEN 0 128 0.0.0.0:22 0.0.0.0:* users:((\"sshd\",pid=1))"
	listeners := parseListeners(short+"\n"+valid+"\n", 10)
	if len(listeners) != 1 || listeners[0].Local != "0.0.0.0:22" {
		t.Fatalf("unexpected listeners: %+v", listeners)
	}
}

func TestRenderSnippetBoundsExpansion(t *testing.T) {
	command := strings.Repeat("{{a}} ", 1000)
	snippet := &Snippet{
		Command:   command,
		Variables: []SnippetVariable{{Name: "a"}},
	}
	// A per-value cap rejects oversized input before any expansion.
	if _, err := renderSnippet(snippet, map[string]string{"a": strings.Repeat("A", maxSnippetValueBytes+1)}); !errors.Is(err, ErrInvalidTarget) {
		t.Fatalf("expected oversized value rejection, got %v", err)
	}
	// Filling the entire body-derived budget must also fail closed rather than
	// materializing the expanded string first.
	values := map[string]string{"a": strings.Repeat("A", maxSnippetValueBytes)}
	if _, err := renderSnippet(snippet, values); !errors.Is(err, ErrInvalidTarget) {
		t.Fatalf("expected rendered-command budget rejection, got %v", err)
	}
	ok, err := renderSnippet(snippet, map[string]string{"a": "value"})
	if err != nil || strings.Count(ok, "'value'") != 1000 {
		t.Fatalf("unexpected rendered command: %v (len=%d)", err, len(ok))
	}
}

func TestSplitSectionsBoundsLines(t *testing.T) {
	var builder strings.Builder
	builder.WriteString("@@df@@\n")
	for index := 0; index < maxSectionLines*3; index++ {
		builder.WriteString("x\n")
	}
	sections := splitSections(builder.String())
	if lines := strings.Count(sections["df"], "\n"); lines != maxSectionLines {
		t.Fatalf("expected %d retained lines, got %d", maxSectionLines, lines)
	}
}

func TestDockerImagesTruncated(t *testing.T) {
	env, service := newAdvancedEnv(t, func(command string) (string, string, int, bool) {
		switch {
		case strings.Contains(command, "docker ps -a"):
			return "", "", 0, true
		case strings.Contains(command, "docker images"):
			var builder strings.Builder
			for index := 0; index < 10; index++ {
				builder.WriteString("{\"ID\":\"sha256:x\",\"Repository\":\"repo\",\"Tag\":\"latest\"}\n")
			}
			return builder.String(), "", 0, true
		}
		return "", "", 0, false
	})
	service.cfg.DockerMaxContainers = 3
	result, err := service.Docker(context.Background(), env.User.ID, env.Server.ID)
	if err != nil {
		t.Fatalf("Docker: %v", err)
	}
	if len(result.Images) != 3 || !result.Truncated {
		t.Fatalf("expected image truncation, got %d (truncated=%v)", len(result.Images), result.Truncated)
	}
}

func TestNonFiniteSnapshotIsNotPersisted(t *testing.T) {
	env, service := newAdvancedEnv(t, nil)
	infinite := math.Inf(1)
	notNaN := math.NaN()
	snapshot := &monitoring.Snapshot{
		ServerID: env.Server.ID, CollectedAt: time.Now().UTC(),
		CPU:    monitoring.CPUMetrics{Available: true, UsagePercent: &infinite},
		Load:   monitoring.LoadAverage{Available: true, One: infinite},
		Memory: monitoring.MemoryMetrics{Available: true, UsedPercent: &notNaN},
	}
	service.RecordSnapshot(context.Background(), snapshot)
	history, err := service.History(context.Background(), env.User.ID, env.Server.ID, "1h")
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(history.Samples) != 1 {
		t.Fatalf("expected one sample row: %+v", history)
	}
	for _, candidate := range []*float64{history.Samples[0].CPUPercent, history.Samples[0].MemoryPercent, history.Samples[0].LoadOne} {
		if candidate != nil && (math.IsInf(*candidate, 0) || math.IsNaN(*candidate)) {
			t.Fatalf("non-finite value persisted: %v", *candidate)
		}
	}
}

func TestMetricInsertDeduplicatesSameCollection(t *testing.T) {
	env, service := newAdvancedEnv(t, nil)
	collected := time.Now().UTC()
	sample := storage.MetricSample{ServerID: env.Server.ID, CollectedAt: collected}
	repository := storage.NewMetricRepository(env.DB)
	if err := repository.Insert(context.Background(), sample); err != nil {
		t.Fatalf("Insert first: %v", err)
	}
	if err := repository.Insert(context.Background(), sample); err != nil {
		t.Fatalf("Insert duplicate: %v", err)
	}
	count, err := repository.Count(context.Background(), env.Server.ID)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one deduplicated sample, got %d", count)
	}
	_ = service
}

func TestAlertRuleIDAndCardinalityBounds(t *testing.T) {
	env, service := newAdvancedEnv(t, nil)
	ctx := context.Background()
	if _, err := service.SaveAlertRule(ctx, env.User.ID, env.Server.ID, "not-an-id", AlertRuleInput{
		Metric: "cpu_percent", Comparator: "gt", Threshold: 80, Enabled: true,
	}); !errors.Is(err, ErrInvalidTarget) {
		t.Fatalf("expected invalid rule ID rejection, got %v", err)
	}
	for index := 0; index < maxAlertRules; index++ {
		if _, err := service.SaveAlertRule(ctx, env.User.ID, env.Server.ID, "", AlertRuleInput{
			Metric: "cpu_percent", Comparator: "gt", Threshold: 80, Enabled: true,
		}); err != nil {
			t.Fatalf("create rule %d: %v", index, err)
		}
	}
	if _, err := service.SaveAlertRule(ctx, env.User.ID, env.Server.ID, "", AlertRuleInput{
		Metric: "cpu_percent", Comparator: "gt", Threshold: 80, Enabled: true,
	}); !errors.Is(err, ErrInvalidTarget) {
		t.Fatalf("expected rule cap rejection, got %v", err)
	}
}

func TestPackageActionLifecycle(t *testing.T) {
	var commands []string
	env, service := newAdvancedEnv(t, func(command string) (string, string, int, bool) {
		if strings.Contains(command, "command -v apt-get") {
			return "apt\n", "", 0, true
		}
		if strings.HasPrefix(command, "apt-get ") {
			commands = append(commands, command)
			return "Setting up package\n", "", 0, true
		}
		return "", "", 0, false
	})
	ctx := context.Background()
	if _, err := service.PackageAction(ctx, env.User.ID, env.Server.ID, "curl", PackageInstall, false); !errors.Is(err, ErrInvalidTarget) {
		t.Fatalf("expected confirmation requirement, got %v", err)
	}
	for _, action := range []string{PackageInstall, PackageUpgrade, PackageRemove} {
		result, err := service.PackageAction(ctx, env.User.ID, env.Server.ID, "curl", action, true)
		if err != nil {
			t.Fatalf("%s: %v", action, err)
		}
		if !result.Success || result.Manager != "apt" {
			t.Fatalf("%s: unexpected result: %+v", action, result)
		}
	}
	want := []string{
		"apt-get install -y --no-install-recommends curl",
		"apt-get install -y --only-upgrade curl",
		"apt-get remove -y curl",
	}
	if len(commands) != len(want) {
		t.Fatalf("unexpected commands: %+v", commands)
	}
	for index := range want {
		if commands[index] != want[index] {
			t.Fatalf("command %d = %q, want %q", index, commands[index], want[index])
		}
	}
	for _, invalid := range []string{"curl; reboot", "../pkg", "pkg name", "-pkg", ""} {
		if _, err := service.PackageAction(ctx, env.User.ID, env.Server.ID, invalid, PackageInstall, true); !errors.Is(err, ErrInvalidTarget) {
			t.Fatalf("expected invalid package %q to be rejected, got %v", invalid, err)
		}
	}
}

func TestPackageActionReportsFailureOutput(t *testing.T) {
	env, service := newAdvancedEnv(t, func(command string) (string, string, int, bool) {
		switch {
		case strings.Contains(command, "command -v apt-get"):
			return "apt\n", "", 0, true
		case strings.HasPrefix(command, "apt-get remove"):
			return "", "E: Could not open lock file\n", 100, true
		}
		return "", "", 0, false
	})
	result, err := service.PackageAction(context.Background(), env.User.ID, env.Server.ID, "curl", PackageRemove, true)
	if err != nil {
		t.Fatalf("PackageAction: %v", err)
	}
	if result.Success || !strings.Contains(result.Output, "lock file") {
		t.Fatalf("expected failed output, got %+v", result)
	}
}

func TestManagedTaskLifecycle(t *testing.T) {
	current := "0 3 * * * /usr/local/bin/backup\n"
	lastStaged := ""
	env, service := newAdvancedEnv(t, func(command string) (string, string, int, bool) {
		switch {
		case command == "crontab -l":
			return current, "", 0, true
		case strings.HasPrefix(command, "crontab /tmp/drovik-crontab-"):
			path := strings.TrimPrefix(command, "crontab ")
			content, err := os.ReadFile(path)
			if err != nil {
				return "", "read staging file failed\n", 1, true
			}
			lastStaged = string(content)
			current = lastStaged
			return "", "", 0, true
		case strings.Contains(command, "@@crontab@@"):
			return "@@crontab@@\n" + current + "@@etc-crontab@@\n@@cron-d@@\n@@timers@@\n", "", 0, true
		}
		return "", "", 0, false
	})
	ctx := context.Background()

	if _, err := service.CreateTask(ctx, env.User.ID, env.Server.ID, TaskInput{Schedule: "*/5 * * * *", Command: "echo hello"}, false); !errors.Is(err, ErrInvalidTarget) {
		t.Fatalf("expected confirmation requirement, got %v", err)
	}
	result, err := service.CreateTask(ctx, env.User.ID, env.Server.ID, TaskInput{
		Schedule: "*/5 * * * *", Command: "echo hello",
	}, true)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if !strings.Contains(lastStaged, "# drovik-task:"+result.Task.ID) || !strings.Contains(lastStaged, "*/5 * * * * echo hello") {
		t.Fatalf("unexpected staged crontab: %q", lastStaged)
	}
	if !strings.Contains(lastStaged, "0 3 * * * /usr/local/bin/backup") {
		t.Fatalf("existing crontab entry was not preserved: %q", lastStaged)
	}

	overview, err := service.Schedules(ctx, env.User.ID, env.Server.ID)
	if err != nil {
		t.Fatalf("Schedules: %v", err)
	}
	if len(overview.ManagedTasks) != 1 || overview.ManagedTasks[0].ID != result.Task.ID {
		t.Fatalf("unexpected managed tasks: %+v", overview.ManagedTasks)
	}

	if err := service.DeleteTask(ctx, env.User.ID, env.Server.ID, result.Task.ID, false); !errors.Is(err, ErrInvalidTarget) {
		t.Fatalf("expected delete confirmation requirement, got %v", err)
	}
	if err := service.DeleteTask(ctx, env.User.ID, env.Server.ID, result.Task.ID, true); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}
	if strings.Contains(lastStaged, taskMarkerPrefix+result.Task.ID) {
		t.Fatalf("managed task marker still present: %q", lastStaged)
	}
	if !strings.Contains(lastStaged, "0 3 * * * /usr/local/bin/backup") {
		t.Fatalf("existing crontab entry was lost: %q", lastStaged)
	}
}

func TestTaskValidation(t *testing.T) {
	for _, input := range []TaskInput{
		{Schedule: "* * * *", Command: "echo hi"},
		{Schedule: "* * * * * *", Command: "echo hi"},
		{Schedule: "bad * * * *", Command: "echo hi"},
		{Schedule: "* * * * *", Command: ""},
		{Schedule: "* * * * *", Command: "echo %"},
		{Schedule: "* * * * *", Command: "echo\nreboot"},
	} {
		if _, err := validateTask(input); !errors.Is(err, ErrInvalidTarget) {
			t.Fatalf("expected %+v to be rejected, got %v", input, err)
		}
	}
}

func TestDockerActionCancellationKeepsAudit(t *testing.T) {
	env, service := newAdvancedEnv(t, func(command string) (string, string, int, bool) {
		if strings.Contains(command, "docker rm ") {
			time.Sleep(300 * time.Millisecond)
			return "", "", 0, true
		}
		return "", "", 0, false
	})
	containerID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	err := service.DockerAction(ctx, env.User.ID, env.Server.ID, containerID, "remove")
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	events, err := storage.NewAuditRepository(env.DB).ListRecent(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListRecent: %v", err)
	}
	found := false
	for _, event := range events {
		if event.EventType == "docker.action" {
			found = true
			if event.Outcome != "unknown" || event.SafeErrorCode == nil || *event.SafeErrorCode != "OUTCOME_UNKNOWN" {
				t.Fatalf("unexpected docker audit event: %+v", event)
			}
		}
	}
	if !found {
		t.Fatal("expected a docker.action audit row for the delivered command")
	}
}

func TestAlertWebhookDelivery(t *testing.T) {
	received := make(chan webhookPayload, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload webhookPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err == nil {
			received <- payload
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	env, service := newAdvancedEnv(t, nil)
	ctx := context.Background()
	rule, err := service.SaveAlertRule(ctx, env.User.ID, env.Server.ID, "", AlertRuleInput{
		Metric: "disk_percent", Comparator: "gte", Threshold: 80, Enabled: true,
		WebhookURL: server.URL + "/hook",
	})
	if err != nil {
		t.Fatalf("SaveAlertRule: %v", err)
	}
	if rule.WebhookEncrypted == "" {
		t.Fatal("expected an encrypted webhook secret")
	}
	if _, err := service.SaveAlertRule(ctx, env.User.ID, env.Server.ID, "", AlertRuleInput{
		Metric: "cpu_percent", Comparator: "gt", Threshold: 1, Enabled: true,
		WebhookURL: "file:///etc/passwd",
	}); !errors.Is(err, ErrInvalidTarget) {
		t.Fatalf("expected invalid webhook scheme rejection, got %v", err)
	}

	snapshot := &monitoring.Snapshot{
		ServerID: env.Server.ID, CollectedAt: time.Now().UTC(),
		CPU: monitoring.CPUMetrics{Available: true}, Memory: monitoring.MemoryMetrics{Available: true},
		Load:        monitoring.LoadAverage{Available: true, One: 0.2},
		Filesystems: []monitoring.Filesystem{{UsedPercent: ptr(92)}}, FilesystemsAvailable: true,
		NetworkAvailable: true,
	}
	service.RecordSnapshot(ctx, snapshot)
	select {
	case payload := <-received:
		if payload.RuleID != rule.ID || payload.Metric != "disk_percent" || payload.Value != 92 {
			t.Fatalf("unexpected webhook payload: %+v", payload)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("webhook was not delivered")
	}
	// A second snapshot inside the cooldown must not notify again.
	service.RecordSnapshot(ctx, snapshot)
	select {
	case payload := <-received:
		t.Fatalf("cooldown was not respected: %+v", payload)
	case <-time.After(300 * time.Millisecond):
	}
}

func TestDestructiveHostOperationsGuarded(t *testing.T) {
	env, service := newAdvancedEnv(t, nil)
	ctx := context.Background()
	if _, err := service.Mount(ctx, env.User.ID, env.Server.ID, "/dev/sdb1", "/mnt/data", "ext4", true); !errors.Is(err, ErrFeatureDisabled) {
		t.Fatalf("expected feature-disabled error, got %v", err)
	}
	service.cfg.EnableDestructiveHostOps = true
	if _, err := service.Mount(ctx, env.User.ID, env.Server.ID, "/dev/sdb1", "/mnt/data", "ext4", false); !errors.Is(err, ErrInvalidTarget) {
		t.Fatalf("expected confirmation requirement, got %v", err)
	}
	for _, mount := range []string{"/", "/boot", "/var", "/mnt/../etc", "relative"} {
		if _, err := service.Mount(ctx, env.User.ID, env.Server.ID, "/dev/sdb1", mount, "ext4", true); !errors.Is(err, ErrInvalidTarget) {
			t.Fatalf("expected mountpoint %q rejection, got %v", mount, err)
		}
	}
	if _, err := service.Format(ctx, env.User.ID, env.Server.ID, "/dev/sdb1", "ext4", "/dev/sdb2", true); !errors.Is(err, ErrInvalidTarget) {
		t.Fatalf("expected typed-device mismatch, got %v", err)
	}
	for _, device := range []string{"/dev/../etc/passwd", "/dev/sdb1;reboot", "sdb1", "/dev/sdb1 extra"} {
		if _, err := service.Format(ctx, env.User.ID, env.Server.ID, device, "ext4", device, true); !errors.Is(err, ErrInvalidTarget) {
			t.Fatalf("expected device %q rejection, got %v", device, err)
		}
	}
}

func TestDestructiveStorageCommands(t *testing.T) {
	var commands []string
	env, service := newAdvancedEnv(t, func(command string) (string, string, int, bool) {
		switch {
		case strings.HasPrefix(command, "lsblk -no MOUNTPOINT"):
			return "", "", 0, true
		case strings.HasPrefix(command, "lsblk -ndo TYPE"):
			return "disk\n", "", 0, true
		case strings.HasPrefix(command, "mount "), strings.HasPrefix(command, "umount "),
			strings.HasPrefix(command, "mkfs."), strings.HasPrefix(command, "parted "):
			commands = append(commands, command)
			return "ok\n", "", 0, true
		}
		return "", "", 0, false
	})
	service.cfg.EnableDestructiveHostOps = true
	ctx := context.Background()
	if _, err := service.Mount(ctx, env.User.ID, env.Server.ID, "/dev/sdb1", "/mnt/data", "ext4", true); err != nil {
		t.Fatalf("Mount: %v", err)
	}
	if _, err := service.Unmount(ctx, env.User.ID, env.Server.ID, "/mnt/data", true); err != nil {
		t.Fatalf("Unmount: %v", err)
	}
	if _, err := service.Format(ctx, env.User.ID, env.Server.ID, "/dev/sdb1", "ext4", "/dev/sdb1", true); err != nil {
		t.Fatalf("Format: %v", err)
	}
	if _, err := service.Partition(ctx, env.User.ID, env.Server.ID, "/dev/sdb", "ext4", "/dev/sdb", true); err != nil {
		t.Fatalf("Partition: %v", err)
	}
	want := []string{
		"mount -t ext4 /dev/sdb1 /mnt/data",
		"umount /mnt/data",
		"mkfs.ext4 -F /dev/sdb1",
		"parted -s /dev/sdb mklabel gpt mkpart primary ext4 1MiB 100%",
	}
	if len(commands) != len(want) {
		t.Fatalf("unexpected commands: %+v", commands)
	}
	for index := range want {
		if commands[index] != want[index] {
			t.Fatalf("command %d = %q, want %q", index, commands[index], want[index])
		}
	}
}

func TestFormatRefusesMountedDevice(t *testing.T) {
	env, service := newAdvancedEnv(t, func(command string) (string, string, int, bool) {
		if strings.HasPrefix(command, "lsblk -no MOUNTPOINT") {
			return "/mnt/data\n", "", 0, true
		}
		return "", "", 0, false
	})
	service.cfg.EnableDestructiveHostOps = true
	if _, err := service.Format(context.Background(), env.User.ID, env.Server.ID, "/dev/sdb1", "ext4", "/dev/sdb1", true); !errors.Is(err, ErrInvalidTarget) {
		t.Fatalf("expected mounted-device rejection, got %v", err)
	}
}

func TestDestructiveNetworkCommands(t *testing.T) {
	var commands []string
	env, service := newAdvancedEnv(t, func(command string) (string, string, int, bool) {
		switch {
		case strings.Contains(command, "command -v ufw"):
			return "ufw\n", "", 0, true
		case strings.HasPrefix(command, "ip "), strings.HasPrefix(command, "ufw "):
			commands = append(commands, command)
			return "ok\n", "", 0, true
		}
		return "", "", 0, false
	})
	service.cfg.EnableDestructiveHostOps = true
	ctx := context.Background()
	if _, err := service.LinkState(ctx, env.User.ID, env.Server.ID, "eth0", "down", true); err != nil {
		t.Fatalf("LinkState: %v", err)
	}
	if _, err := service.AddressChange(ctx, env.User.ID, env.Server.ID, "eth0", "192.0.2.10/24", "add", true); err != nil {
		t.Fatalf("AddressChange: %v", err)
	}
	if _, err := service.RouteChange(ctx, env.User.ID, env.Server.ID, "eth0", "default", "192.0.2.1", "add", true); err != nil {
		t.Fatalf("RouteChange: %v", err)
	}
	if _, err := service.FirewallChange(ctx, env.User.ID, env.Server.ID, 22, "tcp", "allow", true); err != nil {
		t.Fatalf("FirewallChange: %v", err)
	}
	want := []string{
		"ip link set eth0 down",
		"ip addr add 192.0.2.10/24 dev eth0",
		"ip route add default via 192.0.2.1 dev eth0",
		"ufw allow 22/tcp",
	}
	if len(commands) != len(want) {
		t.Fatalf("unexpected commands: %+v", commands)
	}
	for index := range want {
		if commands[index] != want[index] {
			t.Fatalf("command %d = %q, want %q", index, commands[index], want[index])
		}
	}
	for _, iface := range []string{"", "eth0; reboot", "this-interface-is-too-long"} {
		if _, err := service.LinkState(ctx, env.User.ID, env.Server.ID, iface, "up", true); !errors.Is(err, ErrInvalidTarget) {
			t.Fatalf("expected interface %q rejection, got %v", iface, err)
		}
	}
	if _, err := service.AddressChange(ctx, env.User.ID, env.Server.ID, "eth0", "not-a-cidr", "add", true); !errors.Is(err, ErrInvalidTarget) {
		t.Fatalf("expected CIDR rejection, got %v", err)
	}
	if _, err := service.RouteChange(ctx, env.User.ID, env.Server.ID, "eth0", "default", "192.0.2.1;reboot", "add", true); !errors.Is(err, ErrInvalidTarget) {
		t.Fatalf("expected gateway rejection, got %v", err)
	}
	if _, err := service.FirewallChange(ctx, env.User.ID, env.Server.ID, 0, "tcp", "allow", true); !errors.Is(err, ErrInvalidTarget) {
		t.Fatalf("expected port rejection, got %v", err)
	}
}

func TestWebhookErrorsDoNotLeakURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := server.URL + "/hooks/super-secret-token"
	server.Close()
	service := &Service{}
	err := service.postWebhook(context.Background(), &http.Client{Timeout: time.Second}, url, webhookPayload{})
	if err == nil {
		t.Fatal("expected a delivery failure")
	}
	if strings.Contains(err.Error(), "super-secret-token") || strings.Contains(err.Error(), server.URL) {
		t.Fatalf("webhook error leaked its URL: %v", err)
	}
}

func TestAlertNotificationClaimIsAtomic(t *testing.T) {
	env, service := newAdvancedEnv(t, nil)
	rule, err := service.SaveAlertRule(context.Background(), env.User.ID, env.Server.ID, "", AlertRuleInput{
		Metric: "cpu_percent", Comparator: "gt", Threshold: 80, Enabled: true,
	})
	if err != nil {
		t.Fatalf("SaveAlertRule: %v", err)
	}
	now := time.Now().UTC()
	first, err := service.alerts.MarkNotified(context.Background(), env.Server.ID, rule.ID, now, now.Add(-webhookCooldown))
	if err != nil || !first {
		t.Fatalf("first claim: %v %v", first, err)
	}
	second, err := service.alerts.MarkNotified(context.Background(), env.Server.ID, rule.ID, now.Add(time.Second), now.Add(-webhookCooldown))
	if err != nil || second {
		t.Fatalf("second claim must lose the cooldown race: %v %v", second, err)
	}
}

func TestDestructiveCancellationAuditsUnknown(t *testing.T) {
	entered := make(chan struct{})
	env, service := newAdvancedEnv(t, func(command string) (string, string, int, bool) {
		if strings.HasPrefix(command, "lsblk -no MOUNTPOINT") {
			return "", "", 0, true
		}
		if strings.HasPrefix(command, "mkfs.ext4") {
			close(entered)
			time.Sleep(300 * time.Millisecond)
			return "", "", 0, true
		}
		return "", "", 0, false
	})
	service.cfg.EnableDestructiveHostOps = true
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-entered
		cancel()
	}()
	if _, err := service.Format(ctx, env.User.ID, env.Server.ID, "/dev/sdb1", "ext4", "/dev/sdb1", true); err != nil {
		t.Fatalf("Format: %v", err)
	}
	events, err := storage.NewAuditRepository(env.DB).ListRecent(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListRecent: %v", err)
	}
	for _, event := range events {
		if event.EventType == "storage.format" {
			if event.Outcome != "unknown" || event.SafeErrorCode == nil || *event.SafeErrorCode != "OUTCOME_UNKNOWN" {
				t.Fatalf("unexpected cancellation audit: %+v", event)
			}
			return
		}
	}
	t.Fatal("expected a storage.format audit row")
}

func TestSnippetImportExport(t *testing.T) {
	env, service := newAdvancedEnv(t, nil)
	ctx := context.Background()
	created, err := service.CreateSnippet(ctx, env.User.ID, SnippetInput{
		Name: "Restart nginx", Description: "test", Command: "systemctl restart {{service}}",
		Variables: []SnippetVariable{{Name: "service", Default: "nginx"}},
	})
	if err != nil {
		t.Fatalf("CreateSnippet: %v", err)
	}
	exported, err := service.ExportSnippets(ctx, env.User.ID)
	if err != nil || len(exported) != 1 || exported[0].Name != created.Name {
		t.Fatalf("ExportSnippets: %+v %v", exported, err)
	}
	if _, err := service.ImportSnippets(ctx, env.User.ID, []SnippetInput{{
		Name: created.Name, Command: "echo duplicate",
	}}); !errors.Is(err, ErrSnippetExists) {
		t.Fatalf("expected duplicate rejection, got %v", err)
	}
	if err := service.DeleteSnippet(ctx, env.User.ID, created.ID); err != nil {
		t.Fatalf("DeleteSnippet: %v", err)
	}
	imported, err := service.ImportSnippets(ctx, env.User.ID, []SnippetInput{{
		Name: "Restart nginx", Description: "imported", Command: "systemctl restart {{service}}",
		Variables: []SnippetVariable{{Name: "service", Default: "nginx"}},
	}})
	if err != nil || len(imported) != 1 {
		t.Fatalf("ImportSnippets: %+v %v", imported, err)
	}
	if _, err := service.ImportSnippets(ctx, env.User.ID, nil); !errors.Is(err, ErrInvalidTarget) {
		t.Fatalf("expected empty import rejection, got %v", err)
	}
	tooMany := make([]SnippetInput, 101)
	for index := range tooMany {
		tooMany[index] = SnippetInput{Name: "Snippet", Command: "echo hi"}
	}
	if _, err := service.ImportSnippets(ctx, env.User.ID, tooMany); !errors.Is(err, ErrInvalidTarget) {
		t.Fatalf("expected bounded import rejection, got %v", err)
	}
}

func TestManagedTaskEmptyAndSilentCrontab(t *testing.T) {
	var staged string
	installed := 0
	env, service := newAdvancedEnv(t, func(command string) (string, string, int, bool) {
		switch {
		case command == "crontab -l":
			return "", "no crontab for root\n", 1, true
		case strings.HasPrefix(command, "crontab /tmp/drovik-crontab-"):
			content, err := os.ReadFile(strings.TrimPrefix(command, "crontab "))
			if err != nil {
				return "", "read failed\n", 1, true
			}
			staged = string(content)
			installed++
			return "", "", 0, true
		}
		return "", "", 0, false
	})
	ctx := context.Background()
	result, err := service.CreateTask(ctx, env.User.ID, env.Server.ID, TaskInput{
		Schedule: "*/5 * * * *", Command: "echo first task",
	}, true)
	if err != nil {
		t.Fatalf("empty crontab CreateTask: %v", err)
	}
	if installed != 1 || !strings.Contains(staged, "# drovik-task:"+result.Task.ID) {
		t.Fatalf("unexpected staged crontab: %q", staged)
	}

	silentEnv, silentService := newAdvancedEnv(t, func(command string) (string, string, int, bool) {
		if command == "crontab -l" {
			return "", "", 1, true
		}
		if strings.HasPrefix(command, "crontab /tmp/drovik-crontab-") {
			t.Fatal("a silent crontab read must not install anything")
		}
		return "", "", 0, false
	})
	if _, err := silentService.CreateTask(ctx, silentEnv.User.ID, silentEnv.Server.ID, TaskInput{
		Schedule: "*/5 * * * *", Command: "echo should fail",
	}, true); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected silent failure to fail closed, got %v", err)
	}

	truncatedEnv, truncatedService := newAdvancedEnv(t, func(command string) (string, string, int, bool) {
		if command == "crontab -l" {
			return "0 3 * * * /backup\n[output truncated]\n", "", 0, true
		}
		if strings.HasPrefix(command, "crontab /tmp/drovik-crontab-") {
			t.Fatal("a truncated crontab read must not install anything")
		}
		return "", "", 0, false
	})
	if _, err := truncatedService.CreateTask(ctx, truncatedEnv.User.ID, truncatedEnv.Server.ID, TaskInput{
		Schedule: "*/5 * * * *", Command: "echo should fail",
	}, true); !errors.Is(err, ErrInvalidTarget) {
		t.Fatalf("expected truncated crontab rejection, got %v", err)
	}
}
