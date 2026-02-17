package server

import (
	"emby2alist/internal/config"
	"emby2alist/internal/pkg/alist"
	"emby2alist/internal/pkg/mediaserver"
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
	embyHost := s.cfg.Server.Addr
	if !strings.HasPrefix(embyHost, "http://") && !strings.HasPrefix(embyHost, "https://") {
		embyHost = "http://" + embyHost
	}

	alistHost := s.cfg.AlistStrm.AlistHost
	if !strings.HasPrefix(alistHost, "http://") && !strings.HasPrefix(alistHost, "https://") {
		alistHost = "http://" + alistHost
	}

	s.mediaServer = mediaserver.NewClient(s.cfg.Server.Type, embyHost, s.cfg.Server.Auth)
	s.alist = alist.NewClient(
		alistHost,
		s.cfg.AlistStrm.AlistToken,
		s.cfg.AlistStrm.AlistPublicHost,
		s.cfg.AlistStrm.UaPassthrough,
	)
}

func (s *Server) setupRoutes() {
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

	// 1. 处理 HTTPStrm
	if strings.HasPrefix(realPath, "http") {
		if !s.cfg.HttpStrm.Enable {
			s.ReverseProxy(c, false)
			return
		}

		targetPath := realPath
		// HTTP 路径替换
		for _, m := range s.cfg.HttpStrm.PathMappings {
			if m.Old != "" {
				targetPath = strings.Replace(targetPath, m.Old, m.New, 1)
			}
		}

		// 自动解析重定向 (resolve_strm_links)
		if s.cfg.HttpStrm.ResolveStrmLinks {
			logrus.Infof("解析 Strm 链接: %s", targetPath)
			req, _ := http.NewRequest("GET", targetPath, nil)
			req.Header.Set("User-Agent", c.Request.UserAgent())
			resp, err := s.httpClient.Do(req)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode >= 300 && resp.StatusCode < 400 {
					loc := resp.Header.Get("Location")
					if loc != "" {
						logrus.Infof("解析成功: %s", loc)
						targetPath = loc
					}
				}
			} else {
				logrus.Warnf("解析失败: %v", err)
			}
		}

		logrus.Infof("[HttpStrm] 跳转: %s", targetPath)
		c.Redirect(http.StatusFound, targetPath)
		return
	}

	// 2. 处理 AlistStrm (本地路径)
	if !s.cfg.AlistStrm.Enable {
		s.ReverseProxy(c, false)
		return
	}

	alistPath := realPath
	// 路径映射
	matched := false
	for _, m := range s.cfg.AlistStrm.PathMappings {
		if m.Old != "" && strings.HasPrefix(realPath, m.Old) {
			alistPath = strings.Replace(realPath, m.Old, m.New, 1)
			matched = true
			break
		}
	}
	
	if !matched {
		// 没有匹配到映射规则，说明不是网盘文件，走代理
		logrus.Debugf("未匹配到 Alist 映射规则: %s", realPath)
		s.ReverseProxy(c, false)
		return
	}

	alistPath = strings.ReplaceAll(alistPath, "\\", "/")
	logrus.Infof("请求 Alist: %s", alistPath)

	rawUrl, err := s.alist.GetFsGet(alistPath, c.Request.UserAgent())
	if err != nil {
		logrus.Warnf("Alist 获取失败: %v", err)
		s.ReverseProxy(c, false)
		return
	}

	logrus.Infof("[AlistStrm] 跳转: %s", rawUrl)
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