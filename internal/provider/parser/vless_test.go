package parser

import "testing"

func TestParseVLESS(t *testing.T) {
	t.Parallel()

	raw := "vless://11111111-1111-1111-1111-111111111111@example.com:443?encryption=none&security=tls&sni=example.com&type=ws&path=%2Fvless#vless-test"
	node, err := ParseVLESS(raw)
	if err != nil {
		t.Fatalf("ParseVLESS 返回错误: %v", err)
	}

	if node.Protocol != "vless" {
		t.Fatalf("协议错误: got=%s", node.Protocol)
	}
	if node.Address != "example.com" || node.Port != 443 {
		t.Fatalf("地址端口错误: got=%s:%d", node.Address, node.Port)
	}
	if node.RawConfig["uuid"] != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("UUID 错误: got=%v", node.RawConfig["uuid"])
	}
	if node.Name != "vless-test" {
		t.Fatalf("名称错误: got=%s", node.Name)
	}
}

func TestParseVLESSMissingUUID(t *testing.T) {
	t.Parallel()

	_, err := ParseVLESS("vless://@example.com:443")
	if err == nil {
		t.Fatal("预期返回错误，但得到 nil")
	}
	if !IsErrorKind(err, ErrorKindMissingField) {
		t.Fatalf("错误类型不匹配: %v", err)
	}
}
