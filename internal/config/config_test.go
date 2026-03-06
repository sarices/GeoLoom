package config

import "testing"

func TestValidateGeoDefaults(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Gateway: GatewayConfig{SocksPort: 1080},
		Policy:  PolicyConfig{},
		Sources: []Source{{URL: "https://example.com/sub"}},
	}

	if err := cfg.validate(); err != nil {
		t.Fatalf("校验失败: %v", err)
	}

	if cfg.Geo.DNSTimeout != "3s" {
		t.Fatalf("默认 DNS 超时错误: got=%s", cfg.Geo.DNSTimeout)
	}
}

func TestValidateGeoDNSTimeoutInvalid(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Gateway: GatewayConfig{SocksPort: 1080},
		Geo:     GeoConfig{DNSTimeout: "bad-timeout"},
		Sources: []Source{{URL: "https://example.com/sub"}},
	}

	if err := cfg.validate(); err == nil {
		t.Fatal("预期校验失败，但得到 nil")
	}
}

func TestValidatePolicyStrategyNormalize(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Gateway: GatewayConfig{SocksPort: 1080},
		Policy: PolicyConfig{
			Strategy: "UNKNOWN",
		},
		Sources: []Source{{URL: "https://example.com/sub"}},
	}

	if err := cfg.validate(); err != nil {
		t.Fatalf("校验失败: %v", err)
	}
	if cfg.Policy.Strategy != StrategyRandom {
		t.Fatalf("策略回退错误: got=%s want=%s", cfg.Policy.Strategy, StrategyRandom)
	}
}

func TestValidateGeoMMDBURLInvalidScheme(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Gateway: GatewayConfig{SocksPort: 1080},
		Geo: GeoConfig{
			MMDBURL: "ftp://example.com/db.mmdb",
		},
		Sources: []Source{{URL: "https://example.com/sub"}},
	}

	if err := cfg.validate(); err == nil {
		t.Fatal("预期校验失败，但得到 nil")
	}
}

func TestValidateGeoMMDBURLHTTPShouldPass(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Gateway: GatewayConfig{SocksPort: 1080},
		Geo: GeoConfig{
			MMDBURL: "https://example.com/db.mmdb",
		},
		Sources: []Source{{URL: "https://example.com/sub"}},
	}

	if err := cfg.validate(); err != nil {
		t.Fatalf("校验失败: %v", err)
	}
}

func TestValidateHealthCheckDefaults(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Gateway: GatewayConfig{SocksPort: 1080},
		Policy: PolicyConfig{
			HealthCheck: HealthCheckConfig{Enabled: true},
		},
		Sources: []Source{{URL: "https://example.com/sub"}},
	}

	if err := cfg.validate(); err != nil {
		t.Fatalf("校验失败: %v", err)
	}
	if cfg.Policy.HealthCheck.Interval != defaultHealthCheckInterval {
		t.Fatalf("默认 interval 错误: got=%s", cfg.Policy.HealthCheck.Interval)
	}
	if cfg.Policy.HealthCheck.URL != defaultHealthCheckURL {
		t.Fatalf("默认 url 错误: got=%s", cfg.Policy.HealthCheck.URL)
	}
}
