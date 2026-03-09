package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseClashYAMLFixture(t *testing.T) {
	t.Parallel()
	path := filepath.Join("..", "..", "..", "test", "fixtures", "clash-sub.yaml")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 fixture 失败: %v", err)
	}
	nodes, ok, err := ParseClashYAML(content)
	if err != nil {
		t.Fatalf("ParseClashYAML 返回错误: %v", err)
	}
	if !ok {
		t.Fatal("应识别为 Clash YAML")
	}
	if len(nodes) != 2 {
		t.Fatalf("节点数量错误: got=%d want=2", len(nodes))
	}
	if nodes[0].Protocol != "vmess" || nodes[1].Protocol != "socks5" {
		t.Fatalf("协议映射错误: got=%s,%s", nodes[0].Protocol, nodes[1].Protocol)
	}
}

func TestParseClashYAMLSupportsSocks4AndHTTP(t *testing.T) {
	t.Parallel()

	content := []byte(`proxies:
  - name: legacy
    type: socks4
    server: 1.1.1.1
    port: 1080
    username: user4
  - name: web
    type: http
    server: 2.2.2.2
    port: 8080
    username: user
    password: pass
`)
	nodes, ok, err := ParseClashYAML(content)
	if err != nil {
		t.Fatalf("ParseClashYAML 返回错误: %v", err)
	}
	if !ok {
		t.Fatal("应识别为 Clash YAML")
	}
	if len(nodes) != 2 {
		t.Fatalf("节点数量错误: got=%d want=2", len(nodes))
	}
	if nodes[0].Protocol != "socks4" || nodes[1].Protocol != "http" {
		t.Fatalf("协议映射错误: got=%s,%s", nodes[0].Protocol, nodes[1].Protocol)
	}
}
