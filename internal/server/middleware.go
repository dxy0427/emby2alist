package server

import (
	"emby2alist/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"net/http"
	"strings"
)

// ClientFilter 客户端过滤器中间件
func ClientFilter(cfg *config.ClientFilterConf) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !cfg.Enable {
			c.Next()
			return
		}

		userAgent := c.Request.UserAgent()
		var allowed bool

		// 如果没有 UA，在开启过滤的情况下通常视为禁止
		if userAgent == "" {
			allowed = false
		} else {
			switch strings.ToLower(cfg.Mode) {
			case "whitelist": // 白名单模式：必须包含列表中的关键词才允许
				allowed = false
				for _, ua := range cfg.List {
					if strings.Contains(userAgent, ua) {
						allowed = true
						break
					}
				}
			case "blacklist": // 黑名单模式：包含列表中的关键词则禁止
				allowed = true
				for _, ua := range cfg.List {
					if strings.Contains(userAgent, ua) {
						allowed = false
						break
					}
				}
			default:
				// 默认放行，避免配置错误导致全断
				allowed = true
			}
		}

		if !allowed {
			logrus.Infof("[ClientFilter] 拦截请求 UA: %s", userAgent)
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		
		// 调试日志：如果需要查看放行的 UA，取消注释
		// logrus.Debugf("[ClientFilter] 放行请求 UA: %s", userAgent)
		c.Next()
	}
}