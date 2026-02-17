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
	httpClient  *http.Client
}

func NewServer(cfg *config.Config) *Server {
	s := &Server{
		engine: gin.Default(),
		cfg:    cfg,
		cache:  cache.New(10*time.Minute, 20*time.Minute),
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
	s.reloadClients()
	s.setupRoutes()
	return s
}

func (s *Server) reloadClients() {
	embyHost := s.cfg.EmbyHost
	if !strings.HasPrefix(embyHost, "http://") && !strings.HasPrefix(embyHost, "https://") {
		embyHost = "http://" + embyHost
	}

	alistHost := s.cfg.AlistHost
	if !strings.HasPrefix(alistHost, "http://") && !strings.HasPrefix(alistHost, "https://") {
		alistHost = "http://" + alistHost
	}

	s.mediaServer = mediaserver.NewClient(s.cfg.BackendType, embyHost, s.cfg.EmbyApiKey)
	s.alist = alist.NewClient(
		alistHost,
		s.cfg.AlistToken,
		s.cfg.AlistPublicHost,
		s.cfg.AlistSignEnable,
		s.cfg.AlistSignSalt,
		s.cfg.AlistUaPassthrough, // 传入配置
	)
}

func (s *Server) setupRoutes() {
	s.engine.GET("/admin", web.AdminPage)
	s.engine.GET("/api/config", s.getConfig)
	s.engine.POST("/api/config", s.updateConfig)
	s.engine.NoRoute(s.mainHandler)
}

func (s *Server) Run(addr string) error {
	return s.engine.Run(addr)
}

func (s *Server) mainHandler(c *gin.Context) {
	if strings.Contains(c.Request.URL.Path, "/PlaybackInfo") {
		logrus.Infof("拦截 PlaybackInfo: %s", c.ClientIP())
		s.ReverseProxy(c, true)
		return
	}

	path := c.Request.URL.Path
	isStream := strings.Contains(path, "/stream") || strings.Contains(path, "/Download") || strings.Contains(path, "/original")

	if !isStream {
		s.ReverseProxy(c, false)
		return
	}

	itemId := extractItemId(path)
	mediaSourceId := c.Query("MediaSourceId")
	if mediaSourceId == "" {
		mediaSourceId = c.Query("mediaSourceId")
	}

	logrus.Infof("播放请求: ItemID=%s SourceID=%s", itemId, mediaSourceId)

	realPath, _, err := s.mediaServer.GetItemInfo(itemId, mediaSourceId)
	if err != nil {
		logrus.Errorf("获取路径失败: %v, 回源代理", err)
		s.ReverseProxy(c, false)
		return
	}

	targetPath := realPath
	for _, mapping := range s.cfg.PathMappings {
		if mapping.Old != "" {
			targetPath = strings.Replace(targetPath, mapping.Old, mapping.New, 1)
		}
	}
	targetPath = strings.ReplaceAll(targetPath, "\\", "/")

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

	if strings.HasPrefix(targetPath, "http") {
		if s.cfg.ResolveStrmLinks {
			logrus.Infof("正在解析 Strm 链接: %s", targetPath)
			req, _ := http.NewRequest("GET", targetPath, nil)
			req.Header.Set("User-Agent", c.Request.UserAgent()) // Strm 解析也透传 UA
			
			resp, err := s.httpClient.Do(req)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode >= 300 && resp.StatusCode < 400 {
					location := resp.Header.Get("Location")
					if location != "" {
						logrus.Infof("解析成功! 真实地址: %s", location)
						targetPath = location
					}
				}
			} else {
				logrus.Warnf("解析 Strm 链接失败: %v, 将使用原始链接", err)
			}
		}

		logrus.Infof("Strm直连跳转: %s", targetPath)
		c.Redirect(http.StatusFound, targetPath)
		return
	}

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

	// 调用 Alist API，传入 UA
	rawUrl, err := s.alist.GetFsGet(targetPath, c.Request.UserAgent())
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

func (s *Server) getConfig(c *gin.Context) {
	c.JSON(200, s.cfg)
}
func (s *Server) updateConfig(c *gin.Context) {
	var newCfg config.Config
	if err := c.ShouldBindJSON(&newCfg); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	newCfg.ServerPort = s.cfg.ServerPort
	s.cfg.Update(&newCfg)
	s.cfg.Save("config.yaml")
	s.reloadClients()
	c.JSON(200, gin.H{"status": "success"})
}