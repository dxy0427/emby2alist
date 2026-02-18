package server

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// mainHandler 处理所有请求
func (s *Server) mainHandler(c *gin.Context) {
	// 0. 客户端过滤 (Client Filter)
	// 如果被拦截，直接中止请求，不进行登录或播放
	if !s.checkClientFilter(c) {
		return
	}

	// 1. PlaybackInfo 拦截 (修改转码策略)
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

		targetPath := realPath
		// 3.1 路径替换
		for _, m := range s.cfg.HttpStrm.PathMappings {
			if m.Old != "" {
				targetPath = strings.Replace(targetPath, m.Old, m.New, 1)
			}
		}

		// 3.2 自动解析 302 (带缓存)
		if s.cfg.HttpStrm.ResolveStrmLinks {
			finalUrl := targetPath

			// --- 缓存读取 ---
			cacheHit := false
			if s.cfg.Cache.Enable {
				if val, ok := s.urlCache.Load(targetPath); ok {
					cached := val.(cachedURL)
					if time.Now().Before(cached.expiresAt) {
						logrus.Infof("缓存命中: %s", cached.url)
						finalUrl = cached.url
						cacheHit = true
					} else {
						s.urlCache.Delete(targetPath) // 过期删除
					}
				}
			}

			// --- 未命中缓存，执行请求 ---
			if !cacheHit {
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
							finalUrl = loc
							
							// --- 写入缓存 ---
							if s.cfg.Cache.Enable {
								s.urlCache.Store(targetPath, cachedURL{
									url:       finalUrl,
									expiresAt: time.Now().Add(s.cacheTTL),
								})
							}
						}
					}
				} else {
					logrus.Warnf("解析失败: %v", err)
				}
			}
			targetPath = finalUrl
		}

		logrus.Infof("[HttpStrm] 跳转: %s", targetPath)
		c.Redirect(http.StatusFound, targetPath)
		return
	}

	// 如果不是 http 开头，直接代理回源
	s.ReverseProxy(c, false)
}

// checkClientFilter 检查客户端 UA 是否允许访问
// 返回 true 表示允许通过，返回 false 表示已拦截
func (s *Server) checkClientFilter(c *gin.Context) bool {
	if !s.cfg.Client.Enable {
		return true
	}

	ua := strings.ToLower(c.Request.UserAgent())
	mode := strings.ToLower(s.cfg.Client.Mode) // BlackList or WhiteList
	
	// 匹配逻辑
	matched := false
	for _, key := range s.cfg.Client.List {
		if key != "" && strings.Contains(ua, strings.ToLower(key)) {
			matched = true
			break
		}
	}

	// 黑名单模式: 匹配到则禁止
	if mode == "blacklist" {
		if matched {
			logrus.Warnf("客户端被黑名单拦截: UA=%s", c.Request.UserAgent())
			c.String(http.StatusForbidden, "403 Forbidden: Client is blacklisted")
			c.Abort()
			return false
		}
		return true
	}

	// 白名单模式: 没匹配到则禁止
	if mode == "whitelist" {
		if !matched {
			logrus.Warnf("客户端不在白名单中: UA=%s", c.Request.UserAgent())
			c.String(http.StatusForbidden, "403 Forbidden: Client is not whitelisted")
			c.Abort()
			return false
		}
		return true
	}

	return true
}

func extractItemId(path string) string {
	re := regexp.MustCompile(`/(?:videos|items)/([a-zA-Z0-9\-]+)/`)
	matches := re.FindStringSubmatch(strings.ToLower(path))
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}