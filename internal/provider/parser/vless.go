package parser

import (
	"fmt"
	"net/url"
	"strings"

	"geoloom/internal/domain"
)

// ParseVLESS 提供 VLESS 的最小字段解析入口。
func ParseVLESS(rawInput string) (domain.NodeMetadata, error) {
	cleaned := strings.TrimSpace(rawInput)
	if cleaned == "" {
		return domain.NodeMetadata{}, newParseError(ErrorKindInvalidInput, rawInput, "输入不能为空", nil)
	}

	u, err := url.Parse(cleaned)
	if err != nil {
		return domain.NodeMetadata{}, newParseError(ErrorKindInvalidInput, rawInput, "URL 解析失败", err)
	}

	if strings.ToLower(strings.TrimSpace(u.Scheme)) != "vless" {
		return domain.NodeMetadata{}, newParseError(ErrorKindUnsupportedScheme, rawInput, "仅支持 vless://", nil)
	}

	uuid := ""
	if u.User != nil {
		uuid = strings.TrimSpace(u.User.Username())
	}
	if uuid == "" {
		return domain.NodeMetadata{}, newParseError(ErrorKindMissingField, rawInput, "vless 缺少用户 ID", nil)
	}

	host, port, err := parseHostAndPort(u, rawInput)
	if err != nil {
		return domain.NodeMetadata{}, err
	}

	query := u.Query()
	rawConfig := map[string]any{
		"type":        "vless",
		"server":      host,
		"server_port": port,
		"uuid":        uuid,
	}

	if encryption := strings.TrimSpace(query.Get("encryption")); encryption != "" {
		rawConfig["encryption"] = encryption
	}
	if security := strings.TrimSpace(query.Get("security")); security != "" {
		rawConfig["security"] = security
	}
	if flow := strings.TrimSpace(query.Get("flow")); flow != "" {
		rawConfig["flow"] = flow
	}
	if network := strings.TrimSpace(query.Get("type")); network != "" {
		rawConfig["network"] = network
	}
	if sni := strings.TrimSpace(query.Get("sni")); sni != "" {
		rawConfig["sni"] = sni
	}
	if hostHeader := strings.TrimSpace(query.Get("host")); hostHeader != "" {
		rawConfig["host"] = hostHeader
	}
	if path := strings.TrimSpace(query.Get("path")); path != "" {
		rawConfig["path"] = path
	}
	if alpn := splitCommaValues(query.Get("alpn")); len(alpn) > 0 {
		rawConfig["alpn"] = alpn
	}

	name := normalizeNodeName(u.Fragment, fmt.Sprintf("vless-%s:%d", host, port))

	return domain.NodeMetadata{
		ID:        buildNodeID("vless", host, port, uuid),
		Name:      name,
		Protocol:  "vless",
		Address:   host,
		Port:      port,
		RawConfig: rawConfig,
	}, nil
}
