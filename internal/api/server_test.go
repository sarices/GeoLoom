package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeProvider struct{}

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

func TestServerEndpointsShouldReturn200(t *testing.T) {
	t.Parallel()
	server := NewServer(fakeProvider{}, "", "")
	handler := server.Handler()
	for _, path := range []string{"/api/v1/status", "/api/v1/sources", "/api/v1/nodes", "/api/v1/candidates", "/api/v1/health"} {
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
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
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
