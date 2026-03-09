package app

import (
	"context"
	"testing"
	"time"

	"geoloom/internal/config"
	"geoloom/internal/core/singbox"
	"geoloom/internal/domain"
	"geoloom/internal/health"
	"geoloom/internal/provider/parser"
	"geoloom/internal/provider/source"
)

type fakeRuntimeCore struct {
	startCalls   int
	rebuildCalls int
	lastNodes    []domain.NodeMetadata
	stats        singbox.CoreBuildStats
	startErr     error
	rebuildErr   error
}

func (f *fakeRuntimeCore) Start(_ config.Config, nodes []domain.NodeMetadata) error {
	f.startCalls++
	f.lastNodes = append([]domain.NodeMetadata(nil), nodes...)
	f.stats = singbox.CoreBuildStats{SupportedCandidates: len(nodes)}
	if f.startErr != nil {
		return f.startErr
	}
	return nil
}

func (f *fakeRuntimeCore) Rebuild(_ config.Config, nodes []domain.NodeMetadata) error {
	f.rebuildCalls++
	f.lastNodes = append([]domain.NodeMetadata(nil), nodes...)
	f.stats = singbox.CoreBuildStats{SupportedCandidates: len(nodes)}
	if f.rebuildErr != nil {
		return f.rebuildErr
	}
	return nil
}

func (f *fakeRuntimeCore) Close() error { return nil }

func (f *fakeRuntimeCore) LastBuildStats() singbox.CoreBuildStats { return f.stats }

type fakeContentFetcher struct {
	results map[string]source.FetchResult
	errors  map[string]error
}

func (f fakeContentFetcher) Fetch(ctx context.Context, sourceURL string) ([]string, error) {
	_ = ctx
	if err, ok := f.errors[sourceURL]; ok {
		return nil, err
	}
	return f.results[sourceURL].Entries, nil
}

func (f fakeContentFetcher) FetchResult(ctx context.Context, sourceURL string) (source.FetchResult, error) {
	_ = ctx
	if err, ok := f.errors[sourceURL]; ok {
		return source.FetchResult{}, err
	}
	result, ok := f.results[sourceURL]
	if !ok {
		return source.FetchResult{}, nil
	}
	return result, nil
}

func TestDiffCandidateFingerprintsStableAcrossOrder(t *testing.T) {
	t.Parallel()
	before := []domain.NodeMetadata{{Fingerprint: "a"}, {Fingerprint: "b"}}
	after := []domain.NodeMetadata{{Fingerprint: "b"}, {Fingerprint: "a"}}
	added, removed, unchanged := diffCandidateFingerprints(before, after)
	if added != 0 || removed != 0 || unchanged != 2 {
		t.Fatalf("diff 错误: added=%d removed=%d unchanged=%d", added, removed, unchanged)
	}
}

func TestRuntimePayloadMethods(t *testing.T) {
	t.Parallel()
	rt := &Runtime{cfg: config.Config{}}
	rt.snapshot = RuntimeSnapshot{
		Version:          "v1",
		Sources:          []SourceStatus{{Name: "s1", UnsupportedCount: 1}},
		Candidates:       []domain.NodeMetadata{{Fingerprint: "f1"}},
		ResolvedNodes:    []domain.NodeMetadata{{Fingerprint: "f1"}},
		Health:           health.HealthSnapshot{Nodes: map[string]health.NodeStatus{"f1": {LastCheckAt: time.Now(), LastReachable: true}}},
		PenaltyPool:      map[string]time.Time{"f1": time.Now().Add(time.Minute)},
		RawNodeCount:     2,
		DedupedNodeCount: 1,
		ResolvedCount:    1,
		CandidateCount:   1,
	}
	if len(rt.SourcesPayload().(map[string]any)["items"].([]SourceStatus)) != 1 {
		t.Fatal("SourcesPayload 错误")
	}
	if rt.StatusPayload().(map[string]any)["raw_node_count"].(int) != 2 {
		t.Fatal("StatusPayload 聚合字段错误")
	}
	if rt.HealthPayload().(map[string]any)["summary"].(map[string]any)["penalized_nodes"].(int) != 1 {
		t.Fatal("HealthPayload 摘要错误")
	}
}

func TestSnapshotShouldReturnCopies(t *testing.T) {
	t.Parallel()
	rt := &Runtime{}
	rt.snapshot = RuntimeSnapshot{
		RawNodes:      []domain.NodeMetadata{{Fingerprint: "raw-1"}},
		DedupedNodes:  []domain.NodeMetadata{{Fingerprint: "dedup-1"}},
		ResolvedNodes: []domain.NodeMetadata{{Fingerprint: "resolved-1"}},
		Candidates:    []domain.NodeMetadata{{Fingerprint: "candidate-1"}},
		Dropped:       []domain.NodeMetadata{{Fingerprint: "dropped-1"}},
		Sources:       []SourceStatus{{Name: "source-1"}},
	}

	snapshot := rt.Snapshot()
	snapshot.RawNodes[0].Fingerprint = "mutated"
	snapshot.DedupedNodes[0].Fingerprint = "mutated"
	snapshot.ResolvedNodes[0].Fingerprint = "mutated"
	snapshot.Candidates[0].Fingerprint = "mutated"
	snapshot.Dropped[0].Fingerprint = "mutated"
	snapshot.Sources[0].Name = "mutated"

	fresh := rt.Snapshot()
	if fresh.RawNodes[0].Fingerprint != "raw-1" {
		t.Fatalf("RawNodes 不应被外部修改污染: %+v", fresh.RawNodes)
	}
	if fresh.DedupedNodes[0].Fingerprint != "dedup-1" {
		t.Fatalf("DedupedNodes 不应被外部修改污染: %+v", fresh.DedupedNodes)
	}
	if fresh.ResolvedNodes[0].Fingerprint != "resolved-1" {
		t.Fatalf("ResolvedNodes 不应被外部修改污染: %+v", fresh.ResolvedNodes)
	}
	if fresh.Candidates[0].Fingerprint != "candidate-1" {
		t.Fatalf("Candidates 不应被外部修改污染: %+v", fresh.Candidates)
	}
	if fresh.Dropped[0].Fingerprint != "dropped-1" {
		t.Fatalf("Dropped 不应被外部修改污染: %+v", fresh.Dropped)
	}
	if fresh.Sources[0].Name != "source-1" {
		t.Fatalf("Sources 不应被外部修改污染: %+v", fresh.Sources)
	}
}

func TestRuntimePayloadMethodsShouldKeepStableShapeOnEmptySnapshot(t *testing.T) {
	t.Parallel()
	rt := &Runtime{cfg: config.Config{}}

	status := rt.StatusPayload().(map[string]any)
	if _, ok := status["refresh"].(map[string]any); !ok {
		t.Fatalf("StatusPayload.refresh 结构缺失: %+v", status)
	}
	if _, ok := status["api"].(map[string]any); !ok {
		t.Fatalf("StatusPayload.api 结构缺失: %+v", status)
	}
	if _, ok := status["state"].(map[string]any); !ok {
		t.Fatalf("StatusPayload.state 结构缺失: %+v", status)
	}

	healthPayload := rt.HealthPayload().(map[string]any)
	if healthPayload["summary"].(map[string]any)["penalized_nodes"].(int) != 0 {
		t.Fatalf("空快照下 penalized_nodes 应为 0: %+v", healthPayload)
	}
	if healthPayload["summary"].(map[string]any)["tracked_nodes"].(int) != 0 {
		t.Fatalf("空快照下 tracked_nodes 应为 0: %+v", healthPayload)
	}
}

func TestRefreshOnceShouldDriveStartThenSkipThenRebuild(t *testing.T) {
	t.Parallel()
	cfg := config.Config{
		Policy:  config.PolicyConfig{Strategy: config.StrategyRandom},
		Sources: []config.Source{{Name: "s1", Type: config.SourceTypeSource, URL: "https://example.com/sub"}},
	}
	fetcher := fakeContentFetcher{results: map[string]source.FetchResult{
		"https://example.com/sub": {
			Entries: []string{"socks5://1.1.1.1:1080#n1", "socks5://2.2.2.2:1080#n2"},
		},
	}}
	core := &fakeRuntimeCore{}
	rt := NewRuntime(context.Background(), cfg, "", parser.NewDispatcher(fetcher), "v1")
	rt.core = core

	first, err := rt.RefreshOnce(context.Background())
	if err != nil {
		t.Fatalf("首轮 RefreshOnce 返回错误: %v", err)
	}
	if !first.RebuildTriggered || core.startCalls != 1 || core.rebuildCalls != 0 {
		t.Fatalf("首轮应触发 start: result=%+v start=%d rebuild=%d", first, core.startCalls, core.rebuildCalls)
	}
	if first.AddedNodes != 2 || first.RemovedNodes != 0 || first.UnchangedNodes != 0 {
		t.Fatalf("首轮 diff 不符合预期: %+v", first)
	}
	if rt.Snapshot().CandidateCount != 2 || len(rt.Snapshot().Sources) != 1 {
		t.Fatalf("首轮快照不符合预期: %+v", rt.Snapshot())
	}

	second, err := rt.RefreshOnce(context.Background())
	if err != nil {
		t.Fatalf("第二轮 RefreshOnce 返回错误: %v", err)
	}
	if second.RebuildTriggered || core.startCalls != 1 || core.rebuildCalls != 0 {
		t.Fatalf("第二轮不应重建: result=%+v start=%d rebuild=%d", second, core.startCalls, core.rebuildCalls)
	}
	if second.AddedNodes != 0 || second.RemovedNodes != 0 || second.UnchangedNodes != 2 {
		t.Fatalf("第二轮 diff 不符合预期: %+v", second)
	}

	fetcher.results["https://example.com/sub"] = source.FetchResult{
		Entries: []string{"socks5://1.1.1.1:1080#n1", "socks5://3.3.3.3:1080#n3"},
	}
	third, err := rt.RefreshOnce(context.Background())
	if err != nil {
		t.Fatalf("第三轮 RefreshOnce 返回错误: %v", err)
	}
	if !third.RebuildTriggered || core.rebuildCalls != 1 {
		t.Fatalf("第三轮应触发 rebuild: result=%+v start=%d rebuild=%d", third, core.startCalls, core.rebuildCalls)
	}
	if third.AddedNodes != 1 || third.RemovedNodes != 1 || third.UnchangedNodes != 1 {
		t.Fatalf("第三轮 diff 不符合预期: %+v", third)
	}
	snapshot := rt.Snapshot()
	if snapshot.RawNodeCount != 2 || snapshot.DedupedNodeCount != 2 || snapshot.CandidateCount != 2 {
		t.Fatalf("第三轮快照计数错误: %+v", snapshot)
	}
	if snapshot.Sources[0].Success != true || snapshot.Sources[0].NodeCount != 2 || snapshot.Sources[0].InputType != string(parser.InputTypeSource) {
		t.Fatalf("第三轮 source 状态错误: %+v", snapshot.Sources)
	}
}

func TestRefreshOnceShouldReturnErrorWhenAllSourcesFail(t *testing.T) {
	t.Parallel()
	cfg := config.Config{
		Policy: config.PolicyConfig{Strategy: config.StrategyRandom},
		Sources: []config.Source{
			{Name: "s1", Type: config.SourceTypeSource, URL: "https://example.com/a"},
			{Name: "s2", Type: config.SourceTypeSource, URL: "https://example.com/b"},
		},
	}
	fetcher := fakeContentFetcher{errors: map[string]error{
		"https://example.com/a": context.DeadlineExceeded,
		"https://example.com/b": context.Canceled,
	}}
	rt := NewRuntime(context.Background(), cfg, "", parser.NewDispatcher(fetcher), "v1")
	rt.core = &fakeRuntimeCore{}

	_, err := rt.RefreshOnce(context.Background())
	if err == nil {
		t.Fatal("期望 source 全失败时返回错误")
	}
	if got := err.Error(); got != "过滤后无可用候选节点" {
		t.Fatalf("source 全失败错误不符合预期: %v", err)
	}
	if snapshot := rt.Snapshot(); snapshot.CandidateCount != 0 || len(snapshot.Sources) != 0 {
		t.Fatalf("失败时不应污染快照: %+v", snapshot)
	}
}

func TestRefreshOnceShouldReturnErrorWhenFilterDropsAllCandidates(t *testing.T) {
	t.Parallel()
	cfg := config.Config{
		Policy: config.PolicyConfig{
			Strategy: config.StrategyRandom,
			Filter:   config.FilterConfig{Allow: []string{"US"}},
		},
		Sources: []config.Source{{Name: "s1", Type: config.SourceTypeNode, URL: "socks5://1.1.1.1:1080#n1"}},
	}
	rt := NewRuntime(context.Background(), cfg, "", parser.NewDispatcher(nil), "v1")
	rt.core = &fakeRuntimeCore{}

	_, err := rt.RefreshOnce(context.Background())
	if err == nil {
		t.Fatal("期望过滤后无候选时返回错误")
	}
	if got := err.Error(); got != "过滤后无可用候选节点" {
		t.Fatalf("过滤后无候选错误不符合预期: %v", err)
	}
	if snapshot := rt.Snapshot(); snapshot.CandidateCount != 0 || len(snapshot.Sources) != 0 {
		t.Fatalf("失败时不应污染快照: %+v", snapshot)
	}
}

func TestRefreshOnceShouldKeepPreviousSnapshotWhenCoreRebuildFails(t *testing.T) {
	t.Parallel()
	cfg := config.Config{
		Policy:  config.PolicyConfig{Strategy: config.StrategyRandom},
		Sources: []config.Source{{Name: "s1", Type: config.SourceTypeSource, URL: "https://example.com/sub"}},
	}
	fetcher := fakeContentFetcher{results: map[string]source.FetchResult{
		"https://example.com/sub": {
			Entries: []string{"socks5://1.1.1.1:1080#n1", "socks5://2.2.2.2:1080#n2"},
		},
	}}
	core := &fakeRuntimeCore{}
	rt := NewRuntime(context.Background(), cfg, "", parser.NewDispatcher(fetcher), "v1")
	rt.core = core

	first, err := rt.RefreshOnce(context.Background())
	if err != nil {
		t.Fatalf("首轮 RefreshOnce 返回错误: %v", err)
	}
	before := rt.Snapshot()
	if before.CandidateCount != 2 || !first.RebuildTriggered {
		t.Fatalf("首轮快照不符合预期: result=%+v snapshot=%+v", first, before)
	}

	fetcher.results["https://example.com/sub"] = source.FetchResult{
		Entries: []string{"socks5://1.1.1.1:1080#n1", "socks5://3.3.3.3:1080#n3"},
	}
	core.rebuildErr = context.DeadlineExceeded

	_, err = rt.RefreshOnce(context.Background())
	if err == nil {
		t.Fatal("期望 core rebuild 失败时返回错误")
	}
	if got := err.Error(); got != "重建 core wrapper 失败: context deadline exceeded" {
		t.Fatalf("core rebuild 错误不符合预期: %v", err)
	}
	if core.rebuildCalls != 1 {
		t.Fatalf("应已尝试一次 rebuild: %+v", core)
	}
	after := rt.Snapshot()
	if after.CandidateCount != before.CandidateCount || len(after.Candidates) != len(before.Candidates) {
		t.Fatalf("rebuild 失败后不应覆盖旧快照: before=%+v after=%+v", before, after)
	}
	for i := range before.Candidates {
		if after.Candidates[i].Fingerprint != before.Candidates[i].Fingerprint {
			t.Fatalf("rebuild 失败后候选不应变化: before=%+v after=%+v", before.Candidates, after.Candidates)
		}
	}
}

func TestRefreshOnceShouldSupportRemoteTextFallbackWithSocks4AndHTTP(t *testing.T) {
	t.Parallel()
	cfg := config.Config{
		Policy:  config.PolicyConfig{Strategy: config.StrategyRandom},
		Sources: []config.Source{{Name: "s1", Type: config.SourceTypeSource, URL: "https://example.com/sub"}},
	}
	fetcher := fakeContentFetcher{results: map[string]source.FetchResult{
		"https://example.com/sub": {
			Content: []byte("1.1.1.1:1080#n1\nsocks4://legacy@2.2.2.2:1080#n2\nhttp://user:pass@3.3.3.3:8080#n3\n"),
			Entries: []string{"socks5://1.1.1.1:1080#n1", "socks4://legacy@2.2.2.2:1080#n2", "http://user:pass@3.3.3.3:8080#n3"},
		},
	}}
	core := &fakeRuntimeCore{}
	rt := NewRuntime(context.Background(), cfg, "", parser.NewDispatcher(fetcher), "v1")
	rt.core = core

	result, err := rt.RefreshOnce(context.Background())
	if err != nil {
		t.Fatalf("RefreshOnce 返回错误: %v", err)
	}
	if !result.RebuildTriggered || core.startCalls != 1 {
		t.Fatalf("应触发首次构建: result=%+v core=%+v", result, core)
	}
	snapshot := rt.Snapshot()
	if snapshot.CandidateCount != 3 || snapshot.Sources[0].NodeCount != 3 {
		t.Fatalf("快照计数错误: %+v", snapshot)
	}
	gotProtocols := []string{snapshot.Candidates[0].Protocol, snapshot.Candidates[1].Protocol, snapshot.Candidates[2].Protocol}
	wantProtocols := []string{"socks5", "socks4", "http"}
	for i := range wantProtocols {
		if gotProtocols[i] != wantProtocols[i] {
			t.Fatalf("协议顺序错误: got=%v want=%v", gotProtocols, wantProtocols)
		}
	}
}

func TestRefreshOnceShouldAggregateSourceNamesAcrossMixedProtocols(t *testing.T) {
	t.Parallel()
	cfg := config.Config{
		Policy: config.PolicyConfig{Strategy: config.StrategyRandom},
		Sources: []config.Source{
			{Name: "remote-a", Type: config.SourceTypeSource, URL: "https://example.com/a"},
			{Name: "remote-b", Type: config.SourceTypeSource, URL: "https://example.com/b"},
		},
	}
	fetcher := fakeContentFetcher{results: map[string]source.FetchResult{
		"https://example.com/a": {Entries: []string{"socks4://legacy@2.2.2.2:1080#n2", "http://user:pass@3.3.3.3:8080#n3"}},
		"https://example.com/b": {Entries: []string{"socks4://legacy@2.2.2.2:1080#n2", "socks5://4.4.4.4:1080#n4"}},
	}}
	core := &fakeRuntimeCore{}
	rt := NewRuntime(context.Background(), cfg, "", parser.NewDispatcher(fetcher), "v1")
	rt.core = core

	_, err := rt.RefreshOnce(context.Background())
	if err != nil {
		t.Fatalf("RefreshOnce 返回错误: %v", err)
	}
	snapshot := rt.Snapshot()
	if snapshot.RawNodeCount != 4 || snapshot.DedupedNodeCount != 3 {
		t.Fatalf("去重计数错误: %+v", snapshot)
	}
	var merged domain.NodeMetadata
	for _, node := range snapshot.Candidates {
		if node.Protocol == "socks4" {
			merged = node
			break
		}
	}
	if len(merged.SourceNames) != 2 {
		t.Fatalf("SourceNames 聚合错误: %+v", merged)
	}
}

func TestRefresherStartWithNilRuntimeShouldNoop(t *testing.T) {
	t.Parallel()
	NewRefresher(0, nil).Start(context.Background())
}
