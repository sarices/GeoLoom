package geo

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/oschwald/geoip2-golang"

	"geoloom/internal/domain"
)

type fakeDNSResolver struct {
	ips []net.IP
	err error
}

func (f fakeDNSResolver) Resolve(_ context.Context, _ string) ([]net.IP, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.ips, nil
}

type fakeCountryReader struct {
	calls  int
	record map[string]string
	err    error
}

func (f *fakeCountryReader) Country(ip net.IP) (*geoip2.Country, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	country := &geoip2.Country{}
	country.Country.IsoCode = f.record[ip.String()]
	return country, nil
}

func (f *fakeCountryReader) Close() error {
	return nil
}

func TestResolveNodeCountryWithCache(t *testing.T) {
	t.Parallel()

	reader := &fakeCountryReader{record: map[string]string{"1.1.1.1": "US"}}
	resolver, err := newMMDBResolverWithReader(
		reader,
		NewInMemoryCountryCache(),
		fakeDNSResolver{ips: []net.IP{net.ParseIP("1.1.1.1")}},
	)
	if err != nil {
		t.Fatalf("创建 resolver 失败: %v", err)
	}

	node := domain.NodeMetadata{Address: "example.com"}

	country, err := resolver.ResolveNodeCountry(context.Background(), node)
	if err != nil {
		t.Fatalf("首次解析失败: %v", err)
	}
	if country != "US" {
		t.Fatalf("国家码错误: got=%s", country)
	}

	country, err = resolver.ResolveNodeCountry(context.Background(), node)
	if err != nil {
		t.Fatalf("二次解析失败: %v", err)
	}
	if country != "US" {
		t.Fatalf("国家码错误: got=%s", country)
	}
	if reader.calls != 1 {
		t.Fatalf("缓存未生效，MMDB 调用次数: got=%d want=1", reader.calls)
	}
}

func TestResolveNodeCountryDNSError(t *testing.T) {
	t.Parallel()

	resolver, err := newMMDBResolverWithReader(
		&fakeCountryReader{record: map[string]string{"1.1.1.1": "US"}},
		NewInMemoryCountryCache(),
		fakeDNSResolver{err: errors.New("dns failed")},
	)
	if err != nil {
		t.Fatalf("创建 resolver 失败: %v", err)
	}

	_, err = resolver.ResolveNodeCountry(context.Background(), domain.NodeMetadata{Address: "example.com"})
	if err == nil {
		t.Fatal("预期 DNS 错误，但得到 nil")
	}
	if !strings.Contains(err.Error(), "dns failed") {
		t.Fatalf("错误信息不匹配: %v", err)
	}
}

func TestResolveNodeCountryNoCountry(t *testing.T) {
	t.Parallel()

	resolver, err := newMMDBResolverWithReader(
		&fakeCountryReader{record: map[string]string{"1.1.1.1": ""}},
		NewInMemoryCountryCache(),
		fakeDNSResolver{ips: []net.IP{net.ParseIP("1.1.1.1")}},
	)
	if err != nil {
		t.Fatalf("创建 resolver 失败: %v", err)
	}

	_, err = resolver.ResolveNodeCountry(context.Background(), domain.NodeMetadata{Address: "example.com"})
	if err == nil {
		t.Fatal("预期国家码缺失错误，但得到 nil")
	}
	if !strings.Contains(err.Error(), "未能识别节点国家") {
		t.Fatalf("错误信息不匹配: %v", err)
	}
}
