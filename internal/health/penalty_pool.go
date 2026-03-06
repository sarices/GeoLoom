package health

import (
	"sync"
	"time"
)

// CandidateStats 表示单轮候选过滤统计。
type CandidateStats struct {
	Filtered             []string
	PenalizedCount       int
	AllPenalizedFallback bool
}

// PenaltyPool 维护节点惩罚窗口状态。
type PenaltyPool struct {
	nowFunc   func() time.Time
	window    time.Duration
	mu        sync.RWMutex
	penalties map[string]time.Time
}

func NewPenaltyPool(window time.Duration) *PenaltyPool {
	if window <= 0 {
		window = 5 * time.Minute
	}
	return &PenaltyPool{
		nowFunc:   time.Now,
		window:    window,
		penalties: make(map[string]time.Time),
	}
}

func newPenaltyPoolWithNow(window time.Duration, nowFunc func() time.Time) *PenaltyPool {
	pool := NewPenaltyPool(window)
	if nowFunc != nil {
		pool.nowFunc = nowFunc
	}
	return pool
}

// MarkFailure 将节点标记为惩罚状态。
func (p *PenaltyPool) MarkFailure(nodeID string) {
	if nodeID == "" {
		return
	}
	until := p.nowFunc().Add(p.window)
	p.mu.Lock()
	p.penalties[nodeID] = until
	p.mu.Unlock()
}

// MarkSuccess 解除节点惩罚状态。
func (p *PenaltyPool) MarkSuccess(nodeID string) {
	if nodeID == "" {
		return
	}
	p.mu.Lock()
	delete(p.penalties, nodeID)
	p.mu.Unlock()
}

// IsPenalized 判断节点是否在惩罚窗口内。
func (p *PenaltyPool) IsPenalized(nodeID string) bool {
	if nodeID == "" {
		return false
	}
	now := p.nowFunc()

	p.mu.RLock()
	until, exists := p.penalties[nodeID]
	p.mu.RUnlock()
	if !exists {
		return false
	}
	if now.Before(until) {
		return true
	}

	p.mu.Lock()
	if currentUntil, ok := p.penalties[nodeID]; ok && !now.Before(currentUntil) {
		delete(p.penalties, nodeID)
	}
	p.mu.Unlock()
	return false
}

// FilterCandidates 过滤惩罚节点；若全部被惩罚，返回原始节点兜底。
func (p *PenaltyPool) FilterCandidates(nodes []string) []string {
	return p.FilterCandidatesWithStats(nodes).Filtered
}

// FilterCandidatesWithStats 返回过滤结果与统计信息。
func (p *PenaltyPool) FilterCandidatesWithStats(nodes []string) CandidateStats {
	if len(nodes) == 0 {
		return CandidateStats{}
	}

	filtered := make([]string, 0, len(nodes))
	penalizedCount := 0
	for _, node := range nodes {
		if p.IsPenalized(node) {
			penalizedCount++
			continue
		}
		filtered = append(filtered, node)
	}
	if len(filtered) == 0 {
		fallback := make([]string, 0, len(nodes))
		fallback = append(fallback, nodes...)
		return CandidateStats{
			Filtered:             fallback,
			PenalizedCount:       penalizedCount,
			AllPenalizedFallback: true,
		}
	}
	return CandidateStats{
		Filtered:             filtered,
		PenalizedCount:       penalizedCount,
		AllPenalizedFallback: false,
	}
}

// Snapshot 返回当前惩罚快照，便于调试与测试。
func (p *PenaltyPool) Snapshot() map[string]time.Time {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[string]time.Time, len(p.penalties))
	for key, value := range p.penalties {
		result[key] = value
	}
	return result
}
