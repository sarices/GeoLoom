package netresolver

import (
	"context"
	"fmt"
	"net"
	"strings"
)

// DNSResolver 抽象地址解析能力，便于替换与测试。
type DNSResolver interface {
	Resolve(ctx context.Context, host string) ([]net.IP, error)
}

// Resolver 基于 net.Resolver 实现地址解析。
type Resolver struct {
	resolver *net.Resolver
}

// NewDNSResolver 创建 DNS 解析器。
func NewDNSResolver(resolver *net.Resolver) *Resolver {
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	return &Resolver{resolver: resolver}
}

// Resolve 将 host 解析为去重后的 IP 列表。
func (r *Resolver) Resolve(ctx context.Context, host string) ([]net.IP, error) {
	trimmed := strings.TrimSpace(host)
	if trimmed == "" {
		return nil, fmt.Errorf("host 不能为空")
	}

	if ip := net.ParseIP(trimmed); ip != nil {
		return []net.IP{ip}, nil
	}

	addresses, err := r.resolver.LookupIPAddr(ctx, trimmed)
	if err != nil {
		return nil, fmt.Errorf("DNS 解析失败: %w", err)
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("DNS 未返回可用 IP")
	}

	result := make([]net.IP, 0, len(addresses))
	seen := make(map[string]struct{}, len(addresses))
	for _, address := range addresses {
		if address.IP == nil {
			continue
		}
		key := address.IP.String()
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, address.IP)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("DNS 未返回可用 IP")
	}
	return result, nil
}
