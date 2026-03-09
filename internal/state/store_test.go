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
		PenaltyUntil:    map[string]time.Time{"node-a": now.Add(time.Minute)},
		NodeStatuses:    map[string]health.NodeStatus{"node-a": {LastCheckAt: now, LastReachable: true}},
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
