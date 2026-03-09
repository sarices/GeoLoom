package parser

import (
	"context"
	"encoding/base64"
	"errors"
	"reflect"
	"testing"
)

type fakeFetcher struct {
	entries []string
	err     error
}

func (f fakeFetcher) Fetch(_ context.Context, _ string) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.entries, nil
}

func TestDetectInputType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      string
		wantType   InputType
		wantScheme string
		wantErr    ErrorKind
	}{
		{name: "source-https", input: "https://example.com/sub", wantType: InputTypeSource, wantScheme: "https"},
		{name: "source-file", input: "@docs/sub.txt", wantType: InputTypeSource, wantScheme: "file"},
		{name: "source-https-with-token", input: "https://sub.example.com/api/v1/client/subscribe?token=abc123", wantType: InputTypeSource, wantScheme: "https"},
		{name: "node-hy2", input: "hysteria2://pwd@1.1.1.1:443", wantType: InputTypeNode, wantScheme: "hysteria2"},
		{name: "node-socks5", input: "socks5://u:p@1.1.1.1:1080", wantType: InputTypeNode, wantScheme: "socks5"},
		{name: "node-socks4", input: "socks4://legacy@1.1.1.1:1080", wantType: InputTypeNode, wantScheme: "socks4"},
		{name: "node-trojan", input: "trojan://pwd@1.1.1.1:443", wantType: InputTypeNode, wantScheme: "trojan"},
		{name: "node-vmess", input: "vmess://eyJhZGQiOiIxLjEuMS4xIiwicG9ydCI6IjQ0MyIsImlkIjoiMTExMTExMTEtMTExMS0xMTExLTExMTEtMTExMTExMTExMTExIiwiYWlkIjoiMCJ9", wantType: InputTypeNode, wantScheme: "vmess"},
		{name: "node-ss", input: "ss://YWVzLTEyOC1nY206cGFzc0AxLjEuMS4xOjgzODg=", wantType: InputTypeNode, wantScheme: "ss"},
		{name: "unsupported", input: "ftp://example.com/file", wantErr: ErrorKindUnsupportedScheme},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotType, gotScheme, err := DetectInputType(tc.input)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatal("预期返回错误，但得到 nil")
				}
				if !IsErrorKind(err, tc.wantErr) {
					t.Fatalf("错误类型不匹配: got=%v want=%s", err, tc.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("DetectInputType 返回错误: %v", err)
			}
			if gotType != tc.wantType || gotScheme != tc.wantScheme {
				t.Fatalf("识别结果不匹配: got=(%s,%s) want=(%s,%s)", gotType, gotScheme, tc.wantType, tc.wantScheme)
			}
		})
	}
}

func TestDispatcherParseNode(t *testing.T) {
	t.Parallel()

	d := NewDispatcher(nil)
	result, err := d.Parse(context.Background(), "socks5://guest:pass12345@149.130.191.255:6161")
	if err != nil {
		t.Fatalf("Parse 返回错误: %v", err)
	}

	if result.Type != InputTypeNode {
		t.Fatalf("输入类型错误: got=%s", result.Type)
	}
	if len(result.Nodes) != 1 {
		t.Fatalf("节点数量错误: got=%d", len(result.Nodes))
	}
	if result.Nodes[0].Protocol != "socks5" {
		t.Fatalf("协议错误: got=%s", result.Nodes[0].Protocol)
	}
}

func TestDispatcherParseSource(t *testing.T) {
	t.Parallel()

	vmessPayload := `{"v":"2","ps":"vmess-test","add":"vmess.example.com","port":"443","id":"11111111-1111-1111-1111-111111111111","aid":"0","scy":"auto"}`
	vmessLink := "vmess://" + base64.StdEncoding.EncodeToString([]byte(vmessPayload))

	fetcher := fakeFetcher{entries: []string{
		"hysteria2://secret@156.246.91.11:2000?security=tls&sni=www.bing.com#hy2-jp",
		"socks5://guest:pass12345@149.130.191.255:6161",
		"socks4://legacy@149.130.191.254:1080#legacy",
		"http://guest:pass12345@149.130.191.253:8080#http-proxy",
		"vless://11111111-1111-1111-1111-111111111111@example.com:443?encryption=none#vless-test",
		"trojan://secret@trojan.example.com:443?security=tls&sni=trojan.example.com#trojan-test",
		vmessLink,
		"ss://YWVzLTEyOC1nY206c2VjcmV0QHNzLmV4YW1wbGUuY29tOjgzODg=#ss-test",
		"ftp://unsupported-entry",
	}}
	d := NewDispatcher(fetcher)

	result, err := d.Parse(context.Background(), "https://example.com/sub")
	if err != nil {
		t.Fatalf("Parse 返回错误: %v", err)
	}

	if result.Type != InputTypeSource {
		t.Fatalf("输入类型错误: got=%s", result.Type)
	}
	if len(result.Nodes) != 8 {
		t.Fatalf("节点数量错误: got=%d want=8", len(result.Nodes))
	}
	if len(result.Unsupported) != 1 {
		t.Fatalf("不支持条目数量错误: got=%d want=1", len(result.Unsupported))
	}

	gotProtocols := []string{
		result.Nodes[0].Protocol,
		result.Nodes[1].Protocol,
		result.Nodes[2].Protocol,
		result.Nodes[3].Protocol,
		result.Nodes[4].Protocol,
		result.Nodes[5].Protocol,
		result.Nodes[6].Protocol,
		result.Nodes[7].Protocol,
	}
	wantProtocols := []string{"hysteria2", "socks5", "socks4", "http", "vless", "trojan", "vmess", "shadowsocks"}
	if !reflect.DeepEqual(gotProtocols, wantProtocols) {
		t.Fatalf("协议序列不匹配: got=%v want=%v", gotProtocols, wantProtocols)
	}
}

func TestDispatcherParseSourceDirtyEntries(t *testing.T) {
	t.Parallel()

	fetcher := fakeFetcher{entries: []string{
		"SOCKS5://9.9.9.9:1080#upper-socks",
		"http://1.1.1.1:8080#dup-a",
		"http://1.1.1.1:8080#dup-b",
		"ftp://unsupported.example:21#ftp",
		"socks4://legacy@2.2.2.2:99999#bad-port",
		"http://user%zz@3.3.3.3:8080#bad-uri",
		"socks5://user:pass@4.4.4.4:1080#bare-auth",
		"HTTPS://should-stay-unsupported.example.com/path",
	}}
	d := NewDispatcher(fetcher)

	result, err := d.Parse(context.Background(), "https://example.com/sub")
	if err != nil {
		t.Fatalf("Parse 返回错误: %v", err)
	}

	if result.Type != InputTypeSource {
		t.Fatalf("输入类型错误: got=%s", result.Type)
	}
	if len(result.Nodes) != 4 {
		t.Fatalf("节点数量错误: got=%d want=4", len(result.Nodes))
	}
	if len(result.Unsupported) != 4 {
		t.Fatalf("不支持条目数量错误: got=%d want=4 unsupported=%v", len(result.Unsupported), result.Unsupported)
	}

	gotProtocols := []string{
		result.Nodes[0].Protocol,
		result.Nodes[1].Protocol,
		result.Nodes[2].Protocol,
		result.Nodes[3].Protocol,
	}
	wantProtocols := []string{"socks5", "http", "http", "socks5"}
	if !reflect.DeepEqual(gotProtocols, wantProtocols) {
		t.Fatalf("协议序列不匹配: got=%v want=%v", gotProtocols, wantProtocols)
	}

	gotNames := []string{
		result.Nodes[0].Name,
		result.Nodes[1].Name,
		result.Nodes[2].Name,
		result.Nodes[3].Name,
	}
	wantNames := []string{"upper-socks", "dup-a", "dup-b", "bare-auth"}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("节点名称序列不匹配: got=%v want=%v", gotNames, wantNames)
	}

	wantUnsupported := []string{
		"ftp://unsupported.example:21#ftp",
		"socks4://legacy@2.2.2.2:99999#bad-port",
		"http://user%zz@3.3.3.3:8080#bad-uri",
		"HTTPS://should-stay-unsupported.example.com/path",
	}
	if !reflect.DeepEqual(result.Unsupported, wantUnsupported) {
		t.Fatalf("unsupported 不匹配: got=%v want=%v", result.Unsupported, wantUnsupported)
	}
}

func TestDispatcherSourceFetcherErrors(t *testing.T) {
	t.Parallel()

	d := NewDispatcher(nil)
	_, err := d.Parse(context.Background(), "https://example.com/sub")
	if err == nil || !IsErrorKind(err, ErrorKindSourceFetcherMissing) {
		t.Fatalf("预期 source_fetcher_missing 错误，got=%v", err)
	}

	d = NewDispatcher(fakeFetcher{err: errors.New("network down")})
	_, err = d.Parse(context.Background(), "https://example.com/sub")
	if err == nil || !IsErrorKind(err, ErrorKindSourceFetchFailed) {
		t.Fatalf("预期 source_fetch_failed 错误，got=%v", err)
	}

	d = NewDispatcher(fakeFetcher{entries: nil})
	_, err = d.Parse(context.Background(), "https://example.com/sub")
	if err == nil || !IsErrorKind(err, ErrorKindSourceContentEmpty) {
		t.Fatalf("预期 source_content_empty 错误，got=%v", err)
	}

	d = NewDispatcher(fakeFetcher{entries: []string{"ftp://unsupported"}})
	_, err = d.Parse(context.Background(), "https://example.com/sub")
	if err == nil || !IsErrorKind(err, ErrorKindSourceNoSupportedNode) {
		t.Fatalf("预期 source_no_supported_node 错误，got=%v", err)
	}
}
