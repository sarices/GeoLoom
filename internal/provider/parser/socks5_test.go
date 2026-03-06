package parser

import "testing"

func TestParseSocks5(t *testing.T) {
	t.Parallel()

	raw := "socks5://guest:pass12345@149.130.191.255:6161"
	node, err := ParseSocks5(raw)
	if err != nil {
		t.Fatalf("ParseSocks5 返回错误: %v", err)
	}

	if node.Protocol != "socks5" {
		t.Fatalf("协议错误: got=%s", node.Protocol)
	}
	if node.Address != "149.130.191.255" || node.Port != 6161 {
		t.Fatalf("地址端口错误: got=%s:%d", node.Address, node.Port)
	}
	if node.RawConfig["username"] != "guest" {
		t.Fatalf("用户名错误: got=%v", node.RawConfig["username"])
	}
	if node.RawConfig["password"] != "pass12345" {
		t.Fatalf("密码错误: got=%v", node.RawConfig["password"])
	}
}

func TestParseSocks5InvalidPort(t *testing.T) {
	t.Parallel()

	_, err := ParseSocks5("socks5://guest:pass@127.0.0.1:0")
	if err == nil {
		t.Fatal("预期返回错误，但得到 nil")
	}
	if !IsErrorKind(err, ErrorKindInvalidInput) {
		t.Fatalf("错误类型不匹配: %v", err)
	}
}
