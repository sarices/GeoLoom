# Changelog

## [v0.2.2] - 2026-03-09

### 新增
- 增加节点稳定 fingerprint，作为后续健康状态归档、订阅 diff 与状态持久化的统一主键基础。
- 增加跨 source 节点去重能力，在 geo 分析前按协议关键身份字段压缩重复节点。
- 增加节点来源聚合字段 `SourceNames`，便于排障与后续来源维度治理。

### 改进
- 启动主链路改为在 parser 之后、geo 之前执行去重，避免重复节点放大候选池并干扰健康检查与负载策略结果。
- 增加去重统计日志，输出 `raw_nodes`、`deduped_nodes`、`duplicate_nodes`，便于运行时定位重复输入问题。
- 更新 README 与 Docker Compose 示例，默认镜像版本同步至 `v0.2.2`，并补充节点去重相关说明。

## [v0.1.0] - 2026-03-05

### 新增
- 初始化 GeoLoom 可执行 Go 工程，提供 `cmd/geoloom` 入口与基础配置加载能力。
- 支持多源输入分发：订阅（`http/https`）、本地文件（`@file`）与单节点链接输入。
- 支持节点协议最小解析：`hysteria2`、`socks5`、`vless`、`trojan`、`vmess`、`ss`。
- 增加 Geo + Filter 链路：DNS 解析、MMDB 国家识别、IP->国家缓存、allow/block（block 优先）。
- 增加 Core Wrapper：基于 sing-box 的最小可用 SOCKS 入站与统一 `lb-out` 出口。
- 增加负载与健康能力：`random/urltest` 策略、失败惩罚窗口与周期健康检查统计。
- 增加版本管理：支持 `-version` 输出版本、提交与构建时间。

### 改进
- `source.type=source` 支持裸文件路径（无 `@` 前缀）自动识别为本地文件。
- 本地 source 相对路径统一相对配置文件目录解析，避免受当前工作目录影响。
- `geo.mmdb_path` 为空时支持默认读取程序目录 MMDB，并可通过 `geo.mmdb_url` 自动下载。
- 新增 `configs/config.example.yaml`，提供更完整的示例配置模板。

### 说明
- 当前为首个公开版本，版本历史从 `v0.1.0` 开始。
- 发布构建建议通过 `-ldflags` 注入 `Version/Commit/BuildTime`。
