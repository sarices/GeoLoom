package singbox

import (
	"net/netip"
	"testing"

	"geoloom/internal/config"
	"geoloom/internal/domain"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/service"
)

func TestOptionsBuilderBuildSuccess(t *testing.T) {
	t.Parallel()

	builder := NewOptionsBuilder()
	cfg := config.Config{Gateway: config.GatewayConfig{SocksPort: 1080}}
	nodes := []domain.NodeMetadata{
		{
			ID:       "hy2-a",
			Protocol: "hysteria2",
			Address:  "1.1.1.1",
			Port:     443,
			RawConfig: map[string]any{
				"type":        "hysteria2",
				"server":      "1.1.1.1",
				"server_port": 443,
				"password":    "secret",
				"security":    "tls",
				"sni":         "example.com",
				"alpn":        []string{"h3"},
				"insecure":    true,
			},
		},
		{
			ID:       "socks-a",
			Protocol: "socks5",
			Address:  "2.2.2.2",
			Port:     1080,
			RawConfig: map[string]any{
				"type":        "socks5",
				"server":      "2.2.2.2",
				"server_port": 1080,
				"username":    "user",
				"password":    "pass",
			},
		},
		{
			ID:       "vless-a",
			Protocol: "vless",
			Address:  "v.example.com",
			Port:     8443,
			RawConfig: map[string]any{
				"type":        "vless",
				"server":      "v.example.com",
				"server_port": 8443,
				"uuid":        "11111111-1111-1111-1111-111111111111",
				"security":    "tls",
				"sni":         "v.example.com",
				"network":     "ws",
				"path":        "/ws",
				"host":        "cdn.example.com",
			},
		},
	}

	options, err := builder.Build(cfg, nodes)
	if err != nil {
		t.Fatalf("Build 返回错误: %v", err)
	}
	if options == nil {
		t.Fatal("Build 返回 nil options")
	}

	if len(options.Inbounds) != 1 {
		t.Fatalf("inbounds 数量错误: got=%d", len(options.Inbounds))
	}
	if options.Inbounds[0].Type != C.TypeSOCKS {
		t.Fatalf("inbound 类型错误: got=%s", options.Inbounds[0].Type)
	}
	socksInbound, ok := options.Inbounds[0].Options.(*option.SocksInboundOptions)
	if !ok {
		t.Fatalf("inbound options 类型错误: %T", options.Inbounds[0].Options)
	}
	if socksInbound.Listen == nil {
		t.Fatal("inbound 监听地址为空")
	}
	listenAddr := (*socksInbound.Listen).Build(netip.Addr{})
	if listenAddr.String() != defaultListenAddr {
		t.Fatalf("inbound 监听地址错误: got=%s want=%s", listenAddr.String(), defaultListenAddr)
	}

	if len(options.Outbounds) != len(nodes)+2 {
		t.Fatalf("outbounds 数量错误: got=%d want=%d", len(options.Outbounds), len(nodes)+2)
	}

	lb := options.Outbounds[len(options.Outbounds)-1]
	if lb.Type != geoloomRandomOutboundType {
		t.Fatalf("weighted-random lb 类型错误: got=%s want=%s", lb.Type, geoloomRandomOutboundType)
	}
	randomOptions, ok := lb.Options.(*geoloomRandomOutboundOptions)
	if !ok {
		t.Fatalf("weighted-random options 类型断言失败: %T", lb.Options)
	}
	if len(randomOptions.Outbounds) != len(nodes) {
		t.Fatalf("weighted-random outbounds 数量错误: got=%d want=%d", len(randomOptions.Outbounds), len(nodes))
	}
	if len(randomOptions.Weights) != len(nodes) {
		t.Fatalf("weighted-random weights 数量错误: got=%d want=%d", len(randomOptions.Weights), len(nodes))
	}
	for _, tag := range randomOptions.Outbounds {
		if tag == directOutboundTag {
			t.Fatalf("weighted-random outbounds 不应包含 direct: %+v", randomOptions.Outbounds)
		}
	}

	if options.Route == nil || options.Route.Final != defaultLBTag {
		t.Fatalf("route.final 错误: %+v", options.Route)
	}
}

func TestOptionsBuilderBuildURLTestStrategy(t *testing.T) {
	t.Parallel()

	builder := NewOptionsBuilder()
	cfg := config.Config{
		Gateway: config.GatewayConfig{SocksPort: 1080},
		Policy: config.PolicyConfig{
			Strategy: config.StrategyURLTest,
			HealthCheck: config.HealthCheckConfig{
				Interval: "30s",
				URL:      "http://cp.cloudflare.com",
			},
		},
	}
	nodes := []domain.NodeMetadata{
		buildSocksNode("n1", "1.1.1.1", 1080),
		buildSocksNode("n2", "2.2.2.2", 1081),
	}

	options, err := builder.Build(cfg, nodes)
	if err != nil {
		t.Fatalf("Build 返回错误: %v", err)
	}
	lb := options.Outbounds[len(options.Outbounds)-1]
	if lb.Type != C.TypeURLTest {
		t.Fatalf("lb 类型错误: got=%s want=%s", lb.Type, C.TypeURLTest)
	}
	urltest, ok := lb.Options.(*option.URLTestOutboundOptions)
	if !ok {
		t.Fatalf("urltest 类型断言失败: %T", lb.Options)
	}
	if urltest.URL != "http://cp.cloudflare.com" {
		t.Fatalf("urltest URL 错误: got=%s", urltest.URL)
	}
	if len(urltest.Outbounds) != 3 || urltest.Outbounds[2] != "direct" {
		t.Fatalf("urltest outbounds 错误: %+v", urltest.Outbounds)
	}
}

func TestOptionsBuilderBuildRandomStrategyUsesGeoloomRandom(t *testing.T) {
	t.Parallel()

	builder := NewOptionsBuilder()
	cfg := config.Config{Gateway: config.GatewayConfig{SocksPort: 1080}}
	nodes := []domain.NodeMetadata{
		func() domain.NodeMetadata { n := buildSocksNode("a", "1.1.1.1", 1080); n.HealthScore = 95; return n }(),
		func() domain.NodeMetadata { n := buildSocksNode("b", "2.2.2.2", 1081); n.HealthScore = 60; return n }(),
		func() domain.NodeMetadata { n := buildSocksNode("c", "3.3.3.3", 1082); n.HealthScore = 10; return n }(),
	}

	options, err := builder.Build(cfg, nodes)
	if err != nil {
		t.Fatalf("Build 返回错误: %v", err)
	}
	lb := options.Outbounds[len(options.Outbounds)-1]
	if lb.Type != geoloomRandomOutboundType {
		t.Fatalf("lb 类型错误: got=%s want=%s", lb.Type, geoloomRandomOutboundType)
	}
	randomOptions, ok := lb.Options.(*geoloomRandomOutboundOptions)
	if !ok {
		t.Fatalf("random options 类型错误: %T", lb.Options)
	}
	if len(randomOptions.Weights) != 3 {
		t.Fatalf("weights 数量错误: %+v", randomOptions.Weights)
	}
	if randomOptions.Weights[0] <= randomOptions.Weights[1] || randomOptions.Weights[1] <= randomOptions.Weights[2] {
		t.Fatalf("weights 应按 score 递减映射: %+v", randomOptions.Weights)
	}
}

func TestOptionsBuilderBuildHybridStrategyUsesConfiguredTopK(t *testing.T) {
	t.Parallel()

	builder := NewOptionsBuilder()
	cfg := config.Config{
		Gateway: config.GatewayConfig{SocksPort: 1080},
		Policy:  config.PolicyConfig{Strategy: config.StrategyHybrid, HybridTopK: 2},
	}
	nodes := []domain.NodeMetadata{
		func() domain.NodeMetadata { n := buildSocksNode("a", "1.1.1.1", 1080); n.HealthScore = 98; return n }(),
		func() domain.NodeMetadata { n := buildSocksNode("b", "2.2.2.2", 1081); n.HealthScore = 90; return n }(),
		func() domain.NodeMetadata { n := buildSocksNode("c", "3.3.3.3", 1082); n.HealthScore = 70; return n }(),
		func() domain.NodeMetadata { n := buildSocksNode("d", "4.4.4.4", 1083); n.HealthScore = 10; return n }(),
	}

	options, err := builder.Build(cfg, nodes)
	if err != nil {
		t.Fatalf("Build 返回错误: %v", err)
	}
	lb := options.Outbounds[len(options.Outbounds)-1]
	if lb.Type != geoloomRandomOutboundType {
		t.Fatalf("hybrid lb 类型错误: got=%s want=%s", lb.Type, geoloomRandomOutboundType)
	}
	randomOptions, ok := lb.Options.(*geoloomRandomOutboundOptions)
	if !ok {
		t.Fatalf("hybrid options 类型错误: %T", lb.Options)
	}
	if len(randomOptions.Outbounds) != 2 {
		t.Fatalf("hybrid 应只保留 Top-2 基准子集: %+v", randomOptions.Outbounds)
	}
	if len(randomOptions.Weights) != len(randomOptions.Outbounds) {
		t.Fatalf("hybrid weights 数量错误: outbounds=%d weights=%d", len(randomOptions.Outbounds), len(randomOptions.Weights))
	}
	if randomOptions.Weights[0] < randomOptions.Weights[1] {
		t.Fatalf("hybrid 权重应与候选顺序一致: %+v", randomOptions.Weights)
	}
	for _, tag := range randomOptions.Outbounds {
		if tag == options.Outbounds[2].Tag || tag == options.Outbounds[3].Tag {
			t.Fatalf("hybrid 不应包含 Top-K 之外的低质量节点: %+v", randomOptions.Outbounds)
		}
	}
}

func TestOptionsBuilderBuildHybridStrategyShouldIncludeTiedScoreAtConfiguredCutoff(t *testing.T) {
	t.Parallel()

	builder := NewOptionsBuilder()
	cfg := config.Config{
		Gateway: config.GatewayConfig{SocksPort: 1080},
		Policy:  config.PolicyConfig{Strategy: config.StrategyHybrid, HybridTopK: 2},
	}
	nodes := []domain.NodeMetadata{
		func() domain.NodeMetadata { n := buildSocksNode("a", "1.1.1.1", 1080); n.HealthScore = 95; return n }(),
		func() domain.NodeMetadata { n := buildSocksNode("b", "2.2.2.2", 1081); n.HealthScore = 80; return n }(),
		func() domain.NodeMetadata { n := buildSocksNode("c", "3.3.3.3", 1082); n.HealthScore = 80; return n }(),
		func() domain.NodeMetadata { n := buildSocksNode("d", "4.4.4.4", 1083); n.HealthScore = 10; return n }(),
	}

	options, err := builder.Build(cfg, nodes)
	if err != nil {
		t.Fatalf("Build 返回错误: %v", err)
	}
	randomOptions := options.Outbounds[len(options.Outbounds)-1].Options.(*geoloomRandomOutboundOptions)
	if len(randomOptions.Outbounds) != 3 {
		t.Fatalf("并列 cutoff 分数应一并纳入: %+v", randomOptions.Outbounds)
	}
}

func TestOptionsBuilderBuildHybridStrategyShouldFallbackWhenTopKExceedsCandidates(t *testing.T) {
	t.Parallel()

	builder := NewOptionsBuilder()
	cfg := config.Config{
		Gateway: config.GatewayConfig{SocksPort: 1080},
		Policy:  config.PolicyConfig{Strategy: config.StrategyHybrid, HybridTopK: 5},
	}
	nodes := []domain.NodeMetadata{
		func() domain.NodeMetadata { n := buildSocksNode("a", "1.1.1.1", 1080); n.HealthScore = 30; return n }(),
		func() domain.NodeMetadata { n := buildSocksNode("b", "2.2.2.2", 1081); n.HealthScore = 20; return n }(),
	}

	options, err := builder.Build(cfg, nodes)
	if err != nil {
		t.Fatalf("Build 返回错误: %v", err)
	}
	randomOptions := options.Outbounds[len(options.Outbounds)-1].Options.(*geoloomRandomOutboundOptions)
	if len(randomOptions.Outbounds) != 2 || len(randomOptions.Weights) != 2 {
		t.Fatalf("Top-K 大于候选数时应退化为全量子集: outbounds=%+v weights=%+v", randomOptions.Outbounds, randomOptions.Weights)
	}
}

func TestOptionsBuilderBuildHybridStrategyShouldFallbackToDefaultWhenTopKInvalid(t *testing.T) {
	t.Parallel()

	builder := NewOptionsBuilder()
	cfg := config.Config{
		Gateway: config.GatewayConfig{SocksPort: 1080},
		Policy:  config.PolicyConfig{Strategy: config.StrategyHybrid, HybridTopK: 0},
	}
	nodes := []domain.NodeMetadata{
		func() domain.NodeMetadata { n := buildSocksNode("a", "1.1.1.1", 1080); n.HealthScore = 95; return n }(),
		func() domain.NodeMetadata { n := buildSocksNode("b", "2.2.2.2", 1081); n.HealthScore = 80; return n }(),
		func() domain.NodeMetadata { n := buildSocksNode("c", "3.3.3.3", 1082); n.HealthScore = 60; return n }(),
		func() domain.NodeMetadata { n := buildSocksNode("d", "4.4.4.4", 1083); n.HealthScore = 10; return n }(),
	}

	options, err := builder.Build(cfg, nodes)
	if err != nil {
		t.Fatalf("Build 返回错误: %v", err)
	}
	randomOptions := options.Outbounds[len(options.Outbounds)-1].Options.(*geoloomRandomOutboundOptions)
	if len(randomOptions.Outbounds) != 3 {
		t.Fatalf("非法 Top-K 应回退默认值 3: %+v", randomOptions.Outbounds)
	}
}

func TestOptionsBuilderBuildHybridStrategyShouldFallbackWhenCandidatesFew(t *testing.T) {
	t.Parallel()

	builder := NewOptionsBuilder()
	cfg := config.Config{Gateway: config.GatewayConfig{SocksPort: 1080}, Policy: config.PolicyConfig{Strategy: config.StrategyHybrid}}
	nodes := []domain.NodeMetadata{
		func() domain.NodeMetadata { n := buildSocksNode("a", "1.1.1.1", 1080); n.HealthScore = 0; return n }(),
		func() domain.NodeMetadata { n := buildSocksNode("b", "2.2.2.2", 1081); n.HealthScore = 0; return n }(),
	}

	options, err := builder.Build(cfg, nodes)
	if err != nil {
		t.Fatalf("Build 返回错误: %v", err)
	}
	randomOptions := options.Outbounds[len(options.Outbounds)-1].Options.(*geoloomRandomOutboundOptions)
	if len(randomOptions.Outbounds) != 2 || len(randomOptions.Weights) != 2 {
		t.Fatalf("少量候选时应自然退化为全量子集: outbounds=%+v weights=%+v", randomOptions.Outbounds, randomOptions.Weights)
	}
	if randomOptions.Weights[0] != 1 || randomOptions.Weights[1] != 1 {
		t.Fatalf("冷启动低分时应退化为等权: %+v", randomOptions.Weights)
	}
}

func TestOptionsBuilderBuildInvalidPort(t *testing.T) {
	t.Parallel()

	builder := NewOptionsBuilder()
	cfg := config.Config{Gateway: config.GatewayConfig{SocksPort: 0}}
	_, err := builder.Build(cfg, []domain.NodeMetadata{buildSocksNode("n1", "1.1.1.1", 1080)})
	if err == nil {
		t.Fatal("预期 Build 返回错误，但得到 nil")
	}
}

func TestOptionsBuilderBuildUnsupportedProtocol(t *testing.T) {
	t.Parallel()

	builder := NewOptionsBuilder()
	cfg := config.Config{Gateway: config.GatewayConfig{SocksPort: 1080}}
	_, err := builder.Build(cfg, []domain.NodeMetadata{{
		ID:       "n1",
		Protocol: "unknown",
		Address:  "1.1.1.1",
		Port:     443,
		RawConfig: map[string]any{
			"type":        "unknown",
			"server":      "1.1.1.1",
			"server_port": 443,
		},
	}})
	if err == nil {
		t.Fatal("预期 Build 返回错误，但得到 nil")
	}
}

func TestOptionsBuilderBuildUnsupportedNetworkSkip(t *testing.T) {
	t.Parallel()

	builder := NewOptionsBuilder()
	cfg := config.Config{Gateway: config.GatewayConfig{SocksPort: 1080}}
	nodes := []domain.NodeMetadata{
		buildTrojanNode("unsupported", "grpc", true),
		buildSocksNode("supported", "2.2.2.2", 1080),
	}

	options, err := builder.Build(cfg, nodes)
	if err != nil {
		t.Fatalf("Build 返回错误: %v", err)
	}
	if options == nil {
		t.Fatal("Build 返回 nil options")
	}

	stats := builder.LastBuildStats()
	if stats.SupportedCandidates != 1 {
		t.Fatalf("supported candidates 错误: got=%d want=1", stats.SupportedCandidates)
	}
	if len(stats.Unsupported) != 1 {
		t.Fatalf("unsupported 数量错误: got=%d want=1", len(stats.Unsupported))
	}

	if len(options.Outbounds) != 3 {
		t.Fatalf("outbounds 数量错误: got=%d want=3", len(options.Outbounds))
	}
}

func TestOptionsBuilderBuildAllUnsupportedShouldFail(t *testing.T) {
	t.Parallel()

	builder := NewOptionsBuilder()
	cfg := config.Config{Gateway: config.GatewayConfig{SocksPort: 1080}}
	_, err := builder.Build(cfg, []domain.NodeMetadata{
		buildTrojanNode("unsupported-1", "grpc", true),
		buildTrojanNode("unsupported-2", "h2", true),
	})
	if err == nil {
		t.Fatal("预期 Build 返回错误，但得到 nil")
	}

	stats := builder.LastBuildStats()
	if stats.SupportedCandidates != 0 {
		t.Fatalf("supported candidates 错误: got=%d want=0", stats.SupportedCandidates)
	}
	if len(stats.Unsupported) != 2 {
		t.Fatalf("unsupported 数量错误: got=%d want=2", len(stats.Unsupported))
	}
}

func TestOptionsBuilderBuildHardErrorStillFailFast(t *testing.T) {
	t.Parallel()

	builder := NewOptionsBuilder()
	cfg := config.Config{Gateway: config.GatewayConfig{SocksPort: 1080}}
	_, err := builder.Build(cfg, []domain.NodeMetadata{
		buildTrojanNode("missing-password", "tcp", false),
		buildSocksNode("supported", "2.2.2.2", 1080),
	})
	if err == nil {
		t.Fatal("预期 Build 返回错误，但得到 nil")
	}
}

func TestOptionsBuilderBuildDuplicateTags(t *testing.T) {
	t.Parallel()

	builder := NewOptionsBuilder()
	cfg := config.Config{Gateway: config.GatewayConfig{SocksPort: 1080}}
	nodes := []domain.NodeMetadata{
		buildSocksNode("same", "1.1.1.1", 1080),
		buildSocksNode("same", "2.2.2.2", 1081),
	}

	options, err := builder.Build(cfg, nodes)
	if err != nil {
		t.Fatalf("Build 返回错误: %v", err)
	}

	if options.Outbounds[0].Tag == options.Outbounds[1].Tag {
		t.Fatalf("重复节点 tag 未去重: %s", options.Outbounds[0].Tag)
	}
}

func TestOptionsBuilderBuildNewProtocols(t *testing.T) {
	t.Parallel()

	builder := NewOptionsBuilder()
	cfg := config.Config{Gateway: config.GatewayConfig{SocksPort: 1080}}
	nodes := []domain.NodeMetadata{
		{
			ID:       "trojan-node",
			Protocol: "trojan",
			Address:  "trojan.example.com",
			Port:     443,
			RawConfig: map[string]any{
				"type":        "trojan",
				"server":      "trojan.example.com",
				"server_port": 443,
				"password":    "secret",
				"security":    "tls",
				"sni":         "trojan.example.com",
			},
		},
		{
			ID:       "vmess-node",
			Protocol: "vmess",
			Address:  "vmess.example.com",
			Port:     443,
			RawConfig: map[string]any{
				"type":        "vmess",
				"server":      "vmess.example.com",
				"server_port": 443,
				"uuid":        "11111111-1111-1111-1111-111111111111",
				"cipher":      "auto",
			},
		},
		{
			ID:       "ss-node",
			Protocol: "shadowsocks",
			Address:  "ss.example.com",
			Port:     8388,
			RawConfig: map[string]any{
				"type":        "shadowsocks",
				"server":      "ss.example.com",
				"server_port": 8388,
				"method":      "aes-128-gcm",
				"password":    "secret",
			},
		},
	}

	options, err := builder.Build(cfg, nodes)
	if err != nil {
		t.Fatalf("Build 返回错误: %v", err)
	}

	gotTypes := map[string]string{}
	for _, outbound := range options.Outbounds[:3] {
		gotTypes[outbound.Tag] = outbound.Type
	}
	if !containsType(gotTypes, C.TypeTrojan) || !containsType(gotTypes, C.TypeVMess) || !containsType(gotTypes, C.TypeShadowsocks) {
		t.Fatalf("协议映射不完整: %+v", gotTypes)
	}
}

func TestOptionsBuilderBuildSocks4AndHTTP(t *testing.T) {
	t.Parallel()

	builder := NewOptionsBuilder()
	cfg := config.Config{Gateway: config.GatewayConfig{SocksPort: 1080}}
	nodes := []domain.NodeMetadata{
		{
			ID:       "socks4-node",
			Protocol: "socks4",
			Address:  "1.1.1.1",
			Port:     1080,
			RawConfig: map[string]any{
				"type":        "socks4",
				"server":      "1.1.1.1",
				"server_port": 1080,
				"username":    "legacy",
			},
		},
		{
			ID:       "http-node",
			Protocol: "http",
			Address:  "2.2.2.2",
			Port:     8080,
			RawConfig: map[string]any{
				"type":        "http",
				"server":      "2.2.2.2",
				"server_port": 8080,
				"username":    "user",
				"password":    "pass",
			},
		},
	}

	options, err := builder.Build(cfg, nodes)
	if err != nil {
		t.Fatalf("Build 返回错误: %v", err)
	}

	socksOutbound, ok := options.Outbounds[0].Options.(*option.SOCKSOutboundOptions)
	if !ok {
		t.Fatalf("socks4 outbound 类型错误: %T", options.Outbounds[0].Options)
	}
	if socksOutbound.Version != "4" {
		t.Fatalf("socks4 version 错误: got=%q", socksOutbound.Version)
	}

	httpOutbound, ok := options.Outbounds[1].Options.(*option.HTTPOutboundOptions)
	if !ok {
		t.Fatalf("http outbound 类型错误: %T", options.Outbounds[1].Options)
	}
	if httpOutbound.Username != "user" || httpOutbound.Password != "pass" {
		t.Fatalf("http 鉴权字段错误: %+v", httpOutbound)
	}

	stats := builder.LastBuildStats()
	if stats.SupportedCandidates != 2 {
		t.Fatalf("supported candidates 错误: got=%d want=2", stats.SupportedCandidates)
	}
}

func TestEnsureRegistryContextRegistersGeoloomRandomOptions(t *testing.T) {
	t.Parallel()

	ctx := ensureRegistryContext(nil)
	registry := service.FromContext[option.OutboundOptionsRegistry](ctx)
	if registry == nil {
		t.Fatal("缺少 OutboundOptionsRegistry")
	}
	created, ok := registry.CreateOptions(geoloomRandomOutboundType)
	if !ok {
		t.Fatalf("未注册 geoloom weighted-random 类型: %s", geoloomRandomOutboundType)
	}
	if _, typeOK := created.(*geoloomRandomOutboundOptions); !typeOK {
		t.Fatalf("geoloom weighted-random options 类型错误: %T", created)
	}
}

func buildSocksNode(id, addr string, port int) domain.NodeMetadata {
	return domain.NodeMetadata{
		ID:       id,
		Protocol: "socks5",
		Address:  addr,
		Port:     port,
		RawConfig: map[string]any{
			"type":        "socks5",
			"server":      addr,
			"server_port": port,
		},
	}
}

func buildTrojanNode(id, network string, withPassword bool) domain.NodeMetadata {
	raw := map[string]any{
		"type":        "trojan",
		"server":      "trojan.example.com",
		"server_port": 443,
		"network":     network,
	}
	if withPassword {
		raw["password"] = "secret"
	}

	return domain.NodeMetadata{
		ID:        id,
		Protocol:  "trojan",
		Address:   "trojan.example.com",
		Port:      443,
		RawConfig: raw,
	}
}

func containsType(values map[string]string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
