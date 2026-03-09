package source

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

// linkPattern 仅负责提取 URI，不在该层做协议白名单过滤。
var linkPattern = regexp.MustCompile(`(?i)(?:[a-z][a-z0-9+.-]*)://[^\s"'<>]+`)

const defaultMaxBodyBytes int64 = 4 * 1024 * 1024

var schemeWithSeparatorPattern = regexp.MustCompile(`(?i)^[a-z][a-z0-9+.-]*://`)

// FetchResult 表示 source 层抓取结果；既保留原始内容，也保留基于 URI 的基础抽取结果。
type FetchResult struct {
	Content []byte
	Entries []string
}

// SubscriptionFetcher 负责订阅内容抓取与基础文本解析。
type SubscriptionFetcher struct {
	client       *http.Client
	maxBodyBytes int64
}

// NewSubscriptionFetcher 创建默认订阅抓取器。
func NewSubscriptionFetcher(client *http.Client) *SubscriptionFetcher {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &SubscriptionFetcher{
		client:       client,
		maxBodyBytes: defaultMaxBodyBytes,
	}
}

// Fetch 拉取订阅并解析为链接列表。
func (f *SubscriptionFetcher) Fetch(ctx context.Context, sourceURL string) ([]string, error) {
	result, err := f.FetchResult(ctx, sourceURL)
	if err != nil {
		return nil, err
	}
	return result.Entries, nil
}

// FetchResult 拉取订阅并返回原始内容与基础 URI 抽取结果。
func (f *SubscriptionFetcher) FetchResult(ctx context.Context, sourceURL string) (FetchResult, error) {
	if f == nil || f.client == nil {
		return FetchResult{}, fmt.Errorf("订阅抓取器未初始化")
	}

	cleaned := strings.TrimSpace(sourceURL)
	if strings.HasPrefix(cleaned, "@") {
		content, err := readLocalFileContent(cleaned)
		if err != nil {
			return FetchResult{}, err
		}
		return FetchResult{Content: content, Entries: parseEntriesFromLines(string(content))}, nil
	}

	parsed, err := url.Parse(cleaned)
	if err != nil {
		return FetchResult{}, fmt.Errorf("订阅地址解析失败: %w", err)
	}
	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	if scheme != "http" && scheme != "https" {
		return FetchResult{}, fmt.Errorf("订阅地址仅支持 http/https 或 @本地文件")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return FetchResult{}, fmt.Errorf("创建订阅请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "GeoLoom/0.1")

	resp, err := f.client.Do(req)
	if err != nil {
		return FetchResult{}, fmt.Errorf("请求订阅失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return FetchResult{}, fmt.Errorf("订阅请求返回异常状态码: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, f.maxBodyBytes))
	if err != nil {
		return FetchResult{}, fmt.Errorf("读取订阅内容失败: %w", err)
	}

	entries := ParseEntriesFromContent(body)
	lineEntries := parseEntriesFromLines(string(body))
	if len(lineEntries) > len(entries) {
		entries = lineEntries
	} else if len(entries) == 0 {
		entries = lineEntries
	}

	return FetchResult{Content: body, Entries: entries}, nil
}

// ParseEntriesFromContent 从订阅文本中提取可处理的链接。
func ParseEntriesFromContent(content []byte) []string {
	text := strings.TrimSpace(string(content))
	if text == "" {
		return nil
	}

	if decoded, ok := decodeBase64IfPossible(text); ok {
		text = decoded
	}

	return extractLinks(text)
}

func decodeBase64IfPossible(text string) (string, bool) {
	compressed := strings.Join(strings.Fields(text), "")
	if compressed == "" || len(compressed)%4 != 0 {
		return "", false
	}

	decoded, err := base64.StdEncoding.DecodeString(compressed)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(compressed)
		if err != nil {
			return "", false
		}
	}

	decoded = bytes.TrimSpace(decoded)
	if len(decoded) == 0 {
		return "", false
	}

	decodedText := string(decoded)
	if !strings.Contains(decodedText, "://") {
		return "", false
	}
	return decodedText, true
}

func readLocalFileContent(sourceURL string) ([]byte, error) {
	filePath := strings.TrimSpace(strings.TrimPrefix(sourceURL, "@"))
	if filePath == "" {
		return nil, fmt.Errorf("本地文件路径不能为空")
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取本地输入文件失败: %w", err)
	}
	return content, nil
}

func parseEntriesFromLocalFile(sourceURL string) ([]string, error) {
	content, err := readLocalFileContent(sourceURL)
	if err != nil {
		return nil, err
	}
	return parseEntriesFromLines(string(content)), nil
}

func parseEntriesFromLines(text string) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}

	lines := strings.Split(text, "\n")
	result := make([]string, 0, len(lines))
	seen := make(map[string]struct{}, len(lines))
	for _, line := range lines {
		normalized := strings.TrimSpace(line)
		if normalized == "" || strings.HasPrefix(normalized, "#") {
			continue
		}
		if !schemeWithSeparatorPattern.MatchString(normalized) {
			normalized = "socks5://" + normalized
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func extractLinks(text string) []string {
	matches := linkPattern.FindAllString(text, -1)
	if len(matches) == 0 {
		return nil
	}

	result := make([]string, 0, len(matches))
	seen := make(map[string]struct{}, len(matches))
	for _, item := range matches {
		normalized := strings.TrimSpace(item)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}
