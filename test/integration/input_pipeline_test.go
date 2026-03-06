package integration_test

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
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
