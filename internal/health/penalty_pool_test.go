package health

import (
	"testing"
	"time"
)

func TestPenaltyPoolMarkFailureAndExpire(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC)
	pool := newPenaltyPoolWithNow(5*time.Minute, func() time.Time { return now })

	pool.MarkFailure("node-a")
	if !pool.IsPenalized("node-a") {
		t.Fatal("预期 node-a 在惩罚窗口内")
	}

	now = now.Add(6 * time.Minute)
	if pool.IsPenalized("node-a") {
		t.Fatal("预期 node-a 已过惩罚窗口")
	}
}

func TestPenaltyPoolMarkSuccess(t *testing.T) {
	t.Parallel()

	pool := NewPenaltyPool(5 * time.Minute)
	pool.MarkFailure("node-a")
	if !pool.IsPenalized("node-a") {
		t.Fatal("预期 node-a 在惩罚窗口内")
	}

	pool.MarkSuccess("node-a")
	if pool.IsPenalized("node-a") {
		t.Fatal("预期 node-a 惩罚被清除")
	}
}

func TestPenaltyPoolFilterFallbackWhenAllPenalized(t *testing.T) {
	t.Parallel()

	pool := NewPenaltyPool(5 * time.Minute)
	pool.MarkFailure("node-a")
	pool.MarkFailure("node-b")

	stats := pool.FilterCandidatesWithStats([]string{"node-a", "node-b"})
	if len(stats.Filtered) != 2 {
		t.Fatalf("全惩罚兜底失败: got=%d want=2", len(stats.Filtered))
	}
	if !stats.AllPenalizedFallback {
		t.Fatal("全惩罚场景应标记 all_penalized_fallback=true")
	}
	if stats.PenalizedCount != 2 {
		t.Fatalf("惩罚数量错误: got=%d want=2", stats.PenalizedCount)
	}
}

func TestPenaltyPoolFilterCandidatesWithStatsPartialPenalty(t *testing.T) {
	t.Parallel()

	pool := NewPenaltyPool(5 * time.Minute)
	pool.MarkFailure("node-a")

	stats := pool.FilterCandidatesWithStats([]string{"node-a", "node-b"})
	if len(stats.Filtered) != 1 || stats.Filtered[0] != "node-b" {
		t.Fatalf("过滤结果错误: %+v", stats.Filtered)
	}
	if stats.AllPenalizedFallback {
		t.Fatal("部分可用场景不应触发 all_penalized_fallback")
	}
	if stats.PenalizedCount != 1 {
		t.Fatalf("惩罚数量错误: got=%d want=1", stats.PenalizedCount)
	}
}
