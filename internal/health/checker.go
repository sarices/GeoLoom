package health

import (
	"context"
	"log/slog"
	"slices"
	"sync"
	"time"

	"geoloom/internal/domain"

	"github.com/sagernet/sing-box/common/urltest"
)

// ProbeDoer 抽象单节点探测能力，便于替换与测试。
type ProbeDoer interface {
	Probe(ctx context.Context, testURL string, node domain.NodeMetadata) (bool, error)
}

type defaultProbeDoer struct{}

func (d *defaultProbeDoer) Probe(ctx context.Context, testURL string, node domain.NodeMetadata) (bool, error) {
	if node.Address == "" || node.Port <= 0 {
		return false, nil
	}

	result, err := urltest.URLTest(ctx, testURL, staticDialer{node: node})
	if err != nil {
		return false, err
	}
	return result > 0, nil
}

// RebuildFunc 在候选集合变化时触发重建。
type RebuildFunc func(ctx context.Context, candidates []domain.NodeMetadata) error

// Checker 负责健康检查、惩罚更新与重建触发。
type Checker struct {
	interval  time.Duration
	debounce  time.Duration
	testURL   string
	timeout   time.Duration
	doer      ProbeDoer
	pool      *PenaltyPool
	nowFunc   func() time.Time
	rebuildFn RebuildFunc

	mu             sync.Mutex
	lastCandidates []string
	lastRebuildAt  time.Time
}

type candidateStats struct {
	Nodes                []domain.NodeMetadata
	PenalizedNodes       int
	AllPenalizedFallback bool
}

func NewChecker(interval time.Duration, testURL string, pool *PenaltyPool, rebuildFn RebuildFunc) *Checker {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	if testURL == "" {
		testURL = "http://cp.cloudflare.com"
	}
	if pool == nil {
		pool = NewPenaltyPool(5 * time.Minute)
	}

	return &Checker{
		interval:  interval,
		debounce:  3 * time.Second,
		testURL:   testURL,
		timeout:   5 * time.Second,
		doer:      &defaultProbeDoer{},
		pool:      pool,
		nowFunc:   time.Now,
		rebuildFn: rebuildFn,
	}
}

func newCheckerWithDeps(interval time.Duration, testURL string, pool *PenaltyPool, rebuildFn RebuildFunc, doer ProbeDoer, nowFunc func() time.Time) *Checker {
	checker := NewChecker(interval, testURL, pool, rebuildFn)
	if doer != nil {
		checker.doer = doer
	}
	if nowFunc != nil {
		checker.nowFunc = nowFunc
	}
	return checker
}

func (c *Checker) Start(ctx context.Context, nodes []domain.NodeMetadata) {
	if len(nodes) == 0 {
		return
	}
	go c.run(ctx, nodes)
}

func (c *Checker) run(ctx context.Context, nodes []domain.NodeMetadata) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	c.evaluateAndRebuild(ctx, nodes)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.evaluateAndRebuild(ctx, nodes)
		}
	}
}

func (c *Checker) evaluateAndRebuild(ctx context.Context, nodes []domain.NodeMetadata) {
	c.updatePenalties(ctx, nodes)

	candidateStats := c.filterNodeStats(nodes)
	c.logAvailabilityStats(len(nodes), candidateStats)
	candidates := candidateStats.Nodes
	if len(candidates) == 0 {
		return
	}

	candidateIDs := nodeIDs(candidates)
	if !c.shouldRebuild(candidateIDs) {
		return
	}

	if c.rebuildFn != nil {
		_ = c.rebuildFn(ctx, candidates)
	}
}

func (c *Checker) updatePenalties(ctx context.Context, nodes []domain.NodeMetadata) {
	for _, node := range nodes {
		nodeCtx, cancel := context.WithTimeout(ctx, c.timeout)
		reachable, _ := c.doer.Probe(nodeCtx, c.testURL, node)
		cancel()

		if reachable {
			c.pool.MarkSuccess(node.ID)
		} else {
			c.pool.MarkFailure(node.ID)
		}
	}
}

func (c *Checker) filterNodeStats(nodes []domain.NodeMetadata) candidateStats {
	if len(nodes) == 0 {
		return candidateStats{}
	}

	ids := make([]string, 0, len(nodes))
	nodeByID := make(map[string]domain.NodeMetadata, len(nodes))
	for _, node := range nodes {
		ids = append(ids, node.ID)
		nodeByID[node.ID] = node
	}

	filtered := c.pool.FilterCandidatesWithStats(ids)
	available := make([]domain.NodeMetadata, 0, len(filtered.Filtered))
	for _, id := range filtered.Filtered {
		if node, ok := nodeByID[id]; ok {
			available = append(available, node)
		}
	}
	return candidateStats{
		Nodes:                available,
		PenalizedNodes:       filtered.PenalizedCount,
		AllPenalizedFallback: filtered.AllPenalizedFallback,
	}
}

func (c *Checker) logAvailabilityStats(totalNodes int, stats candidateStats) {
	slog.Info("健康检查周期统计",
		"total_nodes", totalNodes,
		"available_proxy_nodes", len(stats.Nodes),
		"penalized_nodes", stats.PenalizedNodes,
		"all_penalized_fallback", stats.AllPenalizedFallback,
	)
}

func (c *Checker) shouldRebuild(candidateIDs []string) bool {
	now := c.nowFunc()

	c.mu.Lock()
	defer c.mu.Unlock()

	if slices.Equal(c.lastCandidates, candidateIDs) {
		return false
	}
	if !c.lastRebuildAt.IsZero() && now.Sub(c.lastRebuildAt) < c.debounce {
		return false
	}

	c.lastCandidates = append([]string(nil), candidateIDs...)
	c.lastRebuildAt = now
	return true
}

func nodeIDs(nodes []domain.NodeMetadata) []string {
	ids := make([]string, 0, len(nodes))
	for _, node := range nodes {
		ids = append(ids, node.ID)
	}
	return ids
}
