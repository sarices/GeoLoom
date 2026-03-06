package health

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	"geoloom/internal/domain"

	M "github.com/sagernet/sing/common/metadata"
)

// staticDialer 为 urltest.URLTest 提供单节点探测拨号能力。
type staticDialer struct {
	node domain.NodeMetadata
}

func (d staticDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	if d.node.Address == "" || d.node.Port <= 0 {
		return nil, fmt.Errorf("节点地址或端口非法")
	}

	address := net.JoinHostPort(strings.TrimSpace(d.node.Address), strconv.Itoa(d.node.Port))
	var dialer net.Dialer
	return dialer.DialContext(ctx, "tcp", address)
}

func (d staticDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	var listenConfig net.ListenConfig
	return listenConfig.ListenPacket(ctx, "udp", "")
}
