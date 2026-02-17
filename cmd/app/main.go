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
		logrus.Warnf("加载配置失败，使用默认配置: %v", err)
		cfg = config.DefaultConfig()
		_ = cfg.Save(cfgPath)
	}

	// 设置 Gin 模式
	gin.SetMode(gin.ReleaseMode)

	// 初始化服务
	srv := server.NewServer(cfg)

	logrus.Info("=======================================")
	logrus.Info("      Emby2Alist Go (完整重构版)      ")
	logrus.Infof(" 服务端口: %d", cfg.ServerPort)
	logrus.Infof(" 管理后台: http://localhost:%d/admin", cfg.ServerPort)
	logrus.Infof(" 后端模式: %s", cfg.BackendType)
	logrus.Info("=======================================")

	// 启动 HTTP 服务
	if err := srv.Run(fmt.Sprintf(":%d", cfg.ServerPort)); err != nil {
		logrus.Fatalf("服务启动失败: %v", err)
	}
}