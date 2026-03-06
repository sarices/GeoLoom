package parser

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"geoloom/internal/domain"
)

// InputType 表示输入归类结果。
type InputType string

const (
	InputTypeSource InputType = "source"
	InputTypeNode   InputType = "node"
)

// DispatchResult 表示输入分发与解析后的输出。
type DispatchResult struct {
	Type        InputType
	SourceURL   string
	Nodes       []domain.NodeMetadata
	RawEntries  []string
	Unsupported []string
}

// SubscriptionFetcher 抽象订阅拉取能力，便于测试注入。
type SubscriptionFetcher interface {
	Fetch(ctx context.Context, sourceURL string) ([]string, error)
}

// Dispatcher 根据 scheme 分发 source/node 解析流程。
type Dispatcher struct {
	fetcher SubscriptionFetcher
}

func NewDispatcher(fetcher SubscriptionFetcher) *Dispatcher {
	return &Dispatcher{fetcher: fetcher}
}

// Parse 对单条输入做分发并解析。
func (d *Dispatcher) Parse(ctx context.Context, rawInput string) (DispatchResult, error) {
	cleaned := strings.TrimSpace(rawInput)
	if cleaned == "" {
		return DispatchResult{}, newParseError(ErrorKindInvalidInput, rawInput, "输入不能为空", nil)
	}

	inputType, scheme, err := DetectInputType(cleaned)
	if err != nil {
		return DispatchResult{}, err
	}

	switch inputType {
	case InputTypeNode:
		node, nodeErr := d.parseNodeByScheme(scheme, cleaned)
		if nodeErr != nil {
			return DispatchResult{}, nodeErr
		}
		return DispatchResult{
			Type:  InputTypeNode,
			Nodes: []domain.NodeMetadata{node},
		}, nil
	case InputTypeSource:
		result := DispatchResult{
			Type:      InputTypeSource,
			SourceURL: cleaned,
		}
		if d.fetcher == nil {
			return result, newParseError(ErrorKindSourceFetcherMissing, cleaned, "未配置订阅抓取器", nil)
		}

		entries, fetchErr := d.fetcher.Fetch(ctx, cleaned)
		if fetchErr != nil {
			return result, newParseError(ErrorKindSourceFetchFailed, cleaned, "订阅拉取失败", fetchErr)
		}
		result.RawEntries = entries
		if len(entries) == 0 {
			return result, newParseError(ErrorKindSourceContentEmpty, cleaned, "订阅内容为空", nil)
		}

		for _, entry := range entries {
			entryType, entryScheme, detectErr := DetectInputType(entry)
			if detectErr != nil || entryType != InputTypeNode {
				result.Unsupported = append(result.Unsupported, entry)
				continue
			}

			node, nodeErr := d.parseNodeByScheme(entryScheme, entry)
			if nodeErr != nil {
				result.Unsupported = append(result.Unsupported, entry)
				continue
			}
			result.Nodes = append(result.Nodes, node)
		}

		if len(result.Nodes) == 0 {
			return result, newParseError(ErrorKindSourceNoSupportedNode, cleaned, "订阅中没有可用节点", nil)
		}

		return result, nil
	default:
		return DispatchResult{}, newParseError(ErrorKindUnsupportedScheme, cleaned, fmt.Sprintf("未知输入类型: %s", inputType), nil)
	}
}

// DetectInputType 识别输入是订阅 source 还是节点链接。
func DetectInputType(rawInput string) (InputType, string, error) {
	cleaned := strings.TrimSpace(rawInput)
	if cleaned == "" {
		return "", "", newParseError(ErrorKindInvalidInput, rawInput, "输入不能为空", nil)
	}

	if strings.HasPrefix(cleaned, "@") {
		return InputTypeSource, "file", nil
	}

	u, err := url.Parse(cleaned)
	if err != nil {
		return "", "", newParseError(ErrorKindInvalidInput, rawInput, "URL 解析失败", err)
	}

	scheme := strings.ToLower(strings.TrimSpace(u.Scheme))
	if scheme == "" {
		return "", "", newParseError(ErrorKindInvalidInput, rawInput, "缺少 scheme", nil)
	}

	switch scheme {
	case "http", "https":
		return InputTypeSource, scheme, nil
	case "hysteria2", "hy2", "socks5", "vless", "trojan", "vmess", "ss":
		return InputTypeNode, scheme, nil
	default:
		return "", scheme, newParseError(ErrorKindUnsupportedScheme, rawInput, "不支持的 scheme", nil)
	}
}

func (d *Dispatcher) parseNodeByScheme(scheme, rawInput string) (domain.NodeMetadata, error) {
	switch scheme {
	case "hysteria2", "hy2":
		return ParseHysteria2(rawInput)
	case "socks5":
		return ParseSocks5(rawInput)
	case "vless":
		return ParseVLESS(rawInput)
	case "trojan":
		return ParseTrojan(rawInput)
	case "vmess":
		return ParseVMess(rawInput)
	case "ss":
		return ParseShadowsocks(rawInput)
	default:
		return domain.NodeMetadata{}, newParseError(ErrorKindUnsupportedScheme, rawInput, "节点协议不支持", nil)
	}
}
