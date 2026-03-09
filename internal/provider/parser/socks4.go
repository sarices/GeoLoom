package parser

import (
	"fmt"
	"net/url"
	"strings"

	"geoloom/internal/domain"
)

// ParseSocks4 解析 socks4:// 节点链接。
func ParseSocks4(rawInput string) (domain.NodeMetadata, error) {
	cleaned := strings.TrimSpace(rawInput)
	if cleaned == "" {
		return domain.NodeMetadata{}, newParseError(ErrorKindInvalidInput, rawInput, "输入不能为空", nil)
	}

	u, err := url.Parse(cleaned)
	if err != nil {
		return domain.NodeMetadata{}, newParseError(ErrorKindInvalidInput, rawInput, "URL 解析失败", err)
	}

	scheme := strings.ToLower(strings.TrimSpace(u.Scheme))
	if scheme != "socks4" {
		return domain.NodeMetadata{}, newParseError(ErrorKindUnsupportedScheme, rawInput, "仅支持 socks4://", nil)
	}

	host, port, err := parseHostAndPort(u, rawInput)
	if err != nil {
		return domain.NodeMetadata{}, err
	}

	username := ""
	if u.User != nil {
		username = strings.TrimSpace(u.User.Username())
	}

	rawConfig := map[string]any{
		"type":        "socks4",
		"server":      host,
		"server_port": port,
	}
	if username != "" {
		rawConfig["username"] = username
	}

	name := normalizeNodeName(u.Fragment, fmt.Sprintf("socks4-%s:%d", host, port))

	return domain.NodeMetadata{
		ID:        buildNodeID("socks4", host, port, username),
		Name:      name,
		Protocol:  "socks4",
		Address:   host,
		Port:      port,
		RawConfig: rawConfig,
	}, nil
}
