package parser

import (
	"errors"
	"testing"
)

func TestParseHysteria2(t *testing.T) {
	t.Parallel()

	raw := "hysteria2://secret@156.246.91.11:2000?security=tls&alpn=h3&insecure=1&sni=www.bing.com#hy2-jp"
	node, err := ParseHysteria2(raw)
	if err != nil {
		t.Fatalf("ParseHysteria2 返回错误: %v", err)
	}

	if node.Protocol != "hysteria2" {
		t.Fatalf("协议错误: got=%s", node.Protocol)
	}
	if node.Address != "156.246.91.11" || node.Port != 2000 {
		t.Fatalf("地址端口错误: got=%s:%d", node.Address, node.Port)
	}
	if node.Name != "hy2-jp" {
		t.Fatalf("节点名称错误: got=%s", node.Name)
	}
	if node.RawConfig["password"] != "secret" {
		t.Fatalf("密码字段错误: got=%v", node.RawConfig["password"])
	}
}

func TestParseHysteria2MissingPassword(t *testing.T) {
	t.Parallel()

	_, err := ParseHysteria2("hysteria2://@1.1.1.1:443")
	if err == nil {
		t.Fatal("预期返回错误，但得到 nil")
	}
	if !IsErrorKind(err, ErrorKindMissingField) {
		t.Fatalf("错误类型不匹配: %v", err)
	}

	var parseErr *ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("错误类型断言失败: %T", err)
	}
}
