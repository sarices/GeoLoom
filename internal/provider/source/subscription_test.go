package source

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestParseEntriesFromContentPlainText(t *testing.T) {
	t.Parallel()

	content := []byte("\nhysteria2://a@1.1.1.1:443#a\nsocks5://u:p@2.2.2.2:1080\n")
	entries := ParseEntriesFromContent(content)

	want := []string{"hysteria2://a@1.1.1.1:443#a", "socks5://u:p@2.2.2.2:1080"}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("解析结果不匹配: got=%v want=%v", entries, want)
	}
}

func TestParseEntriesFromContentKeepsUnsupportedScheme(t *testing.T) {
	t.Parallel()

	content := []byte("hysteria2://a@1.1.1.1:443#a\nftp://unsupported\n")
	entries := ParseEntriesFromContent(content)

	want := []string{"hysteria2://a@1.1.1.1:443#a", "ftp://unsupported"}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("解析结果不匹配: got=%v want=%v", entries, want)
	}
}

func TestParseEntriesFromContentBase64(t *testing.T) {
	t.Parallel()

	raw := "hysteria2://a@1.1.1.1:443#a\nvless://11111111-1111-1111-1111-111111111111@example.com:443#v"
	encoded := base64.StdEncoding.EncodeToString([]byte(raw))
	entries := ParseEntriesFromContent([]byte(encoded))

	want := []string{"hysteria2://a@1.1.1.1:443#a", "vless://11111111-1111-1111-1111-111111111111@example.com:443#v"}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("解析结果不匹配: got=%v want=%v", entries, want)
	}
}

func TestFetchSubscription(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hysteria2://a@1.1.1.1:443#a\n# comment\nsocks5://u:p@2.2.2.2:1080"))
	}))
	defer ts.Close()

	fetcher := NewSubscriptionFetcher(&http.Client{Timeout: 3 * time.Second})
	entries, err := fetcher.Fetch(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("Fetch 返回错误: %v", err)
	}

	want := []string{"hysteria2://a@1.1.1.1:443#a", "socks5://u:p@2.2.2.2:1080"}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("Fetch 结果不匹配: got=%v want=%v", entries, want)
	}
}

func TestFetchSubscriptionWithTokenQuery(t *testing.T) {
	t.Parallel()

	const wantToken = "abc123"
	tokenCh := make(chan string, 1)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case tokenCh <- r.URL.Query().Get("token"):
		default:
		}
		_, _ = w.Write([]byte("socks5://u:p@2.2.2.2:1080"))
	}))
	defer ts.Close()

	fetcher := NewSubscriptionFetcher(&http.Client{Timeout: 3 * time.Second})
	entries, err := fetcher.Fetch(context.Background(), ts.URL+"/api/v1/client/subscribe?token="+wantToken)
	if err != nil {
		t.Fatalf("Fetch 返回错误: %v", err)
	}

	if !reflect.DeepEqual(entries, []string{"socks5://u:p@2.2.2.2:1080"}) {
		t.Fatalf("Fetch 结果不匹配: got=%v", entries)
	}

	select {
	case gotToken := <-tokenCh:
		if gotToken != wantToken {
			t.Fatalf("token query 未透传: got=%q want=%q", gotToken, wantToken)
		}
	default:
		t.Fatal("未收到订阅请求")
	}
}

func TestFetchSubscriptionFromLocalFileWithDefaultSocks5(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file := filepath.Join(dir, "sub.txt")
	content := "\n# comment\n149.130.191.255:6161\nhysteria2://a@1.1.1.1:443#a\n149.130.191.255:6161\n"
	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		t.Fatalf("写入本地输入文件失败: %v", err)
	}

	fetcher := NewSubscriptionFetcher(nil)
	entries, err := fetcher.Fetch(context.Background(), "@"+file)
	if err != nil {
		t.Fatalf("Fetch 返回错误: %v", err)
	}

	want := []string{"socks5://149.130.191.255:6161", "hysteria2://a@1.1.1.1:443#a"}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("Fetch 结果不匹配: got=%v want=%v", entries, want)
	}
}

func TestFetchSubscriptionRemoteTextFallbackToLineParsing(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("\n# comment\n1.1.1.1:1080#hk\nsocks4://legacy@1.1.1.2:1080#legacy\nhttp://user:pass@1.1.1.3:8080#web\n1.1.1.1:1080#hk\n"))
	}))
	defer ts.Close()

	fetcher := NewSubscriptionFetcher(&http.Client{Timeout: 3 * time.Second})
	result, err := fetcher.FetchResult(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("FetchResult 返回错误: %v", err)
	}

	want := []string{"socks5://1.1.1.1:1080#hk", "socks4://legacy@1.1.1.2:1080#legacy", "http://user:pass@1.1.1.3:8080#web"}
	if !reflect.DeepEqual(result.Entries, want) {
		t.Fatalf("远程文本 fallback 结果不匹配: got=%v want=%v", result.Entries, want)
	}
}

func TestFetchSubscriptionRemoteBase64KeepsOriginalExtractionPriority(t *testing.T) {
	t.Parallel()

	raw := "socks5://guest:pass@2.2.2.2:1080#s1\nhttp://user:pass@3.3.3.3:8080#web"
	encoded := base64.StdEncoding.EncodeToString([]byte(raw))
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(encoded))
	}))
	defer ts.Close()

	fetcher := NewSubscriptionFetcher(&http.Client{Timeout: 3 * time.Second})
	result, err := fetcher.FetchResult(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("FetchResult 返回错误: %v", err)
	}

	want := []string{"socks5://guest:pass@2.2.2.2:1080#s1", "http://user:pass@3.3.3.3:8080#web"}
	if !reflect.DeepEqual(result.Entries, want) {
		t.Fatalf("Base64 提取结果不匹配: got=%v want=%v", result.Entries, want)
	}
}

func TestFetchSubscriptionRemoteTextOnlyCommentsShouldStayEmpty(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("\n# comment\n   \n# comment 2\n"))
	}))
	defer ts.Close()

	fetcher := NewSubscriptionFetcher(&http.Client{Timeout: 3 * time.Second})
	result, err := fetcher.FetchResult(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("FetchResult 返回错误: %v", err)
	}
	if len(result.Entries) != 0 {
		t.Fatalf("仅注释远程文本应保持空 entries: got=%v", result.Entries)
	}
}

func TestFetchSubscriptionRemoteProxyListFixture(t *testing.T) {
	t.Parallel()

	fixturePath := filepath.Join("..", "..", "..", "test", "fixtures", "remote-proxy-list.txt")
	content, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("读取 fixture 失败: %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(content)
	}))
	defer ts.Close()

	fetcher := NewSubscriptionFetcher(&http.Client{Timeout: 3 * time.Second})
	result, err := fetcher.FetchResult(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("FetchResult 返回错误: %v", err)
	}

	want := []string{
		"socks5://72.49.49.11:31034",
		"socks5://66.42.224.229:41679",
		"socks4://legacy@192.111.139.163:4145#legacy-us",
		"http://user:pass@8.210.17.35:3128#http-sg",
		"http://198.199.86.11:8080#http-open",
		"socks5://guest:pass@98.178.72.21:10919#socks5-auth",
		"socks5://user2:pass2@203.0.113.10:1080#bare-auth",
	}
	if !reflect.DeepEqual(result.Entries, want) {
		t.Fatalf("proxifly-like fixture 解析结果不匹配: got=%v want=%v", result.Entries, want)
	}
}

func TestFetchSubscriptionRemoteProxyListDirtyFixture(t *testing.T) {
	t.Parallel()

	fixturePath := filepath.Join("..", "..", "..", "test", "fixtures", "remote-proxy-list-dirty.txt")
	content, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("读取 dirty fixture 失败: %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(content)
	}))
	defer ts.Close()

	fetcher := NewSubscriptionFetcher(&http.Client{Timeout: 3 * time.Second})
	result, err := fetcher.FetchResult(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("FetchResult 返回错误: %v", err)
	}

	want := []string{
		"SOCKS5://9.9.9.9:1080#upper-socks",
		"http://1.1.1.1:8080#dup-a",
		"http://1.1.1.1:8080#dup-b",
		"ftp://unsupported.example:21#ftp",
		"socks4://legacy@2.2.2.2:99999#bad-port",
		"http://user%zz@3.3.3.3:8080#bad-uri",
		"socks5://user:pass@4.4.4.4:1080#bare-auth",
		"HTTPS://should-stay-unsupported.example.com/path",
	}
	if !reflect.DeepEqual(result.Entries, want) {
		t.Fatalf("dirty fixture 解析结果不匹配: got=%v want=%v", result.Entries, want)
	}
}

func TestFetchSubscriptionStatusError(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	fetcher := NewSubscriptionFetcher(nil)
	_, err := fetcher.Fetch(context.Background(), ts.URL)
	if err == nil {
		t.Fatal("预期返回错误，但得到 nil")
	}
}
