package parser

import (
	"fmt"
	"net/url"
	"strings"

	"geoloom/internal/domain"
)

// ParseSocks5 解析 socks5:// 节点链接。
func ParseSocks5(rawInput string) (domain.NodeMetadata, error) {
	cleaned := strings.TrimSpace(rawInput)
	if cleaned == "" {
		return domain.NodeMetadata{}, newParseError(ErrorKindInvalidInput, rawInput, "输入不能为空", nil)
	}

	u, err := url.Parse(cleaned)
	if err != nil {
		return domain.NodeMetadata{}, newParseError(ErrorKindInvalidInput, rawInput, "URL 解析失败", err)
	}

	scheme := strings.ToLower(strings.TrimSpace(u.Scheme))
	if scheme != "socks5" {
		return domain.NodeMetadata{}, newParseError(ErrorKindUnsupportedScheme, rawInput, "仅支持 socks5://", nil)
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
		"type":        "socks5",
		"server":      host,
		"server_port": port,
	}
	if username != "" {
		rawConfig["username"] = username
	}
	if password != "" {
		rawConfig["password"] = password
	}

	name := normalizeNodeName(u.Fragment, fmt.Sprintf("socks5-%s:%d", host, port))

	return domain.NodeMetadata{
		ID:        buildNodeID("socks5", host, port, username+":"+password),
		Name:      name,
		Protocol:  "socks5",
		Address:   host,
		Port:      port,
		RawConfig: rawConfig,
	}, nil
}
