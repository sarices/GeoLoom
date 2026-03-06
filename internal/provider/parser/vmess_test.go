package parser

import (
	"encoding/base64"
	"testing"
)

func TestParseVMess(t *testing.T) {
	t.Parallel()

	payload := `{"v":"2","ps":"vmess-test","add":"example.com","port":"443","id":"11111111-1111-1111-1111-111111111111","aid":"0","scy":"auto","net":"ws","host":"cdn.example.com","path":"/ws","tls":"tls","sni":"example.com"}`
	raw := "vmess://" + base64.StdEncoding.EncodeToString([]byte(payload))

	node, err := ParseVMess(raw)
	if err != nil {
		t.Fatalf("ParseVMess 返回错误: %v", err)
	}

	if node.Protocol != "vmess" {
		t.Fatalf("协议错误: got=%s", node.Protocol)
	}
	if node.Address != "example.com" || node.Port != 443 {
		t.Fatalf("地址端口错误: got=%s:%d", node.Address, node.Port)
	}
	if node.RawConfig["uuid"] != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("UUID 错误: got=%v", node.RawConfig["uuid"])
	}
}

func TestParseVMessInvalidPayload(t *testing.T) {
	t.Parallel()

	_, err := ParseVMess("vmess://not-base64")
	if err == nil {
		t.Fatal("预期返回错误，但得到 nil")
	}
	if !IsErrorKind(err, ErrorKindInvalidInput) {
		t.Fatalf("错误类型不匹配: %v", err)
	}
}
