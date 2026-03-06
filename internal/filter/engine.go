package filter

import (
	"strings"

	"geoloom/internal/domain"
)

// Config 定义过滤规则，block 优先于 allow。
type Config struct {
	Allow []string
	Block []string
}

// Result 给出过滤后的节点与剔除节点。
type Result struct {
	Candidates []domain.NodeMetadata
	Dropped    []domain.NodeMetadata
}

// Engine 根据国家码执行 allow/block 过滤。
type Engine struct {
	allowSet map[string]struct{}
	blockSet map[string]struct{}
}

func NewEngine(cfg Config) *Engine {
	return &Engine{
		allowSet: buildCountrySet(cfg.Allow),
		blockSet: buildCountrySet(cfg.Block),
	}
}

func (e *Engine) Filter(nodes []domain.NodeMetadata) Result {
	result := Result{
		Candidates: make([]domain.NodeMetadata, 0, len(nodes)),
		Dropped:    make([]domain.NodeMetadata, 0),
	}

	for _, node := range nodes {
		country := strings.ToUpper(strings.TrimSpace(node.CountryCode))

		if e.inBlock(country) {
			result.Dropped = append(result.Dropped, node)
			continue
		}

		if e.hasAllow() && !e.inAllow(country) {
			result.Dropped = append(result.Dropped, node)
			continue
		}

		result.Candidates = append(result.Candidates, node)
	}

	return result
}

func (e *Engine) hasAllow() bool {
	return len(e.allowSet) > 0
}

func (e *Engine) inAllow(country string) bool {
	_, ok := e.allowSet[country]
	return ok
}

func (e *Engine) inBlock(country string) bool {
	_, ok := e.blockSet[country]
	return ok
}

func buildCountrySet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized := strings.ToUpper(strings.TrimSpace(value))
		if normalized == "" {
			continue
		}
		result[normalized] = struct{}{}
	}
	return result
}
