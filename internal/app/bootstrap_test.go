package app

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"geoloom/internal/config"
	"geoloom/internal/domain"
	"geoloom/internal/health"
)

func TestApplyGeoWithoutMMDB(t *testing.T) {
	nodes := []domain.NodeMetadata{{ID: "n1", Address: "1.1.1.1"}}
	cfg := config.Config{}

	resolved, failed := applyGeo(context.Background(), cfg, nodes, "")
	if failed != 0 {
		t.Fatalf("失败数量错误: got=%d", failed)
	}
	if len(resolved) != 1 {
		t.Fatalf("节点数量错误: got=%d", len(resolved))
	}
}

func TestApplyGeoWithInvalidMMDBPath(t *testing.T) {
	nodes := []domain.NodeMetadata{{ID: "n1", Address: "1.1.1.1"}}
	cfg := config.Config{}
	cfg.Geo.MMDBPath = filepath.Join(t.TempDir(), "not-found.mmdb")
	cfg.Geo.DNSTimeout = "1s"

	resolved, failed := applyGeo(context.Background(), cfg, nodes, "")
	if failed != 0 {
		t.Fatalf("失败数量错误: got=%d", failed)
	}
	if len(resolved) != 1 {
		t.Fatalf("节点数量错误: got=%d", len(resolved))
	}
}

func TestRunWithUnsupportedCoreVariantShouldSucceed(t *testing.T) {
	listen, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("申请测试端口失败: %v", err)
	}
	port := listen.Addr().(*net.TCPAddr).Port
	if closeErr := listen.Close(); closeErr != nil {
		t.Fatalf("释放测试端口失败: %v", closeErr)
	}

	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	cfgText := fmt.Sprintf(`gateway:
  http_port: 8080
  socks_port: %d
policy:
  strategy: random
sources:
  - name: unsupported
    type: node
    url: "trojan://secret@1.1.1.1:443?type=grpc#unsupported"
  - name: supported
    type: node
    url: "socks5://2.2.2.2:1080#supported"
`, port)
	if err := os.WriteFile(cfgPath, []byte(cfgText), 0o600); err != nil {
		t.Fatalf("写入配置失败: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	if err := Run(ctx, cfgPath); err != nil {
		t.Fatalf("Run 返回错误: %v", err)
	}
}

func TestRunWithSourceBareFilePathShouldSucceed(t *testing.T) {
	listen, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("申请测试端口失败: %v", err)
	}
	port := listen.Addr().(*net.TCPAddr).Port
	if closeErr := listen.Close(); closeErr != nil {
		t.Fatalf("释放测试端口失败: %v", closeErr)
	}

	tmpDir := t.TempDir()
	sourceFileName := "nodes.txt"
	sourceFilePath := filepath.Join(tmpDir, sourceFileName)
	sourceText := "2.2.2.2:1080#from-file\n"
	if err := os.WriteFile(sourceFilePath, []byte(sourceText), 0o600); err != nil {
		t.Fatalf("写入 source 文件失败: %v", err)
	}

	cfgPath := filepath.Join(tmpDir, "config.yaml")
	cfgText := fmt.Sprintf(`gateway:
  http_port: 8080
  socks_port: %d
policy:
  strategy: random
sources:
  - name: local-source
    type: source
    url: "%s"
`, port, sourceFileName)
	if err := os.WriteFile(cfgPath, []byte(cfgText), 0o600); err != nil {
		t.Fatalf("写入配置失败: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	if err := Run(ctx, cfgPath); err != nil {
		t.Fatalf("Run 返回错误: %v", err)
	}
}

func TestRunWithSubscribeBareFilePathShouldSucceed(t *testing.T) {
	listen, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("申请测试端口失败: %v", err)
	}
	port := listen.Addr().(*net.TCPAddr).Port
	if closeErr := listen.Close(); closeErr != nil {
		t.Fatalf("释放测试端口失败: %v", closeErr)
	}

	tmpDir := t.TempDir()
	sourceFileName := "nodes-subscribe.txt"
	sourceFilePath := filepath.Join(tmpDir, sourceFileName)
	sourceText := "3.3.3.3:1080#from-subscribe\n"
	if err := os.WriteFile(sourceFilePath, []byte(sourceText), 0o600); err != nil {
		t.Fatalf("写入 subscribe source 文件失败: %v", err)
	}

	cfgPath := filepath.Join(tmpDir, "config.yaml")
	cfgText := fmt.Sprintf(`gateway:
  http_port: 8080
  socks_port: %d
policy:
  strategy: random
sources:
  - name: local-subscribe
    type: subscribe
    url: "%s"
`, port, sourceFileName)
	if err := os.WriteFile(cfgPath, []byte(cfgText), 0o600); err != nil {
		t.Fatalf("写入配置失败: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	if err := Run(ctx, cfgPath); err != nil {
		t.Fatalf("Run 返回错误: %v", err)
	}
}

func TestRunWithInvalidGatewayHTTPPortShouldReturnFieldPath(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	cfgText := `gateway:
  http_port: 0
  socks_port: 1080
sources:
  - name: demo
    type: node
    url: "socks5://1.1.1.1:1080#demo"
`
	if err := os.WriteFile(cfgPath, []byte(cfgText), 0o600); err != nil {
		t.Fatalf("写入配置失败: %v", err)
	}

	err := Run(context.Background(), cfgPath)
	if err == nil {
		t.Fatal("预期 Run 返回错误，但得到 nil")
	}
	if !strings.Contains(err.Error(), "加载配置失败") {
		t.Fatalf("错误信息缺少加载配置失败前缀: %v", err)
	}
	if !strings.Contains(err.Error(), "gateway.http_port") {
		t.Fatalf("错误信息缺少字段路径: %v", err)
	}
}

func TestRunWithInvalidSourceTypeShouldReturnFieldPath(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	cfgText := `gateway:
  http_port: 8080
  socks_port: 1080
sources:
  - name: demo
    type: invalid
    url: "socks5://1.1.1.1:1080#demo"
`
	if err := os.WriteFile(cfgPath, []byte(cfgText), 0o600); err != nil {
		t.Fatalf("写入配置失败: %v", err)
	}

	err := Run(context.Background(), cfgPath)
	if err == nil {
		t.Fatal("预期 Run 返回错误，但得到 nil")
	}
	if !strings.Contains(err.Error(), "加载配置失败") {
		t.Fatalf("错误信息缺少加载配置失败前缀: %v", err)
	}
	if !strings.Contains(err.Error(), "sources[0].type") {
		t.Fatalf("错误信息缺少字段路径: %v", err)
	}
}

func TestPenaltyPoolAllPenalizedFallback(t *testing.T) {
	pool := health.NewPenaltyPool(5 * time.Minute)
	pool.MarkFailure("a")
	pool.MarkFailure("b")

	filtered := pool.FilterCandidates([]string{"a", "b"})
	if len(filtered) != 2 {
		t.Fatalf("全惩罚兜底失败: got=%d want=2", len(filtered))
	}
}
