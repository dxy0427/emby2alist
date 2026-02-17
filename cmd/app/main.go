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
	// 配置日志
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})
	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(logrus.InfoLevel)

	// 加载或创建配置
	cfgPath := "config.yaml"
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		logrus.Warnf("加载配置失败，将生成默认配置: %v", err)
		cfg = config.DefaultConfig()
		_ = cfg.Save(cfgPath)
	}

	// 设置 Gin 模式
	gin.SetMode(gin.ReleaseMode)

	// 初始化服务
	srv := server.NewServer(cfg)

	logrus.Info("=======================================")
	logrus.Info("      Emby2Alist Go (精简重构版)      ")
	logrus.Infof(" 服务端口: %d", cfg.Port)
	logrus.Infof(" 媒体服务器: %s (%s)", cfg.Server.Addr, cfg.Server.Type)
	if cfg.HttpStrm.Enable {
		logrus.Infof(" HTTP Strm: 启用 (解析真实链接: %v)", cfg.HttpStrm.ResolveStrmLinks)
	}
	if cfg.AlistStrm.Enable {
		logrus.Infof(" Alist Strm: 启用 (对接: %s)", cfg.AlistStrm.AlistHost)
	}
	logrus.Info("=======================================")

	// 启动 HTTP 服务
	if err := srv.Run(fmt.Sprintf(":%d", cfg.Port)); err != nil {
		logrus.Fatalf("服务启动失败: %v", err)
	}
}