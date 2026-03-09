package api

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeProvider struct{}

type nullHealthProvider struct{ fakeProvider }

func (fakeProvider) StatusPayload() any { return map[string]any{"ok": true, "raw_node_count": 2} }
func (fakeProvider) SourcesPayload() any {
	return map[string]any{"items": []map[string]any{{"name": "s1", "unsupported_count": 1}}}
}
func (fakeProvider) NodesPayload() any { return map[string]any{"items": []string{"n1"}, "count": 1} }
func (fakeProvider) CandidatesPayload() any {
	return map[string]any{"items": []string{"c1"}, "count": 1}
}
func (fakeProvider) HealthPayload() any {
	return map[string]any{"summary": map[string]any{"penalized_nodes": 1}, "health": true}
}
func (fakeProvider) LogsPayload() any {
	return map[string]any{"items": []map[string]any{{"message": "启动完成", "level": "INFO"}}, "count": 1, "capacity": 300, "truncated": false}
}

func (nullHealthProvider) HealthPayload() any {
	return map[string]any{
		"config":  map[string]any{"enabled": true, "interval": "30s", "url": "https://example.com"},
		"summary": map[string]any{"tracked_nodes": 0, "penalized_nodes": 0, "last_rebuild_at": ""},
		"health": map[string]any{
			"interval":        1,
			"debounce":        2,
			"test_url":        "https://example.com",
			"timeout":         3,
			"last_candidates": nil,
			"last_rebuild_at": "",
			"nodes":           nil,
		},
		"penalty_pool": nil,
	}
}

func TestServerEndpointsShouldReturn200(t *testing.T) {
	t.Parallel()
	server := NewServer(fakeProvider{}, "", "")
	handler := server.Handler()
	for _, path := range []string{"/api/v1/status", "/api/v1/sources", "/api/v1/nodes", "/api/v1/candidates", "/api/v1/health", "/api/v1/logs"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("状态码错误: path=%s got=%d", path, resp.Code)
		}
		if contentType := resp.Header().Get("Content-Type"); contentType != "application/json; charset=utf-8" {
			t.Fatalf("Content-Type 错误: path=%s got=%s", path, contentType)
		}
	}
}

func TestStatusPayloadShape(t *testing.T) {
	t.Parallel()
	server := NewServer(fakeProvider{}, "", "")
	resp := httptest.NewRecorder()
	server.Handler().ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/v1/status", nil))
	var body map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析 JSON 失败: %v", err)
	}
	if body["raw_node_count"].(float64) != 2 {
		t.Fatalf("status 字段错误: %+v", body)
	}
}

func TestSourcesPayloadShape(t *testing.T) {
	t.Parallel()
	server := NewServer(fakeProvider{}, "", "")
	resp := httptest.NewRecorder()
	server.Handler().ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/v1/sources", nil))
	var body map[string][]map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析 JSON 失败: %v", err)
	}
	if len(body["items"]) != 1 || body["items"][0]["unsupported_count"].(float64) != 1 {
		t.Fatalf("sources 字段错误: %+v", body)
	}
}

func TestLogsPayloadShape(t *testing.T) {
	t.Parallel()
	server := NewServer(fakeProvider{}, "", "")
	resp := httptest.NewRecorder()
	server.Handler().ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/v1/logs", nil))
	var body map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析 JSON 失败: %v", err)
	}
	if body["count"].(float64) != 1 {
		t.Fatalf("logs count 字段错误: %+v", body)
	}
	items, ok := body["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("logs items 字段错误: %+v", body)
	}
}

func TestServerAuthShouldRequireConfiguredHeader(t *testing.T) {
	t.Parallel()

	server := NewServer(fakeProvider{}, "X-GeoLoom-Token", "secret-token")
	handler := server.Handler()
	tests := []struct {
		name        string
		headerValue string
		wantCode    int
	}{
		{name: "缺少 header", wantCode: http.StatusUnauthorized},
		{name: "错误 token", headerValue: "wrong-token", wantCode: http.StatusUnauthorized},
		{name: "正确 token", headerValue: "secret-token", wantCode: http.StatusOK},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
			if tt.headerValue != "" {
				req.Header.Set("X-GeoLoom-Token", tt.headerValue)
			}
			resp := httptest.NewRecorder()
			handler.ServeHTTP(resp, req)
			if resp.Code != tt.wantCode {
				t.Fatalf("状态码错误: got=%d want=%d", resp.Code, tt.wantCode)
			}
			if contentType := resp.Header().Get("Content-Type"); contentType != "application/json; charset=utf-8" {
				t.Fatalf("Content-Type 错误: got=%s", contentType)
			}
		})
	}
}

func TestServerUnauthorizedPayloadShape(t *testing.T) {
	t.Parallel()

	server := NewServer(fakeProvider{}, "X-GeoLoom-Token", "secret-token")
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs", nil)
	server.Handler().ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("状态码错误: got=%d want=%d", resp.Code, http.StatusUnauthorized)
	}

	var body map[string]string
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析 JSON 失败: %v", err)
	}
	if body["error"] != "unauthorized" {
		t.Fatalf("错误响应体不符合预期: %+v", body)
	}
}

func TestHealthPayloadShouldAllowNullCollections(t *testing.T) {
	t.Parallel()

	server := NewServer(nullHealthProvider{}, "", "")
	resp := httptest.NewRecorder()
	server.Handler().ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("状态码错误: got=%d", resp.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析 JSON 失败: %v", err)
	}

	healthBody, ok := body["health"].(map[string]any)
	if !ok {
		t.Fatalf("health 字段类型错误: %+v", body)
	}
	if _, exists := healthBody["last_candidates"]; !exists {
		t.Fatalf("health.last_candidates 缺失: %+v", healthBody)
	}
	if value, exists := healthBody["last_candidates"]; !exists || value != nil {
		t.Fatalf("health.last_candidates 应允许为 null: %+v", healthBody)
	}
	if value, exists := healthBody["nodes"]; !exists || value != nil {
		t.Fatalf("health.nodes 应允许为 null: %+v", healthBody)
	}
	if value, exists := body["penalty_pool"]; !exists || value != nil {
		t.Fatalf("penalty_pool 应允许为 null: %+v", body)
	}
}

func TestStaticRouteShouldServeIndexHTML(t *testing.T) {
	t.Parallel()
	server := NewServer(fakeProvider{}, "", "")
	resp := httptest.NewRecorder()
	server.Handler().ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("状态码错误: got=%d", resp.Code)
	}
	if contentType := resp.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
		t.Fatalf("Content-Type 错误: got=%s", contentType)
	}
	if cacheControl := resp.Header().Get("Cache-Control"); cacheControl != "no-cache" {
		t.Fatalf("Cache-Control 错误: got=%s", cacheControl)
	}
}

func TestStaticRouteShouldFallbackForSPAPath(t *testing.T) {
	t.Parallel()
	server := NewServer(fakeProvider{}, "", "")
	resp := httptest.NewRecorder()
	server.Handler().ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/dashboard/health", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("状态码错误: got=%d", resp.Code)
	}
	if contentType := resp.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
		t.Fatalf("Content-Type 错误: got=%s", contentType)
	}
}

func TestAPIUnknownPathShouldNotFallbackToSPA(t *testing.T) {
	t.Parallel()
	server := NewServer(fakeProvider{}, "", "")
	resp := httptest.NewRecorder()
	server.Handler().ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/v1/unknown", nil))
	if resp.Code != http.StatusNotFound {
		t.Fatalf("状态码错误: got=%d want=%d", resp.Code, http.StatusNotFound)
	}
	if contentType := resp.Header().Get("Content-Type"); contentType != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type 错误: got=%s", contentType)
	}
	var body map[string]string
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析 JSON 失败: %v", err)
	}
	if body["error"] != "not found" {
		t.Fatalf("响应体错误: %+v", body)
	}
}

func TestStaticAssetShouldUseImmutableCacheWhenPresent(t *testing.T) {
	t.Parallel()
	assets, err := fs.Glob(frontendDist, "frontenddist/assets/*")
	if err != nil {
		t.Fatalf("枚举 assets 失败: %v", err)
	}
	if len(assets) == 0 {
		t.Fatal("未找到可用前端 assets")
	}
	assetPath := strings.TrimPrefix(assets[0], "frontenddist")
	server := NewServer(fakeProvider{}, "", "")
	resp := httptest.NewRecorder()
	server.Handler().ServeHTTP(resp, httptest.NewRequest(http.MethodGet, assetPath, nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("状态码错误: got=%d path=%s", resp.Code, assetPath)
	}
	if resp.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Fatalf("Cache-Control 错误: got=%s", resp.Header().Get("Cache-Control"))
	}
}
