package parser

import (
	"fmt"
	"strings"

	"geoloom/internal/domain"

	"gopkg.in/yaml.v3"
)

type clashDocument struct {
	Proxies []clashProxy `yaml:"proxies"`
}

type clashProxy struct {
	Name       string `yaml:"name"`
	Type       string `yaml:"type"`
	Server     string `yaml:"server"`
	Port       int    `yaml:"port"`
	UUID       string `yaml:"uuid"`
	Password   string `yaml:"password"`
	Cipher     string `yaml:"cipher"`
	AlterID    int    `yaml:"alterId"`
	TLS        bool   `yaml:"tls"`
	ServerName string `yaml:"servername"`
	SNI        string `yaml:"sni"`
	Network    string `yaml:"network"`
	Host       string `yaml:"host"`
	Path       string `yaml:"path"`
	Username   string `yaml:"username"`
}

// ParseClashYAML 解析 Clash YAML source；ok=false 表示内容并非 Clash YAML。
func ParseClashYAML(content []byte) ([]domain.NodeMetadata, bool, error) {
	trimmed := strings.TrimSpace(string(content))
	if trimmed == "" || (!strings.Contains(trimmed, "proxies:") && !strings.Contains(trimmed, "proxy-providers:")) {
		return nil, false, nil
	}
	var doc clashDocument
	if err := yaml.Unmarshal(content, &doc); err != nil {
		return nil, true, err
	}
	if len(doc.Proxies) == 0 {
		return nil, true, nil
	}
	nodes := make([]domain.NodeMetadata, 0, len(doc.Proxies))
	for _, proxy := range doc.Proxies {
		node, err := mapClashProxy(proxy)
		if err != nil {
			continue
		}
		nodes = append(nodes, node)
	}
	return nodes, true, nil
}

func mapClashProxy(proxy clashProxy) (domain.NodeMetadata, error) {
	protocol := strings.ToLower(strings.TrimSpace(proxy.Type))
	host := strings.TrimSpace(proxy.Server)
	if host == "" || proxy.Port <= 0 {
		return domain.NodeMetadata{}, fmt.Errorf("clash proxy 缺少 server/port")
	}
	name := strings.TrimSpace(proxy.Name)
	if name == "" {
		name = fmt.Sprintf("%s-%s:%d", protocol, host, proxy.Port)
	}
	switch protocol {
	case "socks5":
		raw := map[string]any{"type": "socks5", "server": host, "server_port": proxy.Port}
		if proxy.Username != "" {
			raw["username"] = strings.TrimSpace(proxy.Username)
		}
		if proxy.Password != "" {
			raw["password"] = strings.TrimSpace(proxy.Password)
		}
		return domain.NodeMetadata{ID: buildNodeID("socks5", host, proxy.Port, proxy.Username+":"+proxy.Password), Name: name, Protocol: "socks5", Address: host, Port: proxy.Port, RawConfig: raw}, nil
	case "socks4":
		raw := map[string]any{"type": "socks4", "server": host, "server_port": proxy.Port}
		if proxy.Username != "" {
			raw["username"] = strings.TrimSpace(proxy.Username)
		}
		return domain.NodeMetadata{ID: buildNodeID("socks4", host, proxy.Port, proxy.Username), Name: name, Protocol: "socks4", Address: host, Port: proxy.Port, RawConfig: raw}, nil
	case "http":
		raw := map[string]any{"type": "http", "server": host, "server_port": proxy.Port}
		if proxy.Username != "" {
			raw["username"] = strings.TrimSpace(proxy.Username)
		}
		if proxy.Password != "" {
			raw["password"] = strings.TrimSpace(proxy.Password)
		}
		return domain.NodeMetadata{ID: buildNodeID("http", host, proxy.Port, proxy.Username+":"+proxy.Password), Name: name, Protocol: "http", Address: host, Port: proxy.Port, RawConfig: raw}, nil
	case "trojan":
		raw := map[string]any{"type": "trojan", "server": host, "server_port": proxy.Port, "password": strings.TrimSpace(proxy.Password)}
		if proxy.TLS {
			raw["security"] = "tls"
		}
		if proxy.SNI != "" {
			raw["sni"] = strings.TrimSpace(proxy.SNI)
		} else if proxy.ServerName != "" {
			raw["sni"] = strings.TrimSpace(proxy.ServerName)
		}
		if proxy.Network != "" {
			raw["network"] = strings.TrimSpace(proxy.Network)
		}
		if proxy.Host != "" {
			raw["host"] = strings.TrimSpace(proxy.Host)
		}
		if proxy.Path != "" {
			raw["path"] = strings.TrimSpace(proxy.Path)
		}
		return domain.NodeMetadata{ID: buildNodeID("trojan", host, proxy.Port, proxy.Password), Name: name, Protocol: "trojan", Address: host, Port: proxy.Port, RawConfig: raw}, nil
	case "vmess":
		raw := map[string]any{"type": "vmess", "server": host, "server_port": proxy.Port, "uuid": strings.TrimSpace(proxy.UUID)}
		if proxy.Cipher != "" {
			raw["cipher"] = strings.TrimSpace(proxy.Cipher)
		}
		if proxy.AlterID >= 0 {
			raw["alter_id"] = proxy.AlterID
		}
		if proxy.TLS {
			raw["security"] = "tls"
		}
		if proxy.SNI != "" {
			raw["sni"] = strings.TrimSpace(proxy.SNI)
		} else if proxy.ServerName != "" {
			raw["sni"] = strings.TrimSpace(proxy.ServerName)
		}
		if proxy.Network != "" {
			raw["network"] = strings.TrimSpace(proxy.Network)
		}
		if proxy.Host != "" {
			raw["host"] = strings.TrimSpace(proxy.Host)
		}
		if proxy.Path != "" {
			raw["path"] = strings.TrimSpace(proxy.Path)
		}
		return domain.NodeMetadata{ID: buildNodeID("vmess", host, proxy.Port, proxy.UUID), Name: name, Protocol: "vmess", Address: host, Port: proxy.Port, RawConfig: raw}, nil
	case "vless":
		raw := map[string]any{"type": "vless", "server": host, "server_port": proxy.Port, "uuid": strings.TrimSpace(proxy.UUID)}
		if proxy.TLS {
			raw["security"] = "tls"
		}
		if proxy.SNI != "" {
			raw["sni"] = strings.TrimSpace(proxy.SNI)
		} else if proxy.ServerName != "" {
			raw["sni"] = strings.TrimSpace(proxy.ServerName)
		}
		if proxy.Network != "" {
			raw["network"] = strings.TrimSpace(proxy.Network)
		}
		if proxy.Host != "" {
			raw["host"] = strings.TrimSpace(proxy.Host)
		}
		if proxy.Path != "" {
			raw["path"] = strings.TrimSpace(proxy.Path)
		}
		return domain.NodeMetadata{ID: buildNodeID("vless", host, proxy.Port, proxy.UUID), Name: name, Protocol: "vless", Address: host, Port: proxy.Port, RawConfig: raw}, nil
	case "ss":
		raw := map[string]any{"type": "shadowsocks", "server": host, "server_port": proxy.Port, "method": strings.TrimSpace(proxy.Cipher), "password": strings.TrimSpace(proxy.Password)}
		return domain.NodeMetadata{ID: buildNodeID("shadowsocks", host, proxy.Port, proxy.Cipher+":"+proxy.Password), Name: name, Protocol: "shadowsocks", Address: host, Port: proxy.Port, RawConfig: raw}, nil
	case "hysteria2", "hy2":
		raw := map[string]any{"type": "hysteria2", "server": host, "server_port": proxy.Port, "password": strings.TrimSpace(proxy.Password)}
		if proxy.TLS {
			raw["security"] = "tls"
		}
		if proxy.SNI != "" {
			raw["sni"] = strings.TrimSpace(proxy.SNI)
		} else if proxy.ServerName != "" {
			raw["sni"] = strings.TrimSpace(proxy.ServerName)
		}
		return domain.NodeMetadata{ID: buildNodeID("hysteria2", host, proxy.Port, proxy.Password), Name: name, Protocol: "hysteria2", Address: host, Port: proxy.Port, RawConfig: raw}, nil
	default:
		return domain.NodeMetadata{}, fmt.Errorf("unsupported clash proxy type: %s", protocol)
	}
}
