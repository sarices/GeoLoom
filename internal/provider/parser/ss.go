package parser

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"

	"geoloom/internal/domain"
)

// ParseShadowsocks 解析 ss:// 节点链接（最小子集）。
func ParseShadowsocks(rawInput string) (domain.NodeMetadata, error) {
	cleaned := strings.TrimSpace(rawInput)
	if cleaned == "" {
		return domain.NodeMetadata{}, newParseError(ErrorKindInvalidInput, rawInput, "输入不能为空", nil)
	}

	u, err := url.Parse(cleaned)
	if err != nil {
		return domain.NodeMetadata{}, newParseError(ErrorKindInvalidInput, rawInput, "URL 解析失败", err)
	}

	if strings.ToLower(strings.TrimSpace(u.Scheme)) != "ss" {
		return domain.NodeMetadata{}, newParseError(ErrorKindUnsupportedScheme, rawInput, "仅支持 ss://", nil)
	}

	method, password, host, port, err := parseShadowsocksCredential(u, rawInput)
	if err != nil {
		return domain.NodeMetadata{}, err
	}

	rawConfig := map[string]any{
		"type":        "shadowsocks",
		"server":      host,
		"server_port": port,
		"method":      method,
		"password":    password,
	}

	name := normalizeNodeName(u.Fragment, fmt.Sprintf("ss-%s:%d", host, port))

	return domain.NodeMetadata{
		ID:        buildNodeID("shadowsocks", host, port, method+":"+password),
		Name:      name,
		Protocol:  "shadowsocks",
		Address:   host,
		Port:      port,
		RawConfig: rawConfig,
	}, nil
}

func parseShadowsocksCredential(u *url.URL, rawInput string) (string, string, string, int, error) {
	if u.User != nil {
		method := strings.TrimSpace(u.User.Username())
		password, _ := u.User.Password()
		password = strings.TrimSpace(password)
		host, port, err := parseHostAndPort(u, rawInput)
		if err != nil {
			return "", "", "", 0, err
		}
		if method == "" || password == "" {
			return "", "", "", 0, newParseError(ErrorKindMissingField, rawInput, "ss 缺少 method/password", nil)
		}
		return method, password, host, port, nil
	}

	host := strings.TrimSpace(u.Host)
	if host == "" {
		return "", "", "", 0, newParseError(ErrorKindMissingField, rawInput, "ss 缺少主机地址", nil)
	}

	decoded, err := base64.StdEncoding.DecodeString(host)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(host)
		if err != nil {
			return "", "", "", 0, newParseError(ErrorKindInvalidInput, rawInput, "ss 用户信息解码失败", err)
		}
	}
	decodedText := strings.TrimSpace(string(decoded))
	at := strings.LastIndex(decodedText, "@")
	if at <= 0 || at >= len(decodedText)-1 {
		return "", "", "", 0, newParseError(ErrorKindInvalidInput, rawInput, "ss 用户信息格式非法", nil)
	}
	credential := decodedText[:at]
	hostPort := decodedText[at+1:]

	parts := strings.SplitN(credential, ":", 2)
	if len(parts) != 2 {
		return "", "", "", 0, newParseError(ErrorKindInvalidInput, rawInput, "ss method/password 格式非法", nil)
	}
	method := strings.TrimSpace(parts[0])
	password := strings.TrimSpace(parts[1])
	if method == "" || password == "" {
		return "", "", "", 0, newParseError(ErrorKindMissingField, rawInput, "ss 缺少 method/password", nil)
	}

	fakeURL, err := url.Parse("ss://" + hostPort)
	if err != nil {
		return "", "", "", 0, newParseError(ErrorKindInvalidInput, rawInput, "ss 主机地址解析失败", err)
	}
	hostName, port, err := parseHostAndPort(fakeURL, rawInput)
	if err != nil {
		return "", "", "", 0, err
	}

	return method, password, hostName, port, nil
}
