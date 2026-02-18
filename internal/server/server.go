package server

import (
	"emby2alist/internal/config"
	"emby2alist/internal/pkg/mediaserver"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Server struct {
	engine      *gin.Engine
	cfg         *config.Config
	mediaServer mediaserver.MediaServerClient
	httpClient  *http.Client
	// 缓存相关
	urlCache sync.Map      // 存储解析后的 URL
	cacheTTL time.Duration // 缓存有效期
}

type cachedURL struct {
	url       string
	expiresAt time.Time
}

func NewServer(cfg *config.Config) *Server {
	// 解析 TTL
	ttl, err := time.ParseDuration(cfg.Cache.HttpStrmTTL)
	if err != nil {
		logrus.Warnf("缓存 TTL 解析失败，使用默认值 10m: %v", err)
		ttl = 10 * time.Minute
	}

	s := &Server{
		engine: gin.Default(),
		cfg:    cfg,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse // 禁止自动跳转，手动处理
			},
		},
		cacheTTL: ttl,
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
	s.engine.NoRoute(s.mainHandler)
}

func (s *Server) Run(addr string) error {
	return s.engine.Run(addr)
}