package singbox

import (
	"context"
	"errors"
	"testing"

	"geoloom/internal/config"
	"geoloom/internal/domain"

	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/service"
)

type fakeLifecycleBox struct {
	startErr   error
	closeErr   error
	startCalls int
	closeCalls int
}

func (f *fakeLifecycleBox) Start() error {
	f.startCalls++
	return f.startErr
}

func (f *fakeLifecycleBox) Close() error {
	f.closeCalls++
	return f.closeErr
}

func buildServiceTestNodes() []domain.NodeMetadata {
	return []domain.NodeMetadata{
		{
			ID:       "n1",
			Protocol: "socks5",
			Address:  "1.1.1.1",
			Port:     1080,
			RawConfig: map[string]any{
				"type":        "socks5",
				"server":      "1.1.1.1",
				"server_port": 1080,
			},
		},
	}
}

func TestServiceStartAndClose(t *testing.T) {
	t.Parallel()

	service := NewService(context.Background(), NewOptionsBuilder())
	boxInstance := &fakeLifecycleBox{}
	service.newBox = func(_ context.Context, _ *option.Options) (lifecycleBox, error) {
		return boxInstance, nil
	}

	cfg := config.Config{Gateway: config.GatewayConfig{SocksPort: 1080}}
	if err := service.Start(cfg, buildServiceTestNodes()); err != nil {
		t.Fatalf("Start 返回错误: %v", err)
	}
	if boxInstance.startCalls != 1 {
		t.Fatalf("Start 调用次数错误: got=%d", boxInstance.startCalls)
	}

	if err := service.Close(); err != nil {
		t.Fatalf("Close 返回错误: %v", err)
	}
	if boxInstance.closeCalls != 1 {
		t.Fatalf("Close 调用次数错误: got=%d", boxInstance.closeCalls)
	}
}

func TestServiceStartTwice(t *testing.T) {
	t.Parallel()

	service := NewService(context.Background(), NewOptionsBuilder())
	service.newBox = func(_ context.Context, _ *option.Options) (lifecycleBox, error) {
		return &fakeLifecycleBox{}, nil
	}

	cfg := config.Config{Gateway: config.GatewayConfig{SocksPort: 1080}}
	if err := service.Start(cfg, buildServiceTestNodes()); err != nil {
		t.Fatalf("首次 Start 返回错误: %v", err)
	}
	if err := service.Start(cfg, buildServiceTestNodes()); err == nil {
		t.Fatal("预期二次 Start 返回错误，但得到 nil")
	}
}

func TestServiceRebuild(t *testing.T) {
	t.Parallel()

	service := NewService(context.Background(), NewOptionsBuilder())
	first := &fakeLifecycleBox{}
	second := &fakeLifecycleBox{}
	call := 0
	service.newBox = func(_ context.Context, _ *option.Options) (lifecycleBox, error) {
		call++
		if call == 1 {
			return first, nil
		}
		return second, nil
	}

	cfg := config.Config{Gateway: config.GatewayConfig{SocksPort: 1080}}
	if err := service.Start(cfg, buildServiceTestNodes()); err != nil {
		t.Fatalf("Start 返回错误: %v", err)
	}
	if err := service.Rebuild(cfg, buildServiceTestNodes()); err != nil {
		t.Fatalf("Rebuild 返回错误: %v", err)
	}

	if first.closeCalls != 1 {
		t.Fatalf("旧实例 Close 调用次数错误: got=%d", first.closeCalls)
	}
	if second.startCalls != 1 {
		t.Fatalf("新实例 Start 调用次数错误: got=%d", second.startCalls)
	}
}

func TestServiceStartFailedShouldClose(t *testing.T) {
	t.Parallel()

	service := NewService(context.Background(), NewOptionsBuilder())
	boxInstance := &fakeLifecycleBox{startErr: errors.New("start failed")}
	service.newBox = func(_ context.Context, _ *option.Options) (lifecycleBox, error) {
		return boxInstance, nil
	}

	cfg := config.Config{Gateway: config.GatewayConfig{SocksPort: 1080}}
	if err := service.Start(cfg, buildServiceTestNodes()); err == nil {
		t.Fatal("预期 Start 返回错误，但得到 nil")
	}
	if boxInstance.closeCalls != 1 {
		t.Fatalf("启动失败后应关闭实例: got=%d", boxInstance.closeCalls)
	}
}

func TestServiceStartWithUnsupportedSubset(t *testing.T) {
	t.Parallel()

	service := NewService(context.Background(), NewOptionsBuilder())
	boxInstance := &fakeLifecycleBox{}
	service.newBox = func(_ context.Context, _ *option.Options) (lifecycleBox, error) {
		return boxInstance, nil
	}

	nodes := []domain.NodeMetadata{
		{
			ID:       "unsupported-trojan",
			Protocol: "trojan",
			Address:  "1.1.1.1",
			Port:     443,
			RawConfig: map[string]any{
				"type":        "trojan",
				"server":      "1.1.1.1",
				"server_port": 443,
				"password":    "secret",
				"network":     "grpc",
			},
		},
		buildServiceTestNodes()[0],
	}

	cfg := config.Config{Gateway: config.GatewayConfig{SocksPort: 1080}}
	if err := service.Start(cfg, nodes); err != nil {
		t.Fatalf("Start 返回错误: %v", err)
	}

	stats := service.LastBuildStats()
	if stats.SupportedCandidates != 1 {
		t.Fatalf("supported candidates 错误: got=%d want=1", stats.SupportedCandidates)
	}
	if len(stats.Unsupported) != 1 {
		t.Fatalf("unsupported 数量错误: got=%d want=1", len(stats.Unsupported))
	}
}

func TestServiceEnsureRegistryContext(t *testing.T) {
	t.Parallel()

	ctx := ensureRegistryContext(context.Background())
	if ctx == nil {
		t.Fatal("ensureRegistryContext 返回 nil")
	}
	if !hasRequiredRegistries(ctx) {
		t.Fatal("ensureRegistryContext 未成功注入 registry")
	}
	if registry := service.FromContext[option.OutboundOptionsRegistry](ctx); registry == nil {
		t.Fatal("缺少 OutboundOptionsRegistry")
	} else if created, ok := registry.CreateOptions(geoloomRandomOutboundType); !ok {
		t.Fatalf("未注册 geoloom random type: %s", geoloomRandomOutboundType)
	} else if _, typeOK := created.(*geoloomRandomOutboundOptions); !typeOK {
		t.Fatalf("geoloom random options 类型错误: %T", created)
	}

	service := NewService(ctx, NewOptionsBuilder())
	service.newBox = func(ctx context.Context, opts *option.Options) (lifecycleBox, error) {
		if !hasRequiredRegistries(ctx) {
			t.Fatal("期望上下文已包含 registry")
		}
		if opts == nil {
			t.Fatal("options 不应为 nil")
		}
		return &fakeLifecycleBox{}, nil
	}

	cfg := config.Config{Gateway: config.GatewayConfig{SocksPort: 1080}}
	if err := service.Start(cfg, buildServiceTestNodes()); err != nil {
		t.Fatalf("Start 返回错误: %v", err)
	}
}
