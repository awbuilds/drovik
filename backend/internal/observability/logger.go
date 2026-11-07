package observability

import (
	"context"
	"io"
	"log/slog"
	"strings"
)

const redacted = "[REDACTED]"

// New returns a structured logger with secret redaction applied.
func New(level, format string, w io.Writer) (*slog.Logger, error) {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "info":
		lvl = slog.LevelInfo
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: lvl}
	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(w, opts)
	} else {
		handler = slog.NewTextHandler(w, opts)
	}
	return slog.New(&RedactingHandler{next: handler}), nil
}

// RedactingHandler removes secret-like values before they reach the sink.
type RedactingHandler struct {
	next slog.Handler
}

func (h *RedactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *RedactingHandler) Handle(ctx context.Context, record slog.Record) error {
	clean := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
	record.Attrs(func(attr slog.Attr) bool {
		clean.AddAttrs(redactAttr(attr))
		return true
	})
	return h.next.Handle(ctx, clean)
}

func (h *RedactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clean := make([]slog.Attr, 0, len(attrs))
	for _, attr := range attrs {
		clean = append(clean, redactAttr(attr))
	}
	return &RedactingHandler{next: h.next.WithAttrs(clean)}
}

func (h *RedactingHandler) WithGroup(name string) slog.Handler {
	return &RedactingHandler{next: h.next.WithGroup(name)}
}

func redactAttr(attr slog.Attr) slog.Attr {
	if attr.Equal(slog.Attr{}) {
		return attr
	}
	if attr.Value.Kind() == slog.KindGroup {
		group := attr.Value.Group()
		clean := make([]slog.Attr, 0, len(group))
		for _, child := range group {
			clean = append(clean, redactAttr(child))
		}
		return slog.Group(attr.Key, attrsToAny(clean)...)
	}
	if sensitiveKey(attr.Key) || containsPEM(attr.Value) {
		return slog.String(attr.Key, redacted)
	}
	return attr
}

func attrsToAny(attrs []slog.Attr) []any {
	values := make([]any, 0, len(attrs))
	for _, attr := range attrs {
		values = append(values, attr)
	}
	return values
}

func containsPEM(value slog.Value) bool {
	if value.Kind() != slog.KindString {
		return false
	}
	text := value.String()
	return strings.Contains(text, "-----BEGIN") && strings.Contains(text, "-----END")
}

func sensitiveKey(key string) bool {
	normalized := strings.ToLower(key)
	replacer := strings.NewReplacer("-", "", "_", "", ".", "", " ", "")
	normalized = replacer.Replace(normalized)

	switch {
	case strings.Contains(normalized, "password"):
		return true
	case strings.Contains(normalized, "passphrase"):
		return true
	case strings.Contains(normalized, "secret"):
		return true
	case strings.Contains(normalized, "token"):
		return true
	case strings.Contains(normalized, "authorization"):
		return true
	case strings.Contains(normalized, "cookie"):
		return true
	case strings.Contains(normalized, "privatekey"):
		return true
	case strings.Contains(normalized, "apikey"):
		return true
	case strings.Contains(normalized, "credential"):
		return true
	case normalized == "key" || strings.HasSuffix(normalized, "key") || strings.HasSuffix(normalized, "keys"):
		return true
	default:
		return false
	}
}
