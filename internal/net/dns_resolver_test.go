package netresolver

import (
	"context"
	"net"
	"testing"
)

func TestResolveWithIPAddress(t *testing.T) {
	t.Parallel()

	resolver := NewDNSResolver(nil)
	ips, err := resolver.Resolve(context.Background(), "1.1.1.1")
	if err != nil {
		t.Fatalf("解析 IP 失败: %v", err)
	}
	if len(ips) != 1 || !ips[0].Equal(net.ParseIP("1.1.1.1")) {
		t.Fatalf("解析结果错误: %#v", ips)
	}
}

func TestResolveWithEmptyHost(t *testing.T) {
	t.Parallel()

	resolver := NewDNSResolver(nil)
	_, err := resolver.Resolve(context.Background(), "  ")
	if err == nil {
		t.Fatal("预期空 host 返回错误，但得到 nil")
	}
}
