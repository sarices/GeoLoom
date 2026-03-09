package parser

import (
	"encoding/json"
	"fmt"
	"strings"

	"geoloom/internal/domain"
)

type singboxDocument struct {
	Outbounds []singboxOutbound `json:"outbounds"`
}

type singboxOutbound struct {
	Type       string `json:"type"`
	Tag        string `json:"tag"`
	Server     string `json:"server"`
	ServerPort int    `json:"server_port"`
	UUID       string `json:"uuid"`
	Password   string `json:"password"`
	Method     string `json:"method"`
	Username   string `json:"username"`
	Version    string `json:"version"`
	TLS        *struct {
		ServerName string   `json:"server_name"`
		Enabled    bool     `json:"enabled"`
		ALPN       []string `json:"alpn"`
	} `json:"tls"`
	Transport *struct {
		Type string `json:"type"`
		Path string `json:"path"`
		Host string `json:"host"`
	} `json:"transport"`
}

// ParseSingboxJSON 解析 Sing-box JSON source；ok=false 表示内容并非该格式。
func ParseSingboxJSON(content []byte) ([]domain.NodeMetadata, bool, error) {
	trimmed := strings.TrimSpace(string(content))
	if trimmed == "" || !strings.HasPrefix(trimmed, "{") || !strings.Contains(trimmed, "\"outbounds\"") {
		return nil, false, nil
	}
	var doc singboxDocument
	if err := json.Unmarshal(content, &doc); err != nil {
		return nil, true, err
	}
	if len(doc.Outbounds) == 0 {
		return nil, true, nil
	}
	nodes := make([]domain.NodeMetadata, 0, len(doc.Outbounds))
	for _, outbound := range doc.Outbounds {
		node, err := mapSingboxOutbound(outbound)
		if err != nil {
			continue
		}
		nodes = append(nodes, node)
	}
	return nodes, true, nil
}

func mapSingboxOutbound(outbound singboxOutbound) (domain.NodeMetadata, error) {
	protocol := strings.ToLower(strings.TrimSpace(outbound.Type))
	host := strings.TrimSpace(outbound.Server)
	if host == "" || outbound.ServerPort <= 0 {
		return domain.NodeMetadata{}, fmt.Errorf("sing-box outbound 缺少 server/server_port")
	}
	name := strings.TrimSpace(outbound.Tag)
	if name == "" {
		name = fmt.Sprintf("%s-%s:%d", protocol, host, outbound.ServerPort)
	}
	raw := map[string]any{"type": protocol, "server": host, "server_port": outbound.ServerPort}
	if outbound.TLS != nil && outbound.TLS.Enabled {
		raw["security"] = "tls"
		if outbound.TLS.ServerName != "" {
			raw["sni"] = strings.TrimSpace(outbound.TLS.ServerName)
		}
		if len(outbound.TLS.ALPN) > 0 {
			raw["alpn"] = outbound.TLS.ALPN
		}
	}
	if outbound.Transport != nil {
		if outbound.Transport.Type != "" {
			raw["network"] = strings.TrimSpace(outbound.Transport.Type)
		}
		if outbound.Transport.Path != "" {
			raw["path"] = strings.TrimSpace(outbound.Transport.Path)
		}
		if outbound.Transport.Host != "" {
			raw["host"] = strings.TrimSpace(outbound.Transport.Host)
		}
	}
	switch protocol {
	case "socks":
		version := strings.TrimSpace(outbound.Version)
		if version == "4" || strings.EqualFold(version, "socks4") {
			raw["type"] = "socks4"
			if outbound.Username != "" {
				raw["username"] = strings.TrimSpace(outbound.Username)
			}
			return domain.NodeMetadata{ID: buildNodeID("socks4", host, outbound.ServerPort, outbound.Username), Name: name, Protocol: "socks4", Address: host, Port: outbound.ServerPort, RawConfig: raw}, nil
		}
		raw["type"] = "socks5"
		if outbound.Username != "" {
			raw["username"] = strings.TrimSpace(outbound.Username)
		}
		if outbound.Password != "" {
			raw["password"] = strings.TrimSpace(outbound.Password)
		}
		return domain.NodeMetadata{ID: buildNodeID("socks5", host, outbound.ServerPort, outbound.Username+":"+outbound.Password), Name: name, Protocol: "socks5", Address: host, Port: outbound.ServerPort, RawConfig: raw}, nil
	case "http":
		if outbound.Username != "" {
			raw["username"] = strings.TrimSpace(outbound.Username)
		}
		if outbound.Password != "" {
			raw["password"] = strings.TrimSpace(outbound.Password)
		}
		return domain.NodeMetadata{ID: buildNodeID("http", host, outbound.ServerPort, outbound.Username+":"+outbound.Password), Name: name, Protocol: "http", Address: host, Port: outbound.ServerPort, RawConfig: raw}, nil
	case "trojan":
		raw["password"] = strings.TrimSpace(outbound.Password)
		return domain.NodeMetadata{ID: buildNodeID("trojan", host, outbound.ServerPort, outbound.Password), Name: name, Protocol: "trojan", Address: host, Port: outbound.ServerPort, RawConfig: raw}, nil
	case "vmess":
		raw["uuid"] = strings.TrimSpace(outbound.UUID)
		return domain.NodeMetadata{ID: buildNodeID("vmess", host, outbound.ServerPort, outbound.UUID), Name: name, Protocol: "vmess", Address: host, Port: outbound.ServerPort, RawConfig: raw}, nil
	case "vless":
		raw["uuid"] = strings.TrimSpace(outbound.UUID)
		return domain.NodeMetadata{ID: buildNodeID("vless", host, outbound.ServerPort, outbound.UUID), Name: name, Protocol: "vless", Address: host, Port: outbound.ServerPort, RawConfig: raw}, nil
	case "shadowsocks":
		raw["method"] = strings.TrimSpace(outbound.Method)
		raw["password"] = strings.TrimSpace(outbound.Password)
		return domain.NodeMetadata{ID: buildNodeID("shadowsocks", host, outbound.ServerPort, outbound.Method+":"+outbound.Password), Name: name, Protocol: "shadowsocks", Address: host, Port: outbound.ServerPort, RawConfig: raw}, nil
	case "hysteria2":
		raw["password"] = strings.TrimSpace(outbound.Password)
		return domain.NodeMetadata{ID: buildNodeID("hysteria2", host, outbound.ServerPort, outbound.Password), Name: name, Protocol: "hysteria2", Address: host, Port: outbound.ServerPort, RawConfig: raw}, nil
	default:
		return domain.NodeMetadata{}, fmt.Errorf("unsupported sing-box outbound type: %s", protocol)
	}
}
