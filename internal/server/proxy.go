package server

import (
	"bytes"
	"compress/gzip"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// ReverseProxy 执行反向代理
// modifyResponse: 是否需要拦截并修改响应体 (用于 PlaybackInfo)
func (s *Server) ReverseProxy(c *gin.Context, modifyResponse bool) {
	remote, err := url.Parse(s.cfg.EmbyHost)
	if err != nil {
		c.String(500, "Config Error")
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(remote)

	proxy.Director = func(req *http.Request) {
		req.Header = c.Request.Header
		req.Host = remote.Host
		req.URL.Scheme = remote.Scheme
		req.URL.Host = remote.Host
		req.URL.Path = c.Request.URL.Path
		req.URL.RawQuery = c.Request.URL.RawQuery
		
		// 移除 Accept-Encoding 避免 Emby 返回 gzip，方便我们修改 JSON
		if modifyResponse {
			req.Header.Del("Accept-Encoding")
		}
	}

	if modifyResponse {
		proxy.ModifyResponse = func(resp *http.Response) error {
			if resp.StatusCode != 200 {
				return nil
			}

			// 读取原始 Body
			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			_ = resp.Body.Close()

			// 处理 Gzip (万一上游强制 gzip)
			var respBody []byte
			if resp.Header.Get("Content-Encoding") == "gzip" {
				reader, err := gzip.NewReader(bytes.NewReader(bodyBytes))
				if err == nil {
					respBody, _ = io.ReadAll(reader)
					_ = reader.Close()
				} else {
					respBody = bodyBytes
				}
			} else {
				respBody = bodyBytes
			}

			// 执行修改
			newBody := s.modifyPlaybackInfo(respBody)

			// 重新设置 Body
			resp.Header.Del("Content-Encoding") // 不再压缩
			resp.Header.Del("Content-Length")
			resp.Body = io.NopCloser(bytes.NewReader(newBody))
			return nil
		}
	}

	proxy.ServeHTTP(c.Writer, c.Request)
}

func (s *Server) modifyPlaybackInfo(body []byte) []byte {
	// 如果配置未开启禁用转码，直接返回
	if !s.cfg.DisableTranscode {
		return body
	}

	jsonStr := string(body)
	
	// 修改 MediaSources 数组中的每一个 Source
	// 强制开启 DirectPlay，关闭 Transcoding
	sources := gjson.Get(jsonStr, "MediaSources").Array()
	for i := range sources {
		prefix := fmt.Sprintf("MediaSources.%d", i)
		jsonStr, _ = sjson.Set(jsonStr, prefix+".SupportsDirectPlay", true)
		jsonStr, _ = sjson.Set(jsonStr, prefix+".SupportsDirectStream", true)
		jsonStr, _ = sjson.Set(jsonStr, prefix+".SupportsTranscoding", false)

		// 标记 DirectStreamUrl，防止 Emby 客户端无限重试
		dUrl := gjson.Get(jsonStr, prefix+".DirectStreamUrl").String()
		if dUrl != "" {
			sep := "?"
			if strings.Contains(dUrl, "?") {
				sep = "&"
			}
			dUrl += sep + "Emby2Alist=true"
			jsonStr, _ = sjson.Set(jsonStr, prefix+".DirectStreamUrl", dUrl)
		}
	}
	
	logrus.Debug("Modified PlaybackInfo: Forced DirectPlay")
	return []byte(jsonStr)
}