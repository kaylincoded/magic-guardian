package webui

import (
	"bytes"
	"context"
	"log/slog"
	"sync"
)

// MultiHandler is a slog.Handler that writes to multiple handlers and a LogBuffer.
type MultiHandler struct {
	inner  slog.Handler
	buffer *LogBuffer
	mu     sync.Mutex
}

// NewMultiHandler creates a handler that writes to both a standard handler and a log buffer.
func NewMultiHandler(inner slog.Handler, buffer *LogBuffer) *MultiHandler {
	return &MultiHandler{
		inner:  inner,
		buffer: buffer,
	}
}

func (h *MultiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *MultiHandler) Handle(ctx context.Context, r slog.Record) error {
	// Write to original handler
	err := h.inner.Handle(ctx, r)

	// Also format and write to buffer
	h.mu.Lock()
	defer h.mu.Unlock()
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	handler.Handle(ctx, r)
	if buf.Len() > 0 {
		// Trim trailing newline
		line := buf.String()
		if len(line) > 0 && line[len(line)-1] == '\n' {
			line = line[:len(line)-1]
		}
		h.buffer.Write(line)
	}

	return err
}

func (h *MultiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &MultiHandler{
		inner:  h.inner.WithAttrs(attrs),
		buffer: h.buffer,
	}
}

func (h *MultiHandler) WithGroup(name string) slog.Handler {
	return &MultiHandler{
		inner:  h.inner.WithGroup(name),
		buffer: h.buffer,
	}
}
