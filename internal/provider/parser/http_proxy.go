package parser

import (
	"fmt"
	"net/url"
	"strings"

	"geoloom/internal/domain"
)

// ParseHTTPProxy 解析 http:// 代理节点链接。
func ParseHTTPProxy(rawInput string) (domain.NodeMetadata, error) {
	cleaned := strings.TrimSpace(rawInput)
	if cleaned == "" {
		return domain.NodeMetadata{}, newParseError(ErrorKindInvalidInput, rawInput, "输入不能为空", nil)
	}

	u, err := url.Parse(cleaned)
	if err != nil {
		return domain.NodeMetadata{}, newParseError(ErrorKindInvalidInput, rawInput, "URL 解析失败", err)
	}

	scheme := strings.ToLower(strings.TrimSpace(u.Scheme))
	if scheme != "http" {
		return domain.NodeMetadata{}, newParseError(ErrorKindUnsupportedScheme, rawInput, "仅支持 http://", nil)
	}

	host, port, err := parseHostAndPort(u, rawInput)
	if err != nil {
		return domain.NodeMetadata{}, err
	}

	username := ""
	password := ""
	if u.User != nil {
		username = strings.TrimSpace(u.User.Username())
		password, _ = u.User.Password()
		password = strings.TrimSpace(password)
	}

	rawConfig := map[string]any{
		"type":        "http",
		"server":      host,
		"server_port": port,
	}
	if username != "" {
		rawConfig["username"] = username
	}
	if password != "" {
		rawConfig["password"] = password
	}

	name := normalizeNodeName(u.Fragment, fmt.Sprintf("http-%s:%d", host, port))

	return domain.NodeMetadata{
		ID:        buildNodeID("http", host, port, username+":"+password),
		Name:      name,
		Protocol:  "http",
		Address:   host,
		Port:      port,
		RawConfig: rawConfig,
	}, nil
}
