package server

import (
	"emby2alist/internal/config"
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
	httpClient  *http.Client
	cache       *cache.Cache
}

func NewServer(cfg *config.Config) *Server {
	// 初始化缓存，默认过期时间由配置决定，清理周期设为 10 分钟
	c := cache.New(cfg.Cache.HttpStrmTTL, 10*time.Minute)

	s := &Server{
		engine: gin.Default(),
		cfg:    cfg,
		cache:  c,
		httpClient: &http.Client{
			Timeout: 10 * time.Second, // 稍微增加超时时间以适应重定向解析
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

	s.mediaServer = mediaserver.NewClient(s.cfg.Server.Type, embyHost, s.cfg.Server.Auth)
}

func (s *Server) setupRoutes() {
	// 注册客户端过滤器中间件
	s.engine.Use(ClientFilter(&s.cfg.Client))
	
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

	itemId := extractItemId(path)
	mediaSourceId := c.Query("MediaSourceId")
	if mediaSourceId == "" {
		mediaSourceId = c.Query("mediaSourceId")
	}

	logrus.Infof("播放请求: ItemID=%s SourceID=%s", itemId, mediaSourceId)

	realPath, err := s.mediaServer.GetItemInfo(itemId, mediaSourceId)
	if err != nil {
		logrus.Errorf("获取路径失败: %v, 回源代理", err)
		s.ReverseProxy(c, false)
		return
	}

	// 3. 处理 HTTPStrm (HttpStrm Mode)
	// 本模式不处理本地文件，只处理 http 开头的直链
	if strings.HasPrefix(realPath, "http") {
		if !s.cfg.HttpStrm.Enable {
			s.ReverseProxy(c, false)
			return
		}

		// --- 缓存检查 ---
		// 缓存键使用原始路径即可，因为后续处理是固定的
		cacheKey := "strm:" + realPath
		if s.cfg.Cache.Enable {
			if cachedURL, found := s.cache.Get(cacheKey); found {
				logrus.Infof("[Cache] 命中缓存，直接跳转: %s", cachedURL.(string))
				c.Redirect(http.StatusFound, cachedURL.(string))
				return
			}
		}
		// ---------------

		targetPath := realPath
		// 3.1 路径替换
		for _, m := range s.cfg.HttpStrm.PathMappings {
			if m.Old != "" {
				targetPath = strings.Replace(targetPath, m.Old, m.New, 1)
			}
		}

		// 3.2 自动解析 302
		if s.cfg.HttpStrm.ResolveStrmLinks {
			logrus.Infof("解析 Strm 链接: %s", targetPath)
			req, _ := http.NewRequest("GET", targetPath, nil)
			
			// UA 透传
			if s.cfg.HttpStrm.UaPassthrough {
				req.Header.Set("User-Agent", c.Request.UserAgent())
			} else {
				req.Header.Set("User-Agent", "Mozilla/5.0")
			}

			resp, err := s.httpClient.Do(req)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode >= 300 && resp.StatusCode < 400 {
					loc := resp.Header.Get("Location")
					if loc != "" {
						logrus.Infof("解析成功: %s", loc)
						targetPath = loc
						
						// --- 写入缓存 ---
						if s.cfg.Cache.Enable {
							s.cache.Set(cacheKey, targetPath, cache.DefaultExpiration)
							logrus.Infof("[Cache] 已缓存直链，TTL: %s", s.cfg.Cache.HttpStrmTTL)
						}
						// --------------
					}
				}
			} else {
				logrus.Warnf("解析失败: %v", err)
			}
		} else {
			// 如果没有开启解析302，但开启了缓存，也缓存替换后的路径（尽管此时开销较小，但仍能节省正则/字符串替换开销）
			if s.cfg.Cache.Enable {
				s.cache.Set(cacheKey, targetPath, cache.DefaultExpiration)
			}
		}

		logrus.Infof("[HttpStrm] 跳转: %s", targetPath)
		c.Redirect(http.StatusFound, targetPath)
		return
	}

	// 如果不是 http 开头，且没有 AlistStrm 模式，则直接代理回源
	s.ReverseProxy(c, false)
}

func extractItemId(path string) string {
	re := regexp.MustCompile(`/(?:videos|items)/([a-zA-Z0-9\-]+)/`)
	matches := re.FindStringSubmatch(strings.ToLower(path))
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}