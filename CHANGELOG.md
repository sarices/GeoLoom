# Changelog

## [v0.2.5] - 2026-03-09

### 新增
- 增加控制台运行日志只读观测能力：后端新增 `/api/v1/logs`，前端新增 Logs 页面，用于查看当前进程最近内存日志。

### 改进
- 控制台侧边导航与只读观测链路补齐日志视图，便于本地排障与运行态检查。
- 补充日志 API、前端页面与嵌入式交付的测试与文档说明，保持发布产物与管理能力一致。

## [v0.2.4] - 2026-03-09

### 新增
- 增加嵌入式前端控制台交付方式：前端构建产物由 Go 服务统一提供 `/`、`/assets/*` 与 SPA fallback，形成同源访问体验。

### 改进
- 收敛前端控制台视觉结构，调整主容器留白、左右栏比例、卡片层级与右侧辅助栏阅读顺序。
- 管理 API 与静态页面路由边界进一步明确：`/api/v1/*` 继续独立鉴权，静态页面保持可直接访问。
- 补充前端嵌入、缓存策略、SPA fallback 与鉴权恢复加载的测试、README 与治理文档说明。

## [v0.2.3] - 2026-03-09

### 新增
- 增加统一运行时编排 `Runtime`，对外稳定输出只读管理 API 快照，并支持按配置周期刷新输入源。
- 增加本地只读管理 API：`/api/v1/status`、`/api/v1/sources`、`/api/v1/nodes`、`/api/v1/candidates`、`/api/v1/health`。
- 增加本地 JSON 状态持久化，可恢复健康检查惩罚窗口、最近探测状态与最近国家识别结果。
- 增加管理 API 最小鉴权能力：支持静态 token + 自定义 header，未配置 token 时保持兼容的免鉴权行为。
- 增加 source 内容扩展识别，支持 URI 列表、Clash YAML 与 Sing-box JSON 输入。

### 改进
- README 补充管理 API、状态文件、配置样例与响应示例说明，便于本地观测与排障。
- Docker Compose 默认镜像版本、构建示例与发布脚本默认版本同步更新至 `v0.2.3`。
- 配置层新增 `policy.refresh`、`api`、`state` 默认值归一化与校验，降低运行时配置出错概率。

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
