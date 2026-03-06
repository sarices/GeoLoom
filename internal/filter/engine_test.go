package filter

import (
	"testing"

	"geoloom/internal/domain"
)

func TestFilterBlockPriority(t *testing.T) {
	t.Parallel()

	engine := NewEngine(Config{
		Allow: []string{"US", "JP"},
		Block: []string{"JP"},
	})

	nodes := []domain.NodeMetadata{
		{ID: "n1", CountryCode: "US"},
		{ID: "n2", CountryCode: "JP"},
	}

	result := engine.Filter(nodes)
	if len(result.Candidates) != 1 || result.Candidates[0].ID != "n1" {
		t.Fatalf("候选集错误: %#v", result.Candidates)
	}
	if len(result.Dropped) != 1 || result.Dropped[0].ID != "n2" {
		t.Fatalf("剔除集错误: %#v", result.Dropped)
	}
}

func TestFilterAllowOnly(t *testing.T) {
	t.Parallel()

	engine := NewEngine(Config{Allow: []string{"US", "SG"}})
	nodes := []domain.NodeMetadata{
		{ID: "n1", CountryCode: "US"},
		{ID: "n2", CountryCode: "CN"},
	}

	result := engine.Filter(nodes)
	if len(result.Candidates) != 1 || result.Candidates[0].ID != "n1" {
		t.Fatalf("allow 过滤错误: %#v", result.Candidates)
	}
}

func TestFilterNoPolicyKeepsAll(t *testing.T) {
	t.Parallel()

	engine := NewEngine(Config{})
	nodes := []domain.NodeMetadata{
		{ID: "n1", CountryCode: "US"},
		{ID: "n2", CountryCode: "CN"},
	}

	result := engine.Filter(nodes)
	if len(result.Candidates) != 2 {
		t.Fatalf("无策略时应保留全部: got=%d", len(result.Candidates))
	}
	if len(result.Dropped) != 0 {
		t.Fatalf("无策略时不应剔除: got=%d", len(result.Dropped))
	}
}
