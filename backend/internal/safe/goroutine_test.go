package safe

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

type captureHandler struct {
	mu      sync.Mutex
	buf     bytes.Buffer
	records chan struct{}
	once    sync.Once
}

func newCaptureHandler() *captureHandler {
	return &captureHandler{records: make(chan struct{})}
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *captureHandler) Handle(ctx context.Context, record slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := slog.NewTextHandler(&h.buf, nil).Handle(ctx, record); err != nil {
		return err
	}
	h.once.Do(func() { close(h.records) })
	return nil
}

func (h *captureHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *captureHandler) WithGroup(string) slog.Handler      { return h }

func (h *captureHandler) String() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.buf.String()
}

func TestGoRecoversFromPanic(t *testing.T) {
	handler := newCaptureHandler()
	logger := slog.New(handler)
	done := make(chan struct{})

	Go(context.Background(), logger, "test-goroutine", func(context.Context) {
		defer close(done)
		panic("boom")
	})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("goroutine did not finish")
	}

	select {
	case <-handler.records:
	case <-time.After(2 * time.Second):
		t.Fatal("panic was not logged")
	}

	out := handler.String()
	if !strings.Contains(out, "recovered from panic") || !strings.Contains(out, "boom") {
		t.Fatalf("panic was not logged: %s", out)
	}
}

func TestRecoverNoPanic(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	func() {
		defer Recover(logger, "no-panic")
	}()
	if buf.Len() != 0 {
		t.Fatalf("unexpected log output: %s", buf.String())
	}
}
