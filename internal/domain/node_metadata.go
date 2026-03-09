package domain

import "time"

// NodeMetadata 是协议解析后的统一节点模型。
type NodeMetadata struct {
	ID          string         `json:"id"`
	Fingerprint string         `json:"fingerprint"`
	Name        string         `json:"name"`
	SourceNames []string       `json:"source_names"`
	CountryCode string         `json:"country_code"`
	Protocol    string         `json:"protocol"`
	Address     string         `json:"address"`
	Port        int            `json:"port"`
	LastChecked time.Time      `json:"last_checked"`
	RawConfig   map[string]any `json:"raw_config"`
}
