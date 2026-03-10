package singbox

import (
	"context"
	"net"
	"sync"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	boxOutbound "github.com/sagernet/sing-box/adapter/outbound"
	boxLog "github.com/sagernet/sing-box/log"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
)

type stubRandomIntn struct {
	mu     sync.Mutex
	values []int
	idx    int
}

func (s *stubRandomIntn) Intn(n int) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n <= 0 {
		return 0
	}
	if len(s.values) == 0 {
		return 0
	}
	if s.idx >= len(s.values) {
		return s.values[len(s.values)-1] % n
	}
	value := s.values[s.idx] % n
	s.idx++
	if value < 0 {
		return 0
	}
	return value
}

type stubOutbound struct {
	adapterType   string
	tag           string
	network       []string
	dialCount     int
	packetCount   int
	lastDialNet   string
	lastDialDst   M.Socksaddr
	lastPacketDst M.Socksaddr
}

func (s *stubOutbound) Type() string           { return s.adapterType }
func (s *stubOutbound) Tag() string            { return s.tag }
func (s *stubOutbound) Network() []string      { return s.network }
func (s *stubOutbound) Dependencies() []string { return nil }
func (s *stubOutbound) DialContext(_ context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	s.dialCount++
	s.lastDialNet = network
	s.lastDialDst = destination
	return &net.TCPConn{}, nil
}
func (s *stubOutbound) ListenPacket(_ context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	s.packetCount++
	s.lastPacketDst = destination
	return &net.UDPConn{}, nil
}

func TestGeoloomRandomOutboundDialRandomlyAcrossCandidates(t *testing.T) {
	t.Parallel()

	outA := &stubOutbound{adapterType: "socks", tag: "a", network: []string{N.NetworkTCP, N.NetworkUDP}}
	outB := &stubOutbound{adapterType: "socks", tag: "b", network: []string{N.NetworkTCP, N.NetworkUDP}}

	random := &geoloomRandomOutbound{
		Adapter: boxOutbound.NewAdapter(geoloomRandomOutboundType, defaultLBTag, []string{N.NetworkTCP, N.NetworkUDP}, []string{"a", "b"}),
		lookupOutbound: func(tag string) (adapter.Outbound, bool) {
			switch tag {
			case "a":
				return outA, true
			case "b":
				return outB, true
			default:
				return nil, false
			}
		},
		tags:    []string{"a", "b"},
		weights: []int{3, 1},
		random:  &stubRandomIntn{values: []int{0, 3, 0, 3}},
	}

	dst := M.ParseSocksaddrHostPort("example.com", 443)
	if _, err := random.DialContext(context.Background(), N.NetworkTCP, dst); err != nil {
		t.Fatalf("首次 DialContext 返回错误: %v", err)
	}
	if _, err := random.DialContext(context.Background(), N.NetworkTCP, dst); err != nil {
		t.Fatalf("第二次 DialContext 返回错误: %v", err)
	}

	if outA.dialCount == 0 || outB.dialCount == 0 {
		t.Fatalf("期望随机命中不同候选: a=%d b=%d", outA.dialCount, outB.dialCount)
	}
}

func TestGeoloomRandomOutboundSingleCandidate(t *testing.T) {
	t.Parallel()

	outA := &stubOutbound{adapterType: "socks", tag: "a", network: []string{N.NetworkTCP, N.NetworkUDP}}
	random := &geoloomRandomOutbound{
		Adapter: boxOutbound.NewAdapter(geoloomRandomOutboundType, defaultLBTag, []string{N.NetworkTCP, N.NetworkUDP}, []string{"a"}),
		lookupOutbound: func(tag string) (adapter.Outbound, bool) {
			if tag == "a" {
				return outA, true
			}
			return nil, false
		},
		tags:   []string{"a"},
		random: &stubRandomIntn{values: []int{0, 0}},
	}

	dst := M.ParseSocksaddrHostPort("example.com", 53)
	if _, err := random.ListenPacket(context.Background(), dst); err != nil {
		t.Fatalf("ListenPacket 返回错误: %v", err)
	}
	if outA.packetCount != 1 {
		t.Fatalf("单候选应固定命中: got=%d", outA.packetCount)
	}
}

func TestGeoloomRandomOutboundNetworkMismatch(t *testing.T) {
	t.Parallel()

	outA := &stubOutbound{adapterType: "socks", tag: "a", network: []string{N.NetworkUDP}}
	random := &geoloomRandomOutbound{
		Adapter: boxOutbound.NewAdapter(geoloomRandomOutboundType, defaultLBTag, []string{N.NetworkTCP, N.NetworkUDP}, []string{"a"}),
		lookupOutbound: func(tag string) (adapter.Outbound, bool) {
			if tag == "a" {
				return outA, true
			}
			return nil, false
		},
		tags:   []string{"a"},
		random: &stubRandomIntn{values: []int{0}},
	}

	dst := M.ParseSocksaddrHostPort("example.com", 443)
	if _, err := random.DialContext(context.Background(), N.NetworkTCP, dst); err == nil {
		t.Fatal("网络不匹配时应返回错误")
	}
}

func TestRegisterGeoloomRandom(t *testing.T) {
	t.Parallel()

	registry := boxOutbound.NewRegistry()
	registerGeoloomRandom(registry)

	created, ok := registry.CreateOptions(geoloomRandomOutboundType)
	if !ok {
		t.Fatalf("未注册 geoloom weighted-random: %s", geoloomRandomOutboundType)
	}
	if _, typeOK := created.(*geoloomRandomOutboundOptions); !typeOK {
		t.Fatalf("options 类型错误: %T", created)
	}

	a := &stubOutbound{adapterType: "socks", tag: "a", network: []string{N.NetworkTCP, N.NetworkUDP}}
	outboundManager := &stubOutboundManager{outbounds: map[string]adapter.Outbound{"a": a}}
	ctx := service.ContextWith[adapter.OutboundManager](context.Background(), outboundManager)
	createdOutbound, err := registry.CreateOutbound(ctx, nil, nil, defaultLBTag, geoloomRandomOutboundType, &geoloomRandomOutboundOptions{Outbounds: []string{"a"}})
	if err != nil {
		t.Fatalf("CreateOutbound 返回错误: %v", err)
	}
	if createdOutbound.Type() != geoloomRandomOutboundType {
		t.Fatalf("outbound 类型错误: got=%s", createdOutbound.Type())
	}
}

func TestGeoloomRandomOutboundWeightedPick(t *testing.T) {
	t.Parallel()

	outA := &stubOutbound{adapterType: "socks", tag: "a", network: []string{N.NetworkTCP, N.NetworkUDP}}
	outB := &stubOutbound{adapterType: "socks", tag: "b", network: []string{N.NetworkTCP, N.NetworkUDP}}
	random := &geoloomRandomOutbound{
		Adapter: boxOutbound.NewAdapter(geoloomRandomOutboundType, defaultLBTag, []string{N.NetworkTCP, N.NetworkUDP}, []string{"a", "b"}),
		lookupOutbound: func(tag string) (adapter.Outbound, bool) {
			if tag == "a" {
				return outA, true
			}
			if tag == "b" {
				return outB, true
			}
			return nil, false
		},
		tags:    []string{"a", "b"},
		weights: []int{8, 1},
		random:  &stubRandomIntn{values: []int{0, 1, 2, 3, 4, 5, 6, 7, 8}},
	}
	dst := M.ParseSocksaddrHostPort("example.com", 443)
	for i := 0; i < 9; i++ {
		if _, err := random.DialContext(context.Background(), N.NetworkTCP, dst); err != nil {
			t.Fatalf("DialContext 返回错误: %v", err)
		}
	}
	if outA.dialCount <= outB.dialCount {
		t.Fatalf("高权重节点应命中更多: a=%d b=%d", outA.dialCount, outB.dialCount)
	}
}

func TestNormalizeWeightsShouldFallbackToEqualWeight(t *testing.T) {
	t.Parallel()
	weights := normalizeWeights(nil, 3)
	if len(weights) != 3 || weights[0] != 1 || weights[1] != 1 || weights[2] != 1 {
		t.Fatalf("默认权重错误: %+v", weights)
	}
}

type stubOutboundManager struct {
	outbounds map[string]adapter.Outbound
}

func (s *stubOutboundManager) Start(adapter.StartStage) error { return nil }
func (s *stubOutboundManager) Close() error                   { return nil }
func (s *stubOutboundManager) Outbounds() []adapter.Outbound  { return nil }
func (s *stubOutboundManager) Default() adapter.Outbound      { return nil }
func (s *stubOutboundManager) Remove(string) error            { return nil }
func (s *stubOutboundManager) Create(context.Context, adapter.Router, boxLog.ContextLogger, string, string, any) error {
	return nil
}
func (s *stubOutboundManager) Outbound(tag string) (adapter.Outbound, bool) {
	outbound, ok := s.outbounds[tag]
	return outbound, ok
}
