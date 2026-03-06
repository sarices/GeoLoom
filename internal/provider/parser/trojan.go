package parser

import (
	"fmt"
	"net/url"
	"strings"

	"geoloom/internal/domain"
)

// ParseTrojan 解析 trojan:// 节点链接。
func ParseTrojan(rawInput string) (domain.NodeMetadata, error) {
	cleaned := strings.TrimSpace(rawInput)
	if cleaned == "" {
		return domain.NodeMetadata{}, newParseError(ErrorKindInvalidInput, rawInput, "输入不能为空", nil)
	}

	u, err := url.Parse(cleaned)
	if err != nil {
		return domain.NodeMetadata{}, newParseError(ErrorKindInvalidInput, rawInput, "URL 解析失败", err)
	}

	if strings.ToLower(strings.TrimSpace(u.Scheme)) != "trojan" {
		return domain.NodeMetadata{}, newParseError(ErrorKindUnsupportedScheme, rawInput, "仅支持 trojan://", nil)
	}

	password := ""
	if u.User != nil {
		password = strings.TrimSpace(u.User.Username())
	}
	if password == "" {
		return domain.NodeMetadata{}, newParseError(ErrorKindMissingField, rawInput, "trojan 缺少密码", nil)
	}

	host, port, err := parseHostAndPort(u, rawInput)
	if err != nil {
		return domain.NodeMetadata{}, err
	}

	query := u.Query()
	rawConfig := map[string]any{
		"type":        "trojan",
		"server":      host,
		"server_port": port,
		"password":    password,
	}

	if security := strings.TrimSpace(query.Get("security")); security != "" {
		rawConfig["security"] = security
	}
	if sni := strings.TrimSpace(query.Get("sni")); sni != "" {
		rawConfig["sni"] = sni
	}
	if insecure := strings.TrimSpace(query.Get("allowInsecure")); insecure != "" {
		rawConfig["insecure"] = insecure
	}
	if network := strings.TrimSpace(query.Get("type")); network != "" {
		rawConfig["network"] = network
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

	name := normalizeNodeName(u.Fragment, fmt.Sprintf("trojan-%s:%d", host, port))

	return domain.NodeMetadata{
		ID:        buildNodeID("trojan", host, port, password),
		Name:      name,
		Protocol:  "trojan",
		Address:   host,
		Port:      port,
		RawConfig: rawConfig,
	}, nil
}
