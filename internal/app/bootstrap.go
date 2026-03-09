package app

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"geoloom/internal/api"
	"geoloom/internal/config"
	"geoloom/internal/domain"
	"geoloom/internal/geo"
	netresolver "geoloom/internal/net"
	"geoloom/internal/observability"
	"geoloom/internal/provider/parser"
	"geoloom/internal/provider/source"
)

var urlSchemePattern = regexp.MustCompile(`(?i)^[a-z][a-z0-9+.-]*://`)

const defaultMMDBFileName = "GeoLite2-Country.mmdb"

var (
	httpDo       = http.DefaultClient.Do
	osStat       = os.Stat
	osWriteFile  = os.WriteFile
	osExecutable = os.Executable
)

// Run 负责应用启动与生命周期收敛。
func Run(ctx context.Context, configPath string, args ...any) error {
	var version string
	var logBuffer *observability.LogBuffer
	if len(args) > 0 {
		if value, ok := args[0].(string); ok {
			version = value
		}
	}
	if len(args) > 1 {
		if value, ok := args[1].(*observability.LogBuffer); ok {
			logBuffer = value
		}
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	slog.Info("GeoLoom 启动完成",
		"socks_port", cfg.Gateway.SocksPort,
		"source_count", len(cfg.Sources),
		"strategy", cfg.Policy.Strategy,
	)

	runtime := NewRuntime(ctx, cfg, configPath, parser.NewDispatcher(source.NewSubscriptionFetcher(nil)), version, logBuffer)
	if err := runtime.Start(ctx); err != nil {
		return err
	}
	defer func() {
		if closeErr := runtime.Close(); closeErr != nil {
			slog.Warn("关闭运行时失败", "error", closeErr)
		}
	}()

	if cfg.Policy.Refresh.Enabled {
		interval, err := time.ParseDuration(cfg.Policy.Refresh.Interval)
		if err != nil || interval <= 0 {
			interval = 10 * time.Minute
		}
		NewRefresher(interval, runtime).Start(ctx)
	}

	if cfg.API.Enabled {
		server := &http.Server{Addr: cfg.API.Listen, Handler: api.NewServer(runtime, cfg.API.AuthHeader, cfg.API.Token).Handler()}
		go func() {
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				slog.Warn("管理 API 启动失败", "error", err)
			}
		}()
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := server.Shutdown(shutdownCtx); err != nil {
				slog.Warn("关闭管理 API 失败", "error", err)
			}
		}()
	}

	coreStats := runtime.Snapshot().CoreStats
	slog.Info("core wrapper 启动成功",
		"inbound", "socks",
		"socks_port", cfg.Gateway.SocksPort,
		"strategy", cfg.Policy.Strategy,
		"candidate_nodes", len(runtime.Snapshot().Candidates),
		"core_supported_candidates", coreStats.SupportedCandidates,
		"core_unsupported_count", len(coreStats.Unsupported),
	)

	<-ctx.Done()
	slog.Info("GeoLoom 收到退出信号", "reason", ctx.Err())
	return nil
}

func collectNodes(ctx context.Context, cfg config.Config, configPath string, dispatcher *parser.Dispatcher) ([]domain.NodeMetadata, error) {
	_, nodes, _, err := collectNodesDetailed(ctx, cfg, configPath, dispatcher)
	return nodes, err
}

func buildSourceName(src config.Source, normalizedURL string) string {
	if name := strings.TrimSpace(src.Name); name != "" {
		return name
	}
	return strings.TrimSpace(strings.TrimPrefix(normalizedURL, "@"))
}

func normalizeSourceURL(src config.Source, configPath string) string {
	cleanedURL := strings.TrimSpace(src.URL)
	if !config.IsSourceLikeType(src.Type) {
		return cleanedURL
	}
	if cleanedURL == "" || strings.HasPrefix(cleanedURL, "@") || hasURLScheme(cleanedURL) {
		return cleanedURL
	}

	resolvedPath := resolvePathByConfigPath(cleanedURL, configPath)
	return "@" + resolvedPath
}

func hasURLScheme(raw string) bool {
	return urlSchemePattern.MatchString(strings.TrimSpace(raw))
}

func resolvePathByConfigPath(rawPath, configPath string) string {
	cleanedPath := strings.TrimSpace(rawPath)
	if cleanedPath == "" || filepath.IsAbs(cleanedPath) {
		return filepath.Clean(cleanedPath)
	}

	cleanedConfigPath := strings.TrimSpace(configPath)
	if cleanedConfigPath == "" {
		return filepath.Clean(cleanedPath)
	}

	basePath := cleanedConfigPath
	if !filepath.IsAbs(basePath) {
		if absConfigPath, err := filepath.Abs(basePath); err == nil {
			basePath = absConfigPath
		}
	}
	baseDir := filepath.Dir(basePath)
	return filepath.Clean(filepath.Join(baseDir, cleanedPath))
}

func prepareMMDBPath(ctx context.Context, geoCfg config.GeoConfig, configPath string) (string, error) {
	trimmedPath := strings.TrimSpace(geoCfg.MMDBPath)
	if trimmedPath != "" {
		return resolvePathByConfigPath(trimmedPath, configPath), nil
	}

	exeDir, err := resolveExecutableDir()
	if err != nil {
		return "", fmt.Errorf("解析可执行目录失败: %w", err)
	}
	defaultMMDBPath := filepath.Join(exeDir, defaultMMDBFileName)
	if _, statErr := osStat(defaultMMDBPath); statErr == nil {
		return defaultMMDBPath, nil
	}

	mmdbURL := strings.TrimSpace(geoCfg.MMDBURL)
	if mmdbURL == "" {
		return "", nil
	}

	if err := downloadMMDB(ctx, mmdbURL, defaultMMDBPath); err != nil {
		return "", err
	}
	return defaultMMDBPath, nil
}

func resolveExecutableDir() (string, error) {
	exePath, err := osExecutable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(exePath), nil
}

func downloadMMDB(ctx context.Context, mmdbURL, targetPath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mmdbURL, nil)
	if err != nil {
		return fmt.Errorf("创建 MMDB 下载请求失败: %w", err)
	}
	resp, err := httpDo(req)
	if err != nil {
		return fmt.Errorf("下载 MMDB 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("下载 MMDB 返回异常状态码: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024*1024))
	if err != nil {
		return fmt.Errorf("读取 MMDB 响应失败: %w", err)
	}
	if len(body) == 0 {
		return fmt.Errorf("下载 MMDB 内容为空")
	}

	if err := osWriteFile(targetPath, body, 0o600); err != nil {
		return fmt.Errorf("写入 MMDB 文件失败: %w", err)
	}
	slog.Info("MMDB 下载完成", "path", targetPath)
	return nil
}

func applyGeo(ctx context.Context, cfg config.Config, nodes []domain.NodeMetadata, configPath string) ([]domain.NodeMetadata, int) {
	if len(nodes) == 0 {
		return nil, 0
	}

	mmdbPath, err := prepareMMDBPath(ctx, cfg.Geo, configPath)
	if err != nil {
		slog.Warn("准备 MMDB 路径失败，跳过地理识别", "error", err)
		return nodes, 0
	}
	if mmdbPath == "" {
		slog.Warn("未配置 geo.mmdb_path 且未命中默认 MMDB，跳过地理识别")
		return nodes, 0
	}

	timeout, err := time.ParseDuration(cfg.Geo.DNSTimeout)
	if err != nil || timeout <= 0 {
		timeout = 3 * time.Second
	}

	dnsResolver := netresolver.NewDNSResolver(nil)
	geoResolver, err := geo.NewMMDBResolverFromPath(mmdbPath, geo.NewInMemoryCountryCache(), dnsResolver)
	if err != nil {
		slog.Warn("初始化 MMDB resolver 失败，跳过地理识别", "error", err)
		return nodes, 0
	}
	defer func() {
		if closeErr := geoResolver.Close(); closeErr != nil {
			slog.Warn("关闭 MMDB resolver 失败", "error", closeErr)
		}
	}()

	resolved := make([]domain.NodeMetadata, 0, len(nodes))
	failed := 0
	for _, node := range nodes {
		nodeCtx, cancel := context.WithTimeout(ctx, timeout)
		country, resolveErr := geoResolver.ResolveNodeCountry(nodeCtx, node)
		cancel()
		if resolveErr != nil {
			failed++
			slog.Warn("节点地理识别失败",
				"node_id", node.ID,
				"address", node.Address,
				"error", resolveErr,
			)
			continue
		}

		node.CountryCode = country
		resolved = append(resolved, node)
	}

	return resolved, failed
}
