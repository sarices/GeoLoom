package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"geoloom/internal/health"
)

// Snapshot 表示可持久化的健康状态快照。
type Snapshot struct {
	PenaltyUntil    map[string]time.Time         `json:"penalty_until"`
	NodeStatuses    map[string]health.NodeStatus `json:"node_statuses"`
	LastCountryCode map[string]string            `json:"last_country_code,omitempty"`
}

// Store 负责本地 JSON 状态持久化。
type Store struct {
	path string
	mu   sync.Mutex
}

func NewStore(path string) *Store {
	return &Store{path: strings.TrimSpace(path)}
}

func (s *Store) Load() (Snapshot, error) {
	if s == nil || s.path == "" {
		return Snapshot{}, nil
	}
	content, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return Snapshot{}, nil
		}
		return Snapshot{}, fmt.Errorf("读取状态文件失败: %w", err)
	}
	if len(strings.TrimSpace(string(content))) == 0 {
		return Snapshot{}, nil
	}
	var snapshot Snapshot
	if err := json.Unmarshal(content, &snapshot); err != nil {
		return Snapshot{}, fmt.Errorf("解析状态文件失败: %w", err)
	}
	if snapshot.PenaltyUntil == nil {
		snapshot.PenaltyUntil = map[string]time.Time{}
	}
	if snapshot.NodeStatuses == nil {
		snapshot.NodeStatuses = map[string]health.NodeStatus{}
	}
	if snapshot.LastCountryCode == nil {
		snapshot.LastCountryCode = map[string]string{}
	}
	filteredPenalty := make(map[string]time.Time, len(snapshot.PenaltyUntil))
	now := time.Now()
	for key, until := range snapshot.PenaltyUntil {
		if key == "" || !now.Before(until) {
			continue
		}
		filteredPenalty[key] = until
	}
	snapshot.PenaltyUntil = filteredPenalty
	return snapshot, nil
}

func (s *Store) Save(snapshot Snapshot) error {
	if s == nil || s.path == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("创建状态目录失败: %w", err)
	}
	content, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化状态失败: %w", err)
	}
	if err := os.WriteFile(s.path, content, 0o600); err != nil {
		return fmt.Errorf("写入状态文件失败: %w", err)
	}
	return nil
}
