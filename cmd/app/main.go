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

	// 必须存在 config.yaml
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		logrus.Fatalf("无法加载 config.yaml: %v", err)
	}

	gin.SetMode(gin.ReleaseMode)
	srv := server.NewServer(cfg)

	logrus.Info("=== Emby2Alist Lite (HttpStrm Only) ===")
	logrus.Infof("Port: %d", cfg.Port)
	logrus.Infof("Server: %s (%s)", cfg.Server.Addr, cfg.Server.Type)
	logrus.Infof("HttpStrm Enable: %v", cfg.HttpStrm.Enable)
	logrus.Infof("Auto Resolve 302: %v", cfg.HttpStrm.ResolveStrmLinks)
	
	if err := srv.Run(fmt.Sprintf(":%d", cfg.Port)); err != nil {
		logrus.Fatalf("Start Error: %v", err)
	}
}