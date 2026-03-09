package parser

import "testing"

func TestParseSocks4(t *testing.T) {
	t.Parallel()

	raw := "socks4://legacy@149.130.191.255:6161#legacy-node"
	node, err := ParseSocks4(raw)
	if err != nil {
		t.Fatalf("ParseSocks4 返回错误: %v", err)
	}

	if node.Protocol != "socks4" {
		t.Fatalf("协议错误: got=%s", node.Protocol)
	}
	if node.Address != "149.130.191.255" || node.Port != 6161 {
		t.Fatalf("地址端口错误: got=%s:%d", node.Address, node.Port)
	}
	if node.RawConfig["username"] != "legacy" {
		t.Fatalf("用户名错误: got=%v", node.RawConfig["username"])
	}
	if node.Name != "legacy-node" {
		t.Fatalf("名称错误: got=%s", node.Name)
	}
}

func TestParseSocks4InvalidPort(t *testing.T) {
	t.Parallel()

	_, err := ParseSocks4("socks4://legacy@127.0.0.1:0")
	if err == nil {
		t.Fatal("预期返回错误，但得到 nil")
	}
	if !IsErrorKind(err, ErrorKindInvalidInput) {
		t.Fatalf("错误类型不匹配: %v", err)
	}
}
