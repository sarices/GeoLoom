package health

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"geoloom/internal/domain"
)

type stubDoer struct {
	mu       sync.Mutex
	statuses map[string]int
	failing  map[string]bool
}

func (s *stubDoer) Probe(_ context.Context, _ string, node domain.NodeMetadata) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	host := node.Address
	if s.failing[host] {
		return false, errors.New("dial failed")
	}
	status := s.statuses[host]
	if status == 0 {
		status = 204
	}
	return status >= 200 && status < 500, nil
}

func TestCheckerEvaluateAndRebuild(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC)
	pool := newPenaltyPoolWithNow(5*time.Minute, func() time.Time { return now })
	doer := &stubDoer{
		statuses: map[string]int{"good.example.com": 204, "bad.example.com": 204},
		failing:  map[string]bool{"bad.example.com": true},
	}

	rebuildCalls := 0
	var gotCandidates [][]string
	checker := newCheckerWithDeps(30*time.Second, "http://cp.cloudflare.com", pool, func(_ context.Context, candidates []domain.NodeMetadata) error {
		rebuildCalls++
		ids := make([]string, 0, len(candidates))
		for _, node := range candidates {
			ids = append(ids, node.ID)
		}
		gotCandidates = append(gotCandidates, ids)
		return nil
	}, doer, func() time.Time { return now })
	checker.debounce = 0

	nodes := []domain.NodeMetadata{
		{ID: "good.example.com", Address: "good.example.com", Port: 443},
		{ID: "bad.example.com", Address: "bad.example.com", Port: 443},
	}

	stats := checker.filterNodeStats(nodes)
	if stats.PenalizedNodes != 0 {
		t.Fatalf("初始惩罚数量应为 0: got=%d", stats.PenalizedNodes)
	}

	checker.evaluateAndRebuild(context.Background(), nodes)
	if rebuildCalls != 1 {
		t.Fatalf("重建次数错误: got=%d want=1", rebuildCalls)
	}
	if len(gotCandidates) != 1 || len(gotCandidates[0]) != 1 || gotCandidates[0][0] != "good.example.com" {
		t.Fatalf("候选集合错误: %+v", gotCandidates)
	}

	now = now.Add(1 * time.Second)
	checker.evaluateAndRebuild(context.Background(), nodes)
	if rebuildCalls != 1 {
		t.Fatalf("候选未变化时不应重复重建: got=%d", rebuildCalls)
	}

	doer.mu.Lock()
	doer.failing["bad.example.com"] = false
	doer.mu.Unlock()
	now = now.Add(6 * time.Minute)
	checker.evaluateAndRebuild(context.Background(), nodes)
	if rebuildCalls != 2 {
		t.Fatalf("节点恢复后应重建: got=%d want=2", rebuildCalls)
	}
}

func TestCheckerDebounce(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC)
	pool := newPenaltyPoolWithNow(5*time.Minute, func() time.Time { return now })
	doer := &stubDoer{
		statuses: map[string]int{"n1.example.com": 204, "n2.example.com": 204},
		failing:  map[string]bool{"n2.example.com": true},
	}

	rebuildCalls := 0
	checker := newCheckerWithDeps(30*time.Second, "http://cp.cloudflare.com", pool, func(_ context.Context, _ []domain.NodeMetadata) error {
		rebuildCalls++
		return nil
	}, doer, func() time.Time { return now })
	checker.debounce = 5 * time.Second

	nodes := []domain.NodeMetadata{
		{ID: "n1.example.com", Address: "n1.example.com", Port: 443},
		{ID: "n2.example.com", Address: "n2.example.com", Port: 443},
	}

	checker.evaluateAndRebuild(context.Background(), nodes)
	if rebuildCalls != 1 {
		t.Fatalf("首次重建次数错误: got=%d", rebuildCalls)
	}

	doer.mu.Lock()
	doer.failing["n1.example.com"] = true
	doer.failing["n2.example.com"] = true
	doer.mu.Unlock()
	now = now.Add(2 * time.Second)
	checker.evaluateAndRebuild(context.Background(), nodes)
	if rebuildCalls != 1 {
		t.Fatalf("防抖窗口内不应重建: got=%d", rebuildCalls)
	}

	now = now.Add(5 * time.Second)
	doer.mu.Lock()
	doer.failing["n1.example.com"] = false
	doer.failing["n2.example.com"] = false
	doer.mu.Unlock()
	checker.evaluateAndRebuild(context.Background(), nodes)
	if rebuildCalls != 2 {
		t.Fatalf("超过防抖窗口后应重建: got=%d", rebuildCalls)
	}
}
