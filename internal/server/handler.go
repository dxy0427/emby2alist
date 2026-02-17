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
	httpClient  *http.Client // 专用 HTTP Client
}

func NewServer(cfg *config.Config) *Server {
	s := &Server{
		engine: gin.Default(),
		cfg:    cfg,
		cache:  cache.New(10*time.Minute, 20*time.Minute),
		// 创建一个不自动跟随跳转的 Client，专门用来获取 302 Location
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse // 遇到跳转时停止，返回 Response
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
	// 1. PlaybackInfo 拦截
	if strings.Contains(c.Request.URL.Path, "/PlaybackInfo") {
		logrus.Infof("拦截 PlaybackInfo: %s", c.ClientIP())
		s.ReverseProxy(c, true)
		return
	}

	// 2. 识别流媒体请求
	path := c.Request.URL.Path
	isStream := strings.Contains(path, "/stream") || strings.Contains(path, "/Download") || strings.Contains(path, "/original")

	if !isStream {
		s.ReverseProxy(c, false)
		return
	}

	// 3. 提取 ItemID
	itemId := extractItemId(path)
	mediaSourceId := c.Query("MediaSourceId")
	if mediaSourceId == "" {
		mediaSourceId = c.Query("mediaSourceId")
	}

	logrus.Infof("播放请求: ItemID=%s SourceID=%s", itemId, mediaSourceId)

	// 获取真实路径 (原始路径)
	realPath, _, err := s.mediaServer.GetItemInfo(itemId, mediaSourceId)
	if err != nil {
		logrus.Errorf("获取路径失败: %v, 回源代理", err)
		s.ReverseProxy(c, false)
		return
	}

	// 4. 路径映射 (手动修复逻辑保留，优先级高)
	targetPath := realPath
	for _, mapping := range s.cfg.PathMappings {
		if mapping.Old != "" {
			targetPath = strings.Replace(targetPath, mapping.Old, mapping.New, 1)
		}
	}
	targetPath = strings.ReplaceAll(targetPath, "\\", "/")

	// 5. 规则引擎
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

	// ==========================================
	// 6. Strm 文件 (Http 链接) 处理核心逻辑
	// ==========================================
	if strings.HasPrefix(targetPath, "http") {
		// 检查是否开启了“解析 Strm 重定向”功能
		if s.cfg.ResolveStrmLinks {
			logrus.Infof("正在解析 Strm 链接: %s", targetPath)
			
			// 发起请求，不跟随跳转
			req, _ := http.NewRequest("GET", targetPath, nil)
			// 可选：伪装 UA 防止被拒绝
			req.Header.Set("User-Agent", "Mozilla/5.0")
			
			resp, err := s.httpClient.Do(req)
			if err == nil {
				defer resp.Body.Close()
				// 如果状态码是 3xx，获取 Location
				if resp.StatusCode >= 300 && resp.StatusCode < 400 {
					location := resp.Header.Get("Location")
					if location != "" {
						logrus.Infof("解析成功! 真实地址: %s", location)
						targetPath = location
					}
				} else if resp.StatusCode == 200 {
					// 某些情况下虽然没跳，但我们可能就想用这个链接（如 Alist 直链）
					// 这里不做额外处理，直接用原始链接
				}
			} else {
				logrus.Warnf("解析 Strm 链接失败: %v, 将使用原始链接", err)
			}
		}

		logrus.Infof("Strm直连跳转: %s", targetPath)
		c.Redirect(http.StatusFound, targetPath)
		return
	}

	// 7. 本地文件处理 (挂载检测)
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

	// 8. 请求 Alist
	rawUrl, err := s.alist.GetFsGet(targetPath)
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