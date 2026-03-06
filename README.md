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
  - `vless://`
  - `trojan://`
  - `vmess://`
  - `ss://`

### Geo + Filter
- DNS 解析目标地址
- MaxMind MMDB 国家识别
- IP -> 国家缓存
- allow/block 过滤（`block` 优先）

### Core / LoadBalance / Health
- 最小可用拓扑：SOCKS 入站 + 统一 `lb-out` 出口
- 负载策略：
  - `random`：连接级随机选择候选代理
  - `urltest`：基于探测结果选择
- 健康检查：失败惩罚窗口、全惩罚兜底、周期统计日志

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

示例输出：

```text
GeoLoom version=dev commit=unknown build_time=unknown
```

生产构建建议通过 `-ldflags` 注入版本信息：

```bash
go build -ldflags "-X main.Version=v0.1.0 -X main.Commit=$(git rev-parse --short HEAD) -X main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" ./cmd/geoloom
```

## 多环境打包（build-all）

### 一键打包

```bash
bash scripts/release/build-all.sh v0.1.0
```

可选参数（按顺序覆盖）：
- `VERSION`：版本号（默认 `v0.1.0`）
- `COMMIT`：提交短哈希（默认自动读取，失败回退 `unknown`）
- `BUILD_TIME`：UTC 时间（默认当前时间，ISO8601）

例如：

```bash
bash scripts/release/build-all.sh v0.1.0 abc1234 2026-03-05T09:00:00Z
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
  strategy: random
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

参考 `configs/config.yaml`：

```yaml
gateway:
  http_port: 8080
  socks_port: 1080

policy:
  strategy: random
  filter:
    allow: []
    block: []
  health_check:
    enabled: true
    interval: 5m
    url: http://cp.cloudflare.com

geo:
  # 显式路径优先：为空时进入默认逻辑。
  mmdb_path: ""
  # 当 mmdb_path 为空且程序目录无 GeoLite2-Country.mmdb 时，使用该地址自动下载（仅 http/https）。
  mmdb_url: ""
  dns_timeout: 3s

sources:
  - name: docs-sub
    type: source
    url: "sub.txt" # 可写裸文件路径 / @文件路径 / http(s)订阅
```

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

## 说明

当前仓库仍以 MVP 到阶段化演进为主，默认配置偏向最小可运行链路。如用于生产环境，请补充：
- 完整 MMDB 数据与更新流程
- 资源与并发限制
- 更严格的监控、日志与错误恢复策略
