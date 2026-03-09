package app

import (
	"context"
	"log/slog"
	"time"
)

// Refresher 负责按周期触发运行时刷新。
type Refresher struct {
	interval time.Duration
	runtime  *Runtime
}

func NewRefresher(interval time.Duration, runtime *Runtime) *Refresher {
	if interval <= 0 {
		interval = 10 * time.Minute
	}
	return &Refresher{interval: interval, runtime: runtime}
}

func (r *Refresher) Start(ctx context.Context) {
	if r == nil || r.runtime == nil {
		return
	}
	go r.run(ctx)
}

func (r *Refresher) run(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := r.runtime.RefreshOnce(ctx); err != nil {
				slog.Warn("周期刷新失败", "error", err)
			}
		}
	}
}
