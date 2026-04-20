package security

import (
	"net/url"
	"strings"
)

type originPattern struct {
	scheme string
	host   string // 不含端口
	port   string // "" 表示无端口要求；"*" 表示任意端口
}

// OriginChecker 检查 Origin header 是否在白名单内。
// 支持精确匹配（http://panel.example.com）和端口通配（http://127.0.0.1:*）。
type OriginChecker struct {
	patterns []originPattern
}

// NewOriginChecker 用允许的 origin 列表构造 checker。
// 无效模式被静默忽略。空列表会拒绝所有 origin。
func NewOriginChecker(allowed []string) *OriginChecker {
	c := &OriginChecker{}
	for _, raw := range allowed {
		p, ok := parsePattern(raw)
		if ok {
			c.patterns = append(c.patterns, p)
		}
	}
	return c
}

// Allow 判断 origin 是否被允许。空 origin 一律拒绝（防止无 Origin 的 curl/CLI）。
func (c *OriginChecker) Allow(origin string) bool {
	if origin == "" || len(c.patterns) == 0 {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}
	host, port := splitHostPort(u.Host)
	for _, p := range c.patterns {
		if p.scheme != u.Scheme {
			continue
		}
		if p.host != host {
			continue
		}
		if p.port == "*" {
			if port == "" {
				continue // 模式要求端口（任意），但 origin 无端口
			}
			return true
		}
		if p.port == port {
			return true
		}
	}
	return false
}

func parsePattern(raw string) (originPattern, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return originPattern{}, false
	}
	// 处理端口通配 http://host:*
	wild := strings.HasSuffix(raw, ":*")
	if wild {
		raw = strings.TrimSuffix(raw, ":*")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return originPattern{}, false
	}
	host, port := splitHostPort(u.Host)
	if wild {
		port = "*"
	}
	return originPattern{scheme: u.Scheme, host: host, port: port}, true
}

func splitHostPort(hostport string) (string, string) {
	// IPv6 [::1]:port 处理
	if strings.HasPrefix(hostport, "[") {
		if end := strings.Index(hostport, "]"); end > 0 {
			host := hostport[1:end]
			if end+1 < len(hostport) && hostport[end+1] == ':' {
				return host, hostport[end+2:]
			}
			return host, ""
		}
	}
	idx := strings.LastIndex(hostport, ":")
	if idx < 0 {
		return hostport, ""
	}
	return hostport[:idx], hostport[idx+1:]
}
