package main

import (
	"emby2alist/internal/config"
	"emby2alist/internal/server"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"os"
)

func main() {
	// 自定义日志格式，移除日志级别前缀，使用斜杠分隔日期
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006/01/02 15:04:05",
		DisableColors:   true,
		DisableLevelTruncation: true,
		ForceQuote:      false,
	})
	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(logrus.InfoLevel)

	// 必须存在 config.yaml
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		logrus.Fatalf("无法加载 config.yaml: %v", err)
	}

	gin.SetMode(gin.ReleaseMode)
	srv := server.NewServer(cfg)
	
	// 初始化日志配置
	server.InitLogger(cfg)

	logrus.Info("=== Emby2Alist Lite (HttpStrm Only) ===")
	logrus.Infof("监听端口: %d", cfg.Port)
	logrus.Infof("媒体服务器: %s (%s)", cfg.Server.Addr, cfg.Server.Type)
	logrus.Infof("HttpStrm 功能: %v", cfg.HttpStrm.Enable)
	logrus.Infof("自动解析 302: %v", cfg.HttpStrm.ResolveStrmLinks)
	logrus.Infof("缓存功能: %v (TTL: %s)", cfg.Cache.Enable, cfg.Cache.HttpStrmTTL)
	if cfg.Client.Enable {
		logrus.Infof("客户端过滤: 开启 (模式: %s, 数量: %d)", cfg.Client.Mode, len(cfg.Client.List))
	} else {
		logrus.Info("客户端过滤: 关闭")
	}
	
	if err := srv.Run(fmt.Sprintf(":%d", cfg.Port)); err != nil {
		logrus.Fatalf("Start Error: %v", err)
	}
}