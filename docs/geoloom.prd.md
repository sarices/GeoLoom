# 项目名称：GeoLoom (智能地域感知代理网关)

## 1. 项目愿景

**GeoLoom** 旨在解决多协议代理管理混乱、地域不可控以及单节点负载过重的问题。通过集成 `Sing-box` SDK，它能将杂乱的订阅节点转化为一个支持地域过滤、随机负载均衡的高性能统一代理入口。

---

## 2. 技术架构 (Technical Architecture)

### 2.1 核心组件

- **Provider (供应层)**：负责解析多种格式的订阅（Base64, Clash YAML, Sing-box JSON）。
- **Geo-Analyzer (地理分析层)**：利用 MaxMind MMDB 对节点 IP 进行物理定位。
- **Filter Engine (过滤引擎)**：执行基于国家代码（ISO 3166-1）的黑白名单策略。
- **Core Wrapper (内核包装器)**：封装 `sagernet/sing-box` SDK，负责动态生成内存配置并启动。
- **Load Balancer (负载均衡器)**：实现 `Random`、`Round-Robin` 或 `URL-Test` (最低延迟) 调度。

---

## 3. 详细功能说明

### 3.1 节点解析与协议转换

- **支持协议**：Hysteria2, VLESS (XTLS-Reality), VMess, Shadowsocks, Trojan, TUIC.
- **自动转换**：程序需将各种格式统一映射为 `sing-box` 的 `Outbound` 结构体。

### 3.2 地域识别逻辑 (Geo-Logic)

- **离线识别**：集成 `GeoLite2-Country.mmdb`。
- **缓存机制**：节点 IP 对应的国家信息应缓存，避免重复解析。
- **动态参数**：支持在启动或运行时通过配置文件更新 `Include_Regions` (如：仅用 JP, US) 和 `Exclude_Regions` (如：禁用 CN, RU)。

### 3.3 随机 / 加权转发策略

- **Stateless Random**：每次 TCP/UDP 握手请求在候选集合中做连接级随机选择。
- **Weighted Load Balance**：根据健康探测与质量评分分配权重，质量越高，被选中的概率越高。

---

## 4. 关键数据结构 (Go 语言定义)

### 4.1 节点元数据

```go
type NodeMetadata struct {
    ID          string                 `json:"id"`           // 唯一标识
    Name        string                 `json:"name"`         // 节点原名称
    CountryCode string                 `json:"country_code"` // 如 "US", "HK"
    Protocol    string                 `json:"protocol"`     // vless, hy2...
    LastChecked time.Time              `json:"last_checked"` // 最后心跳时间
    RawConfig   map[string]interface{} `json:"raw_config"`   // 传给 Sing-box 的配置
}

```

### 4.2 过滤策略配置

```yaml
# config.yaml
gateway:
  http_port: 8080
  socks_port: 1080

policy:
  strategy: "random" # 可选 random（全候选 weighted-random）/ urltest（单优选）/ hybrid（高质量子集 weighted-random）
  hybrid_top_k: 3 # 仅对 hybrid 生效；表示高质量子集的基础大小，若 cutoff 分数并列会一并纳入
  filter:
    allow: ["US", "SG", "JP"] # 仅允许这些地区
    block: ["CN"] # 显式禁止
  health_check:
    enabled: true
    interval: "5m"
    url: "http://cp.cloudflare.com"

sources:
  - name: "ProviderA"
    type: "subscribe"
    url: "https://example.com/sub"
```

---

## 5. 核心代码实现细节 (基于 Sing-box SDK)

### 5.1 动态构建内核配置

你需要将筛选后的节点动态组装成 Sing-box 能够理解的 `option.Options`。

```go
func createSingBoxOptions(nodes []NodeMetadata) (*option.Options, error) {
    var outbounds []option.Outbound
    var tags []string

    for _, node := range nodes {
        // 将 NodeMetadata.RawConfig 转换为 option.Outbound
        outbound, _ := convertToSingBoxOutbound(node)
        outbounds = append(outbounds, outbound)
        tags = append(tags, node.ID)
    }

    // 添加负载均衡器（URLTest 或 weighted-random / hybrid weighted-random）
    balancer := option.Outbound{
        Type: "urltest", // 或 geoloom 自定义 weighted-random（random / hybrid）
        Tag:  "lb-out",
        URLTestOptions: option.URLTestOutboundOptions{
            Outbounds: tags,
            Interval:  option.Duration(5 * time.Minute),
        },
    }
    outbounds = append(outbounds, balancer)

    // 配置路由规则：所有 Inbound 流量 -> lb-out
    return &option.Options{
        Inbounds: []option.Inbound{...},
        Outbounds: outbounds,
        Route: &option.RouterOptions{
            Rules: []option.Rule{
                { Outbound: "lb-out" },
            },
        },
    }, nil
}

```

### 5.2 启动与热更新

利用 `box.NewService(opt)` 启动。当订阅更新或地域参数改变时，调用 `service.Close()` 并重新初始化，实现无感切换。

---

## 6. 异常处理与性能

1. **节点失效惩罚**：若随机选中的节点连接失败，需将其标记为 `Down`，并在 300 秒内不再选中。
2. **DNS 优化**：在 GeoLoom 内部集成 DNS 预解析，防止解析外部节点 IP 时的延迟。
3. **并发控制**：Go 语言中，每个连接都是一个 Goroutine，需要限制最大打开的文件句柄数（ulimit）。

---

## 7. 下一步行动建议

1. **第一步**：编写 `parser` 包，先支持将一个 VLESS 链接解析为 `NodeMetadata`。
2. **第二步**：引入 `geoip2-golang`，实现根据 `NodeMetadata.Addr` 自动填充 `CountryCode`。
3. **第三步**：尝试使用 `sing-box` SDK 启动一个最简单的 SOCKS5 转发。

**你需要我为你提供一个“节点连接失败自动踢出并重新随机选择”的具体逻辑实现代码吗？**
