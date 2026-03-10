# GeoLoom

GeoLoom 是一个基于 Go 的代理节点聚合与筛选工具：
- 接收多种来源的节点输入（订阅、本地文件、单节点链接）
- 按地理策略（allow/block）筛选候选节点
- 交由 sing-box 内核构建统一出口并对外提供 SOCKS5 代理入口

## 当前能力

### 输入层
- `source` 输入：
  - `http://` / `https://` 订阅链接
  - `@本地文件路径`
  - **裸文件路径（无 `@` 前缀）**，会自动按本地文件处理（相对路径相对配置文件目录解析）
- `node` 输入（最小字段子集）：
  - `hysteria2://`
  - `socks5://`
  - `socks4://`
  - `vless://`
  - `trojan://`
  - `vmess://`
  - `ss://`
- `source` 内容扩展：支持 URI 列表、Clash YAML、Sing-box JSON，以及远程/本地逐行文本文件。

### Geo + Filter
- DNS 解析目标地址
- MaxMind MMDB 国家识别
- IP -> 国家缓存
- allow/block 过滤（`block` 优先）
- 节点稳定 fingerprint 与跨 source 去重（geo 前执行，避免重复节点放大候选池）

### Core / LoadBalance / Health / Runtime
- 最小可用拓扑：SOCKS 入站 + 统一 `lb-out` 出口
- 负载策略：
  - `random`：基于 `health_score` 的全候选 weighted-random，连接级随机时更倾向高质量候选代理
  - `urltest`：基于主动探测结果选择当前更优单节点
  - `hybrid`：先收敛到当前高质量候选子集，再在子集内做 weighted-random；可通过 `policy.hybrid_top_k` 控制基础子集大小，若 cutoff 名次存在并列 `health_score`，会把同分节点一并纳入
- 健康检查：失败惩罚窗口、全惩罚兜底、周期统计日志
- 运行时编排：统一 `Runtime` 快照、按 `Fingerprint` 增量刷新、source 状态跟踪
- 管理 API：默认本地回环监听，提供 `/api/v1/status|sources|nodes|candidates|health|logs`，可选静态 token header 鉴权
- 健康状态持久化：本地 JSON 文件恢复惩罚窗口与最近检查状态
- source 内容扩展：支持 URI 列表、Clash YAML、Sing-box JSON

### 管理 API 与状态文件
- 管理 API 默认建议仅监听本地回环地址，例如 `127.0.0.1:9090`，避免在未做鉴权时直接暴露到公网。
- 默认未配置 `api.token` 时保持免鉴权；配置后，所有管理 API 路由都必须携带指定 header。
- 默认鉴权 header 为 `X-GeoLoom-Token`，可通过 `api.auth_header` 自定义。
- 当前只提供只读接口：
  - `GET /api/v1/status`：运行状态、节点聚合计数、refresh/api/state 配置摘要
  - `GET /api/v1/sources`：每个 source 的最近处理状态、输入类型、unsupported 数与错误摘要
  - `GET /api/v1/nodes`：当前已完成 geo 解析的节点列表
  - `GET /api/v1/candidates`：当前参与 core 的候选节点列表
  - `GET /api/v1/health`：健康检查配置摘要、最近跟踪节点数、惩罚池与内部健康快照
  - `GET /api/v1/logs`：当前进程最近内存日志，便于控制台只读查看与本地排障
- `state.path` 指向本地 JSON 状态文件，当前保存：
  - `Fingerprint -> penalty_until`
  - `Fingerprint -> last_check_at / last_reachable`
  - `Fingerprint -> last_country_code`
- 状态恢复语义：
  - 状态文件不存在或为空：按空状态启动，不报错
  - 状态文件损坏：记录 warn，并降级为空状态继续启动
  - 已过期惩罚窗口：加载时自动清理，不会错误恢复

### 管理 API 响应示例

#### `GET /api/v1/status`

```json
{
  "version": "v0.2.6",
  "started_at": "2026-03-09T10:00:00Z",
  "last_refresh_at": "2026-03-09T10:05:00Z",
  "source_count": 2,
  "strategy": "random",
  "raw_node_count": 12,
  "deduped_node_count": 10,
  "resolved_node_count": 9,
  "candidate_node_count": 6,
  "dropped_node_count": 3,
  "core_supported_count": 6,
  "core_unsupported_count": 1,
  "refresh": {
    "enabled": true,
    "interval": "10m"
  },
  "api": {
    "enabled": true,
    "listen": "127.0.0.1:9090"
  },
  "state": {
    "enabled": true,
    "path": "geoloom-state.json"
  }
}
```

#### `GET /api/v1/sources`

```json
{
  "items": [
    {
      "name": "remote-subscription",
      "type": "source",
      "url": "https://example.com/subscription.txt",
      "normalized_url": "https://example.com/subscription.txt",
      "input_type": "source",
      "node_count": 8,
      "unsupported_count": 2,
      "success": true,
      "updated_at": "2026-03-09T10:05:00Z"
    },
    {
      "name": "local-file",
      "type": "source",
      "url": "sub.txt",
      "normalized_url": "@/etc/geoloom/sub.txt",
      "node_count": 0,
      "unsupported_count": 0,
      "success": false,
      "error": "订阅拉取失败: 读取本地输入文件失败: open /etc/geoloom/sub.txt: no such file or directory",
      "updated_at": "2026-03-09T10:05:00Z"
    }
  ]
}
```

#### `GET /api/v1/nodes`

```json
{
  "count": 2,
  "items": [
    {
      "id": "socks5-1.1.1.1-1080",
      "fingerprint": "socks5-aaaabbbbcccc",
      "name": "hk-entry",
      "source_names": [
        "remote-subscription"
      ],
      "country_code": "HK",
      "protocol": "socks5",
      "address": "1.1.1.1",
      "port": 1080,
      "last_checked": "0001-01-01T00:00:00Z",
      "raw_config": {
        "protocol": "socks5",
        "address": "1.1.1.1",
        "port": 1080
      }
    },
    {
      "id": "trojan-2.2.2.2-443",
      "fingerprint": "trojan-ddddeeeeffff",
      "name": "sg-edge",
      "source_names": [
        "remote-subscription",
        "manual-node"
      ],
      "country_code": "SG",
      "protocol": "trojan",
      "address": "2.2.2.2",
      "port": 443,
      "last_checked": "0001-01-01T00:00:00Z",
      "raw_config": {
        "protocol": "trojan",
        "address": "2.2.2.2",
        "port": 443,
        "sni": "edge.example.com"
      }
    }
  ]
}
```

#### `GET /api/v1/candidates`

```json
{
  "count": 3,
  "items": [
    {
      "id": "trojan-2.2.2.2-443",
      "fingerprint": "trojan-ddddeeeeffff",
      "name": "sg-edge",
      "source_names": [
        "remote-subscription",
        "manual-node"
      ],
      "country_code": "SG",
      "protocol": "trojan",
      "address": "2.2.2.2",
      "port": 443,
      "last_checked": "0001-01-01T00:00:00Z",
      "health_score": 93,
      "raw_config": {
        "protocol": "trojan",
        "address": "2.2.2.2",
        "port": 443,
        "sni": "edge.example.com"
      }
    },
    {
      "id": "socks5-1.1.1.1-1080",
      "fingerprint": "socks5-aaaabbbbcccc",
      "name": "hk-entry",
      "source_names": [
        "remote-subscription"
      ],
      "country_code": "HK",
      "protocol": "socks5",
      "address": "1.1.1.1",
      "port": 1080,
      "last_checked": "0001-01-01T00:00:00Z",
      "health_score": 78,
      "raw_config": {
        "protocol": "socks5",
        "address": "1.1.1.1",
        "port": 1080
      }
    },
    {
      "id": "vmess-3.3.3.3-443",
      "fingerprint": "vmess-111122223333",
      "name": "jp-backup",
      "source_names": [
        "remote-subscription"
      ],
      "country_code": "JP",
      "protocol": "vmess",
      "address": "3.3.3.3",
      "port": 443,
      "last_checked": "0001-01-01T00:00:00Z",
      "health_score": 42,
      "raw_config": {
        "protocol": "vmess",
        "address": "3.3.3.3",
        "port": 443,
        "security": "tls"
      }
    }
  ]
}
```

说明：
- `items` 已按当前 runtime 候选优先顺序返回，高分节点通常会排在前面。
- `health_score` 是 weighted-random 的输入分值，当前会映射为离散权重而不是直接作为概率值使用。
- 若 `policy.strategy=random`，GeoLoom 会基于这里的候选顺序与 `health_score` 对全候选生成 weighted-random 配置；若为 `hybrid`，则仅截取当前高质量子集后再生成 weighted-random；`policy.hybrid_top_k` 表示基础截断数量，若 cutoff 名次存在并列 `health_score` 会一并纳入；若为 `urltest`，则仍沿用 sing-box `urltest` 行为。

#### `/api/v1/nodes`、`/api/v1/candidates`、`/api/v1/health` 字段边界对照

| 视图 | 主要用途 | 数据范围 | 是否包含运行时排序 | 是否包含质量分值 | 关键字段 | 适合场景 |
| --- | --- | --- | --- | --- | --- | --- |
| `/api/v1/nodes` | 展示解析 + Geo 后的节点全量视图 | `resolved_nodes` | 否 | 否 | `id`、`fingerprint`、`source_names`、`country_code`、`protocol`、`address`、`port`、`raw_config` | 节点清单、来源排查、协议/地域分布查看 |
| `/api/v1/candidates` | 展示当前参与 core 的候选集合 | `candidates` | 是 | 是，`health_score` | `/api/v1/nodes` 共有字段 + `health_score` | 观察当前生效候选、weighted-random 倾斜方向、候选优先级 |
| `/api/v1/health` | 展示健康检查与 penalty 的运行时观测 | health checker + penalty pool 内部状态 | 间接包含（`health.last_candidates`） | 是，`health.nodes[*].score` | `summary.ready_nodes`、`summary.degraded_nodes`、`health.nodes[*].success_count`、`failure_count`、`consecutive_failures`、`score`、`penalty_pool` | 健康排障、稳定性分析、解释 why 某节点被降权/惩罚 |

补充说明：
- `/api/v1/nodes` 更偏静态视图，回答“系统识别到了哪些节点”。
- `/api/v1/candidates` 更偏选路视图，回答“当前真正参与 core 选路的是哪些节点”。
- `/api/v1/health` 更偏观测视图，回答“这些节点最近为什么被优先、降权或惩罚”。
- 若只关心当前可用出口优先级，优先看 `/api/v1/candidates`；若要解释候选质量变化原因，再联动看 `/api/v1/health`。

#### `GET /api/v1/health`

```json
{
  "config": {
    "enabled": true,
    "interval": "5m",
    "url": "http://cp.cloudflare.com"
  },
  "summary": {
    "tracked_nodes": 6,
    "penalized_nodes": 1,
    "ready_nodes": 4,
    "degraded_nodes": 1,
    "last_rebuild_at": "2026-03-09T10:03:00Z"
  },
  "health": {
    "interval": 300000000000,
    "debounce": 30000000000,
    "test_url": "http://cp.cloudflare.com",
    "timeout": 5000000000,
    "last_candidates": [
      "socks5-aaaabbbbcccc",
      "socks5-ddddeeeeffff"
    ],
    "last_rebuild_at": "2026-03-09T10:03:00Z",
    "nodes": {
      "socks5-aaaabbbbcccc": {
        "last_check_at": "2026-03-09T10:04:30Z",
        "last_reachable": true,
        "last_success_at": "2026-03-09T10:04:30Z",
        "last_failure_at": "2026-03-09T09:58:00Z",
        "consecutive_failures": 0,
        "success_count": 12,
        "failure_count": 2,
        "score": 93
      }
    }
  },
  "penalty_pool": {
    "socks5-ddddeeeeffff": "2026-03-09T10:08:00Z"
  }
}
```

#### `GET /api/v1/logs`

```json
{
  "count": 2,
  "capacity": 300,
  "truncated": false,
  "items": [
    {
      "time": "2026-03-09T07:30:21Z",
      "level": "INFO",
      "message": "GeoLoom 版本信息",
      "attrs": {
        "version": "v0.2.6",
        "commit": "bc7bfb2",
        "build_time": "2026-03-09T07:30:21Z"
      },
      "text": "2026-03-09T07:30:21Z INFO  GeoLoom 版本信息 version=v0.2.6 commit=bc7bfb2 build_time=2026-03-09T07:30:21Z"
    },
    {
      "time": "2026-03-09T07:31:02Z",
      "level": "WARN",
      "message": "状态文件损坏，已忽略并按空状态启动",
      "attrs": {
        "error": "invalid character 'x' looking for beginning of value",
        "path": "geoloom-state.json"
      },
      "text": "2026-03-09T07:31:02Z WARN  状态文件损坏，已忽略并按空状态启动 error=invalid character 'x' looking for beginning of value path=geoloom-state.json"
    }
  ]
}
```

说明：
- `count` 表示当前返回的日志条数。
- `capacity` 表示内存日志环形缓冲的容量上限。
- `truncated=true` 表示日志写入量已超过缓冲容量，当前仅保留最近日志。
- `items[*].attrs` 为该条日志附带的结构化属性，具体 key 会随日志事件不同而变化。
- `items[*].text` 为便于直接展示与复制排障的格式化文本。

说明：
- `status` 适合做总览卡片与配置摘要展示。
- `health.summary` 适合做轻量状态面板；`health` 与 `penalty_pool` 适合调试与排障。
- `candidates.items[*].health_score` 表示当前 runtime 为 weighted-random 计算后的候选质量分值；冷启动或缺少历史健康数据时可退化为默认分值。
- `hybrid` 默认使用 `policy.hybrid_top_k=3` 作为高质量子集的基础大小；若 cutoff 名次存在并列分数，会把同分节点一并纳入，避免硬截断。
- `health.nodes[*].score` 与 success/failure 统计字段可用于观察节点近期稳定性，但当前仍是基于周期探测的最小评分模型，并非真实流量反馈。
- `logs` 适合做控制台只读日志面板与最近运行事件排查。
- `health.interval`、`health.debounce`、`health.timeout` 当前按 Go `time.Duration` 的 JSON 语义输出为纳秒整数；如后续面向外部稳定开放，可再评估是否改为字符串时长。
- 未配置 `api.token` 时，`/api/v1/*` 保持当前免鉴权行为。
- 配置 `api.token` 后，所有管理 API 请求都需要携带 `api.auth_header` 指定的 header。
- 调用示例：`curl -H 'X-GeoLoom-Token: your-static-token' http://127.0.0.1:9090/api/v1/status`

## 目录结构（核心）

```text
cmd/geoloom/               # 程序入口
internal/app/              # 启动编排与生命周期
internal/provider/         # 输入分发与协议解析
internal/geo/              # 地理识别与缓存
internal/filter/           # 地域过滤策略
internal/core/singbox/     # sing-box options 构建与服务封装
internal/health/           # 健康检查与惩罚池
configs/config.yaml        # 默认示例配置
docs/geoloom.prd.md        # 产品与架构约束
```

## 快速开始

### 1) 准备环境
- Go（建议使用当前稳定版）
- Node.js 20+（用于 `frontend/` 管理面板开发）

### 2) 拉起程序

```bash
go run ./cmd/geoloom -config configs/config.yaml
```

### 3) 构建二进制

```bash
go build ./cmd/geoloom
```

### 4) 查看版本

```bash
go run ./cmd/geoloom -version
```

### 4) 前端管理面板（frontend）

仓库内已提供 `frontend/` 子目录，用于消费现有只读管理 API：
- `/api/v1/status`
- `/api/v1/sources`
- `/api/v1/nodes`
- `/api/v1/candidates`
- `/api/v1/health`
- `/api/v1/logs`

安装依赖：

```bash
cd frontend && npm install
```

开发启动：

```bash
cd frontend && npm run dev
```

生产构建（构建产物会输出到 `internal/api/frontenddist/`，供 Go 服务嵌入）：

```bash
cd frontend && npm run build
```

说明：
- Vite 开发服务器默认运行在 `http://127.0.0.1:5173`。
- 开发期通过 Vite proxy 将 `/api/*` 转发到 `http://127.0.0.1:9090`。
- 生产/本地运行期由 GeoLoom 统一提供：
  - `/api/v1/*`：只读 JSON API
  - `/assets/*`：前端静态资源
  - `/`：前端入口页
  - 其他非 `/api/` 路径：SPA fallback 到 `index.html`
- 若管理 API 配置了 `api.token`，静态页面仍可直接打开，但页面内对 `/api/v1/*` 的请求必须携带 `api.auth_header` 指定的 header；可在右侧侧栏中填写 token 与 header 名，默认 header 为 `X-GeoLoom-Token`。
- 推荐的鉴权联调回归步骤：
  1. 在配置中设置 `api.token` 与 `api.auth_header`，启动 GeoLoom。
  2. 直接访问根路径 `/`，确认页面可打开，但初始 API 请求返回 401。
  3. 确认页面出现“鉴权失败，请检查 Token/Header。”且右侧状态为“未连接”。
  4. 在右侧栏填写正确的 header/token 并保存。
  5. 确认 `/api/v1/status|sources|nodes|candidates|health|logs` 恢复为 200，页面状态切换为“已连接”。
- 若你的 API 不在默认地址，可在开发期页面侧栏中修改 `API Base URL`（例如 `http://127.0.0.1:9090`）；嵌入模式下默认留空走同源。
- 当前前端默认中文，并支持中英切换与亮暗色模式切换。

### 5) Docker Compose 部署（GHCR 镜像）

> 适用于已发布镜像，例如 `ghcr.io/sarices/geoloom:v0.2.6`。

1. 准备配置文件（示例）：

```bash
cp configs/config.example.prod.yaml configs/config.prod.yaml
```

2. 修改 `docker-compose.yml` 中的配置挂载（默认挂载 `configs/config.example.prod.yaml`），建议改为你自己的配置文件：

```yaml
volumes:
  - ./configs/config.prod.yaml:/etc/geoloom/config.yaml:ro
```

3. 启动：

```bash
docker compose up -d
```

4. 查看日志：

```bash
docker compose logs -f geoloom
```

5. 停止：

```bash
docker compose down
```

示例输出：

```text
GeoLoom version=dev commit=unknown build_time=unknown
```

生产构建建议通过 `-ldflags` 注入版本信息：

```bash
go build -ldflags "-X main.Version=v0.2.6 -X main.Commit=$(git rev-parse --short HEAD) -X main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" ./cmd/geoloom
```

## 多环境打包（build-all）

### 一键打包

```bash
bash scripts/release/build-all.sh v0.2.6
```

可选参数（按顺序覆盖）：
- `VERSION`：版本号（默认 `v0.2.6`）
- `COMMIT`：提交短哈希（默认自动读取，失败回退 `unknown`）
- `BUILD_TIME`：UTC 时间（默认当前时间，ISO8601）

例如：

```bash
bash scripts/release/build-all.sh v0.2.6 abc1234 2026-03-05T09:00:00Z
```

### 输出结构

执行后会生成 `dist/` 目录，包含 6 个平台目录及对应压缩包：

- `geoloom_<version>_darwin_amd64.tar.gz`
- `geoloom_<version>_darwin_arm64.tar.gz`
- `geoloom_<version>_linux_amd64.tar.gz`
- `geoloom_<version>_linux_arm64.tar.gz`
- `geoloom_<version>_windows_amd64.zip`
- `geoloom_<version>_windows_arm64.zip`

每个包内包含：
- 可执行文件（Windows 为 `geoloom.exe`）
- `config.example.yaml`
- `README.md`
- `CHANGELOG.md`

### 版本信息校验

解压任意产物后执行：

```bash
./geoloom -version
```

Windows：

```powershell
.\geoloom.exe -version
```

应输出 `version/commit/build_time` 三项注入信息。

## 快速自检（推荐）

### 1) 最小可运行示例

假设你在 `configs/` 目录下准备了一个 `sub.txt`，内容示例：

```text
2.2.2.2:1080#demo-node
```

对应配置：

```yaml
gateway:
  http_port: 8080
  socks_port: 1080

policy:
  strategy: random # 可选 random / urltest / hybrid
  hybrid_top_k: 3 # 仅对 hybrid 生效；若 cutoff 分数并列，最终入池数可能大于该值
  filter:
    allow: []
    block: []
  health_check:
    enabled: false

geo:
  # 显式路径优先：为空时进入默认逻辑。
  mmdb_path: ""
  # 当 mmdb_path 为空且程序目录无 GeoLite2-Country.mmdb 时，使用该地址自动下载（仅 http/https）。
  mmdb_url: ""
  dns_timeout: 3s

sources:
  - name: local-source
    type: source
    url: "sub.txt"
```

### 2) 启动命令

```bash
go run ./cmd/geoloom -config configs/config.yaml
```

### 3) 预期日志关键字

启动成功后，建议重点观察以下日志关键字：
- `GeoLoom 启动完成`
- `输入源处理成功`
- `节点去重完成`（关注 `raw_nodes/deduped_nodes/duplicate_nodes`）
- `节点过滤完成`
- `core wrapper 启动成功`

若配置了 `health_check.enabled: true`，周期内还会看到：
- `健康检查触发重建成功`（在候选变化时）
- 可用性统计字段（如 `available_proxy_nodes`）

### 4) 常见问题定位

- 报错“缺少 scheme”
  - 检查 `type` 是否为 `source`；仅 `source` 支持裸文件路径自动识别。
- 过滤后无候选节点
  - 检查 `policy.filter.allow/block` 是否过严。
- 地理识别未生效
  - 检查 `geo.mmdb_path` 是否已配置且文件可读。

## 配置说明（示例）

参考 `configs/config.example.yaml`：

```yaml
# 字段释义详见 configs/config.example.yaml；下面保留一份常用示例。
gateway:
  http_port: 8080
  socks_port: 1080

policy:
  strategy: random # 可选 random / urltest / hybrid
  hybrid_top_k: 3 # 仅对 hybrid 生效；若 cutoff 分数并列，最终入池数可能大于该值
  filter:
    allow: []
    block: []
  health_check:
    enabled: true
    interval: 5m
    url: http://cp.cloudflare.com
  refresh:
    enabled: true
    interval: 10m

geo:
  mmdb_path: ""
  mmdb_url: "https://cdn.jsdelivr.net/gh/Loyalsoldier/geoip@release/GeoLite2-Country.mmdb"
  dns_timeout: 3s

api:
  enabled: true
  listen: 127.0.0.1:9090
  token: ""
  auth_header: X-GeoLoom-Token

state:
  enabled: true
  path: geoloom-state.json

sources:
  - name: remote-source
    type: source
    url: "https://example.com/subscribe?token=YOUR_TOKEN"

  - name: local-file
    type: source
    url: "sub.txt"

  - name: subscribe-alias
    type: subscribe
    url: "@./providers/mixed-proxies.txt"

  - name: manual-node-socks5
    type: node
    url: "socks5://user:pass@203.0.113.10:1080#manual-socks5"

  - name: manual-node-vless
    type: node
    url: "vless://11111111-1111-1111-1111-111111111111@example.com:443?encryption=none&security=tls&sni=example.com#manual-vless"
```

说明：
- `sources.type` 支持 `source` / `subscribe` / `node`。
- `source` 与 `subscribe` 当前都按“输入源”处理；`subscribe` 主要用于兼容历史写法。
- 顶层 `type: source|subscribe` 下的 `http://` / `https://` 表示远程 source URL，不是 HTTP 代理节点。
- 若要表达 HTTP 代理节点，建议写在 source 文本内容里使用显式 `http://user:pass@host:port#name` 条目。

生产环境参考模板：`configs/config.example.prod.yaml`（默认启用健康检查、API 与状态持久化）。

### source 裸文件路径行为
当 `type: source` 且 `url` 不带 scheme、也不以 `@` 开头时：
- 绝对路径：直接作为本地文件读取
- 相对路径：相对 `-config` 指定配置文件所在目录解析

例如：

```yaml
sources:
  - name: local-list
    type: source
    url: "socks5_share_200.txt"
```

### source 文本文件解析行为
本地文件与远程 `http/https` source 在文本文件语义上保持一致：
- 空行与以 `#` 开头的注释行会被忽略。
- 显式协议条目会原样保留，当前支持常见的 `socks5://`、`socks4://`、`http://` 等节点条目。
- 裸行（如 `1.2.3.4:1080#name`、`user:pass@1.2.3.4:1080#name`）会默认补全为 `socks5://`。
- 对远程 `http/https` source，仍优先兼容现有 URI 列表、Base64 URI 列表、Clash YAML、Sing-box JSON；若正文是逐行文本文件，GeoLoom 会自动按逐行规则补齐与解析。

注意：
- 顶层配置里的 `http://` / `https://` 仍表示远程 source URL。
- 若要在 source 内容里表达 HTTP 代理节点，必须写成显式 `http://user:pass@host:port#name` 条目。
- 裸 `host:port` 不会自动猜测为 `socks4/http`，仍按 `socks5` 处理。
- source 层会尽量保留逐行文本里的原始条目；非法端口、非法 URI、未知 scheme 等脏数据通常会在 parser/dispatcher 层进入 `unsupported`，而不是在 source 层静默丢弃。
- 同地址同认证但仅 `#name` 不同的条目，在 source/parser 阶段会分别保留；进入 `domain.DedupNodes` 后，`name` 不参与节点身份判断，最终会按协议关键字段合并，并保留首条记录的名称。

远程文本文件示例：

```yaml
sources:
  - name: remote-mixed-text
    type: source
    url: "https://example.com/nodes.txt"
```

示例文件内容：

```text
# 默认按 socks5 处理
1.2.3.4:1080#default-socks5

# 显式协议
socks5://user:pass@5.6.7.8:1080#socks5-node
socks4://legacy@9.9.9.9:1080#legacy-node
http://user:pass@8.8.8.8:8080#http-node
```

### geo.mmdb_path / geo.mmdb_url 默认行为

地理识别配置按以下优先级生效：
1. `geo.mmdb_path` 非空：直接使用该路径（相对路径相对配置文件目录解析）；
2. `geo.mmdb_path` 为空：尝试读取程序目录下 `GeoLite2-Country.mmdb`；
3. 若 2 不存在且 `geo.mmdb_url` 非空：自动下载到程序目录 `GeoLite2-Country.mmdb` 后使用；
4. 若以上均不满足：跳过地理识别。

`geo.mmdb_url` 仅支持 `http/https`。

## 质量检查命令

```bash
go fmt ./...
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/geoloom
```

## 文档

- 产品需求与架构约束：`docs/geoloom.prd.md`
- 输入样例来源：`docs/sub.txt`（测试请使用脱敏样例）

## License

本项目使用 [MIT License](./LICENSE)。

## 说明

当前仓库仍以 MVP 到阶段化演进为主，默认配置偏向最小可运行链路。如用于生产环境，请补充：
- 完整 MMDB 数据与更新流程
- 资源与并发限制
- 更严格的监控、日志与错误恢复策略
