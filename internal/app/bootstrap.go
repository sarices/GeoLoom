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

	"geoloom/internal/config"
	"geoloom/internal/core/singbox"
	"geoloom/internal/domain"
	"geoloom/internal/filter"
	"geoloom/internal/geo"
	"geoloom/internal/health"
	netresolver "geoloom/internal/net"
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
func Run(ctx context.Context, configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	slog.Info("GeoLoom 启动完成",
		"socks_port", cfg.Gateway.SocksPort,
		"source_count", len(cfg.Sources),
		"strategy", cfg.Policy.Strategy,
	)

	dispatcher := parser.NewDispatcher(source.NewSubscriptionFetcher(nil))
	allNodes, err := collectNodes(ctx, cfg, configPath, dispatcher)
	if err != nil {
		return err
	}

	resolvedNodes, geoFailed := applyGeo(ctx, cfg, allNodes, configPath)
	filterEngine := filter.NewEngine(filter.Config{
		Allow: cfg.Policy.Filter.Allow,
		Block: cfg.Policy.Filter.Block,
	})
	filtered := filterEngine.Filter(resolvedNodes)

	slog.Info("节点过滤完成",
		"input_nodes", len(allNodes),
		"geo_resolved_nodes", len(resolvedNodes),
		"geo_failed_nodes", geoFailed,
		"candidate_nodes", len(filtered.Candidates),
		"dropped_nodes", len(filtered.Dropped),
	)

	if len(filtered.Candidates) == 0 {
		return fmt.Errorf("过滤后无可用候选节点")
	}

	coreService := singbox.NewService(ctx, singbox.NewOptionsBuilder())
	if err := coreService.Start(cfg, filtered.Candidates); err != nil {
		return fmt.Errorf("启动 core wrapper 失败: %w", err)
	}
	defer func() {
		if closeErr := coreService.Close(); closeErr != nil {
			slog.Warn("关闭 core wrapper 失败", "error", closeErr)
		}
	}()

	if cfg.Policy.HealthCheck.Enabled {
		interval, err := time.ParseDuration(cfg.Policy.HealthCheck.Interval)
		if err != nil || interval <= 0 {
			interval = 30 * time.Second
		}
		penaltyPool := health.NewPenaltyPool(5 * time.Minute)
		checker := health.NewChecker(interval, cfg.Policy.HealthCheck.URL, penaltyPool, func(rebuildCtx context.Context, candidates []domain.NodeMetadata) error {
			if err := coreService.Rebuild(cfg, candidates); err != nil {
				slog.Warn("健康检查触发重建失败", "error", err, "candidate_nodes", len(candidates))
				return err
			}
			slog.Info("健康检查触发重建成功", "candidate_nodes", len(candidates))
			return nil
		})
		checker.Start(ctx, filtered.Candidates)
	}

	coreStats := coreService.LastBuildStats()
	slog.Info("core wrapper 启动成功",
		"inbound", "socks",
		"socks_port", cfg.Gateway.SocksPort,
		"strategy", cfg.Policy.Strategy,
		"candidate_nodes", len(filtered.Candidates),
		"core_supported_candidates", coreStats.SupportedCandidates,
		"core_unsupported_count", len(coreStats.Unsupported),
	)

	<-ctx.Done()
	slog.Info("GeoLoom 收到退出信号", "reason", ctx.Err())
	return nil
}

func collectNodes(ctx context.Context, cfg config.Config, configPath string, dispatcher *parser.Dispatcher) ([]domain.NodeMetadata, error) {
	allNodes := make([]domain.NodeMetadata, 0)
	for _, src := range cfg.Sources {
		normalizedURL := normalizeSourceURL(src, configPath)
		result, parseErr := dispatcher.Parse(ctx, normalizedURL)
		if parseErr != nil {
			slog.Warn("输入源处理失败",
				"source", src.Name,
				"url", src.URL,
				"normalized_url", normalizedURL,
				"error", parseErr,
			)
			continue
		}

		sourceName := buildSourceName(src, normalizedURL)
		for i := range result.Nodes {
			if sourceName != "" {
				result.Nodes[i].SourceNames = []string{sourceName}
			}
		}

		slog.Info("输入源处理成功",
			"source", src.Name,
			"input_type", result.Type,
			"node_count", len(result.Nodes),
			"unsupported_count", len(result.Unsupported),
		)
		allNodes = append(allNodes, result.Nodes...)
	}

	rawCount := len(allNodes)
	deduped, err := domain.DedupNodes(allNodes)
	if err != nil {
		return nil, fmt.Errorf("节点去重失败: %w", err)
	}
	allNodes = deduped.Nodes

	slog.Info("节点去重完成",
		"raw_nodes", rawCount,
		"deduped_nodes", len(allNodes),
		"duplicate_nodes", deduped.DuplicateCount,
	)
	return allNodes, nil
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
