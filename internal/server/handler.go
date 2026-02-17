package server

import (
"emby2alist/internal/config"
"emby2alist/internal/pkg/alist"
"emby2alist/internal/pkg/mediaserver"
"fmt"
"github.com/gin-gonic/gin"
"github.com/sirupsen/logrus"
"net/http"
"regexp"
"strings"
"time"
)

type Server struct {
engine *gin.Engine
cfg *config.Config
mediaServer mediaserver.MediaServerClient
alist *alist.Client
httpClient *http.Client
}

func NewServer(cfg *config.Config) *Server {
s := &Server{
engine: gin.Default(),
cfg: cfg,
httpClient: &http.Client{
Timeout: 10 * time.Second,
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

alistHost := s.cfg.AlistStrm.AlistHost
if alistHost != "" {
	if !strings.HasPrefix(alistHost, "http://") && !strings.HasPrefix(alistHost, "https://") {
		alistHost = "http://" + alistHost
	}
	s.alist = alist.NewClient(
		alistHost,
		s.cfg.AlistStrm.AlistToken,
		s.cfg.AlistStrm.AlistPublicHost,
		s.cfg.AlistStrm.AlistUaPassthrough,
	)
}

}

func (s *Server) setupRoutes() {
s.engine.NoRoute(s.mainHandler)
}

func (s *Server) Run(addr string) error {
return s.engine.Run(addr)
}

func (s *Server) mainHandler(c *gin.Context) {
// 拦截 PlaybackInfo 进行 JSON 修改 (禁用转码等)
if strings.Contains(c.Request.URL.Path, "/PlaybackInfo") {
logrus.Debugf("拦截 PlaybackInfo: %s", c.ClientIP())
s.ReverseProxy(c, true)
return
}

path := c.Request.URL.Path
isStream := strings.Contains(path, "/stream") || strings.Contains(path, "/Download") || strings.Contains(path, "/original")

// 如果不是流媒体请求，直接反向代理
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
	logrus.Errorf("获取 Emby 路径失败: %v, 回源代理", err)
	s.ReverseProxy(c, false)
	return
}

logrus.Infof("原始路径: %s", realPath)

// ===========================
// 场景 1: HTTP Strm 处理
// ===========================
if strings.HasPrefix(realPath, "http") {
	if !s.cfg.HttpStrm.Enable {
		logrus.Warn("检测到 HTTP 链接但 http_strm 未启用，回源代理")
		s.ReverseProxy(c, false)
		return
	}

	targetPath := realPath
	// 1. 全局路径映射 (old -> new)
	for _, mapping := range s.cfg.PathMappings {
		if mapping.Old != "" {
			targetPath = strings.Replace(targetPath, mapping.Old, mapping.New, 1)
		}
	}

	// 2. 解析 Strm 真实链接 (Resolve Redirect)
	if s.cfg.HttpStrm.ResolveStrmLinks {
		logrus.Infof("正在解析 Strm 链接: %s", targetPath)
		req, _ := http.NewRequest("GET", targetPath, nil)
		// 如果开启透传，带上客户端 UA
		if s.cfg.HttpStrm.AlistUaPassthrough {
			req.Header.Set("User-Agent", c.Request.UserAgent())
		}
		
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
			logrus.Warnf("解析 Strm 链接失败: %v, 将使用原始/映射后链接", err)
		}
	}

	logrus.Infof("HTTP Strm 跳转 -> %s", targetPath)
	c.Redirect(http.StatusFound, targetPath)
	return
}

// ===========================
// 场景 2: Alist 本地路径映射处理
// ===========================
if s.cfg.AlistStrm.Enable && s.alist != nil {
	// 路径标准化
	targetPath := strings.ReplaceAll(realPath, "\\", "/")
	
	// 应用 Alist 专用映射
	matched := false
	for _, mapping := range s.cfg.AlistStrm.PathMappings {
		if strings.HasPrefix(targetPath, mapping.Old) {
			targetPath = strings.Replace(targetPath, mapping.Old, mapping.New, 1)
			matched = true
			break
		}
	}

	// 如果没有匹配到任何映射，且路径不是以 / 开头（理论上本地路径都是/开头），或者你希望非映射路径也尝试请求alist
	// 这里假设只有匹配了映射或者是绝对路径才请求
	if matched || strings.HasPrefix(targetPath, "/") {
		logrus.Infof("请求 Alist API: %s", targetPath)
		
		rawUrl, err := s.alist.GetFsGet(targetPath, c.Request.UserAgent())
		if err != nil {
			logrus.Warnf("Alist 获取失败: %v, 回源代理", err)
			s.ReverseProxy(c, false)
			return
		}

		logrus.Infof("Alist 跳转 -> %s", rawUrl)
		c.Redirect(http.StatusFound, rawUrl)
		return
	}
}

// 既不是 HTTP，也无法匹配 Alist 规则
logrus.Debugf("无法处理的路径，回源代理: %s", realPath)
s.ReverseProxy(c, false)

}

func extractItemId(path string) string {
re := regexp.MustCompile(/(?:videos|items)/([a-zA-Z0-9\-]+)/)
matches := re.FindStringSubmatch(strings.ToLower(path))
if len(matches) >= 2 {
return matches[1]
}
return ""
}
