package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSingboxJSONFixture(t *testing.T) {
	t.Parallel()
	path := filepath.Join("..", "..", "..", "test", "fixtures", "singbox-sub.json")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 fixture 失败: %v", err)
	}
	nodes, ok, err := ParseSingboxJSON(content)
	if err != nil {
		t.Fatalf("ParseSingboxJSON 返回错误: %v", err)
	}
	if !ok {
		t.Fatal("应识别为 Sing-box JSON")
	}
	if len(nodes) != 2 {
		t.Fatalf("节点数量错误: got=%d want=2", len(nodes))
	}
	if nodes[0].Protocol != "vless" || nodes[1].Protocol != "socks5" {
		t.Fatalf("协议映射错误: got=%s,%s", nodes[0].Protocol, nodes[1].Protocol)
	}
}

func TestParseSingboxJSONSupportsSocks4AndHTTP(t *testing.T) {
	t.Parallel()

	content := []byte(`{
  "outbounds": [
    {
      "type": "socks",
      "tag": "legacy",
      "server": "1.1.1.1",
      "server_port": 1080,
      "version": "4",
      "username": "user4"
    },
    {
      "type": "http",
      "tag": "web",
      "server": "2.2.2.2",
      "server_port": 8080,
      "username": "user",
      "password": "pass"
    }
  ]
}`)
	nodes, ok, err := ParseSingboxJSON(content)
	if err != nil {
		t.Fatalf("ParseSingboxJSON 返回错误: %v", err)
	}
	if !ok {
		t.Fatal("应识别为 Sing-box JSON")
	}
	if len(nodes) != 2 {
		t.Fatalf("节点数量错误: got=%d want=2", len(nodes))
	}
	if nodes[0].Protocol != "socks4" || nodes[1].Protocol != "http" {
		t.Fatalf("协议映射错误: got=%s,%s", nodes[0].Protocol, nodes[1].Protocol)
	}
}
