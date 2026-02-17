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
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})
	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(logrus.InfoLevel)

	// 必须存在 config.yaml，否则报错退出
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		logrus.Fatalf("无法加载 config.yaml: %v", err)
	}

	gin.SetMode(gin.ReleaseMode)
	srv := server.NewServer(cfg)

	logrus.Info("=== Emby2Alist Configured Mode ===")
	logrus.Infof("Port: %d", cfg.Port)
	logrus.Infof("Server: %s (%s)", cfg.Server.Addr, cfg.Server.Type)
	logrus.Infof("HttpStrm: %v", cfg.HttpStrm.Enable)
	logrus.Infof("AlistStrm: %v", cfg.AlistStrm.Enable)
	
	if err := srv.Run(fmt.Sprintf(":%d", cfg.Port)); err != nil {
		logrus.Fatalf("Start Error: %v", err)
	}
}