package health

import (
	"context"
	"log/slog"
	"slices"
	"sort"
	"strings"
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

// NodeStatus 表示健康检查的最新状态摘要。
type NodeStatus struct {
	LastCheckAt         time.Time `json:"last_check_at"`
	LastReachable       bool      `json:"last_reachable"`
	LastSuccessAt       time.Time `json:"last_success_at"`
	LastFailureAt       time.Time `json:"last_failure_at"`
	ConsecutiveFailures int       `json:"consecutive_failures"`
	SuccessCount        int       `json:"success_count"`
	FailureCount        int       `json:"failure_count"`
	Score               int       `json:"score"`
}

// HealthSnapshot 表示当前健康检查状态快照。
type HealthSnapshot struct {
	Interval       time.Duration         `json:"interval"`
	Debounce       time.Duration         `json:"debounce"`
	TestURL        string                `json:"test_url"`
	Timeout        time.Duration         `json:"timeout"`
	LastCandidates []string              `json:"last_candidates"`
	LastRebuildAt  time.Time             `json:"last_rebuild_at"`
	Nodes          map[string]NodeStatus `json:"nodes"`
}

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
	nodeStatuses   map[string]NodeStatus
	nodes          []domain.NodeMetadata
	started        bool
}

type candidateStats struct {
	Nodes                []domain.NodeMetadata
	PenalizedNodes       int
	DegradedNodes        int
	ReadyNodes           int
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
		interval:     interval,
		debounce:     3 * time.Second,
		testURL:      testURL,
		timeout:      5 * time.Second,
		doer:         &defaultProbeDoer{},
		pool:         pool,
		nowFunc:      time.Now,
		rebuildFn:    rebuildFn,
		nodeStatuses: make(map[string]NodeStatus),
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
	c.SetNodes(nodes)
	if len(nodes) == 0 {
		return
	}

	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return
	}
	c.started = true
	c.mu.Unlock()

	go c.run(ctx)
}

// SetNodes 更新当前健康检查候选集合，并为新节点初始化状态。
func (c *Checker) SetNodes(nodes []domain.NodeMetadata) {
	copied := append([]domain.NodeMetadata(nil), nodes...)
	c.mu.Lock()
	c.nodes = copied
	if c.nodeStatuses == nil {
		c.nodeStatuses = make(map[string]NodeStatus)
	}
	for _, node := range copied {
		if key := domain.NodeKey(node); key != "" {
			if _, exists := c.nodeStatuses[key]; !exists {
				c.nodeStatuses[key] = NodeStatus{Score: 100}
			}
		}
	}
	c.mu.Unlock()
}

func (c *Checker) run(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	c.evaluateAndRebuild(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.evaluateAndRebuild(ctx)
		}
	}
}

func (c *Checker) evaluateAndRebuild(ctx context.Context) {
	nodes := c.currentNodes()
	if len(nodes) == 0 {
		return
	}
	c.updatePenalties(ctx, nodes)

	candidateStats := c.filterNodeStats(nodes)
	c.logAvailabilityStats(len(nodes), candidateStats)
	candidates := candidateStats.Nodes
	if len(candidates) == 0 {
		return
	}

	candidateIDs := nodeKeys(candidates)
	candidateSignature := candidateSelectionSignature(candidates)
	if !c.shouldRebuild(candidateIDs, candidateSignature) {
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

		nodeKey := domain.NodeKey(node)
		if nodeKey == "" {
			continue
		}

		now := c.nowFunc()
		status := c.nodeStatus(nodeKey)
		status.LastCheckAt = now
		status.LastReachable = reachable
		if reachable {
			status.LastSuccessAt = now
			status.SuccessCount++
			status.ConsecutiveFailures = 0
			c.pool.MarkSuccess(nodeKey)
		} else {
			status.LastFailureAt = now
			status.FailureCount++
			status.ConsecutiveFailures++
			c.pool.MarkFailure(nodeKey)
		}
		status.Score = c.computeScore(status, c.pool.IsPenalized(nodeKey))
		c.recordNodeStatus(nodeKey, status)
	}
}

func (c *Checker) filterNodeStats(nodes []domain.NodeMetadata) candidateStats {
	if len(nodes) == 0 {
		return candidateStats{}
	}

	ids := make([]string, 0, len(nodes))
	nodeByID := make(map[string]domain.NodeMetadata, len(nodes))
	statuses := c.snapshotNodeStatuses()
	for _, node := range nodes {
		nodeKey := domain.NodeKey(node)
		if nodeKey == "" {
			continue
		}
		ids = append(ids, nodeKey)
		nodeByID[nodeKey] = node
	}

	filtered := c.pool.FilterCandidatesWithStats(ids)
	penaltyStates := c.pool.EvaluateCandidates(ids)
	penaltyByID := make(map[string]CandidatePenaltyState, len(penaltyStates))
	for _, state := range penaltyStates {
		penaltyByID[state.NodeKey] = state
	}

	available := make([]domain.NodeMetadata, 0, len(filtered.Filtered))
	for _, id := range filtered.Filtered {
		node, ok := nodeByID[id]
		if !ok {
			continue
		}
		status := statuses[id]
		node.HealthScore = status.Score
		available = append(available, node)
	}

	sort.SliceStable(available, func(i, j int) bool {
		left := available[i]
		right := available[j]
		leftPenalty := penaltyByID[domain.NodeKey(left)]
		rightPenalty := penaltyByID[domain.NodeKey(right)]
		if filtered.AllPenalizedFallback && leftPenalty.Penalized != rightPenalty.Penalized {
			return !leftPenalty.Penalized
		}
		if left.HealthScore != right.HealthScore {
			return left.HealthScore > right.HealthScore
		}
		return domain.NodeKey(left) < domain.NodeKey(right)
	})

	readyNodes := 0
	degradedNodes := 0
	for _, node := range available {
		state := penaltyByID[domain.NodeKey(node)]
		if state.Penalized {
			continue
		}
		if node.HealthScore >= 80 {
			readyNodes++
		} else {
			degradedNodes++
		}
	}

	return candidateStats{
		Nodes:                available,
		PenalizedNodes:       filtered.PenalizedCount,
		DegradedNodes:        degradedNodes,
		ReadyNodes:           readyNodes,
		AllPenalizedFallback: filtered.AllPenalizedFallback,
	}
}

func (c *Checker) logAvailabilityStats(totalNodes int, stats candidateStats) {
	slog.Info("健康检查周期统计",
		"total_nodes", totalNodes,
		"available_proxy_nodes", len(stats.Nodes),
		"ready_nodes", stats.ReadyNodes,
		"degraded_nodes", stats.DegradedNodes,
		"penalized_nodes", stats.PenalizedNodes,
		"all_penalized_fallback", stats.AllPenalizedFallback,
	)
}

func (c *Checker) shouldRebuild(candidateIDs []string, candidateSignature string) bool {
	now := c.nowFunc()

	c.mu.Lock()
	defer c.mu.Unlock()

	lastSignature := candidateSelectionSignatureFromIDs(c.lastCandidates, c.nodeStatuses)
	if slices.Equal(c.lastCandidates, candidateIDs) && lastSignature == candidateSignature {
		return false
	}
	if !c.lastRebuildAt.IsZero() && now.Sub(c.lastRebuildAt) < c.debounce {
		return false
	}

	c.lastCandidates = append([]string(nil), candidateIDs...)
	c.lastRebuildAt = now
	return true
}

func (c *Checker) currentNodes() []domain.NodeMetadata {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]domain.NodeMetadata(nil), c.nodes...)
}

func nodeKeys(nodes []domain.NodeMetadata) []string {
	ids := make([]string, 0, len(nodes))
	for _, node := range nodes {
		if key := domain.NodeKey(node); key != "" {
			ids = append(ids, key)
		}
	}
	return ids
}

func candidateSelectionSignature(nodes []domain.NodeMetadata) string {
	return candidateSelectionSignatureFromNodes(nodes)
}

func candidateSelectionSignatureFromNodes(nodes []domain.NodeMetadata) string {
	parts := make([]string, 0, len(nodes))
	for _, node := range nodes {
		key := domain.NodeKey(node)
		if key == "" {
			continue
		}
		parts = append(parts, key+"#"+scoreBucket(node.HealthScore))
	}
	return strings.Join(parts, "|")
}

func candidateSelectionSignatureFromIDs(ids []string, statuses map[string]NodeStatus) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, id+"#"+scoreBucket(statuses[id].Score))
	}
	return strings.Join(parts, "|")
}

func scoreBucket(score int) string {
	switch {
	case score >= 90:
		return "p3"
	case score >= 70:
		return "p2"
	case score >= 40:
		return "p1"
	default:
		return "p0"
	}
}

func (c *Checker) recordNodeStatus(nodeKey string, status NodeStatus) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.nodeStatuses == nil {
		c.nodeStatuses = make(map[string]NodeStatus)
	}
	c.nodeStatuses[nodeKey] = status
}

func (c *Checker) nodeStatus(nodeKey string) NodeStatus {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.nodeStatuses == nil {
		c.nodeStatuses = make(map[string]NodeStatus)
	}
	status := c.nodeStatuses[nodeKey]
	if status.Score == 0 {
		status.Score = 100
	}
	return status
}

func (c *Checker) snapshotNodeStatuses() map[string]NodeStatus {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make(map[string]NodeStatus, len(c.nodeStatuses))
	for key, value := range c.nodeStatuses {
		result[key] = value
	}
	return result
}

func (c *Checker) computeScore(status NodeStatus, penalized bool) int {
	score := 100
	if !status.LastFailureAt.IsZero() && (status.LastSuccessAt.IsZero() || status.LastFailureAt.After(status.LastSuccessAt)) {
		score -= 20
	}
	score -= status.ConsecutiveFailures * 15
	score -= status.FailureCount * 5
	score += minInt(status.SuccessCount, 4) * 3
	if penalized {
		score -= 35
	}
	if score < 5 {
		return 5
	}
	if score > 100 {
		return 100
	}
	return score
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

// Snapshot 返回健康检查内部状态快照，供观测与持久化复用。
func (c *Checker) Snapshot() HealthSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()

	nodes := make(map[string]NodeStatus, len(c.nodeStatuses))
	for key, value := range c.nodeStatuses {
		nodes[key] = value
	}

	return HealthSnapshot{
		Interval:       c.interval,
		Debounce:       c.debounce,
		TestURL:        c.testURL,
		Timeout:        c.timeout,
		LastCandidates: append([]string(nil), c.lastCandidates...),
		LastRebuildAt:  c.lastRebuildAt,
		Nodes:          nodes,
	}
}

// RestoreSnapshot 恢复检查器状态，供重启冷启动复用。
func (c *Checker) RestoreSnapshot(snapshot HealthSnapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.lastCandidates = append([]string(nil), snapshot.LastCandidates...)
	c.lastRebuildAt = snapshot.LastRebuildAt
	c.nodeStatuses = make(map[string]NodeStatus, len(snapshot.Nodes))
	for key, value := range snapshot.Nodes {
		if key == "" {
			continue
		}
		c.nodeStatuses[key] = value
	}
}
