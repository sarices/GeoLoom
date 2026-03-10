package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"geoloom/internal/health"
)

func TestStoreLoadSaveRoundTrip(t *testing.T) {
	t.Parallel()
	store := NewStore(filepath.Join(t.TempDir(), "state.json"))
	now := time.Date(2026, 3, 9, 10, 0, 0, 0, time.UTC)
	want := Snapshot{
		PenaltyUntil: map[string]time.Time{"node-a": now.Add(time.Minute)},
		NodeStatuses: map[string]health.NodeStatus{"node-a": {
			LastCheckAt:         now,
			LastReachable:       true,
			LastSuccessAt:       now,
			ConsecutiveFailures: 0,
			SuccessCount:        2,
			FailureCount:        1,
			Score:               93,
		}},
		LastCountryCode: map[string]string{"node-a": "JP"},
	}
	if err := store.Save(want); err != nil {
		t.Fatalf("Save 返回错误: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load 返回错误: %v", err)
	}
	if got.LastCountryCode["node-a"] != "JP" || got.PenaltyUntil["node-a"].IsZero() {
		t.Fatalf("状态恢复不匹配: %+v", got)
	}
	if got.NodeStatuses["node-a"].Score != 93 || got.NodeStatuses["node-a"].SuccessCount != 2 {
		t.Fatalf("NodeStatus 扩展字段恢复不匹配: %+v", got.NodeStatuses["node-a"])
	}
}

func TestStoreLoadShouldReturnEmptySnapshotWhenFileMissing(t *testing.T) {
	t.Parallel()
	got, err := NewStore(filepath.Join(t.TempDir(), "missing.json")).Load()
	if err != nil {
		t.Fatalf("缺失文件不应返回错误: %v", err)
	}
	if len(got.PenaltyUntil) != 0 || len(got.NodeStatuses) != 0 || len(got.LastCountryCode) != 0 {
		t.Fatalf("缺失文件应返回空快照: %+v", got)
	}
}

func TestStoreLoadShouldReturnEmptySnapshotWhenFileEmpty(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "empty.json")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("写入空文件失败: %v", err)
	}
	got, err := NewStore(path).Load()
	if err != nil {
		t.Fatalf("空文件不应返回错误: %v", err)
	}
	if len(got.PenaltyUntil) != 0 || len(got.NodeStatuses) != 0 || len(got.LastCountryCode) != 0 {
		t.Fatalf("空文件应返回空快照: %+v", got)
	}
}

func TestStoreLoadAndSaveShouldNoopWhenPathEmpty(t *testing.T) {
	t.Parallel()
	store := NewStore("")
	if err := store.Save(Snapshot{PenaltyUntil: map[string]time.Time{"node-a": time.Now().Add(time.Minute)}}); err != nil {
		t.Fatalf("空路径 Save 不应报错: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("空路径 Load 不应报错: %v", err)
	}
	if len(got.PenaltyUntil) != 0 || len(got.NodeStatuses) != 0 || len(got.LastCountryCode) != 0 {
		t.Fatalf("空路径应返回空快照: %+v", got)
	}
}

func TestStoreLoadBrokenFileShouldFail(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "broken.json")
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatalf("写入损坏文件失败: %v", err)
	}
	_, err := NewStore(path).Load()
	if err == nil {
		t.Fatal("预期损坏文件返回错误")
	}
}

func TestStoreLoadShouldDropExpiredPenalty(t *testing.T) {
	t.Parallel()
	store := NewStore(filepath.Join(t.TempDir(), "state.json"))
	past := time.Now().Add(-time.Minute)
	if err := store.Save(Snapshot{PenaltyUntil: map[string]time.Time{"expired": past}}); err != nil {
		t.Fatalf("Save 返回错误: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load 返回错误: %v", err)
	}
	if len(got.PenaltyUntil) != 0 {
		t.Fatalf("过期惩罚应被清理: %+v", got.PenaltyUntil)
	}
}

func TestStoreLoadShouldAcceptLegacyNodeStatusJSON(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "legacy.json")
	content := []byte(`{"penalty_until":{},"node_statuses":{"node-a":{"last_check_at":"2026-03-09T10:00:00Z","last_reachable":true}},"last_country_code":{}}`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("写入 legacy 文件失败: %v", err)
	}
	got, err := NewStore(path).Load()
	if err != nil {
		t.Fatalf("legacy JSON 不应加载失败: %v", err)
	}
	if got.NodeStatuses["node-a"].LastReachable != true {
		t.Fatalf("legacy NodeStatus 加载错误: %+v", got.NodeStatuses)
	}
}
