package geo

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/oschwald/geoip2-golang"

	"geoloom/internal/domain"
	netresolver "geoloom/internal/net"
)

// CountryResolver 提供节点国家识别能力。
type CountryResolver interface {
	ResolveNodeCountry(ctx context.Context, node domain.NodeMetadata) (string, error)
}

type countryReader interface {
	Country(ip net.IP) (*geoip2.Country, error)
	Close() error
}

// MMDBResolver 通过 MMDB + DNS + Cache 解析节点国家。
type MMDBResolver struct {
	db          countryReader
	cache       CountryCache
	dnsResolver netresolver.DNSResolver
}

func NewMMDBResolver(db *geoip2.Reader, cache CountryCache, dnsResolver netresolver.DNSResolver) (*MMDBResolver, error) {
	if db == nil {
		return nil, fmt.Errorf("MMDB reader 不能为空")
	}
	return newMMDBResolverWithReader(db, cache, dnsResolver)
}

func newMMDBResolverWithReader(db countryReader, cache CountryCache, dnsResolver netresolver.DNSResolver) (*MMDBResolver, error) {
	if db == nil {
		return nil, fmt.Errorf("MMDB reader 不能为空")
	}
	if cache == nil {
		cache = NewInMemoryCountryCache()
	}
	if dnsResolver == nil {
		dnsResolver = netresolver.NewDNSResolver(nil)
	}

	return &MMDBResolver{
		db:          db,
		cache:       cache,
		dnsResolver: dnsResolver,
	}, nil
}

func NewMMDBResolverFromPath(mmdbPath string, cache CountryCache, dnsResolver netresolver.DNSResolver) (*MMDBResolver, error) {
	trimmed := strings.TrimSpace(mmdbPath)
	if trimmed == "" {
		return nil, fmt.Errorf("MMDB 路径不能为空")
	}

	db, err := geoip2.Open(trimmed)
	if err != nil {
		return nil, fmt.Errorf("打开 MMDB 失败: %w", err)
	}

	resolver, err := newMMDBResolverWithReader(db, cache, dnsResolver)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return resolver, nil
}

func (r *MMDBResolver) Close() error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.db.Close()
}

// ResolveNodeCountry 先解析地址，再查缓存，最后 MMDB 查询国家码。
func (r *MMDBResolver) ResolveNodeCountry(ctx context.Context, node domain.NodeMetadata) (string, error) {
	if r == nil || r.db == nil {
		return "", fmt.Errorf("MMDB resolver 未初始化")
	}

	host := strings.TrimSpace(node.Address)
	if host == "" {
		return "", fmt.Errorf("节点地址为空")
	}

	ips, err := r.dnsResolver.Resolve(ctx, host)
	if err != nil {
		return "", err
	}

	for _, ip := range ips {
		country, lookupErr := r.resolveCountryByIP(ip)
		if lookupErr != nil {
			continue
		}
		if country != "" {
			return country, nil
		}
	}

	return "", fmt.Errorf("未能识别节点国家: %s", host)
}

func (r *MMDBResolver) resolveCountryByIP(ip net.IP) (string, error) {
	if ip == nil {
		return "", fmt.Errorf("IP 不能为空")
	}

	key := ip.String()
	if key == "" {
		return "", fmt.Errorf("IP 非法")
	}

	if cached, ok := r.cache.Get(key); ok {
		return cached, nil
	}

	record, err := r.db.Country(ip)
	if err != nil {
		return "", fmt.Errorf("MMDB 查询失败: %w", err)
	}

	countryCode := strings.ToUpper(strings.TrimSpace(record.Country.IsoCode))
	if countryCode == "" {
		return "", fmt.Errorf("MMDB 未返回国家码")
	}

	r.cache.Set(key, countryCode)
	return countryCode, nil
}
