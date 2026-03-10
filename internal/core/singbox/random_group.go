package singbox

import (
	"context"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	boxOutbound "github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
)

const geoloomRandomOutboundType = "geoloom-random"

type geoloomRandomOutboundOptions struct {
	Outbounds []string `json:"outbounds"`
	Weights   []int    `json:"weights,omitempty"`
}

type randomIntn interface {
	Intn(n int) int
}

var _ adapter.OutboundGroup = (*geoloomRandomOutbound)(nil)

type geoloomRandomOutbound struct {
	boxOutbound.Adapter

	lookupOutbound func(tag string) (adapter.Outbound, bool)
	tags           []string
	weights        []int

	randomMu sync.Mutex
	random   randomIntn

	stateMu   sync.RWMutex
	lastProxy string
}

func registerGeoloomRandom(registry *boxOutbound.Registry) {
	boxOutbound.Register[geoloomRandomOutboundOptions](registry, geoloomRandomOutboundType, newGeoloomRandomOutbound)
}

func newGeoloomRandomOutbound(ctx context.Context, _ adapter.Router, _ log.ContextLogger, tag string, options geoloomRandomOutboundOptions) (adapter.Outbound, error) {
	if len(options.Outbounds) == 0 {
		return nil, E.New("missing outbounds")
	}
	outboundManager := service.FromContext[adapter.OutboundManager](ctx)
	if outboundManager == nil {
		return nil, E.New("missing outbound manager in context")
	}

	candidateTags := append([]string(nil), options.Outbounds...)
	candidateWeights := normalizeWeights(options.Weights, len(candidateTags))
	return &geoloomRandomOutbound{
		Adapter:        boxOutbound.NewAdapter(geoloomRandomOutboundType, tag, []string{N.NetworkTCP, N.NetworkUDP}, candidateTags),
		lookupOutbound: outboundManager.Outbound,
		tags:           candidateTags,
		weights:        candidateWeights,
		random:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}, nil
}

func (g *geoloomRandomOutbound) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	selected, err := g.pickOutboundByNetwork(network)
	if err != nil {
		return nil, err
	}
	return selected.DialContext(ctx, network, destination)
}

func (g *geoloomRandomOutbound) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	selected, err := g.pickOutboundByNetwork(N.NetworkUDP)
	if err != nil {
		return nil, err
	}
	return selected.ListenPacket(ctx, destination)
}

func (g *geoloomRandomOutbound) Now() string {
	g.stateMu.RLock()
	defer g.stateMu.RUnlock()
	if g.lastProxy != "" {
		return g.lastProxy
	}
	if len(g.tags) == 0 {
		return ""
	}
	return g.tags[0]
}

func (g *geoloomRandomOutbound) All() []string {
	g.stateMu.RLock()
	defer g.stateMu.RUnlock()
	result := make([]string, 0, len(g.tags))
	result = append(result, g.tags...)
	return result
}

func (g *geoloomRandomOutbound) pickOutboundByNetwork(network string) (adapter.Outbound, error) {
	normalized := N.NetworkName(network)
	candidates := make([]adapter.Outbound, 0, len(g.tags))
	candidateWeights := make([]int, 0, len(g.tags))
	for index, tag := range g.tags {
		detour, loaded := g.lookupOutbound(tag)
		if !loaded {
			return nil, E.New("outbound not found: ", tag)
		}
		if !common.Contains(detour.Network(), normalized) {
			continue
		}
		candidates = append(candidates, detour)
		weight := 1
		if index < len(g.weights) && g.weights[index] > 0 {
			weight = g.weights[index]
		}
		candidateWeights = append(candidateWeights, weight)
	}
	if len(candidates) == 0 {
		return nil, E.New("missing supported outbound")
	}

	selected := candidates[g.nextIndex(candidateWeights)]
	g.stateMu.Lock()
	g.lastProxy = selected.Tag()
	g.stateMu.Unlock()
	return selected, nil
}

func (g *geoloomRandomOutbound) nextIndex(weights []int) int {
	if len(weights) <= 1 {
		return 0
	}

	g.randomMu.Lock()
	defer g.randomMu.Unlock()
	if g.random == nil {
		g.random = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	total := 0
	for _, weight := range weights {
		if weight > 0 {
			total += weight
		}
	}
	if total <= 0 {
		return g.random.Intn(len(weights))
	}
	target := g.random.Intn(total)
	cursor := 0
	for index, weight := range weights {
		if weight <= 0 {
			continue
		}
		cursor += weight
		if target < cursor {
			return index
		}
	}
	return len(weights) - 1
}

func normalizeWeights(weights []int, size int) []int {
	if size <= 0 {
		return nil
	}
	result := make([]int, size)
	for index := 0; index < size; index++ {
		result[index] = 1
		if index < len(weights) && weights[index] > 0 {
			result[index] = weights[index]
		}
	}
	return result
}
