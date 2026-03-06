package parser

import (
	"encoding/base64"
	"testing"
)

func TestParseShadowsocksWithUserInfo(t *testing.T) {
	t.Parallel()

	raw := "ss://aes-128-gcm:secret@example.com:8388#ss-test"
	node, err := ParseShadowsocks(raw)
	if err != nil {
		t.Fatalf("ParseShadowsocks 返回错误: %v", err)
	}

	if node.Protocol != "shadowsocks" {
		t.Fatalf("协议错误: got=%s", node.Protocol)
	}
	if node.Address != "example.com" || node.Port != 8388 {
		t.Fatalf("地址端口错误: got=%s:%d", node.Address, node.Port)
	}
	if node.RawConfig["method"] != "aes-128-gcm" {
		t.Fatalf("method 错误: got=%v", node.RawConfig["method"])
	}
}

func TestParseShadowsocksWithBase64Host(t *testing.T) {
	t.Parallel()

	encoded := base64.RawStdEncoding.EncodeToString([]byte("aes-256-gcm:secret@example.com:8388"))
	raw := "ss://" + encoded + "#ss-b64"

	node, err := ParseShadowsocks(raw)
	if err != nil {
		t.Fatalf("ParseShadowsocks 返回错误: %v", err)
	}
	if node.Address != "example.com" || node.Port != 8388 {
		t.Fatalf("地址端口错误: got=%s:%d", node.Address, node.Port)
	}
}

func TestParseShadowsocksInvalid(t *testing.T) {
	t.Parallel()

	_, err := ParseShadowsocks("ss://invalid")
	if err == nil {
		t.Fatal("预期返回错误，但得到 nil")
	}
	if !IsErrorKind(err, ErrorKindInvalidInput) {
		t.Fatalf("错误类型不匹配: %v", err)
	}
}
