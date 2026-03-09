package domain

import (
	"reflect"
	"testing"
)

func TestDedupNodesEmpty(t *testing.T) {
	t.Parallel()

	result, err := DedupNodes(nil)
	if err != nil {
		t.Fatalf("空输入去重失败: %v", err)
	}
	if len(result.Nodes) != 0 || result.DuplicateCount != 0 {
		t.Fatalf("空输入结果错误: %+v", result)
	}
}

func TestDedupNodesNoDuplicates(t *testing.T) {
	t.Parallel()

	nodes := []NodeMetadata{
		{Protocol: "socks5", Address: "1.1.1.1", Port: 1080, SourceNames: []string{"a"}, RawConfig: map[string]any{"username": "u1", "password": "p1"}},
		{Protocol: "socks5", Address: "2.2.2.2", Port: 1080, SourceNames: []string{"b"}, RawConfig: map[string]any{"username": "u2", "password": "p2"}},
	}

	result, err := DedupNodes(nodes)
	if err != nil {
		t.Fatalf("去重失败: %v", err)
	}
	if len(result.Nodes) != 2 || result.DuplicateCount != 0 {
		t.Fatalf("无重复场景结果错误: %+v", result)
	}
}

func TestDedupNodesCollapseDuplicates(t *testing.T) {
	t.Parallel()

	nodes := []NodeMetadata{
		{ID: "n1", Name: "first", Protocol: "socks5", Address: "1.1.1.1", Port: 1080, SourceNames: []string{"a"}, RawConfig: map[string]any{"username": "u", "password": "p"}},
		{ID: "n2", Name: "second", Protocol: "socks5", Address: "1.1.1.1", Port: 1080, SourceNames: []string{"b"}, RawConfig: map[string]any{"username": "u", "password": "p"}},
	}

	result, err := DedupNodes(nodes)
	if err != nil {
		t.Fatalf("去重失败: %v", err)
	}
	if len(result.Nodes) != 1 {
		t.Fatalf("重复节点应压缩为 1 条: got=%d", len(result.Nodes))
	}
	if result.DuplicateCount != 1 {
		t.Fatalf("DuplicateCount 错误: got=%d", result.DuplicateCount)
	}
	if result.Nodes[0].ID != "n1" || result.Nodes[0].Name != "first" {
		t.Fatalf("应保留首条节点记录: %+v", result.Nodes[0])
	}
}

func TestDedupNodesMergeSourceNames(t *testing.T) {
	t.Parallel()

	nodes := []NodeMetadata{
		{Protocol: "socks5", Address: "1.1.1.1", Port: 1080, SourceNames: []string{"a"}, RawConfig: map[string]any{"username": "u", "password": "p"}},
		{Protocol: "socks5", Address: "1.1.1.1", Port: 1080, SourceNames: []string{"b", "a"}, RawConfig: map[string]any{"username": "u", "password": "p"}},
	}

	result, err := DedupNodes(nodes)
	if err != nil {
		t.Fatalf("去重失败: %v", err)
	}
	want := []string{"a", "b"}
	if !reflect.DeepEqual(result.Nodes[0].SourceNames, want) {
		t.Fatalf("SourceNames 合并错误: got=%v want=%v", result.Nodes[0].SourceNames, want)
	}
}

func TestDedupNodesKeepFirstSeenOrder(t *testing.T) {
	t.Parallel()

	nodes := []NodeMetadata{
		{ID: "a", Protocol: "socks5", Address: "1.1.1.1", Port: 1080, RawConfig: map[string]any{"username": "u1", "password": "p1"}},
		{ID: "b", Protocol: "socks5", Address: "2.2.2.2", Port: 1080, RawConfig: map[string]any{"username": "u2", "password": "p2"}},
		{ID: "c", Protocol: "socks5", Address: "1.1.1.1", Port: 1080, RawConfig: map[string]any{"username": "u1", "password": "p1"}},
	}

	result, err := DedupNodes(nodes)
	if err != nil {
		t.Fatalf("去重失败: %v", err)
	}
	if len(result.Nodes) != 2 {
		t.Fatalf("去重后数量错误: got=%d", len(result.Nodes))
	}
	if result.Nodes[0].ID != "a" || result.Nodes[1].ID != "b" {
		t.Fatalf("去重后顺序错误: %+v", result.Nodes)
	}
}

func TestDedupNodesSupportSocks4AndHTTP(t *testing.T) {
	t.Parallel()

	nodes := []NodeMetadata{
		{Protocol: "socks4", Address: "1.1.1.1", Port: 1080, SourceNames: []string{"a"}, RawConfig: map[string]any{"username": "legacy"}},
		{Protocol: "http", Address: "2.2.2.2", Port: 8080, SourceNames: []string{"b"}, RawConfig: map[string]any{"username": "user", "password": "pass"}},
	}

	result, err := DedupNodes(nodes)
	if err != nil {
		t.Fatalf("去重失败: %v", err)
	}
	if len(result.Nodes) != 2 {
		t.Fatalf("新增协议去重数量错误: got=%d want=2", len(result.Nodes))
	}
	if result.Nodes[0].Fingerprint == "" || result.Nodes[1].Fingerprint == "" {
		t.Fatalf("新增协议应生成 fingerprint: %+v", result.Nodes)
	}
}

func TestDedupNodesCollapseHTTPDuplicatesWithDifferentNames(t *testing.T) {
	t.Parallel()

	nodes := []NodeMetadata{
		{
			ID:          "http-a",
			Name:        "dup-a",
			Protocol:    "http",
			Address:     "1.1.1.1",
			Port:        8080,
			SourceNames: []string{"dirty-source-a"},
			RawConfig: map[string]any{
				"username": "user",
				"password": "pass",
			},
		},
		{
			ID:          "http-b",
			Name:        "dup-b",
			Protocol:    "http",
			Address:     "1.1.1.1",
			Port:        8080,
			SourceNames: []string{"dirty-source-b"},
			RawConfig: map[string]any{
				"username": "user",
				"password": "pass",
			},
		},
	}

	result, err := DedupNodes(nodes)
	if err != nil {
		t.Fatalf("去重失败: %v", err)
	}
	if len(result.Nodes) != 1 {
		t.Fatalf("HTTP 重复节点应压缩为 1 条: got=%d", len(result.Nodes))
	}
	if result.DuplicateCount != 1 {
		t.Fatalf("DuplicateCount 错误: got=%d want=1", result.DuplicateCount)
	}
	if result.Nodes[0].ID != "http-a" || result.Nodes[0].Name != "dup-a" {
		t.Fatalf("应保留首条 HTTP 节点记录: %+v", result.Nodes[0])
	}
	wantSources := []string{"dirty-source-a", "dirty-source-b"}
	if !reflect.DeepEqual(result.Nodes[0].SourceNames, wantSources) {
		t.Fatalf("HTTP 重复节点 SourceNames 合并错误: got=%v want=%v", result.Nodes[0].SourceNames, wantSources)
	}
}

func TestDedupNodesFillFingerprint(t *testing.T) {
	t.Parallel()

	nodes := []NodeMetadata{{Protocol: "shadowsocks", Address: "1.1.1.1", Port: 8388, RawConfig: map[string]any{"method": "aes-128-gcm", "password": "p"}}}
	result, err := DedupNodes(nodes)
	if err != nil {
		t.Fatalf("去重失败: %v", err)
	}
	if result.Nodes[0].Fingerprint == "" {
		t.Fatal("去重结果应回填 Fingerprint")
	}
}
