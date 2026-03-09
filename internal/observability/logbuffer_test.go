package observability

import (
	"log/slog"
	"testing"
)

func TestLogBufferShouldKeepLatestEntries(t *testing.T) {
	t.Parallel()

	buffer := NewLogBuffer(3)
	buffer.Append(LogEntry{Message: "one", Text: "one"})
	buffer.Append(LogEntry{Message: "two", Text: "two"})
	buffer.Append(LogEntry{Message: "three", Text: "three"})
	buffer.Append(LogEntry{Message: "four", Text: "four"})

	items, count, capacity, truncated := buffer.Snapshot()
	if count != 3 {
		t.Fatalf("count 错误: got=%d want=3", count)
	}
	if capacity != 3 {
		t.Fatalf("capacity 错误: got=%d want=3", capacity)
	}
	if !truncated {
		t.Fatal("写满后应标记 truncated=true")
	}
	if got := []string{items[0].Message, items[1].Message, items[2].Message}; got[0] != "two" || got[1] != "three" || got[2] != "four" {
		t.Fatalf("保留最新日志错误: got=%v", got)
	}
}

func TestTeeHandlerShouldCaptureStructuredAttrs(t *testing.T) {
	t.Parallel()

	buffer := NewLogBuffer(4)
	handler := NewTeeHandler(slog.NewTextHandler(testWriter{}, nil), buffer)
	logger := slog.New(handler).With("component", "runtime")
	logger.Info("刷新完成", "count", 2, "ok", true)

	items, count, _, _ := buffer.Snapshot()
	if count != 1 || len(items) != 1 {
		t.Fatalf("日志条数错误: count=%d len=%d", count, len(items))
	}
	entry := items[0]
	if entry.Message != "刷新完成" {
		t.Fatalf("message 错误: got=%s", entry.Message)
	}
	if entry.Attrs["component"] != "runtime" {
		t.Fatalf("component attr 错误: %+v", entry.Attrs)
	}
	if entry.Attrs["count"] != int64(2) {
		t.Fatalf("count attr 错误: %+v", entry.Attrs)
	}
	if entry.Attrs["ok"] != true {
		t.Fatalf("ok attr 错误: %+v", entry.Attrs)
	}
	if entry.Text == "" {
		t.Fatal("text 不应为空")
	}
}

type testWriter struct{}

func (testWriter) Write(p []byte) (int, error) { return len(p), nil }
