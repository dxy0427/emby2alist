package server

import (
	"emby2alist/internal/config"
	"net"
	"regexp"
	"strings"
)

// MatchMode 执行规则匹配
// 逻辑：Rule.Group 相同的规则视为 AND 关系；不同 Group 之间视为 OR 关系。
func MatchMode(rules []config.RouteRule, ctx map[string]interface{}) (bool, string) {
	groups := make(map[string][]config.RouteRule)
	noGroupRules := []config.RouteRule{}

	for _, r := range rules {
		if r.Group == "" {
			noGroupRules = append(noGroupRules, r)
		} else {
			groups[r.Group] = append(groups[r.Group], r)
		}
	}

	// 1. 检查无分组规则 (OR)
	for _, r := range noGroupRules {
		if checkRule(r, ctx) {
			return true, r.Mode
		}
	}

	// 2. 检查有分组规则 (AND within Group, OR between Groups)
	for _, gRules := range groups {
		allMatch := true
		mode := ""
		for _, r := range gRules {
			mode = r.Mode
			if !checkRule(r, ctx) {
				allMatch = false
				break
			}
		}
		if allMatch && len(gRules) > 0 {
			return true, mode
		}
	}

	return false, ""
}

func checkRule(r config.RouteRule, ctx map[string]interface{}) bool {
	valRaw, ok := ctx[r.Target]
	if !ok {
		return false
	}
	val := valRaw.(string)

	switch r.Matcher {
	case "contains":
		return strings.Contains(val, r.Value)
	case "startsWith":
		return strings.HasPrefix(val, r.Value)
	case "endsWith":
		return strings.HasSuffix(val, r.Value)
	case "eq":
		return val == r.Value
	case "neq":
		return val != r.Value
	case "regex":
		ok, _ := regexp.MatchString(r.Value, val)
		return ok
	case "cidr":
		_, ipNet, err := net.ParseCIDR(r.Value)
		if err == nil {
			ip := net.ParseIP(val)
			return ipNet.Contains(ip)
		}
	}
	return false
}