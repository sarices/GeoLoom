package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	StrategyRandom  = "random"
	StrategyURLTest = "urltest"

	SourceTypeSource    = "source"
	SourceTypeSubscribe = "subscribe"
	SourceTypeNode      = "node"

	defaultHealthCheckInterval = "5m"
	defaultHealthCheckURL      = "http://cp.cloudflare.com"
	defaultRefreshInterval     = "10m"
	defaultAPIListen           = "127.0.0.1:9090"
	defaultAPIAuthHeader       = "X-GeoLoom-Token"
	defaultStatePath           = "geoloom-state.json"

	minPort = 1
	maxPort = 65535
)

// Config 定义 GeoLoom 启动配置。
type Config struct {
	Gateway GatewayConfig `yaml:"gateway"`
	Policy  PolicyConfig  `yaml:"policy"`
	Geo     GeoConfig     `yaml:"geo"`
	API     APIConfig     `yaml:"api"`
	State   StateConfig   `yaml:"state"`
	Sources []Source      `yaml:"sources"`
}

// GatewayConfig 定义网关监听端口。
type GatewayConfig struct {
	HTTPPort  int `yaml:"http_port"`
	SocksPort int `yaml:"socks_port"`
}

// PolicyConfig 定义转发策略与过滤规则。
type PolicyConfig struct {
	Strategy    string            `yaml:"strategy"`
	Filter      FilterConfig      `yaml:"filter"`
	HealthCheck HealthCheckConfig `yaml:"health_check"`
	Refresh     RefreshConfig     `yaml:"refresh"`
}

// FilterConfig 定义地域白名单与黑名单。
type FilterConfig struct {
	Allow []string `yaml:"allow"`
	Block []string `yaml:"block"`
}

// HealthCheckConfig 定义健康检查参数。
type HealthCheckConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Interval string `yaml:"interval"`
	URL      string `yaml:"url"`
}

// RefreshConfig 定义订阅刷新参数。
type RefreshConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Interval string `yaml:"interval"`
}

// GeoConfig 定义地理识别参数。
type GeoConfig struct {
	MMDBPath   string `yaml:"mmdb_path"`
	MMDBURL    string `yaml:"mmdb_url"`
	DNSTimeout string `yaml:"dns_timeout"`
}

// APIConfig 定义管理 API 参数。
type APIConfig struct {
	Enabled    bool   `yaml:"enabled"`
	Listen     string `yaml:"listen"`
	Token      string `yaml:"token"`
	AuthHeader string `yaml:"auth_header"`
}

// StateConfig 定义运行时状态持久化参数。
type StateConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}

// Source 定义输入源。
type Source struct {
	Name string `yaml:"name"`
	Type string `yaml:"type"`
	URL  string `yaml:"url"`
}

// Load 从指定路径读取并校验配置。
func Load(path string) (Config, error) {
	if strings.TrimSpace(path) == "" {
		return Config{}, fmt.Errorf("配置文件路径不能为空")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return Config{}, fmt.Errorf("解析配置文件失败: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.Gateway.HTTPPort < minPort || c.Gateway.HTTPPort > maxPort {
		return fmt.Errorf("gateway.http_port 必须在 %d~%d 之间", minPort, maxPort)
	}
	if c.Gateway.SocksPort < minPort || c.Gateway.SocksPort > maxPort {
		return fmt.Errorf("gateway.socks_port 必须在 %d~%d 之间", minPort, maxPort)
	}
	if c.Gateway.HTTPPort == c.Gateway.SocksPort {
		return fmt.Errorf("gateway.http_port 与 gateway.socks_port 不能相同")
	}

	c.Policy.Strategy = normalizeStrategy(c.Policy.Strategy)
	if c.Policy.HealthCheck.Enabled {
		c.Policy.HealthCheck = normalizeHealthCheck(c.Policy.HealthCheck)
		interval, err := time.ParseDuration(c.Policy.HealthCheck.Interval)
		if err != nil {
			return fmt.Errorf("policy.health_check.interval 非法: %w", err)
		}
		if interval <= 0 {
			return fmt.Errorf("policy.health_check.interval 必须大于 0")
		}
		parsedHealthCheckURL, err := url.Parse(c.Policy.HealthCheck.URL)
		if err != nil {
			return fmt.Errorf("policy.health_check.url 非法: %w", err)
		}
		healthCheckScheme := strings.ToLower(strings.TrimSpace(parsedHealthCheckURL.Scheme))
		if healthCheckScheme != "http" && healthCheckScheme != "https" {
			return fmt.Errorf("policy.health_check.url 仅支持 http/https")
		}
	}

	if c.Policy.Refresh.Enabled {
		c.Policy.Refresh = normalizeRefresh(c.Policy.Refresh)
		interval, err := time.ParseDuration(c.Policy.Refresh.Interval)
		if err != nil {
			return fmt.Errorf("policy.refresh.interval 非法: %w", err)
		}
		if interval <= 0 {
			return fmt.Errorf("policy.refresh.interval 必须大于 0")
		}
	}

	c.Policy.Filter.Allow = normalizeCountryCodes(c.Policy.Filter.Allow)
	c.Policy.Filter.Block = normalizeCountryCodes(c.Policy.Filter.Block)
	c.Geo.MMDBPath = strings.TrimSpace(c.Geo.MMDBPath)
	c.Geo.MMDBURL = strings.TrimSpace(c.Geo.MMDBURL)
	if c.Geo.MMDBURL != "" {
		parsedURL, err := url.Parse(c.Geo.MMDBURL)
		if err != nil {
			return fmt.Errorf("geo.mmdb_url 非法: %w", err)
		}
		scheme := strings.ToLower(strings.TrimSpace(parsedURL.Scheme))
		if scheme != "http" && scheme != "https" {
			return fmt.Errorf("geo.mmdb_url 仅支持 http/https")
		}
	}

	if strings.TrimSpace(c.Geo.DNSTimeout) == "" {
		c.Geo.DNSTimeout = "3s"
	}
	if _, err := time.ParseDuration(c.Geo.DNSTimeout); err != nil {
		return fmt.Errorf("geo.dns_timeout 非法: %w", err)
	}

	if c.API.Enabled {
		c.API = normalizeAPI(c.API)
		if strings.TrimSpace(c.API.Listen) == "" {
			return fmt.Errorf("api.listen 不能为空")
		}
		if strings.TrimSpace(c.API.AuthHeader) == "" {
			return fmt.Errorf("api.auth_header 不能为空")
		}
	}

	if c.State.Enabled {
		c.State = normalizeState(c.State)
		if strings.TrimSpace(c.State.Path) == "" {
			return fmt.Errorf("state.path 不能为空")
		}
	}

	if len(c.Sources) == 0 {
		return fmt.Errorf("sources 不能为空")
	}
	for i := range c.Sources {
		c.Sources[i].Name = strings.TrimSpace(c.Sources[i].Name)
		c.Sources[i].Type = NormalizeSourceType(c.Sources[i].Type)
		c.Sources[i].URL = strings.TrimSpace(c.Sources[i].URL)
		if !IsValidSourceType(c.Sources[i].Type) {
			return fmt.Errorf("sources[%d].type 非法: %s", i, c.Sources[i].Type)
		}
		if c.Sources[i].URL == "" {
			return fmt.Errorf("sources[%d].url 不能为空", i)
		}
	}

	return nil
}

func normalizeStrategy(raw string) string {
	strategy := strings.ToLower(strings.TrimSpace(raw))
	switch strategy {
	case "", StrategyRandom:
		return StrategyRandom
	case StrategyURLTest:
		return StrategyURLTest
	default:
		return StrategyRandom
	}
}

func normalizeHealthCheck(cfg HealthCheckConfig) HealthCheckConfig {
	if strings.TrimSpace(cfg.Interval) == "" {
		cfg.Interval = defaultHealthCheckInterval
	}
	if strings.TrimSpace(cfg.URL) == "" {
		cfg.URL = defaultHealthCheckURL
	}
	return cfg
}

func normalizeRefresh(cfg RefreshConfig) RefreshConfig {
	if strings.TrimSpace(cfg.Interval) == "" {
		cfg.Interval = defaultRefreshInterval
	}
	return cfg
}

func normalizeAPI(cfg APIConfig) APIConfig {
	if strings.TrimSpace(cfg.Listen) == "" {
		cfg.Listen = defaultAPIListen
	}
	cfg.Token = strings.TrimSpace(cfg.Token)
	cfg.AuthHeader = strings.TrimSpace(cfg.AuthHeader)
	if cfg.AuthHeader == "" {
		cfg.AuthHeader = defaultAPIAuthHeader
	}
	return cfg
}

func normalizeState(cfg StateConfig) StateConfig {
	if strings.TrimSpace(cfg.Path) == "" {
		cfg.Path = defaultStatePath
	}
	return cfg
}

func normalizeCountryCodes(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized := strings.ToUpper(strings.TrimSpace(value))
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func NormalizeSourceType(raw string) string {
	return strings.TrimSpace(strings.ToLower(raw))
}

func IsValidSourceType(raw string) bool {
	switch NormalizeSourceType(raw) {
	case SourceTypeSource, SourceTypeSubscribe, SourceTypeNode:
		return true
	default:
		return false
	}
}

func IsSourceLikeType(raw string) bool {
	switch NormalizeSourceType(raw) {
	case SourceTypeSource, SourceTypeSubscribe:
		return true
	default:
		return false
	}
}
