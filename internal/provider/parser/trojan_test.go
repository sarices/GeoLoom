package parser

import "testing"

func TestParseTrojan(t *testing.T) {
	t.Parallel()

	raw := "trojan://secret@example.com:443?security=tls&sni=example.com&type=ws&host=cdn.example.com&path=%2Fws#trojan-test"
	node, err := ParseTrojan(raw)
	if err != nil {
		t.Fatalf("ParseTrojan 返回错误: %v", err)
	}

	if node.Protocol != "trojan" {
		t.Fatalf("协议错误: got=%s", node.Protocol)
	}
	if node.Address != "example.com" || node.Port != 443 {
		t.Fatalf("地址端口错误: got=%s:%d", node.Address, node.Port)
	}
	if node.RawConfig["password"] != "secret" {
		t.Fatalf("密码字段错误: got=%v", node.RawConfig["password"])
	}
	if node.Name != "trojan-test" {
		t.Fatalf("名称错误: got=%s", node.Name)
	}
}

func TestParseTrojanMissingPassword(t *testing.T) {
	t.Parallel()

	_, err := ParseTrojan("trojan://@example.com:443")
	if err == nil {
		t.Fatal("预期返回错误，但得到 nil")
	}
	if !IsErrorKind(err, ErrorKindMissingField) {
		t.Fatalf("错误类型不匹配: %v", err)
	}
}
