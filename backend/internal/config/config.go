package config

import (
	"bufio"
	"encoding/base64"
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	EnvDevelopment = "development"
	EnvTest        = "test"
	EnvProduction  = "production"
)

type Config struct {
	Env      string
	HTTPAddr string

	PublicURL      *url.URL
	TrustedOrigins []string

	LogLevel  string
	LogFormat string

	DataDir   string
	DBPath    string
	StaticDir string

	EncryptionKey []byte

	SessionIdleTTL     time.Duration
	SessionAbsoluteTTL time.Duration
	CookieSecure       bool

	LoginRateLimit  int
	LoginRateWindow time.Duration

	ShutdownTimeout  time.Duration
	ReadinessTimeout time.Duration

	SSHDialTimeout      time.Duration
	SSHHandshakeTimeout time.Duration
	SSHOperationTimeout time.Duration

	EgressAllowPrivate    bool
	EgressAllowedPrefixes []netip.Prefix
	EgressDeniedPrefixes  []netip.Prefix

	SSHMaxSessionsPerUser int
	SSHMaxSessionsGlobal  int

	SFTPMaxUploadBytes      int64
	SFTPMaxEditorBytes      int64
	SFTPMaxListEntries      int
	SFTPMaxRecursiveEntries int
	SFTPOperationTimeout    time.Duration

	MonitorRefreshInterval time.Duration
	MonitorStaleAfter      time.Duration
	MonitorTimeout         time.Duration
	MonitorMaxOutputBytes  int
	MonitorMaxFilesystems  int
	MonitorMaxInterfaces   int
}

func Load() (*Config, error) {
	if err := loadDotEnv(".env"); err != nil {
		return nil, err
	}
	return load(os.LookupEnv)
}

func load(lookup func(string) (string, bool)) (*Config, error) {
	env, err := parseString(lookup, "DROVIK_ENV", EnvDevelopment)
	if err != nil {
		return nil, err
	}
	switch env {
	case EnvDevelopment, EnvTest, EnvProduction:
	default:
		return nil, fmt.Errorf("DROVIK_ENV must be one of development, test, production; got %q", env)
	}

	httpAddr, err := parseString(lookup, "DROVIK_HTTP_ADDR", "127.0.0.1:8080")
	if err != nil {
		return nil, err
	}
	logLevel, err := parseString(lookup, "DROVIK_LOG_LEVEL", "info")
	if err != nil {
		return nil, err
	}
	switch logLevel {
	case "debug", "info", "warn", "error":
	default:
		return nil, fmt.Errorf("DROVIK_LOG_LEVEL must be debug, info, warn or error; got %q", logLevel)
	}

	logFormat, err := parseString(lookup, "DROVIK_LOG_FORMAT", "")
	if err != nil {
		return nil, err
	}
	if logFormat == "" {
		if env == EnvProduction {
			logFormat = "json"
		} else {
			logFormat = "text"
		}
	}
	if logFormat != "json" && logFormat != "text" {
		return nil, fmt.Errorf("DROVIK_LOG_FORMAT must be json or text; got %q", logFormat)
	}

	publicRaw, err := parseString(lookup, "DROVIK_PUBLIC_URL", "http://localhost:8080")
	if err != nil {
		return nil, err
	}
	publicURL, err := url.Parse(publicRaw)
	if err != nil {
		return nil, fmt.Errorf("DROVIK_PUBLIC_URL is invalid: %w", err)
	}
	if publicURL.Scheme != "http" && publicURL.Scheme != "https" {
		return nil, errors.New("DROVIK_PUBLIC_URL must use http or https")
	}
	if publicURL.Host == "" {
		return nil, errors.New("DROVIK_PUBLIC_URL must include a host")
	}
	if publicURL.Path != "" && publicURL.Path != "/" {
		return nil, errors.New("DROVIK_PUBLIC_URL must not include a path")
	}
	publicURL.Path = ""

	origins, err := parseOrigins(lookup, publicURL)
	if err != nil {
		return nil, err
	}

	cookieSecure, err := parseBool(lookup, "DROVIK_COOKIE_SECURE", publicURL.Scheme == "https")
	if err != nil {
		return nil, err
	}
	if env == EnvProduction {
		if publicURL.Scheme != "https" {
			return nil, errors.New("DROVIK_PUBLIC_URL must use https in production")
		}
		if !cookieSecure {
			return nil, errors.New("DROVIK_COOKIE_SECURE must be true in production")
		}
	}

	idleTTL, err := parseDuration(lookup, "DROVIK_SESSION_IDLE_TTL", 12*time.Hour)
	if err != nil {
		return nil, err
	}
	absoluteTTL, err := parseDuration(lookup, "DROVIK_SESSION_ABSOLUTE_TTL", 7*24*time.Hour)
	if err != nil {
		return nil, err
	}
	if idleTTL <= 0 {
		return nil, errors.New("DROVIK_SESSION_IDLE_TTL must be greater than zero")
	}
	if absoluteTTL <= idleTTL {
		return nil, errors.New("DROVIK_SESSION_ABSOLUTE_TTL must be greater than DROVIK_SESSION_IDLE_TTL")
	}

	loginLimit, err := parseInt(lookup, "DROVIK_LOGIN_RATE_LIMIT", 10)
	if err != nil {
		return nil, err
	}
	if loginLimit <= 0 {
		return nil, errors.New("DROVIK_LOGIN_RATE_LIMIT must be greater than zero")
	}
	loginWindow, err := parseDuration(lookup, "DROVIK_LOGIN_RATE_WINDOW", time.Minute)
	if err != nil {
		return nil, err
	}
	if loginWindow <= 0 {
		return nil, errors.New("DROVIK_LOGIN_RATE_WINDOW must be greater than zero")
	}

	shutdownTimeout, err := parseDuration(lookup, "DROVIK_SHUTDOWN_TIMEOUT", 10*time.Second)
	if err != nil {
		return nil, err
	}
	readinessTimeout, err := parseDuration(lookup, "DROVIK_READINESS_TIMEOUT", 2*time.Second)
	if err != nil {
		return nil, err
	}

	sshDialTimeout, err := parseDuration(lookup, "DROVIK_SSH_DIAL_TIMEOUT", 10*time.Second)
	if err != nil {
		return nil, err
	}
	sshHandshakeTimeout, err := parseDuration(lookup, "DROVIK_SSH_HANDSHAKE_TIMEOUT", 15*time.Second)
	if err != nil {
		return nil, err
	}
	sshOperationTimeout, err := parseDuration(lookup, "DROVIK_SSH_OPERATION_TIMEOUT", 30*time.Second)
	if err != nil {
		return nil, err
	}
	if sshDialTimeout <= 0 || sshHandshakeTimeout <= 0 || sshOperationTimeout <= 0 {
		return nil, errors.New("SSH timeouts must be greater than zero")
	}

	egressAllowPrivate, err := parseBool(lookup, "DROVIK_EGRESS_ALLOW_PRIVATE", true)
	if err != nil {
		return nil, err
	}
	egressAllowed, err := parsePrefixes(lookup, "DROVIK_EGRESS_ALLOWED_CIDRS")
	if err != nil {
		return nil, err
	}
	egressDenied, err := parsePrefixes(lookup, "DROVIK_EGRESS_DENIED_CIDRS")
	if err != nil {
		return nil, err
	}

	sftpMaxUpload, err := parseInt64(lookup, "DROVIK_SFTP_MAX_UPLOAD_BYTES", 2*1024*1024*1024)
	if err != nil {
		return nil, err
	}
	sftpMaxEditor, err := parseInt64(lookup, "DROVIK_SFTP_MAX_EDITOR_BYTES", 1*1024*1024)
	if err != nil {
		return nil, err
	}
	sftpMaxList, err := parseInt(lookup, "DROVIK_SFTP_MAX_LIST_ENTRIES", 5000)
	if err != nil {
		return nil, err
	}
	sftpMaxRecursive, err := parseInt(lookup, "DROVIK_SFTP_MAX_RECURSIVE_ENTRIES", 10000)
	if err != nil {
		return nil, err
	}
	sftpTimeout, err := parseDuration(lookup, "DROVIK_SFTP_OPERATION_TIMEOUT", 2*time.Minute)
	if err != nil {
		return nil, err
	}
	if sftpMaxUpload <= 0 || sftpMaxEditor <= 0 || sftpMaxList <= 0 || sftpMaxRecursive <= 0 || sftpTimeout <= 0 {
		return nil, errors.New("SFTP limits must be greater than zero")
	}

	monitorRefresh, err := parseDuration(lookup, "DROVIK_MONITOR_REFRESH_INTERVAL", 5*time.Second)
	if err != nil {
		return nil, err
	}
	monitorStaleAfter, err := parseDuration(lookup, "DROVIK_MONITOR_STALE_AFTER", time.Minute)
	if err != nil {
		return nil, err
	}
	monitorTimeout, err := parseDuration(lookup, "DROVIK_MONITOR_TIMEOUT", 15*time.Second)
	if err != nil {
		return nil, err
	}
	monitorMaxOutput, err := parseInt(lookup, "DROVIK_MONITOR_MAX_OUTPUT_BYTES", 256*1024)
	if err != nil {
		return nil, err
	}
	monitorMaxFilesystems, err := parseInt(lookup, "DROVIK_MONITOR_MAX_FILESYSTEMS", 64)
	if err != nil {
		return nil, err
	}
	monitorMaxInterfaces, err := parseInt(lookup, "DROVIK_MONITOR_MAX_INTERFACES", 64)
	if err != nil {
		return nil, err
	}
	if monitorRefresh < time.Second {
		return nil, errors.New("DROVIK_MONITOR_REFRESH_INTERVAL must be at least 1s")
	}
	if monitorStaleAfter < monitorRefresh || monitorTimeout <= 0 || monitorMaxOutput <= 0 || monitorMaxFilesystems <= 0 || monitorMaxInterfaces <= 0 {
		return nil, errors.New("monitoring limits must be positive and the stale window must not be shorter than the refresh interval")
	}

	maxSessionsPerUser, err := parseInt(lookup, "DROVIK_SSH_MAX_SESSIONS_PER_USER", 2)
	if err != nil {
		return nil, err
	}
	maxSessionsGlobal, err := parseInt(lookup, "DROVIK_SSH_MAX_SESSIONS_GLOBAL", 10)
	if err != nil {
		return nil, err
	}
	if maxSessionsPerUser <= 0 || maxSessionsGlobal <= 0 {
		return nil, errors.New("SSH session limits must be greater than zero")
	}
	if maxSessionsPerUser > maxSessionsGlobal {
		maxSessionsPerUser = maxSessionsGlobal
	}

	dataDir, err := parseString(lookup, "DROVIK_DATA_DIR", "./data")
	if err != nil {
		return nil, err
	}
	dbPath, err := parseString(lookup, "DROVIK_DB_PATH", "./data/drovik.db")
	if err != nil {
		return nil, err
	}
	staticDir, err := parseString(lookup, "DROVIK_STATIC_DIR", "../frontend/dist")
	if err != nil {
		return nil, err
	}

	key, err := loadEncryptionKey(lookup)
	if err != nil {
		return nil, err
	}

	return &Config{
		Env:                     env,
		HTTPAddr:                httpAddr,
		PublicURL:               publicURL,
		TrustedOrigins:          origins,
		LogLevel:                logLevel,
		LogFormat:               logFormat,
		DataDir:                 filepath.Clean(dataDir),
		DBPath:                  filepath.Clean(dbPath),
		StaticDir:               filepath.Clean(staticDir),
		EncryptionKey:           key,
		SessionIdleTTL:          idleTTL,
		SessionAbsoluteTTL:      absoluteTTL,
		CookieSecure:            cookieSecure,
		LoginRateLimit:          loginLimit,
		LoginRateWindow:         loginWindow,
		ShutdownTimeout:         shutdownTimeout,
		ReadinessTimeout:        readinessTimeout,
		SSHDialTimeout:          sshDialTimeout,
		SSHHandshakeTimeout:     sshHandshakeTimeout,
		SSHOperationTimeout:     sshOperationTimeout,
		EgressAllowPrivate:      egressAllowPrivate,
		EgressAllowedPrefixes:   egressAllowed,
		EgressDeniedPrefixes:    egressDenied,
		SSHMaxSessionsPerUser:   maxSessionsPerUser,
		SSHMaxSessionsGlobal:    maxSessionsGlobal,
		SFTPMaxUploadBytes:      sftpMaxUpload,
		SFTPMaxEditorBytes:      sftpMaxEditor,
		SFTPMaxListEntries:      sftpMaxList,
		SFTPMaxRecursiveEntries: sftpMaxRecursive,
		SFTPOperationTimeout:    sftpTimeout,
		MonitorRefreshInterval:  monitorRefresh,
		MonitorStaleAfter:       monitorStaleAfter,
		MonitorTimeout:          monitorTimeout,
		MonitorMaxOutputBytes:   monitorMaxOutput,
		MonitorMaxFilesystems:   monitorMaxFilesystems,
		MonitorMaxInterfaces:    monitorMaxInterfaces,
	}, nil
}

func parseOrigins(lookup func(string) (string, bool), publicURL *url.URL) ([]string, error) {
	raw, err := parseString(lookup, "DROVIK_TRUSTED_ORIGINS", "")
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	origins := make([]string, 0, 4)

	add := func(value string) error {
		value = strings.TrimRight(strings.TrimSpace(value), "/")
		if value == "" {
			return nil
		}
		u, err := url.Parse(value)
		if err != nil {
			return fmt.Errorf("trusted origin %q is invalid: %w", value, err)
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return fmt.Errorf("trusted origin %q must use http or https", value)
		}
		if u.Host == "" {
			return fmt.Errorf("trusted origin %q must include a host", value)
		}
		if u.Path != "" && u.Path != "/" {
			return fmt.Errorf("trusted origin %q must not include a path", value)
		}
		origin := u.Scheme + "://" + u.Host
		if _, ok := seen[origin]; !ok {
			seen[origin] = struct{}{}
			origins = append(origins, origin)
		}
		return nil
	}

	if err := add(publicURL.Scheme + "://" + publicURL.Host); err != nil {
		return nil, err
	}
	for _, part := range strings.Split(raw, ",") {
		if err := add(part); err != nil {
			return nil, err
		}
	}
	return origins, nil
}

func loadEncryptionKey(lookup func(string) (string, bool)) ([]byte, error) {
	encoded, err := parseString(lookup, "DROVIK_ENCRYPTION_KEY", "")
	if err != nil {
		return nil, err
	}
	if encoded != "" {
		key, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, errors.New("DROVIK_ENCRYPTION_KEY must be valid base64")
		}
		if len(key) != 32 {
			return nil, fmt.Errorf("DROVIK_ENCRYPTION_KEY must decode to 32 bytes, got %d", len(key))
		}
		return key, nil
	}

	path, err := parseString(lookup, "DROVIK_ENCRYPTION_KEY_FILE", "")
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, errors.New("DROVIK_ENCRYPTION_KEY or DROVIK_ENCRYPTION_KEY_FILE is required")
	}
	//nolint:gosec // key file path comes from operator-controlled configuration
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read DROVIK_ENCRYPTION_KEY_FILE: %w", err)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil {
		return nil, errors.New("DROVIK_ENCRYPTION_KEY_FILE must contain valid base64")
	}
	if len(decoded) != 32 {
		return nil, fmt.Errorf("DROVIK_ENCRYPTION_KEY_FILE must decode to 32 bytes, got %d", len(decoded))
	}
	return decoded, nil
}

func parseString(lookup func(string) (string, bool), key, fallback string) (string, error) {
	value, ok := lookup(key)
	if !ok {
		return fallback, nil
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, nil
	}
	if strings.ContainsRune(value, '\x00') {
		return "", fmt.Errorf("%s contains a NUL byte", key)
	}
	return value, nil
}

func parseBool(lookup func(string) (string, bool), key string, fallback bool) (bool, error) {
	value, ok := lookup(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return false, fmt.Errorf("%s must be true or false", key)
	}
	return parsed, nil
}

func parseInt(lookup func(string) (string, bool), key string, fallback int) (int, error) {
	value, ok := lookup(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", key)
	}
	return parsed, nil
}

func parsePrefixes(lookup func(string) (string, bool), key string) ([]netip.Prefix, error) {
	raw, err := parseString(lookup, key, "")
	if err != nil {
		return nil, err
	}
	if raw == "" {
		return nil, nil
	}
	prefixes := make([]netip.Prefix, 0, 4)
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(part)
		if err != nil {
			return nil, fmt.Errorf("%s contains an invalid CIDR %q", key, part)
		}
		prefixes = append(prefixes, prefix.Masked())
	}
	return prefixes, nil
}

func parseInt64(lookup func(string) (string, bool), key string, fallback int64) (int64, error) {
	value, ok := lookup(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", key)
	}
	return parsed, nil
}

func parseDuration(lookup func(string) (string, bool), key string, fallback time.Duration) (time.Duration, error) {
	value, ok := lookup(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration such as 12h or 30m", key)
	}
	return parsed, nil
}

func loadDotEnv(path string) error {
	//nolint:gosec // dotenv path is local development configuration
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("set %s from %s: %w", key, path, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	return nil
}
