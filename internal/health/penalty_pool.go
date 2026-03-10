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

// CandidatePenaltyState 表示节点在当前轮次的惩罚评估结果。
type CandidatePenaltyState struct {
	NodeKey      string    `json:"node_key"`
	Penalized    bool      `json:"penalized"`
	PenaltyUntil time.Time `json:"penalty_until"`
	Fallback     bool      `json:"fallback"`
}

// PenaltyState 表示单节点惩罚状态。
type PenaltyState struct {
	PenaltyUntil time.Time `json:"penalty_until"`
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
func (p *PenaltyPool) MarkFailure(nodeKey string) {
	if nodeKey == "" {
		return
	}
	until := p.nowFunc().Add(p.window)
	p.mu.Lock()
	p.penalties[nodeKey] = until
	p.mu.Unlock()
}

// MarkSuccess 解除节点惩罚状态。
func (p *PenaltyPool) MarkSuccess(nodeKey string) {
	if nodeKey == "" {
		return
	}
	p.mu.Lock()
	delete(p.penalties, nodeKey)
	p.mu.Unlock()
}

// IsPenalized 判断节点是否在惩罚窗口内。
func (p *PenaltyPool) IsPenalized(nodeKey string) bool {
	_, penalized := p.penaltyUntil(nodeKey)
	return penalized
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

// EvaluateCandidates 返回节点惩罚状态明细，并保留全惩罚兜底语义。
func (p *PenaltyPool) EvaluateCandidates(nodes []string) []CandidatePenaltyState {
	if len(nodes) == 0 {
		return nil
	}
	stats := p.FilterCandidatesWithStats(nodes)
	result := make([]CandidatePenaltyState, 0, len(nodes))
	for _, node := range nodes {
		until, penalized := p.penaltyUntil(node)
		result = append(result, CandidatePenaltyState{
			NodeKey:      node,
			Penalized:    penalized,
			PenaltyUntil: until,
			Fallback:     stats.AllPenalizedFallback,
		})
	}
	return result
}

func (p *PenaltyPool) penaltyUntil(nodeKey string) (time.Time, bool) {
	if nodeKey == "" {
		return time.Time{}, false
	}
	now := p.nowFunc()

	p.mu.RLock()
	until, exists := p.penalties[nodeKey]
	p.mu.RUnlock()
	if !exists {
		return time.Time{}, false
	}
	if now.Before(until) {
		return until, true
	}

	p.mu.Lock()
	if currentUntil, ok := p.penalties[nodeKey]; ok && !now.Before(currentUntil) {
		delete(p.penalties, nodeKey)
	}
	p.mu.Unlock()
	return time.Time{}, false
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

// Restore 从持久化状态恢复惩罚快照，自动丢弃已过期项。
func (p *PenaltyPool) Restore(snapshot map[string]time.Time) {
	if p == nil {
		return
	}
	now := p.nowFunc()
	p.mu.Lock()
	defer p.mu.Unlock()

	p.penalties = make(map[string]time.Time, len(snapshot))
	for key, until := range snapshot {
		if key == "" || !now.Before(until) {
			continue
		}
		p.penalties[key] = until
	}
}
