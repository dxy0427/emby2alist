package alist

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/go-resty/resty/v2"
	"github.com/sirupsen/logrus"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	client        *resty.Client
	host          string
	token         string
	publicHost    string
	signEnable    bool
	signSalt      string
	uaPassthrough bool
}

func NewClient(host, token, publicHost string, signEnable bool, signSalt string, uaPassthrough bool) *Client {
	return &Client{
		client:        resty.New().SetTimeout(10 * time.Second).SetRetryCount(2), // 增加重试
		host:          strings.TrimRight(host, "/"),
		token:         token,
		publicHost:    strings.TrimRight(publicHost, "/"),
		signEnable:    signEnable,
		signSalt:      signSalt,
		uaPassthrough: uaPassthrough,
	}
}

type fsGetResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		RawURL string `json:"raw_url"`
		Sign   string `json:"sign"`
	} `json:"data"`
}

// GetFsGet 调用 Alist /api/fs/get 获取直链
// userAgent: 客户端的 UA，如果开启透传则会发给 Alist
func (a *Client) GetFsGet(filePath string, userAgent string) (string, error) {
	if !strings.HasPrefix(filePath, "/") {
		filePath = "/" + filePath
	}

	reqBody := map[string]interface{}{
		"path":     filePath,
		"password": "",
	}

	req := a.client.R().
		SetHeader("Authorization", a.token).
		SetHeader("Content-Type", "application/json").
		SetBody(reqBody)

	// 如果开启透传，带上客户端 UA
	if a.uaPassthrough && userAgent != "" {
		req.SetHeader("User-Agent", userAgent)
		logrus.Debugf("Alist Request with UA: %s", userAgent)
	}

	resp, err := req.Post(a.host + "/api/fs/get")

	if err != nil {
		return "", err
	}

	var result fsGetResponse
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return "", fmt.Errorf("alist json parse fail: %v", err)
	}

	if result.Code != 200 {
		// 403 可能意味着 Token 错误
		return "", fmt.Errorf("alist api error: %s (code %d)", result.Message, result.Code)
	}

	rawUrl := result.Data.RawURL
	if rawUrl == "" {
		return "", fmt.Errorf("alist returned empty raw_url")
	}

	// 替换公网地址
	if a.publicHost != "" {
		if strings.HasPrefix(rawUrl, a.host) {
			rawUrl = strings.Replace(rawUrl, a.host, a.publicHost, 1)
		}
	}

	// 签名处理
	if a.signEnable {
		rawUrl = a.signUrl(rawUrl, 0)
	}

	return rawUrl, nil
}

func (a *Client) signUrl(rawUrl string, expireHours int) string {
	if a.signSalt == "" {
		return rawUrl
	}
	
	u, err := url.Parse(rawUrl)
	if err != nil {
		return rawUrl
	}

	path := u.Path
	path, _ = url.QueryUnescape(path)
	
	timeVal := int64(0)
	if expireHours > 0 {
		timeVal = time.Now().Add(time.Duration(expireHours) * time.Hour).Unix()
	}
	
	signData := fmt.Sprintf("%s:%d", path, timeVal)
	
	h := hmac.New(sha256.New, []byte(a.signSalt))
	h.Write([]byte(signData))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))
	
	signature = strings.ReplaceAll(signature, "+", "-")
	signature = strings.ReplaceAll(signature, "/", "_")
	
	finalSign := fmt.Sprintf("%s:%d", signature, timeVal)

	q := u.Query()
	q.Set("sign", finalSign)
	u.RawQuery = q.Encode()
	
	return u.String()
}