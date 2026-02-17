package server

import (
	"emby2alist/internal/config"
	"emby2alist/internal/pkg/alist"
	"emby2alist/internal/pkg/mediaserver"
	"emby2alist/internal/web"
	"github.com/gin-gonic/gin"
	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type Server struct {
	engine      *gin.Engine
	cfg         *config.Config
	mediaServer mediaserver.MediaServerClient
	alist       *alist.Client
	cache       *cache.Cache
}

func NewServer(cfg *config.Config) *Server {
	s := &Server{
		engine: gin.Default(),
		cfg:    cfg,
		cache:  cache.New(10*time.Minute, 20*time.Minute),
	}
	s.reloadClients()
	s.setupRoutes()
	return s
}

func (s *Server) reloadClients() {
	s.mediaServer = mediaserver.NewClient(s.cfg.BackendType, s.cfg.EmbyHost, s.cfg.EmbyApiKey)
	s.alist = alist.NewClient(
		s.cfg.AlistHost,
		s.cfg.AlistToken,
		s.cfg.AlistPublicHost,
		s.cfg.AlistSignEnable,
		s.cfg.AlistSignSalt,
	)
}

func (s *Server) setupRoutes() {
	// Web UI
	s.engine.GET("/admin", web.AdminPage)
	s.engine.GET("/api/config", s.getConfig)
	s.engine.POST("/api/config", s.updateConfig)

	// 通用代理
	s.engine.NoRoute(s.mainHandler)
}

func (s *Server) Run(addr string) error {
	return s.engine.Run(addr)
}

func (s *Server) mainHandler(c *gin.Context) {
	// 1. PlaybackInfo 拦截 (ModifyResponse)
	if strings.Contains(c.Request.URL.Path, "/PlaybackInfo") {
		logrus.Infof("拦截 PlaybackInfo: %s", c.ClientIP())
		s.ReverseProxy(c, true)
		return
	}

	// 2. 识别流媒体请求
	// Emby: /emby/videos/{Id}/stream
	// Jellyfin: /videos/{Id}/stream
	// Download: /Items/{Id}/Download
	path := c.Request.URL.Path
	isStream := strings.Contains(path, "/stream") || strings.Contains(path, "/Download") || strings.Contains(path, "/original")
	
	if !isStream {
		s.ReverseProxy(c, false)
		return
	}

	// 3. 提取 ItemID
	itemId := extractItemId(path)
	mediaSourceId := c.Query("MediaSourceId")
	if mediaSourceId == "" { mediaSourceId = c.Query("mediaSourceId") }

	logrus.Infof("播放请求: ItemID=%s SourceID=%s", itemId, mediaSourceId)

	// 获取真实路径
	realPath, _, err := s.mediaServer.GetItemInfo(itemId, mediaSourceId)
	if err != nil {
		logrus.Errorf("获取路径失败: %v, 回源代理", err)
		s.ReverseProxy(c, false)
		return
	}

	// 4. 规则引擎
	ctx := map[string]interface{}{
		"uri":         c.Request.RequestURI,
		"remote_addr": c.ClientIP(),
		"ua":          c.Request.UserAgent(),
		"filePath":    realPath,
		"userId":      c.Query("UserId"),
	}
	
	matched, mode := MatchMode(s.cfg.RouteRules, ctx)
	if matched {
		logrus.Infof("规则命中: Mode=%s", mode)
		if mode == "block" {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		if mode == "proxy" {
			s.ReverseProxy(c, false)
			return
		}
	}

	// 5. Strm 文件 (Http 链接) 直接跳转
	if strings.HasPrefix(realPath, "http") {
		logrus.Infof("Strm直连: %s", realPath)
		c.Redirect(http.StatusFound, realPath)
		return
	}

	// 6. 本地文件处理 (挂载检测)
	isMounted := false
	for _, mnt := range s.cfg.MountPaths {
		if strings.HasPrefix(realPath, mnt) {
			isMounted = true
			break
		}
	}
	
	if !isMounted {
		logrus.Debugf("非挂载路径: %s, 回源代理", realPath)
		s.ReverseProxy(c, false)
		return
	}

	// 7. 路径映射
	alistPath := realPath
	for _, mapping := range s.cfg.PathMappings {
		if mapping.Old != "" {
			alistPath = strings.Replace(alistPath, mapping.Old, mapping.New, 1)
		}
	}
	alistPath = strings.ReplaceAll(alistPath, "\\", "/")

	// 8. 请求 Alist
	rawUrl, err := s.alist.GetFsGet(alistPath)
	if err != nil {
		logrus.Warnf("Alist 获取失败: %v, 回源代理", err)
		s.ReverseProxy(c, false)
		return
	}

	logrus.Infof("重定向 -> %s", rawUrl)
	c.Redirect(http.StatusFound, rawUrl)
}

func extractItemId(path string) string {
	re := regexp.MustCompile(`/(?:videos|items)/([a-zA-Z0-9\-]+)/`)
	matches := re.FindStringSubmatch(strings.ToLower(path))
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

// 配置 API
func (s *Server) getConfig(c *gin.Context) {
	c.JSON(200, s.cfg)
}
func (s *Server) updateConfig(c *gin.Context) {
	var newCfg config.Config
	if err := c.ShouldBindJSON(&newCfg); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	newCfg.ServerPort = s.cfg.ServerPort // 保护端口不被修改
	s.cfg.Update(&newCfg)
	s.cfg.Save("config.yaml")
	s.reloadClients()
	c.JSON(200, gin.H{"status": "success"})
}