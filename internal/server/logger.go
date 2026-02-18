package server

import (
	"emby2alist/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// verboseEnabled 是否启用详细日志
var verboseEnabled bool

// InitLogger 初始化日志配置
func InitLogger(cfg *config.Config) {
	verboseEnabled = cfg.Logging.Verbose
}

// customGinLogFormat 自定义 GIN 日志格式
func customGinLogFormat(param gin.LogFormatterParams) string {
	// 如果启用了详细日志，记录所有请求
	if verboseEnabled {
		if param.StatusCode >= 400 {
			logrus.WithFields(logrus.Fields{
				"status":     param.StatusCode,
				"method":     param.Method,
				"path":       param.Path,
				"ip":         param.ClientIP,
				"latency":    param.Latency,
				"user_agent": param.Request.UserAgent(),
				"error":      param.ErrorMessage,
			}).Errorf("HTTP Request Failed")
		} else {
			logrus.WithFields(logrus.Fields{
				"status":  param.StatusCode,
				"method":  param.Method,
				"path":    param.Path,
				"ip":      param.ClientIP,
				"latency": param.Latency,
			}).Infof("HTTP Request Success")
		}
		return ""
	}
	
	// 如果未启用详细日志，只记录错误请求（4xx, 5xx）
	if param.StatusCode >= 400 {
		logrus.WithFields(logrus.Fields{
			"status":     param.StatusCode,
			"method":     param.Method,
			"path":       param.Path,
			"ip":         param.ClientIP,
			"latency":    param.Latency,
			"user_agent": param.Request.UserAgent(),
			"error":      param.ErrorMessage,
		}).Errorf("HTTP Request Failed")
		return ""
	}
	
	// 对于成功的请求，不记录详细日志，减少日志噪音
	return ""
}