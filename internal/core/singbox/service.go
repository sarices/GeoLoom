package singbox

import (
	"context"
	"fmt"
	"sync"

	"geoloom/internal/config"
	"geoloom/internal/domain"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/endpoint"
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/adapter/outbound"
	boxService "github.com/sagernet/sing-box/adapter/service"
	"github.com/sagernet/sing-box/common/urltest"
	"github.com/sagernet/sing-box/dns"
	protocolLocalDNS "github.com/sagernet/sing-box/dns/transport/local"
	"github.com/sagernet/sing-box/option"
	protocolDirect "github.com/sagernet/sing-box/protocol/direct"
	protocolGroup "github.com/sagernet/sing-box/protocol/group"
	protocolHysteria2 "github.com/sagernet/sing-box/protocol/hysteria2"
	protocolShadowsocks "github.com/sagernet/sing-box/protocol/shadowsocks"
	protocolSocks "github.com/sagernet/sing-box/protocol/socks"
	protocolTrojan "github.com/sagernet/sing-box/protocol/trojan"
	protocolVless "github.com/sagernet/sing-box/protocol/vless"
	protocolVmess "github.com/sagernet/sing-box/protocol/vmess"
	"github.com/sagernet/sing/service"
)

type lifecycleBox interface {
	Start() error
	Close() error
}

type boxFactory func(ctx context.Context, options *option.Options) (lifecycleBox, error)

// Service 封装 sing-box 实例生命周期。
type Service struct {
	mu      sync.Mutex
	ctx     context.Context
	builder *OptionsBuilder
	newBox  boxFactory
	current lifecycleBox

	lastBuildStats CoreBuildStats
}

func NewService(ctx context.Context, builder *OptionsBuilder) *Service {
	if ctx == nil {
		ctx = context.Background()
	}
	if builder == nil {
		builder = NewOptionsBuilder()
	}

	return &Service{
		ctx:     ctx,
		builder: builder,
		newBox:  defaultBoxFactory,
	}
}

// Start 首次启动 sing-box 实例。
func (s *Service) Start(cfg config.Config, nodes []domain.NodeMetadata) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.current != nil {
		return fmt.Errorf("sing-box 服务已启动")
	}
	return s.startLocked(cfg, nodes)
}

// Rebuild 关闭旧实例并基于最新配置重建实例。
func (s *Service) Rebuild(cfg config.Config, nodes []domain.NodeMetadata) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.closeLocked(); err != nil {
		return fmt.Errorf("关闭旧 sing-box 实例失败: %w", err)
	}
	return s.startLocked(cfg, nodes)
}

// Close 关闭当前 sing-box 实例。
func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.closeLocked(); err != nil {
		return fmt.Errorf("关闭 sing-box 实例失败: %w", err)
	}
	return nil
}

func (s *Service) LastBuildStats() CoreBuildStats {
	s.mu.Lock()
	defer s.mu.Unlock()

	return CoreBuildStats{
		Unsupported:         append([]string(nil), s.lastBuildStats.Unsupported...),
		SupportedCandidates: s.lastBuildStats.SupportedCandidates,
	}
}

func (s *Service) startLocked(cfg config.Config, nodes []domain.NodeMetadata) error {
	options, err := s.builder.Build(cfg, nodes)
	if err != nil {
		s.lastBuildStats = s.builder.LastBuildStats()
		return fmt.Errorf("构建 sing-box options 失败: %w", err)
	}
	s.lastBuildStats = s.builder.LastBuildStats()

	instance, err := s.newBox(s.ctx, options)
	if err != nil {
		return fmt.Errorf("创建 sing-box 实例失败: %w", err)
	}

	if err := instance.Start(); err != nil {
		_ = instance.Close()
		return fmt.Errorf("启动 sing-box 实例失败: %w", err)
	}

	s.current = instance
	return nil
}

func (s *Service) closeLocked() error {
	if s.current == nil {
		return nil
	}

	current := s.current
	s.current = nil
	return current.Close()
}

func defaultBoxFactory(ctx context.Context, options *option.Options) (lifecycleBox, error) {
	instance, err := box.New(box.Options{
		Options: *options,
		Context: ensureRegistryContext(ctx),
	})
	if err != nil {
		return nil, err
	}
	return instance, nil
}

func ensureRegistryContext(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if hasRequiredRegistries(ctx) {
		return ctx
	}

	inboundRegistry := inbound.NewRegistry()
	outboundRegistry := outbound.NewRegistry()
	endpointRegistry := endpoint.NewRegistry()
	dnsTransportRegistry := dns.NewTransportRegistry()
	serviceRegistry := boxService.NewRegistry()

	protocolSocks.RegisterInbound(inboundRegistry)
	protocolSocks.RegisterOutbound(outboundRegistry)
	protocolDirect.RegisterInbound(inboundRegistry)
	protocolDirect.RegisterOutbound(outboundRegistry)
	protocolGroup.RegisterSelector(outboundRegistry)
	protocolGroup.RegisterURLTest(outboundRegistry)
	registerGeoloomRandom(outboundRegistry)
	protocolVless.RegisterOutbound(outboundRegistry)
	protocolHysteria2.RegisterOutbound(outboundRegistry)
	protocolShadowsocks.RegisterOutbound(outboundRegistry)
	protocolTrojan.RegisterOutbound(outboundRegistry)
	protocolVmess.RegisterOutbound(outboundRegistry)
	protocolLocalDNS.RegisterTransport(dnsTransportRegistry)

	historyStorage := urltest.NewHistoryStorage()
	ctx = service.ContextWithPtr(ctx, &historyStorage)

	return box.Context(ctx, inboundRegistry, outboundRegistry, endpointRegistry, dnsTransportRegistry, serviceRegistry)
}

func hasRequiredRegistries(ctx context.Context) bool {
	if ctx == nil {
		return false
	}

	return service.FromContext[adapter.InboundRegistry](ctx) != nil &&
		service.FromContext[adapter.OutboundRegistry](ctx) != nil &&
		service.FromContext[adapter.EndpointRegistry](ctx) != nil &&
		service.FromContext[adapter.DNSTransportRegistry](ctx) != nil &&
		service.FromContext[adapter.ServiceRegistry](ctx) != nil &&
		service.FromContext[option.InboundOptionsRegistry](ctx) != nil &&
		service.FromContext[option.OutboundOptionsRegistry](ctx) != nil &&
		service.FromContext[option.EndpointOptionsRegistry](ctx) != nil &&
		service.FromContext[option.DNSTransportOptionsRegistry](ctx) != nil &&
		service.FromContext[option.ServiceOptionsRegistry](ctx) != nil
}
