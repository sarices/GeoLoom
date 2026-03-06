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
