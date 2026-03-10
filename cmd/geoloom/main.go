package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"geoloom/internal/app"
	"geoloom/internal/observability"
)

var (
	executablePath = os.Executable
	fileStat       = os.Stat

	// 通过 -ldflags 注入版本信息，默认值用于本地开发构建。
	Version   = "v0.2.7"
	Commit    = "unknown"
	BuildTime = "unknown"
)

func main() {
	configPathFlag := flag.String("config", "", "配置文件路径（默认优先使用可执行文件同目录 config.yaml）")
	versionFlag := flag.Bool("version", false, "输出版本信息并退出")
	flag.Parse()

	if *versionFlag {
		printVersion(os.Stdout)
		return
	}

	logBuffer := observability.NewLogBuffer(300)
	observability.SetDefaultLogBuffer(logBuffer)
	slog.SetDefault(slog.New(observability.NewTeeHandler(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{}), logBuffer)))

	configPath := resolveConfigPath(*configPathFlag)
	slog.Info("GeoLoom 版本信息",
		"version", Version,
		"commit", Commit,
		"build_time", BuildTime,
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx, configPath, Version, logBuffer); err != nil {
		slog.Error("GeoLoom 启动失败", "error", err)
		os.Exit(1)
	}
}

func printVersion(w io.Writer) {
	_, _ = fmt.Fprintf(w, "GeoLoom version=%s commit=%s build_time=%s\n", Version, Commit, BuildTime)
}

func resolveConfigPath(rawFlagValue string) string {
	flagValue := strings.TrimSpace(rawFlagValue)
	if flagValue != "" {
		return flagValue
	}

	exePath, err := executablePath()
	if err == nil {
		candidate := filepath.Join(filepath.Dir(exePath), "config.yaml")
		if _, statErr := fileStat(candidate); statErr == nil {
			return candidate
		}
	}

	return filepath.Join("configs", "config.yaml")
}
