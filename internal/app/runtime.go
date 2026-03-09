package app

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"geoloom/internal/config"
	"geoloom/internal/core/singbox"
	"geoloom/internal/domain"
	"geoloom/internal/filter"
	"geoloom/internal/health"
	"geoloom/internal/provider/parser"
	"geoloom/internal/state"
)

// SourceStatus 表示单个 source 最近一次处理状态。
type SourceStatus struct {
	Name             string    `json:"name"`
	Type             string    `json:"type"`
	URL              string    `json:"url"`
	Normalized       string    `json:"normalized_url"`
	InputType        string    `json:"input_type,omitempty"`
	NodeCount        int       `json:"node_count"`
	UnsupportedCount int       `json:"unsupported_count"`
	Success          bool      `json:"success"`
	Error            string    `json:"error,omitempty"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// RuntimeSnapshot 表示当前生效运行时快照。
type RuntimeSnapshot struct {
	StartedAt        time.Time              `json:"started_at"`
	LastRefreshAt    time.Time              `json:"last_refresh_at"`
	Version          string                 `json:"version"`
	SourceCount      int                    `json:"source_count"`
	Strategy         string                 `json:"strategy"`
	RawNodes         []domain.NodeMetadata  `json:"raw_nodes"`
	DedupedNodes     []domain.NodeMetadata  `json:"deduped_nodes"`
	ResolvedNodes    []domain.NodeMetadata  `json:"resolved_nodes"`
	Candidates       []domain.NodeMetadata  `json:"candidates"`
	Dropped          []domain.NodeMetadata  `json:"dropped"`
	Sources          []SourceStatus         `json:"sources"`
	Health           health.HealthSnapshot  `json:"health"`
	PenaltyPool      map[string]time.Time   `json:"penalty_pool"`
	CoreStats        singbox.CoreBuildStats `json:"core_stats"`
	RawNodeCount     int                    `json:"raw_node_count"`
	DedupedNodeCount int                    `json:"deduped_node_count"`
	ResolvedCount    int                    `json:"resolved_node_count"`
	CandidateCount   int                    `json:"candidate_node_count"`
	DroppedCount     int                    `json:"dropped_node_count"`
}

// RefreshResult 表示一次刷新执行结果。
type RefreshResult struct {
	AddedNodes       int  `json:"added_nodes"`
	RemovedNodes     int  `json:"removed_nodes"`
	UnchangedNodes   int  `json:"unchanged_nodes"`
	RebuildTriggered bool `json:"rebuild_triggered"`
	CandidateCount   int  `json:"candidate_count"`
	DedupedCount     int  `json:"deduped_count"`
	RawCount         int  `json:"raw_count"`
	GeoFailedCount   int  `json:"geo_failed_count"`
	DroppedCount     int  `json:"dropped_count"`
}

type runtimeCore interface {
	Start(cfg config.Config, nodes []domain.NodeMetadata) error
	Rebuild(cfg config.Config, nodes []domain.NodeMetadata) error
	Close() error
	LastBuildStats() singbox.CoreBuildStats
}

// Runtime 封装统一刷新、快照与重建流程。
type Runtime struct {
	cfg        config.Config
	configPath string
	dispatcher *parser.Dispatcher
	core       runtimeCore
	checker    *health.Checker
	penalty    *health.PenaltyPool
	store      *state.Store
	startedAt  time.Time
	version    string

	mu       sync.RWMutex
	snapshot RuntimeSnapshot
}

func NewRuntime(ctx context.Context, cfg config.Config, configPath string, dispatcher *parser.Dispatcher, version string) *Runtime {
	if dispatcher == nil {
		dispatcher = parser.NewDispatcher(nil)
	}
	penalty := health.NewPenaltyPool(5 * time.Minute)
	core := singbox.NewService(ctx, singbox.NewOptionsBuilder())
	rt := &Runtime{
		cfg:        cfg,
		configPath: configPath,
		dispatcher: dispatcher,
		core:       core,
		penalty:    penalty,
		startedAt:  time.Now(),
		version:    strings.TrimSpace(version),
	}
	checker := health.NewChecker(parseDurationOrDefault(cfg.Policy.HealthCheck.Interval, 30*time.Second), cfg.Policy.HealthCheck.URL, penalty, func(rebuildCtx context.Context, candidates []domain.NodeMetadata) error {
		return rt.applyCandidates(rebuildCtx, candidates, true)
	})
	rt.checker = checker
	if cfg.State.Enabled {
		rt.store = state.NewStore(resolvePathByConfigPath(cfg.State.Path, configPath))
		if snapshot, err := rt.store.Load(); err != nil {
			slog.Warn("加载状态文件失败，降级为空状态", "error", err)
		} else {
			rt.penalty.Restore(snapshot.PenaltyUntil)
			rt.checker.RestoreSnapshot(health.HealthSnapshot{Nodes: snapshot.NodeStatuses})
		}
	}
	return rt
}

func (r *Runtime) Start(ctx context.Context) error {
	result, err := r.RefreshOnce(ctx)
	if err != nil {
		return err
	}
	slog.Info("运行时首次刷新完成",
		"candidate_nodes", result.CandidateCount,
		"rebuild_triggered", result.RebuildTriggered,
	)
	if r.cfg.Policy.HealthCheck.Enabled {
		r.checker.Start(ctx, r.Snapshot().Candidates)
	}
	return nil
}

func (r *Runtime) RefreshOnce(ctx context.Context) (RefreshResult, error) {
	rawNodes, dedupedNodes, sources, err := collectNodesDetailed(ctx, r.cfg, r.configPath, r.dispatcher)
	if err != nil {
		return RefreshResult{}, err
	}
	resolvedNodes, geoFailed := applyGeo(ctx, r.cfg, dedupedNodes, r.configPath)
	filterEngine := filter.NewEngine(filter.Config{Allow: r.cfg.Policy.Filter.Allow, Block: r.cfg.Policy.Filter.Block})
	filtered := filterEngine.Filter(resolvedNodes)
	if len(filtered.Candidates) == 0 {
		return RefreshResult{}, fmt.Errorf("过滤后无可用候选节点")
	}

	before := r.Snapshot().Candidates
	added, removed, unchanged := diffCandidateFingerprints(before, filtered.Candidates)
	rebuildNeeded := len(before) == 0 || added > 0 || removed > 0
	if err := r.applyCandidates(ctx, filtered.Candidates, rebuildNeeded); err != nil {
		return RefreshResult{}, err
	}

	r.mu.Lock()
	r.snapshot = RuntimeSnapshot{
		StartedAt:        r.startedAt,
		LastRefreshAt:    time.Now(),
		Version:          r.version,
		SourceCount:      len(r.cfg.Sources),
		Strategy:         r.cfg.Policy.Strategy,
		RawNodes:         append([]domain.NodeMetadata(nil), rawNodes...),
		DedupedNodes:     append([]domain.NodeMetadata(nil), dedupedNodes...),
		ResolvedNodes:    append([]domain.NodeMetadata(nil), resolvedNodes...),
		Candidates:       append([]domain.NodeMetadata(nil), filtered.Candidates...),
		Dropped:          append([]domain.NodeMetadata(nil), filtered.Dropped...),
		Sources:          append([]SourceStatus(nil), sources...),
		Health:           r.checker.Snapshot(),
		PenaltyPool:      r.penalty.Snapshot(),
		CoreStats:        r.core.LastBuildStats(),
		RawNodeCount:     len(rawNodes),
		DedupedNodeCount: len(dedupedNodes),
		ResolvedCount:    len(resolvedNodes),
		CandidateCount:   len(filtered.Candidates),
		DroppedCount:     len(filtered.Dropped),
	}
	r.mu.Unlock()
	_ = r.flushState()

	result := RefreshResult{
		AddedNodes:       added,
		RemovedNodes:     removed,
		UnchangedNodes:   unchanged,
		RebuildTriggered: rebuildNeeded,
		CandidateCount:   len(filtered.Candidates),
		DedupedCount:     len(dedupedNodes),
		RawCount:         len(rawNodes),
		GeoFailedCount:   geoFailed,
		DroppedCount:     len(filtered.Dropped),
	}
	slog.Info("运行时刷新完成",
		"added_nodes", added,
		"removed_nodes", removed,
		"unchanged_nodes", unchanged,
		"rebuild_triggered", rebuildNeeded,
	)
	return result, nil
}

func (r *Runtime) applyCandidates(ctx context.Context, candidates []domain.NodeMetadata, rebuild bool) error {
	if len(candidates) == 0 {
		return fmt.Errorf("候选节点不能为空")
	}
	if rebuild {
		if len(r.Snapshot().Candidates) == 0 {
			if err := r.core.Start(r.cfg, candidates); err != nil {
				return fmt.Errorf("启动 core wrapper 失败: %w", err)
			}
		} else if err := r.core.Rebuild(r.cfg, candidates); err != nil {
			return fmt.Errorf("重建 core wrapper 失败: %w", err)
		}
	}
	r.checker.SetNodes(candidates)
	_ = ctx
	return nil
}

func (r *Runtime) Snapshot() RuntimeSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	snapshot := r.snapshot
	snapshot.RawNodes = append([]domain.NodeMetadata(nil), snapshot.RawNodes...)
	snapshot.DedupedNodes = append([]domain.NodeMetadata(nil), snapshot.DedupedNodes...)
	snapshot.ResolvedNodes = append([]domain.NodeMetadata(nil), snapshot.ResolvedNodes...)
	snapshot.Candidates = append([]domain.NodeMetadata(nil), snapshot.Candidates...)
	snapshot.Dropped = append([]domain.NodeMetadata(nil), snapshot.Dropped...)
	snapshot.Sources = append([]SourceStatus(nil), snapshot.Sources...)
	return snapshot
}

func (r *Runtime) StatusPayload() any {
	snapshot := r.Snapshot()
	return map[string]any{
		"version":                snapshot.Version,
		"started_at":             snapshot.StartedAt,
		"last_refresh_at":        snapshot.LastRefreshAt,
		"source_count":           snapshot.SourceCount,
		"strategy":               snapshot.Strategy,
		"raw_node_count":         snapshot.RawNodeCount,
		"deduped_node_count":     snapshot.DedupedNodeCount,
		"resolved_node_count":    snapshot.ResolvedCount,
		"candidate_node_count":   snapshot.CandidateCount,
		"dropped_node_count":     snapshot.DroppedCount,
		"core_supported_count":   snapshot.CoreStats.SupportedCandidates,
		"core_unsupported_count": len(snapshot.CoreStats.Unsupported),
		"refresh": map[string]any{
			"enabled":  r.cfg.Policy.Refresh.Enabled,
			"interval": r.cfg.Policy.Refresh.Interval,
		},
		"api": map[string]any{
			"enabled": r.cfg.API.Enabled,
			"listen":  r.cfg.API.Listen,
		},
		"state": map[string]any{
			"enabled": r.cfg.State.Enabled,
			"path":    r.cfg.State.Path,
		},
	}
}

func (r *Runtime) SourcesPayload() any {
	return map[string]any{"items": r.Snapshot().Sources}
}

func (r *Runtime) NodesPayload() any {
	snapshot := r.Snapshot()
	return map[string]any{"items": snapshot.ResolvedNodes, "count": len(snapshot.ResolvedNodes)}
}

func (r *Runtime) CandidatesPayload() any {
	snapshot := r.Snapshot()
	return map[string]any{"items": snapshot.Candidates, "count": len(snapshot.Candidates)}
}

func (r *Runtime) HealthPayload() any {
	snapshot := r.Snapshot()
	return map[string]any{
		"config": map[string]any{
			"enabled":  r.cfg.Policy.HealthCheck.Enabled,
			"interval": r.cfg.Policy.HealthCheck.Interval,
			"url":      r.cfg.Policy.HealthCheck.URL,
		},
		"summary": map[string]any{
			"tracked_nodes":   len(snapshot.Health.Nodes),
			"penalized_nodes": len(snapshot.PenaltyPool),
			"last_rebuild_at": snapshot.Health.LastRebuildAt,
		},
		"health":       snapshot.Health,
		"penalty_pool": snapshot.PenaltyPool,
	}
}

func (r *Runtime) Close() error {
	if err := r.flushState(); err != nil {
		slog.Warn("退出前写入状态失败", "error", err)
	}
	return r.core.Close()
}

func (r *Runtime) flushState() error {
	if r.store == nil {
		return nil
	}
	healthSnapshot := r.checker.Snapshot()
	country := make(map[string]string)
	for _, node := range r.Snapshot().ResolvedNodes {
		if key := domain.NodeKey(node); key != "" && node.CountryCode != "" {
			country[key] = node.CountryCode
		}
	}
	return r.store.Save(state.Snapshot{PenaltyUntil: r.penalty.Snapshot(), NodeStatuses: healthSnapshot.Nodes, LastCountryCode: country})
}

func parseDurationOrDefault(raw string, fallback time.Duration) time.Duration {
	parsed, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func diffCandidateFingerprints(before, after []domain.NodeMetadata) (added, removed, unchanged int) {
	beforeSet := make(map[string]struct{}, len(before))
	afterSet := make(map[string]struct{}, len(after))
	for _, node := range before {
		if key := domain.NodeKey(node); key != "" {
			beforeSet[key] = struct{}{}
		}
	}
	for _, node := range after {
		if key := domain.NodeKey(node); key != "" {
			afterSet[key] = struct{}{}
		}
	}
	for key := range afterSet {
		if _, ok := beforeSet[key]; ok {
			unchanged++
		} else {
			added++
		}
	}
	for key := range beforeSet {
		if _, ok := afterSet[key]; !ok {
			removed++
		}
	}
	return
}

func collectNodesDetailed(ctx context.Context, cfg config.Config, configPath string, dispatcher *parser.Dispatcher) ([]domain.NodeMetadata, []domain.NodeMetadata, []SourceStatus, error) {
	rawNodes := make([]domain.NodeMetadata, 0)
	sources := make([]SourceStatus, 0, len(cfg.Sources))
	for _, src := range cfg.Sources {
		normalizedURL := normalizeSourceURL(src, configPath)
		status := SourceStatus{Name: src.Name, Type: src.Type, URL: src.URL, Normalized: normalizedURL, UpdatedAt: time.Now()}
		result, parseErr := dispatcher.Parse(ctx, normalizedURL)
		if parseErr != nil {
			status.Error = parseErr.Error()
			sources = append(sources, status)
			slog.Warn("输入源处理失败", "source", src.Name, "url", src.URL, "normalized_url", normalizedURL, "error", parseErr)
			continue
		}
		status.Success = true
		status.InputType = string(result.Type)
		status.NodeCount = len(result.Nodes)
		status.UnsupportedCount = len(result.Unsupported)
		sourceName := buildSourceName(src, normalizedURL)
		for i := range result.Nodes {
			if sourceName != "" {
				result.Nodes[i].SourceNames = []string{sourceName}
			}
		}
		sources = append(sources, status)
		rawNodes = append(rawNodes, result.Nodes...)
	}
	deduped, err := domain.DedupNodes(rawNodes)
	if err != nil {
		return nil, nil, sources, fmt.Errorf("节点去重失败: %w", err)
	}
	slog.Info("节点去重完成", "raw_nodes", len(rawNodes), "deduped_nodes", len(deduped.Nodes), "duplicate_nodes", deduped.DuplicateCount)
	return rawNodes, deduped.Nodes, sources, nil
}
