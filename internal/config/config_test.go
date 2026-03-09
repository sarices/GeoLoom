package config

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func baseValidConfig() Config {
	return Config{
		Gateway: GatewayConfig{
			HTTPPort:  8080,
			SocksPort: 1080,
		},
		Policy:  PolicyConfig{},
		Sources: []Source{{Type: "node", URL: "socks5://1.1.1.1:1080#demo"}},
	}
}

func TestValidateGeoDefaults(t *testing.T) {
	t.Parallel()

	cfg := baseValidConfig()
	if err := cfg.validate(); err != nil {
		t.Fatalf("校验失败: %v", err)
	}

	if cfg.Geo.DNSTimeout != "3s" {
		t.Fatalf("默认 DNS 超时错误: got=%s", cfg.Geo.DNSTimeout)
	}
}

func TestValidateGeoDNSTimeoutInvalid(t *testing.T) {
	t.Parallel()

	cfg := baseValidConfig()
	cfg.Geo.DNSTimeout = "bad-timeout"

	if err := cfg.validate(); err == nil {
		t.Fatal("预期校验失败，但得到 nil")
	}
}

func TestValidatePolicyStrategyNormalize(t *testing.T) {
	t.Parallel()

	cfg := baseValidConfig()
	cfg.Policy.Strategy = "UNKNOWN"

	if err := cfg.validate(); err != nil {
		t.Fatalf("校验失败: %v", err)
	}
	if cfg.Policy.Strategy != StrategyRandom {
		t.Fatalf("策略回退错误: got=%s want=%s", cfg.Policy.Strategy, StrategyRandom)
	}
}

func TestValidateGeoMMDBURLInvalidScheme(t *testing.T) {
	t.Parallel()

	cfg := baseValidConfig()
	cfg.Geo.MMDBURL = "ftp://example.com/db.mmdb"

	if err := cfg.validate(); err == nil {
		t.Fatal("预期校验失败，但得到 nil")
	}
}

func TestValidateGeoMMDBURLHTTPShouldPass(t *testing.T) {
	t.Parallel()

	cfg := baseValidConfig()
	cfg.Geo.MMDBURL = "https://example.com/db.mmdb"

	if err := cfg.validate(); err != nil {
		t.Fatalf("校验失败: %v", err)
	}
}

func TestValidateAPIDefaults(t *testing.T) {
	t.Parallel()

	cfg := baseValidConfig()
	cfg.API = APIConfig{Enabled: true}

	if err := cfg.validate(); err != nil {
		t.Fatalf("校验失败: %v", err)
	}
	if cfg.API.Listen != defaultAPIListen {
		t.Fatalf("默认 listen 错误: got=%s want=%s", cfg.API.Listen, defaultAPIListen)
	}
	if cfg.API.AuthHeader != defaultAPIAuthHeader {
		t.Fatalf("默认 auth_header 错误: got=%s want=%s", cfg.API.AuthHeader, defaultAPIAuthHeader)
	}
	if cfg.API.Token != "" {
		t.Fatalf("默认 token 错误: got=%s", cfg.API.Token)
	}
}

func TestValidateAPITokenShouldPass(t *testing.T) {
	t.Parallel()

	cfg := baseValidConfig()
	cfg.API = APIConfig{Enabled: true, Token: "  secret-token  ", AuthHeader: "  X-Custom-Token  "}

	if err := cfg.validate(); err != nil {
		t.Fatalf("校验失败: %v", err)
	}
	if cfg.API.Token != "secret-token" {
		t.Fatalf("token 归一化错误: got=%s", cfg.API.Token)
	}
	if cfg.API.AuthHeader != "X-Custom-Token" {
		t.Fatalf("auth_header 归一化错误: got=%s", cfg.API.AuthHeader)
	}
}

func TestValidateGatewayPortErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{
			name: "http_port 小于等于 0",
			mutate: func(cfg *Config) {
				cfg.Gateway.HTTPPort = 0
			},
			wantErr: "gateway.http_port",
		},
		{
			name: "http_port 超过上限",
			mutate: func(cfg *Config) {
				cfg.Gateway.HTTPPort = 70000
			},
			wantErr: "gateway.http_port",
		},
		{
			name: "socks_port 小于等于 0",
			mutate: func(cfg *Config) {
				cfg.Gateway.SocksPort = 0
			},
			wantErr: "gateway.socks_port",
		},
		{
			name: "socks_port 超过上限",
			mutate: func(cfg *Config) {
				cfg.Gateway.SocksPort = 65536
			},
			wantErr: "gateway.socks_port",
		},
		{
			name: "http 与 socks 端口冲突",
			mutate: func(cfg *Config) {
				cfg.Gateway.HTTPPort = cfg.Gateway.SocksPort
			},
			wantErr: "gateway.http_port 与 gateway.socks_port 不能相同",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := baseValidConfig()
			tt.mutate(&cfg)
			err := cfg.validate()
			if err == nil {
				t.Fatal("预期校验失败，但得到 nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("错误信息不符合预期: got=%v want_contains=%s", err, tt.wantErr)
			}
		})
	}
}

func TestValidateSourcesErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{
			name: "sources 为空",
			mutate: func(cfg *Config) {
				cfg.Sources = nil
			},
			wantErr: "sources 不能为空",
		},
		{
			name: "source type 非法",
			mutate: func(cfg *Config) {
				cfg.Sources[0].Type = "invalid"
			},
			wantErr: "sources[0].type",
		},
		{
			name: "source url 为空",
			mutate: func(cfg *Config) {
				cfg.Sources[0].URL = "   "
			},
			wantErr: "sources[0].url",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := baseValidConfig()
			tt.mutate(&cfg)
			err := cfg.validate()
			if err == nil {
				t.Fatal("预期校验失败，但得到 nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("错误信息不符合预期: got=%v want_contains=%s", err, tt.wantErr)
			}
		})
	}
}

func TestValidateHealthCheckEnabledErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{
			name: "interval 非法格式",
			mutate: func(cfg *Config) {
				cfg.Policy.HealthCheck = HealthCheckConfig{Enabled: true, Interval: "bad", URL: "https://example.com"}
			},
			wantErr: "policy.health_check.interval",
		},
		{
			name: "interval 非正数",
			mutate: func(cfg *Config) {
				cfg.Policy.HealthCheck = HealthCheckConfig{Enabled: true, Interval: "0s", URL: "https://example.com"}
			},
			wantErr: "policy.health_check.interval 必须大于 0",
		},
		{
			name: "url 非法",
			mutate: func(cfg *Config) {
				cfg.Policy.HealthCheck = HealthCheckConfig{Enabled: true, Interval: "10s", URL: "http://[::1"}
			},
			wantErr: "policy.health_check.url",
		},
		{
			name: "url scheme 非 http/https",
			mutate: func(cfg *Config) {
				cfg.Policy.HealthCheck = HealthCheckConfig{Enabled: true, Interval: "10s", URL: "ftp://example.com"}
			},
			wantErr: "policy.health_check.url 仅支持 http/https",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := baseValidConfig()
			tt.mutate(&cfg)
			err := cfg.validate()
			if err == nil {
				t.Fatal("预期校验失败，但得到 nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("错误信息不符合预期: got=%v want_contains=%s", err, tt.wantErr)
			}
		})
	}
}

func TestSourceTypeHelpers(t *testing.T) {
	t.Parallel()

	normalized := NormalizeSourceType("  SuBsCriBe  ")
	if normalized != SourceTypeSubscribe {
		t.Fatalf("NormalizeSourceType 错误: got=%s want=%s", normalized, SourceTypeSubscribe)
	}

	if !IsValidSourceType(SourceTypeNode) {
		t.Fatal("IsValidSourceType 对 node 应返回 true")
	}
	if IsValidSourceType("invalid") {
		t.Fatal("IsValidSourceType 对非法类型应返回 false")
	}

	if !IsSourceLikeType(SourceTypeSource) {
		t.Fatal("IsSourceLikeType 对 source 应返回 true")
	}
	if !IsSourceLikeType(SourceTypeSubscribe) {
		t.Fatal("IsSourceLikeType 对 subscribe 应返回 true")
	}
	if IsSourceLikeType(SourceTypeNode) {
		t.Fatal("IsSourceLikeType 对 node 应返回 false")
	}
}

func TestLoadRepoConfigsShouldPass(t *testing.T) {
	t.Parallel()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("获取当前测试文件路径失败")
	}

	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	tests := []struct {
		name       string
		configPath string
	}{
		{
			name:       "默认配置",
			configPath: filepath.Join(repoRoot, "configs", "config.yaml"),
		},
		{
			name:       "示例配置",
			configPath: filepath.Join(repoRoot, "configs", "config.example.yaml"),
		},
		{
			name:       "生产示例配置",
			configPath: filepath.Join(repoRoot, "configs", "config.example.prod.yaml"),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg, err := Load(tt.configPath)
			if err != nil {
				t.Fatalf("加载配置失败: path=%s err=%v", tt.configPath, err)
			}
			if len(cfg.Sources) == 0 {
				t.Fatalf("配置 sources 不应为空: path=%s", tt.configPath)
			}
		})
	}
}

func TestValidateHealthCheckDisabledAllowsIncompleteFields(t *testing.T) {
	t.Parallel()

	cfg := baseValidConfig()
	cfg.Policy.HealthCheck = HealthCheckConfig{
		Enabled:  false,
		Interval: "bad-duration",
		URL:      "ftp://example.com",
	}

	if err := cfg.validate(); err != nil {
		t.Fatalf("disabled 场景不应因 health_check 字段失败: %v", err)
	}
}

func TestValidateRefreshDefaults(t *testing.T) {
	t.Parallel()

	cfg := baseValidConfig()
	cfg.Policy.Refresh = RefreshConfig{Enabled: true}
	cfg.API = APIConfig{Enabled: true}
	cfg.State = StateConfig{Enabled: true}

	if err := cfg.validate(); err != nil {
		t.Fatalf("校验失败: %v", err)
	}
	if cfg.Policy.Refresh.Interval != defaultRefreshInterval {
		t.Fatalf("默认 refresh interval 错误: got=%s", cfg.Policy.Refresh.Interval)
	}
	if cfg.API.Listen != defaultAPIListen {
		t.Fatalf("默认 api listen 错误: got=%s", cfg.API.Listen)
	}
	if cfg.State.Path != defaultStatePath {
		t.Fatalf("默认 state path 错误: got=%s", cfg.State.Path)
	}
}

func TestValidateRefreshEnabledErrors(t *testing.T) {
	t.Parallel()
	cfg := baseValidConfig()
	cfg.Policy.Refresh = RefreshConfig{Enabled: true, Interval: "bad"}
	if err := cfg.validate(); err == nil {
		t.Fatal("预期 refresh 校验失败，但得到 nil")
	}
}
