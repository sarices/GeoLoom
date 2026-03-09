package domain

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// BuildNodeFingerprint 基于协议关键身份字段构建稳定节点指纹。
func BuildNodeFingerprint(node NodeMetadata) (string, error) {
	protocol := normalizeLower(node.Protocol)
	if protocol == "" {
		return "", fmt.Errorf("节点协议不能为空")
	}

	address := normalizeAddress(node.Address)
	port := strconv.Itoa(node.Port)
	raw := node.RawConfig
	if raw == nil {
		raw = map[string]any{}
	}

	parts := []string{"protocol=" + protocol, "address=" + address, "port=" + port}
	switch protocol {
	case "socks5":
		parts = append(parts,
			"username="+normalizeString(raw["username"]),
			"password="+normalizeString(raw["password"]),
		)
	case "socks4":
		parts = append(parts,
			"username="+normalizeString(raw["username"]),
		)
	case "http":
		parts = append(parts,
			"username="+normalizeString(raw["username"]),
			"password="+normalizeString(raw["password"]),
		)
	case "hysteria2":
		parts = append(parts,
			"password="+normalizeString(raw["password"]),
			"security="+normalizeLower(raw["security"]),
			"sni="+normalizeHost(raw["sni"]),
		)
	case "vless":
		parts = append(parts,
			"uuid="+normalizeString(raw["uuid"]),
			"flow="+normalizeString(raw["flow"]),
			"security="+normalizeLower(raw["security"]),
			"network="+normalizeLower(raw["network"]),
			"sni="+normalizeHost(raw["sni"]),
			"host="+normalizeHost(raw["host"]),
			"path="+normalizePath(raw["path"]),
		)
	case "trojan":
		parts = append(parts,
			"password="+normalizeString(raw["password"]),
			"security="+normalizeLower(raw["security"]),
			"network="+normalizeLower(raw["network"]),
			"sni="+normalizeHost(raw["sni"]),
			"host="+normalizeHost(raw["host"]),
			"path="+normalizePath(raw["path"]),
		)
	case "vmess":
		parts = append(parts,
			"uuid="+normalizeString(raw["uuid"]),
			"alter_id="+normalizeIntString(raw["alter_id"]),
			"cipher="+normalizeLower(raw["cipher"]),
			"security="+normalizeLower(raw["security"]),
			"network="+normalizeLower(raw["network"]),
			"sni="+normalizeHost(raw["sni"]),
			"host="+normalizeHost(raw["host"]),
			"path="+normalizePath(raw["path"]),
		)
	case "shadowsocks":
		parts = append(parts,
			"method="+normalizeString(raw["method"]),
			"password="+normalizeString(raw["password"]),
		)
	default:
		return "", fmt.Errorf("不支持的节点协议: %s", protocol)
	}

	sum := sha1.Sum([]byte(strings.Join(parts, "|")))
	return fmt.Sprintf("%s-%s", protocol, hex.EncodeToString(sum[:])[:12]), nil
}

func normalizeString(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func normalizeLower(value any) string {
	return strings.ToLower(normalizeString(value))
}

func normalizeHost(value any) string {
	return normalizeAddress(normalizeString(value))
}

func normalizeAddress(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if ip := net.ParseIP(trimmed); ip != nil {
		return ip.String()
	}
	return strings.ToLower(trimmed)
}

func normalizePath(value any) string {
	return normalizeString(value)
}

func normalizeIntString(value any) string {
	trimmed := strings.TrimSpace(fmt.Sprint(value))
	if trimmed == "" || trimmed == "<nil>" {
		return ""
	}
	return trimmed
}
