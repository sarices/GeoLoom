package parser

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"geoloom/internal/domain"
)

// ParseHysteria2 解析 hysteria2:// 或 hy2:// 节点链接。
func ParseHysteria2(rawInput string) (domain.NodeMetadata, error) {
	cleaned := strings.TrimSpace(rawInput)
	if cleaned == "" {
		return domain.NodeMetadata{}, newParseError(ErrorKindInvalidInput, rawInput, "输入不能为空", nil)
	}

	u, err := url.Parse(cleaned)
	if err != nil {
		return domain.NodeMetadata{}, newParseError(ErrorKindInvalidInput, rawInput, "URL 解析失败", err)
	}

	scheme := strings.ToLower(strings.TrimSpace(u.Scheme))
	if scheme != "hysteria2" && scheme != "hy2" {
		return domain.NodeMetadata{}, newParseError(ErrorKindUnsupportedScheme, rawInput, "仅支持 hysteria2:// 或 hy2://", nil)
	}

	password := ""
	if u.User != nil {
		password = strings.TrimSpace(u.User.Username())
	}
	if password == "" {
		return domain.NodeMetadata{}, newParseError(ErrorKindMissingField, rawInput, "hysteria2 缺少认证字段", nil)
	}

	host, port, err := parseHostAndPort(u, rawInput)
	if err != nil {
		return domain.NodeMetadata{}, err
	}

	query := u.Query()
	insecure, _ := strconv.ParseBool(strings.TrimSpace(query.Get("insecure")))

	rawConfig := map[string]any{
		"type":        "hysteria2",
		"server":      host,
		"server_port": port,
		"password":    password,
		"insecure":    insecure,
	}

	if security := strings.TrimSpace(query.Get("security")); security != "" {
		rawConfig["security"] = security
	}
	if sni := strings.TrimSpace(query.Get("sni")); sni != "" {
		rawConfig["sni"] = sni
	}
	if alpn := splitCommaValues(query.Get("alpn")); len(alpn) > 0 {
		rawConfig["alpn"] = alpn
	}

	name := normalizeNodeName(u.Fragment, fmt.Sprintf("hy2-%s:%d", host, port))

	return domain.NodeMetadata{
		ID:        buildNodeID("hysteria2", host, port, password),
		Name:      name,
		Protocol:  "hysteria2",
		Address:   host,
		Port:      port,
		RawConfig: rawConfig,
	}, nil
}
