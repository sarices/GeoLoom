package integration_test

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"geoloom/internal/provider/parser"
	"geoloom/internal/provider/source"
)

func TestInputPipelineFromSubscription(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hysteria2://secret@156.246.91.11:2000?security=tls&sni=www.bing.com#hy2-jp\n" +
			"socks5://guest:pass12345@149.130.191.255:6161\n" +
			"vless://11111111-1111-1111-1111-111111111111@example.com:443?encryption=none#vless-test\n" +
			"ftp://unsupported\n"))
	}))
	defer ts.Close()

	fetcher := source.NewSubscriptionFetcher(&http.Client{Timeout: 3 * time.Second})
	dispatcher := parser.NewDispatcher(fetcher)

	result, err := dispatcher.Parse(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("输入链路解析失败: %v", err)
	}

	if result.Type != parser.InputTypeSource {
		t.Fatalf("输入类型错误: got=%s", result.Type)
	}
	if len(result.Nodes) != 3 {
		t.Fatalf("节点数量错误: got=%d want=3", len(result.Nodes))
	}
	if len(result.Unsupported) != 1 {
		t.Fatalf("不支持条目数量错误: got=%d want=1", len(result.Unsupported))
	}
}

func TestInputPipelineFromRemoteTextFallback(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("# comment\n1.1.1.1:1080#hk\nsocks4://legacy@2.2.2.2:1080#legacy\nhttp://user:pass@3.3.3.3:8080#web\n"))
	}))
	defer ts.Close()

	fetcher := source.NewSubscriptionFetcher(&http.Client{Timeout: 3 * time.Second})
	dispatcher := parser.NewDispatcher(fetcher)

	result, err := dispatcher.Parse(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("远程文本输入链路解析失败: %v", err)
	}

	if result.Type != parser.InputTypeSource {
		t.Fatalf("输入类型错误: got=%s", result.Type)
	}
	if len(result.Nodes) != 3 {
		t.Fatalf("节点数量错误: got=%d want=3", len(result.Nodes))
	}
	gotProtocols := []string{result.Nodes[0].Protocol, result.Nodes[1].Protocol, result.Nodes[2].Protocol}
	wantProtocols := []string{"socks5", "socks4", "http"}
	for i := range wantProtocols {
		if gotProtocols[i] != wantProtocols[i] {
			t.Fatalf("协议顺序错误: got=%v want=%v", gotProtocols, wantProtocols)
		}
	}
}

func TestInputPipelineFromRemoteProxyListFixture(t *testing.T) {
	t.Parallel()

	fixturePath := filepath.Join("..", "..", "test", "fixtures", "remote-proxy-list.txt")
	content, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("读取 fixture 失败: %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(content)
	}))
	defer ts.Close()

	fetcher := source.NewSubscriptionFetcher(&http.Client{Timeout: 3 * time.Second})
	dispatcher := parser.NewDispatcher(fetcher)

	result, err := dispatcher.Parse(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("proxifly-like fixture 输入链路解析失败: %v", err)
	}

	if result.Type != parser.InputTypeSource {
		t.Fatalf("输入类型错误: got=%s", result.Type)
	}
	if len(result.Nodes) != 7 {
		t.Fatalf("节点数量错误: got=%d want=7", len(result.Nodes))
	}
	if len(result.Unsupported) != 0 {
		t.Fatalf("不支持条目数量错误: got=%d want=0", len(result.Unsupported))
	}
	gotProtocols := map[string]int{}
	for _, node := range result.Nodes {
		gotProtocols[node.Protocol]++
	}
	if gotProtocols["socks5"] != 4 || gotProtocols["socks4"] != 1 || gotProtocols["http"] != 2 {
		t.Fatalf("协议分布错误: got=%v", gotProtocols)
	}
}

func TestInputPipelineFromRemoteProxyListDirtyFixture(t *testing.T) {
	t.Parallel()

	fixturePath := filepath.Join("..", "..", "test", "fixtures", "remote-proxy-list-dirty.txt")
	content, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("读取 dirty fixture 失败: %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(content)
	}))
	defer ts.Close()

	fetcher := source.NewSubscriptionFetcher(&http.Client{Timeout: 3 * time.Second})
	dispatcher := parser.NewDispatcher(fetcher)

	result, err := dispatcher.Parse(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("dirty fixture 输入链路解析失败: %v", err)
	}

	if result.Type != parser.InputTypeSource {
		t.Fatalf("输入类型错误: got=%s", result.Type)
	}
	if len(result.Nodes) != 4 {
		t.Fatalf("节点数量错误: got=%d want=4", len(result.Nodes))
	}
	if len(result.Unsupported) != 4 {
		t.Fatalf("不支持条目数量错误: got=%d want=4 unsupported=%v", len(result.Unsupported), result.Unsupported)
	}

	gotProtocols := map[string]int{}
	gotNames := []string{}
	for _, node := range result.Nodes {
		gotProtocols[node.Protocol]++
		gotNames = append(gotNames, node.Name)
	}
	if gotProtocols["socks5"] != 2 || gotProtocols["http"] != 2 {
		t.Fatalf("协议分布错误: got=%v", gotProtocols)
	}
	if gotNames[1] != "dup-a" || gotNames[2] != "dup-b" {
		t.Fatalf("重复但 name 不同的 HTTP 条目应保留: got=%v", gotNames)
	}

	wantUnsupported := []string{
		"ftp://unsupported.example:21#ftp",
		"socks4://legacy@2.2.2.2:99999#bad-port",
		"http://user%zz@3.3.3.3:8080#bad-uri",
		"HTTPS://should-stay-unsupported.example.com/path",
	}
	for i := range wantUnsupported {
		if result.Unsupported[i] != wantUnsupported[i] {
			t.Fatalf("unsupported 顺序错误: got=%v want=%v", result.Unsupported, wantUnsupported)
		}
	}
}

func TestInputPipelineFromBase64Subscription(t *testing.T) {
	t.Parallel()

	raw := "hysteria2://secret@156.246.91.11:2000#hy2\n" +
		"socks5://guest:pass12345@149.130.191.255:6161\n"
	encoded := base64.StdEncoding.EncodeToString([]byte(raw))

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(encoded))
	}))
	defer ts.Close()

	dispatcher := parser.NewDispatcher(source.NewSubscriptionFetcher(nil))
	result, err := dispatcher.Parse(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("base64 输入链路解析失败: %v", err)
	}

	if len(result.Nodes) != 2 {
		t.Fatalf("节点数量错误: got=%d want=2", len(result.Nodes))
	}
}
