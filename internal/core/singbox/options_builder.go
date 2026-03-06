package singbox

import (
	"errors"
	"fmt"
	"math"
	"net/netip"
	"strconv"
	"strings"
	"time"
	"unicode"

	"geoloom/internal/config"
	"geoloom/internal/domain"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json/badoption"
)

const (
	defaultInboundTag  = "in-socks"
	directOutboundTag  = "direct"
	defaultLBTag       = "lb-out"
	defaultListenAddr  = "0.0.0.0"
	maxValidPortNumber = 65535
)

// CoreBuildStats 表示 Core Builder 最近一次构建统计。
type CoreBuildStats struct {
	Unsupported         []string
	SupportedCandidates int
}

// OptionsBuilder 将候选节点转换为 sing-box option.Options。
type OptionsBuilder struct {
	lastBuildStats CoreBuildStats
}

func NewOptionsBuilder() *OptionsBuilder {
	return &OptionsBuilder{}
}

func (b *OptionsBuilder) LastBuildStats() CoreBuildStats {
	return CoreBuildStats{
		Unsupported:         append([]string(nil), b.lastBuildStats.Unsupported...),
		SupportedCandidates: b.lastBuildStats.SupportedCandidates,
	}
}

// Build 构建最小可用 sing-box 配置：SOCKS 入站 + 节点出站 + direct + lb-out(random/urltest) + final route。
func (b *OptionsBuilder) Build(cfg config.Config, nodes []domain.NodeMetadata) (*option.Options, error) {
	b.lastBuildStats = CoreBuildStats{}

	if len(nodes) == 0 {
		return nil, fmt.Errorf("候选节点为空，无法构建 sing-box 配置")
	}
	if cfg.Gateway.SocksPort <= 0 || cfg.Gateway.SocksPort > maxValidPortNumber {
		return nil, fmt.Errorf("gateway.socks_port 非法: %d", cfg.Gateway.SocksPort)
	}

	inbounds, err := buildInbounds(cfg)
	if err != nil {
		return nil, err
	}

	nodeOutbounds := make([]option.Outbound, 0, len(nodes))
	nodeTags := make([]string, 0, len(nodes))
	tagSeen := make(map[string]int, len(nodes)+2)
	unsupported := make([]string, 0)

	for i, node := range nodes {
		outbound, err := buildNodeOutbound(node, i, tagSeen)
		if err != nil {
			if isNodeUnsupportedError(err) {
				unsupported = append(unsupported, err.Error())
				continue
			}
			return nil, err
		}
		nodeOutbounds = append(nodeOutbounds, outbound)
		nodeTags = append(nodeTags, outbound.Tag)
	}

	if len(nodeOutbounds) == 0 {
		b.lastBuildStats = CoreBuildStats{
			Unsupported:         append([]string(nil), unsupported...),
			SupportedCandidates: 0,
		}
		return nil, fmt.Errorf("所有候选节点均为当前阶段不支持的协议变体，无法构建 sing-box 配置")
	}

	b.lastBuildStats = CoreBuildStats{
		Unsupported:         append([]string(nil), unsupported...),
		SupportedCandidates: len(nodeOutbounds),
	}

	lbOutbound := buildLBOutbound(cfg, nodeTags)

	outbounds := make([]option.Outbound, 0, len(nodeOutbounds)+2)
	outbounds = append(outbounds, nodeOutbounds...)
	outbounds = append(outbounds,
		option.Outbound{
			Type:    C.TypeDirect,
			Tag:     directOutboundTag,
			Options: &option.DirectOutboundOptions{},
		},
		lbOutbound,
	)

	return &option.Options{
		Log: &option.LogOptions{
			Level:     "info",
			Timestamp: true,
		},
		Inbounds:  inbounds,
		Outbounds: outbounds,
		Route: &option.RouteOptions{
			Final: defaultLBTag,
		},
	}, nil
}

func buildLBOutbound(cfg config.Config, nodeTags []string) option.Outbound {
	strategy := cfg.Policy.Strategy
	if strategy == config.StrategyURLTest {
		return buildURLTestOutbound(nodeTags, cfg.Policy.HealthCheck)
	}
	return buildRandomOutbound(nodeTags)
}

func buildRandomOutbound(nodeTags []string) option.Outbound {
	outbounds := make([]string, 0, len(nodeTags))
	outbounds = append(outbounds, nodeTags...)
	return option.Outbound{
		Type: geoloomRandomOutboundType,
		Tag:  defaultLBTag,
		Options: &geoloomRandomOutboundOptions{
			Outbounds: outbounds,
		},
	}
}

func buildURLTestOutbound(nodeTags []string, health config.HealthCheckConfig) option.Outbound {
	url := strings.TrimSpace(health.URL)
	if url == "" {
		url = "https://www.gstatic.com/generate_204"
	}
	intervalText := strings.TrimSpace(health.Interval)
	if intervalText == "" {
		intervalText = "5m"
	}
	interval, err := time.ParseDuration(intervalText)
	if err != nil || interval <= 0 {
		interval = 5 * time.Minute
	}

	outbounds := make([]string, 0, len(nodeTags)+1)
	outbounds = append(outbounds, nodeTags...)
	outbounds = append(outbounds, directOutboundTag)

	return option.Outbound{
		Type: C.TypeURLTest,
		Tag:  defaultLBTag,
		Options: &option.URLTestOutboundOptions{
			Outbounds: outbounds,
			URL:       url,
			Interval:  badoption.Duration(interval),
		},
	}
}

func buildInbounds(cfg config.Config) ([]option.Inbound, error) {
	listenAddr, err := netip.ParseAddr(defaultListenAddr)
	if err != nil {
		return nil, fmt.Errorf("解析默认监听地址失败: %w", err)
	}
	listen := badoption.Addr(listenAddr)

	return []option.Inbound{
		{
			Type: C.TypeSOCKS,
			Tag:  defaultInboundTag,
			Options: &option.SocksInboundOptions{
				ListenOptions: option.ListenOptions{
					Listen:     &listen,
					ListenPort: uint16(cfg.Gateway.SocksPort),
				},
			},
		},
	}, nil
}

type nodeUnsupportedError struct {
	nodeID string
	reason string
}

func (e nodeUnsupportedError) Error() string {
	return fmt.Sprintf("节点 %s %s", e.nodeID, e.reason)
}

func newNodeUnsupportedError(nodeID, reason string) error {
	return nodeUnsupportedError{nodeID: nodeID, reason: reason}
}

func isNodeUnsupportedError(err error) bool {
	if err == nil {
		return false
	}
	var target nodeUnsupportedError
	return errors.As(err, &target)
}

func normalizeNodeIDForError(node domain.NodeMetadata, index int) string {
	id := strings.TrimSpace(node.ID)
	if id != "" {
		return id
	}
	return fmt.Sprintf("index=%d", index)
}

func buildNodeOutbound(node domain.NodeMetadata, index int, tagSeen map[string]int) (option.Outbound, error) {
	protocol := normalizeProtocol(node)
	if protocol == "" {
		return option.Outbound{}, fmt.Errorf("节点[%d] 缺少协议类型", index)
	}

	server, serverPort, err := resolveServer(node)
	if err != nil {
		return option.Outbound{}, err
	}

	tag := ensureUniqueTag(buildNodeTag(node, index), tagSeen)

	switch protocol {
	case "socks5":
		options := &option.SOCKSOutboundOptions{
			ServerOptions: option.ServerOptions{
				Server:     server,
				ServerPort: serverPort,
			},
		}
		if username, ok := readString(node.RawConfig, "username"); ok {
			options.Username = username
		}
		if password, ok := readString(node.RawConfig, "password"); ok {
			options.Password = password
		}
		return option.Outbound{Type: C.TypeSOCKS, Tag: tag, Options: options}, nil
	case "hysteria2":
		options := &option.Hysteria2OutboundOptions{
			ServerOptions: option.ServerOptions{
				Server:     server,
				ServerPort: serverPort,
			},
		}
		password, ok := readString(node.RawConfig, "password")
		if !ok {
			return option.Outbound{}, fmt.Errorf("节点 %s 缺少 hysteria2 password 字段", node.ID)
		}
		options.Password = password

		security, _ := readString(node.RawConfig, "security")
		sni, _ := readString(node.RawConfig, "sni")
		alpn, err := readStringSlice(node.RawConfig, "alpn")
		if err != nil {
			return option.Outbound{}, fmt.Errorf("节点 %s alpn 字段非法: %w", node.ID, err)
		}
		insecure, _, err := readBool(node.RawConfig, "insecure")
		if err != nil {
			return option.Outbound{}, fmt.Errorf("节点 %s insecure 字段非法: %w", node.ID, err)
		}

		if strings.EqualFold(security, "tls") || sni != "" || len(alpn) > 0 || insecure {
			tlsOptions := &option.OutboundTLSOptions{Enabled: true, Insecure: insecure}
			if sni != "" {
				tlsOptions.ServerName = sni
			}
			if len(alpn) > 0 {
				tlsOptions.ALPN = badoption.Listable[string](alpn)
			}
			options.OutboundTLSOptionsContainer.TLS = tlsOptions
		}

		return option.Outbound{Type: C.TypeHysteria2, Tag: tag, Options: options}, nil
	case "vless":
		options := &option.VLESSOutboundOptions{
			ServerOptions: option.ServerOptions{
				Server:     server,
				ServerPort: serverPort,
			},
		}
		uuid, ok := readString(node.RawConfig, "uuid")
		if !ok {
			return option.Outbound{}, fmt.Errorf("节点 %s 缺少 vless uuid 字段", node.ID)
		}
		options.UUID = uuid

		if flow, ok := readString(node.RawConfig, "flow"); ok {
			options.Flow = flow
		}

		security, _ := readString(node.RawConfig, "security")
		sni, _ := readString(node.RawConfig, "sni")
		alpn, err := readStringSlice(node.RawConfig, "alpn")
		if err != nil {
			return option.Outbound{}, fmt.Errorf("节点 %s alpn 字段非法: %w", node.ID, err)
		}
		if strings.EqualFold(security, "tls") || sni != "" || len(alpn) > 0 {
			tlsOptions := &option.OutboundTLSOptions{Enabled: true}
			if sni != "" {
				tlsOptions.ServerName = sni
			}
			if len(alpn) > 0 {
				tlsOptions.ALPN = badoption.Listable[string](alpn)
			}
			options.OutboundTLSOptionsContainer.TLS = tlsOptions
		}

		network, _ := readString(node.RawConfig, "network")
		switch strings.ToLower(strings.TrimSpace(network)) {
		case "", "tcp":
			// 默认 TCP，无需额外 transport 配置。
		case "ws":
			wsOptions := option.V2RayWebsocketOptions{}
			if path, ok := readString(node.RawConfig, "path"); ok {
				wsOptions.Path = path
			}
			if hostHeader, ok := readString(node.RawConfig, "host"); ok {
				wsOptions.Headers = badoption.HTTPHeader{
					"Host": badoption.Listable[string]{hostHeader},
				}
			}
			options.Transport = &option.V2RayTransportOptions{
				Type:             C.V2RayTransportTypeWebsocket,
				WebsocketOptions: wsOptions,
			}
		default:
			nodeID := normalizeNodeIDForError(node, index)
			return option.Outbound{}, newNodeUnsupportedError(nodeID, fmt.Sprintf("使用了当前阶段未支持的 vless network=%q", network))
		}

		return option.Outbound{Type: C.TypeVLESS, Tag: tag, Options: options}, nil
	case "trojan":
		options := &option.TrojanOutboundOptions{
			ServerOptions: option.ServerOptions{
				Server:     server,
				ServerPort: serverPort,
			},
		}
		password, ok := readString(node.RawConfig, "password")
		if !ok {
			return option.Outbound{}, fmt.Errorf("节点 %s 缺少 trojan password 字段", node.ID)
		}
		options.Password = password

		security, _ := readString(node.RawConfig, "security")
		sni, _ := readString(node.RawConfig, "sni")
		alpn, err := readStringSlice(node.RawConfig, "alpn")
		if err != nil {
			return option.Outbound{}, fmt.Errorf("节点 %s alpn 字段非法: %w", node.ID, err)
		}
		insecure, _, err := readBool(node.RawConfig, "insecure")
		if err != nil {
			return option.Outbound{}, fmt.Errorf("节点 %s insecure 字段非法: %w", node.ID, err)
		}
		if strings.EqualFold(security, "tls") || sni != "" || len(alpn) > 0 || insecure {
			tlsOptions := &option.OutboundTLSOptions{Enabled: true, Insecure: insecure}
			if sni != "" {
				tlsOptions.ServerName = sni
			}
			if len(alpn) > 0 {
				tlsOptions.ALPN = badoption.Listable[string](alpn)
			}
			options.OutboundTLSOptionsContainer.TLS = tlsOptions
		}

		network, _ := readString(node.RawConfig, "network")
		switch strings.ToLower(strings.TrimSpace(network)) {
		case "", "tcp":
			// 默认 TCP。
		case "ws":
			wsOptions := option.V2RayWebsocketOptions{}
			if path, ok := readString(node.RawConfig, "path"); ok {
				wsOptions.Path = path
			}
			if hostHeader, ok := readString(node.RawConfig, "host"); ok {
				wsOptions.Headers = badoption.HTTPHeader{
					"Host": badoption.Listable[string]{hostHeader},
				}
			}
			options.Transport = &option.V2RayTransportOptions{
				Type:             C.V2RayTransportTypeWebsocket,
				WebsocketOptions: wsOptions,
			}
		default:
			nodeID := normalizeNodeIDForError(node, index)
			return option.Outbound{}, newNodeUnsupportedError(nodeID, fmt.Sprintf("使用了当前阶段未支持的 trojan network=%q", network))
		}
		return option.Outbound{Type: C.TypeTrojan, Tag: tag, Options: options}, nil
	case "vmess":
		options := &option.VMessOutboundOptions{
			ServerOptions: option.ServerOptions{
				Server:     server,
				ServerPort: serverPort,
			},
		}
		uuid, ok := readString(node.RawConfig, "uuid")
		if !ok {
			return option.Outbound{}, fmt.Errorf("节点 %s 缺少 vmess uuid 字段", node.ID)
		}
		options.UUID = uuid

		if security, ok := readString(node.RawConfig, "cipher"); ok {
			options.Security = security
		}
		if alterID, ok, err := readInt(node.RawConfig, "alter_id"); err != nil {
			return option.Outbound{}, fmt.Errorf("节点 %s alter_id 字段非法: %w", node.ID, err)
		} else if ok {
			options.AlterId = alterID
		}

		transportSecurity, _ := readString(node.RawConfig, "security")
		sni, _ := readString(node.RawConfig, "sni")
		alpn, err := readStringSlice(node.RawConfig, "alpn")
		if err != nil {
			return option.Outbound{}, fmt.Errorf("节点 %s alpn 字段非法: %w", node.ID, err)
		}
		if strings.EqualFold(transportSecurity, "tls") || sni != "" || len(alpn) > 0 {
			tlsOptions := &option.OutboundTLSOptions{Enabled: true}
			if sni != "" {
				tlsOptions.ServerName = sni
			}
			if len(alpn) > 0 {
				tlsOptions.ALPN = badoption.Listable[string](alpn)
			}
			options.OutboundTLSOptionsContainer.TLS = tlsOptions
		}

		network, _ := readString(node.RawConfig, "network")
		switch strings.ToLower(strings.TrimSpace(network)) {
		case "", "tcp":
			// 默认 TCP。
		case "ws":
			wsOptions := option.V2RayWebsocketOptions{}
			if path, ok := readString(node.RawConfig, "path"); ok {
				wsOptions.Path = path
			}
			if hostHeader, ok := readString(node.RawConfig, "host"); ok {
				wsOptions.Headers = badoption.HTTPHeader{
					"Host": badoption.Listable[string]{hostHeader},
				}
			}
			options.Transport = &option.V2RayTransportOptions{
				Type:             C.V2RayTransportTypeWebsocket,
				WebsocketOptions: wsOptions,
			}
		default:
			nodeID := normalizeNodeIDForError(node, index)
			return option.Outbound{}, newNodeUnsupportedError(nodeID, fmt.Sprintf("使用了当前阶段未支持的 vmess network=%q", network))
		}
		return option.Outbound{Type: C.TypeVMess, Tag: tag, Options: options}, nil
	case "shadowsocks":
		options := &option.ShadowsocksOutboundOptions{
			ServerOptions: option.ServerOptions{
				Server:     server,
				ServerPort: serverPort,
			},
		}
		method, ok := readString(node.RawConfig, "method")
		if !ok {
			return option.Outbound{}, fmt.Errorf("节点 %s 缺少 shadowsocks method 字段", node.ID)
		}
		password, ok := readString(node.RawConfig, "password")
		if !ok {
			return option.Outbound{}, fmt.Errorf("节点 %s 缺少 shadowsocks password 字段", node.ID)
		}
		options.Method = method
		options.Password = password
		return option.Outbound{Type: C.TypeShadowsocks, Tag: tag, Options: options}, nil
	default:
		return option.Outbound{}, fmt.Errorf("节点 %s 使用了未支持协议: %s", node.ID, protocol)
	}
}

func resolveServer(node domain.NodeMetadata) (string, uint16, error) {
	server := strings.TrimSpace(node.Address)
	if rawServer, ok := readString(node.RawConfig, "server"); ok {
		server = rawServer
	}
	if server == "" {
		return "", 0, fmt.Errorf("节点 %s 缺少 server/address", node.ID)
	}

	port := node.Port
	if rawPort, ok, err := readInt(node.RawConfig, "server_port"); err != nil {
		return "", 0, fmt.Errorf("节点 %s server_port 字段非法: %w", node.ID, err)
	} else if ok {
		port = rawPort
	}
	if port <= 0 || port > maxValidPortNumber {
		return "", 0, fmt.Errorf("节点 %s 端口非法: %d", node.ID, port)
	}

	return server, uint16(port), nil
}

func normalizeProtocol(node domain.NodeMetadata) string {
	protocol := strings.ToLower(strings.TrimSpace(node.Protocol))
	if protocol == "" {
		rawType, _ := readString(node.RawConfig, "type")
		protocol = strings.ToLower(strings.TrimSpace(rawType))
	}

	switch protocol {
	case "socks":
		return "socks5"
	case "ss":
		return "shadowsocks"
	case "hy2":
		return "hysteria2"
	default:
		return protocol
	}
}

func buildNodeTag(node domain.NodeMetadata, index int) string {
	base := strings.TrimSpace(node.ID)
	if base == "" {
		base = fmt.Sprintf("node-%d", index+1)
	}
	base = sanitizeTag(base)
	if base == "" {
		base = fmt.Sprintf("node-%d", index+1)
	}
	if base == directOutboundTag || base == defaultLBTag || base == defaultInboundTag {
		base = "node-" + base
	}
	return base
}

func sanitizeTag(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))
	for _, r := range value {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), r == '-', r == '_', r == '.':
			builder.WriteRune(unicode.ToLower(r))
		default:
			builder.WriteByte('-')
		}
	}
	return strings.Trim(builder.String(), "-")
}

func ensureUniqueTag(base string, seen map[string]int) string {
	count := seen[base]
	if count == 0 {
		seen[base] = 1
		return base
	}
	count++
	seen[base] = count
	return fmt.Sprintf("%s-%d", base, count)
}

func readString(raw map[string]any, key string) (string, bool) {
	if raw == nil {
		return "", false
	}
	value, exists := raw[key]
	if !exists || value == nil {
		return "", false
	}
	text, ok := value.(string)
	if !ok {
		return "", false
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return "", false
	}
	return text, true
}

func readInt(raw map[string]any, key string) (int, bool, error) {
	if raw == nil {
		return 0, false, nil
	}
	value, exists := raw[key]
	if !exists || value == nil {
		return 0, false, nil
	}

	switch typed := value.(type) {
	case int:
		return typed, true, nil
	case int8:
		return int(typed), true, nil
	case int16:
		return int(typed), true, nil
	case int32:
		return int(typed), true, nil
	case int64:
		return int(typed), true, nil
	case uint:
		return int(typed), true, nil
	case uint8:
		return int(typed), true, nil
	case uint16:
		return int(typed), true, nil
	case uint32:
		return int(typed), true, nil
	case uint64:
		if typed > math.MaxInt {
			return 0, true, fmt.Errorf("数值超出 int 范围: %d", typed)
		}
		return int(typed), true, nil
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return 0, true, fmt.Errorf("非法浮点值: %v", typed)
		}
		if math.Trunc(typed) != typed {
			return 0, true, fmt.Errorf("非整数值: %v", typed)
		}
		if typed > float64(math.MaxInt) || typed < float64(math.MinInt) {
			return 0, true, fmt.Errorf("数值超出 int 范围: %v", typed)
		}
		return int(typed), true, nil
	case float32:
		floatValue := float64(typed)
		if math.IsNaN(floatValue) || math.IsInf(floatValue, 0) {
			return 0, true, fmt.Errorf("非法浮点值: %v", typed)
		}
		if math.Trunc(floatValue) != floatValue {
			return 0, true, fmt.Errorf("非整数值: %v", typed)
		}
		if floatValue > float64(math.MaxInt) || floatValue < float64(math.MinInt) {
			return 0, true, fmt.Errorf("数值超出 int 范围: %v", typed)
		}
		return int(floatValue), true, nil
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return 0, true, err
		}
		return parsed, true, nil
	default:
		return 0, true, fmt.Errorf("不支持的数据类型: %T", value)
	}
}

func readBool(raw map[string]any, key string) (bool, bool, error) {
	if raw == nil {
		return false, false, nil
	}
	value, exists := raw[key]
	if !exists || value == nil {
		return false, false, nil
	}

	switch typed := value.(type) {
	case bool:
		return typed, true, nil
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(typed))
		if err != nil {
			return false, true, err
		}
		return parsed, true, nil
	default:
		return false, true, fmt.Errorf("不支持的数据类型: %T", value)
	}
}

func readStringSlice(raw map[string]any, key string) ([]string, error) {
	if raw == nil {
		return nil, nil
	}
	value, exists := raw[key]
	if !exists || value == nil {
		return nil, nil
	}

	normalize := func(items []string) []string {
		result := make([]string, 0, len(items))
		for _, item := range items {
			normalized := strings.TrimSpace(item)
			if normalized == "" {
				continue
			}
			result = append(result, normalized)
		}
		if len(result) == 0 {
			return nil
		}
		return result
	}

	switch typed := value.(type) {
	case []string:
		return normalize(typed), nil
	case []any:
		items := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("元素类型非法: %T", item)
			}
			items = append(items, text)
		}
		return normalize(items), nil
	case string:
		return normalize([]string{typed}), nil
	default:
		return nil, fmt.Errorf("不支持的数据类型: %T", value)
	}
}
