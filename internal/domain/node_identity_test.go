package domain

import "testing"

func TestBuildNodeFingerprintStable(t *testing.T) {
	t.Parallel()

	node := NodeMetadata{
		Protocol: "socks5",
		Address:  "1.1.1.1",
		Port:     1080,
		RawConfig: map[string]any{
			"username": "user",
			"password": "pass",
		},
	}

	first, err := BuildNodeFingerprint(node)
	if err != nil {
		t.Fatalf("首次构建 fingerprint 失败: %v", err)
	}
	second, err := BuildNodeFingerprint(node)
	if err != nil {
		t.Fatalf("再次构建 fingerprint 失败: %v", err)
	}
	if first != second {
		t.Fatalf("fingerprint 不稳定: first=%s second=%s", first, second)
	}
}

func TestBuildNodeFingerprintIgnoreName(t *testing.T) {
	t.Parallel()

	base := NodeMetadata{
		Name:     "a",
		Protocol: "socks5",
		Address:  "1.1.1.1",
		Port:     1080,
		RawConfig: map[string]any{
			"username": "user",
			"password": "pass",
		},
	}
	other := base
	other.Name = "b"

	left, _ := BuildNodeFingerprint(base)
	right, _ := BuildNodeFingerprint(other)
	if left != right {
		t.Fatalf("名称不应影响 fingerprint: left=%s right=%s", left, right)
	}
}

func TestBuildNodeFingerprintDifferentProtocols(t *testing.T) {
	t.Parallel()

	socks := NodeMetadata{Protocol: "socks5", Address: "1.1.1.1", Port: 1080, RawConfig: map[string]any{"username": "u", "password": "p"}}
	ss := NodeMetadata{Protocol: "shadowsocks", Address: "1.1.1.1", Port: 1080, RawConfig: map[string]any{"method": "aes-128-gcm", "password": "p"}}

	socksFP, _ := BuildNodeFingerprint(socks)
	ssFP, _ := BuildNodeFingerprint(ss)
	if socksFP == ssFP {
		t.Fatalf("不同协议 fingerprint 不应相同: %s", socksFP)
	}
}

func TestBuildNodeFingerprintDifferentKeyFields(t *testing.T) {
	t.Parallel()

	left := NodeMetadata{Protocol: "trojan", Address: "example.com", Port: 443, RawConfig: map[string]any{"password": "a", "security": "tls", "network": "ws", "sni": "example.com", "host": "cdn.example.com", "path": "/ws"}}
	right := left
	right.RawConfig = map[string]any{"password": "b", "security": "tls", "network": "ws", "sni": "example.com", "host": "cdn.example.com", "path": "/ws"}

	leftFP, _ := BuildNodeFingerprint(left)
	rightFP, _ := BuildNodeFingerprint(right)
	if leftFP == rightFP {
		t.Fatalf("关键认证字段变化应影响 fingerprint: %s", leftFP)
	}
}

func TestBuildNodeFingerprintNormalizeCase(t *testing.T) {
	t.Parallel()

	left := NodeMetadata{Protocol: "vless", Address: "EXAMPLE.COM", Port: 443, RawConfig: map[string]any{"uuid": "id", "security": "TLS", "network": "WS", "sni": "SNI.EXAMPLE.COM", "host": "CDN.EXAMPLE.COM", "path": "/ws"}}
	right := NodeMetadata{Protocol: "vless", Address: "example.com", Port: 443, RawConfig: map[string]any{"uuid": "id", "security": "tls", "network": "ws", "sni": "sni.example.com", "host": "cdn.example.com", "path": "/ws"}}

	leftFP, _ := BuildNodeFingerprint(left)
	rightFP, _ := BuildNodeFingerprint(right)
	if leftFP != rightFP {
		t.Fatalf("大小写归一化后 fingerprint 应相同: left=%s right=%s", leftFP, rightFP)
	}
}

func TestBuildNodeFingerprintSocks4AndHTTP(t *testing.T) {
	t.Parallel()

	socks4 := NodeMetadata{Protocol: "socks4", Address: "1.1.1.1", Port: 1080, RawConfig: map[string]any{"username": "legacy"}}
	httpNode := NodeMetadata{Protocol: "http", Address: "1.1.1.1", Port: 8080, RawConfig: map[string]any{"username": "user", "password": "pass"}}

	socks4FP, err := BuildNodeFingerprint(socks4)
	if err != nil {
		t.Fatalf("socks4 fingerprint 构建失败: %v", err)
	}
	httpFP, err := BuildNodeFingerprint(httpNode)
	if err != nil {
		t.Fatalf("http fingerprint 构建失败: %v", err)
	}
	if socks4FP == "" || httpFP == "" {
		t.Fatalf("fingerprint 不应为空: socks4=%q http=%q", socks4FP, httpFP)
	}
	if socks4FP == httpFP {
		t.Fatalf("不同协议 fingerprint 不应相同: socks4=%s http=%s", socks4FP, httpFP)
	}
}

func TestBuildNodeFingerprintHTTPIgnoreName(t *testing.T) {
	t.Parallel()

	left := NodeMetadata{
		Name:     "dup-a",
		Protocol: "http",
		Address:  "1.1.1.1",
		Port:     8080,
		RawConfig: map[string]any{
			"username": "user",
			"password": "pass",
		},
	}
	right := left
	right.Name = "dup-b"

	leftFP, err := BuildNodeFingerprint(left)
	if err != nil {
		t.Fatalf("left fingerprint 构建失败: %v", err)
	}
	rightFP, err := BuildNodeFingerprint(right)
	if err != nil {
		t.Fatalf("right fingerprint 构建失败: %v", err)
	}
	if leftFP != rightFP {
		t.Fatalf("HTTP 节点名称不应影响 fingerprint: left=%s right=%s", leftFP, rightFP)
	}
}

func TestBuildNodeFingerprintOptionalFieldsEmpty(t *testing.T) {
	t.Parallel()

	node := NodeMetadata{Protocol: "vmess", Address: "vm.example.com", Port: 443, RawConfig: map[string]any{"uuid": "id"}}
	if _, err := BuildNodeFingerprint(node); err != nil {
		t.Fatalf("缺省可选字段不应报错: %v", err)
	}
}
