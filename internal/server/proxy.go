package server

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

func (s *Server) ReverseProxy(c *gin.Context, modifyResponse bool) {
	hostUrl := s.cfg.Server.Addr
	if !strings.HasPrefix(hostUrl, "http://") && !strings.HasPrefix(hostUrl, "https://") {
		hostUrl = "http://" + hostUrl
	}

	remote, err := url.Parse(hostUrl)
	if err != nil {
		c.String(500, "Config Error: Invalid Server Addr")
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
		
		if modifyResponse {
			req.Header.Del("Accept-Encoding")
		}
	}

	if modifyResponse {
		proxy.ModifyResponse = func(resp *http.Response) error {
			if resp.StatusCode != 200 {
				return nil
			}

			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			_ = resp.Body.Close()

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

			newBody := s.modifyPlaybackInfo(respBody)

			resp.Header.Del("Content-Encoding")
			resp.Header.Del("Content-Length")
			resp.Body = io.NopCloser(bytes.NewReader(newBody))
			return nil
		}
	}

	proxy.ServeHTTP(c.Writer, c.Request)
}

func (s *Server) modifyPlaybackInfo(body []byte) []byte {
	// 如果 HTTP 和 Alist 模式都禁用了转码，则全局禁用
	disableTranscode := s.cfg.HttpStrm.DisableTranscode || s.cfg.AlistStrm.DisableTranscode
	if !disableTranscode {
		return body
	}

	jsonStr := string(body)
	
	sources := gjson.Get(jsonStr, "MediaSources").Array()
	for i := range sources {
		prefix := fmt.Sprintf("MediaSources.%d", i)
		jsonStr, _ = sjson.Set(jsonStr, prefix+".SupportsDirectPlay", true)
		jsonStr, _ = sjson.Set(jsonStr, prefix+".SupportsDirectStream", true)
		jsonStr, _ = sjson.Set(jsonStr, prefix+".SupportsTranscoding", false)

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
	
	return []byte(jsonStr)
}