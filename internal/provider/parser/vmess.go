package parser

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"geoloom/internal/domain"
)

type vmessLinkPayload struct {
	Address  string `json:"add"`
	Port     any    `json:"port"`
	UUID     string `json:"id"`
	AlterID  any    `json:"aid"`
	Security string `json:"scy"`
	Network  string `json:"net"`
	Host     string `json:"host"`
	Path     string `json:"path"`
	TLS      string `json:"tls"`
	SNI      string `json:"sni"`
	ALPN     string `json:"alpn"`
	Name     string `json:"ps"`
}

// ParseVMess 解析 vmess:// 节点链接（最小子集）。
func ParseVMess(rawInput string) (domain.NodeMetadata, error) {
	cleaned := strings.TrimSpace(rawInput)
	if cleaned == "" {
		return domain.NodeMetadata{}, newParseError(ErrorKindInvalidInput, rawInput, "输入不能为空", nil)
	}

	u, err := url.Parse(cleaned)
	if err != nil {
		return domain.NodeMetadata{}, newParseError(ErrorKindInvalidInput, rawInput, "URL 解析失败", err)
	}

	if strings.ToLower(strings.TrimSpace(u.Scheme)) != "vmess" {
		return domain.NodeMetadata{}, newParseError(ErrorKindUnsupportedScheme, rawInput, "仅支持 vmess://", nil)
	}

	payloadText, err := decodeVmessPayload(strings.TrimSpace(strings.TrimPrefix(cleaned, "vmess://")))
	if err != nil {
		return domain.NodeMetadata{}, newParseError(ErrorKindInvalidInput, rawInput, "vmess payload 解码失败", err)
	}

	var payload vmessLinkPayload
	if err := json.Unmarshal([]byte(payloadText), &payload); err != nil {
		return domain.NodeMetadata{}, newParseError(ErrorKindInvalidInput, rawInput, "vmess payload 解析失败", err)
	}

	host := strings.TrimSpace(payload.Address)
	if host == "" {
		return domain.NodeMetadata{}, newParseError(ErrorKindMissingField, rawInput, "vmess 缺少 add 字段", nil)
	}

	port, err := parseFlexibleInt(payload.Port)
	if err != nil || port <= 0 || port > 65535 {
		return domain.NodeMetadata{}, newParseError(ErrorKindInvalidInput, rawInput, "vmess 端口非法", err)
	}

	uuid := strings.TrimSpace(payload.UUID)
	if uuid == "" {
		return domain.NodeMetadata{}, newParseError(ErrorKindMissingField, rawInput, "vmess 缺少 id 字段", nil)
	}

	rawConfig := map[string]any{
		"type":        "vmess",
		"server":      host,
		"server_port": port,
		"uuid":        uuid,
	}
	if security := strings.TrimSpace(payload.Security); security != "" {
		rawConfig["cipher"] = security
	}
	if alterID, err := parseFlexibleInt(payload.AlterID); err == nil && alterID >= 0 {
		rawConfig["alter_id"] = alterID
	}
	if network := strings.TrimSpace(payload.Network); network != "" {
		rawConfig["network"] = network
	}
	if hostHeader := strings.TrimSpace(payload.Host); hostHeader != "" {
		rawConfig["host"] = hostHeader
	}
	if path := strings.TrimSpace(payload.Path); path != "" {
		rawConfig["path"] = path
	}
	if strings.EqualFold(strings.TrimSpace(payload.TLS), "tls") {
		rawConfig["security"] = "tls"
	}
	if sni := strings.TrimSpace(payload.SNI); sni != "" {
		rawConfig["sni"] = sni
	}
	if alpn := splitCommaValues(payload.ALPN); len(alpn) > 0 {
		rawConfig["alpn"] = alpn
	}

	name := normalizeNodeName(payload.Name, fmt.Sprintf("vmess-%s:%d", host, port))

	return domain.NodeMetadata{
		ID:        buildNodeID("vmess", host, port, uuid),
		Name:      name,
		Protocol:  "vmess",
		Address:   host,
		Port:      port,
		RawConfig: rawConfig,
	}, nil
}

func parseFlexibleInt(value any) (int, error) {
	switch typed := value.(type) {
	case nil:
		return 0, fmt.Errorf("数值为空")
	case int:
		return typed, nil
	case int8:
		return int(typed), nil
	case int16:
		return int(typed), nil
	case int32:
		return int(typed), nil
	case int64:
		return int(typed), nil
	case uint:
		return int(typed), nil
	case uint8:
		return int(typed), nil
	case uint16:
		return int(typed), nil
	case uint32:
		return int(typed), nil
	case uint64:
		if typed > uint64(^uint(0)>>1) {
			return 0, fmt.Errorf("数值超出范围")
		}
		return int(typed), nil
	case float64:
		if typed != float64(int(typed)) {
			return 0, fmt.Errorf("非整数数值")
		}
		return int(typed), nil
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return 0, err
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("不支持的数据类型: %T", value)
	}
}

func decodeVmessPayload(payload string) (string, error) {
	if payload == "" {
		return "", fmt.Errorf("payload 为空")
	}
	compact := strings.Join(strings.Fields(payload), "")
	decoded, err := base64.StdEncoding.DecodeString(compact)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(compact)
		if err != nil {
			return "", err
		}
	}
	result := strings.TrimSpace(string(decoded))
	if result == "" {
		return "", fmt.Errorf("payload 为空")
	}
	return result, nil
}
