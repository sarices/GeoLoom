package parser

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

func buildNodeID(protocol, address string, port int, seed string) string {
	raw := fmt.Sprintf("%s|%s|%d|%s", protocol, address, port, seed)
	sum := sha1.Sum([]byte(raw))
	token := hex.EncodeToString(sum[:])[:12]
	return fmt.Sprintf("%s-%s", protocol, token)
}

func normalizeNodeName(fragment, fallback string) string {
	fragment = strings.TrimSpace(fragment)
	if fragment == "" {
		return fallback
	}

	decoded, err := url.QueryUnescape(fragment)
	if err != nil {
		decoded = fragment
	}
	decoded = strings.TrimSpace(decoded)
	if decoded == "" {
		return fallback
	}
	return decoded
}

func parseHostAndPort(u *url.URL, rawInput string) (string, int, error) {
	host := strings.TrimSpace(u.Hostname())
	if host == "" {
		return "", 0, newParseError(ErrorKindMissingField, rawInput, "缺少主机地址", nil)
	}

	portText := strings.TrimSpace(u.Port())
	if portText == "" {
		return "", 0, newParseError(ErrorKindMissingField, rawInput, "缺少端口", nil)
	}

	port, err := strconv.Atoi(portText)
	if err != nil || port <= 0 || port > 65535 {
		return "", 0, newParseError(ErrorKindInvalidInput, rawInput, "端口非法", err)
	}

	return host, port, nil
}

func splitCommaValues(raw string) []string {
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		normalized := strings.TrimSpace(part)
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
