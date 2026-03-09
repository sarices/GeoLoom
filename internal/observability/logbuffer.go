package observability

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// LogEntry 表示一条可供 API 返回的最近日志。
type LogEntry struct {
	Time    time.Time      `json:"time"`
	Level   string         `json:"level"`
	Message string         `json:"message"`
	Attrs   map[string]any `json:"attrs"`
	Text    string         `json:"text"`
}

// LogBuffer 提供线程安全的最近日志环形缓冲。
type LogBuffer struct {
	mu        sync.RWMutex
	entries   []LogEntry
	next      int
	size      int
	truncated bool
}

func NewLogBuffer(capacity int) *LogBuffer {
	if capacity <= 0 {
		capacity = 1
	}
	return &LogBuffer{entries: make([]LogEntry, capacity)}
}

func (b *LogBuffer) Capacity() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.entries)
}

func (b *LogBuffer) Append(entry LogEntry) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.entries) == 0 {
		return
	}
	b.entries[b.next] = cloneEntry(entry)
	b.next = (b.next + 1) % len(b.entries)
	if b.size < len(b.entries) {
		b.size++
		return
	}
	b.truncated = true
}

func (b *LogBuffer) Snapshot() (items []LogEntry, count int, capacity int, truncated bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	capacity = len(b.entries)
	count = b.size
	truncated = b.truncated
	items = make([]LogEntry, 0, b.size)
	if b.size == 0 {
		return items, count, capacity, truncated
	}
	start := 0
	if b.size == len(b.entries) {
		start = b.next
	}
	for i := 0; i < b.size; i++ {
		idx := (start + i) % len(b.entries)
		items = append(items, cloneEntry(b.entries[idx]))
	}
	return items, count, capacity, truncated
}

func cloneEntry(entry LogEntry) LogEntry {
	copied := entry
	if entry.Attrs != nil {
		copied.Attrs = make(map[string]any, len(entry.Attrs))
		for key, value := range entry.Attrs {
			copied.Attrs[key] = value
		}
	}
	return copied
}

var (
	defaultBufferMu sync.RWMutex
	defaultBuffer   *LogBuffer
)

func SetDefaultLogBuffer(buffer *LogBuffer) {
	defaultBufferMu.Lock()
	defer defaultBufferMu.Unlock()
	defaultBuffer = buffer
}

func DefaultLogBuffer() *LogBuffer {
	defaultBufferMu.RLock()
	defer defaultBufferMu.RUnlock()
	return defaultBuffer
}

// TeeHandler 将日志同时写入下游 handler 与内存缓冲。
type TeeHandler struct {
	next   slog.Handler
	buffer *LogBuffer
	attrs  []slog.Attr
	groups []string
}

func NewTeeHandler(next slog.Handler, buffer *LogBuffer) *TeeHandler {
	return &TeeHandler{next: next, buffer: buffer}
}

func (h *TeeHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *TeeHandler) Handle(ctx context.Context, record slog.Record) error {
	if h.buffer != nil {
		h.buffer.Append(buildLogEntry(record, h.attrs, h.groups))
	}
	return h.next.Handle(ctx, record)
}

func (h *TeeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	combined := append([]slog.Attr(nil), h.attrs...)
	combined = append(combined, attrs...)
	return &TeeHandler{next: h.next.WithAttrs(attrs), buffer: h.buffer, attrs: combined, groups: append([]string(nil), h.groups...)}
}

func (h *TeeHandler) WithGroup(name string) slog.Handler {
	groups := append([]string(nil), h.groups...)
	groups = append(groups, name)
	return &TeeHandler{next: h.next.WithGroup(name), buffer: h.buffer, attrs: append([]slog.Attr(nil), h.attrs...), groups: groups}
}

func buildLogEntry(record slog.Record, baseAttrs []slog.Attr, groups []string) LogEntry {
	attrs := make(map[string]any)
	parts := make([]string, 0, len(baseAttrs)+record.NumAttrs())
	for _, attr := range baseAttrs {
		appendAttr(attrs, &parts, groups, attr)
	}
	record.Attrs(func(attr slog.Attr) bool {
		appendAttr(attrs, &parts, groups, attr)
		return true
	})
	text := fmt.Sprintf("%s %-5s %s", record.Time.Format(time.RFC3339), strings.ToUpper(record.Level.String()), record.Message)
	if len(parts) > 0 {
		text += " " + strings.Join(parts, " ")
	}
	return LogEntry{
		Time:    record.Time,
		Level:   strings.ToUpper(record.Level.String()),
		Message: record.Message,
		Attrs:   attrs,
		Text:    text,
	}
}

func appendAttr(target map[string]any, parts *[]string, groups []string, attr slog.Attr) {
	attr.Value = attr.Value.Resolve()
	if attr.Equal(slog.Attr{}) {
		return
	}
	if attr.Value.Kind() == slog.KindGroup {
		nextGroups := append(append([]string(nil), groups...), attr.Key)
		for _, child := range attr.Value.Group() {
			appendAttr(target, parts, nextGroups, child)
		}
		return
	}
	key := attr.Key
	if len(groups) > 0 {
		key = strings.Join(append(append([]string(nil), groups...), attr.Key), ".")
	}
	value := attrValue(attr.Value)
	target[key] = value
	*parts = append(*parts, fmt.Sprintf("%s=%v", key, value))
}

func attrValue(value slog.Value) any {
	value = value.Resolve()
	switch value.Kind() {
	case slog.KindBool:
		return value.Bool()
	case slog.KindDuration:
		return value.Duration().String()
	case slog.KindFloat64:
		return value.Float64()
	case slog.KindInt64:
		return value.Int64()
	case slog.KindString:
		return value.String()
	case slog.KindTime:
		return value.Time().Format(time.RFC3339Nano)
	case slog.KindUint64:
		return value.Uint64()
	case slog.KindGroup:
		items := make(map[string]any)
		parts := make([]string, 0)
		for _, attr := range value.Group() {
			appendAttr(items, &parts, nil, attr)
		}
		return items
	case slog.KindAny:
		return value.Any()
	default:
		return value.String()
	}
}
