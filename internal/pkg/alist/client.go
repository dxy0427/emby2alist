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
	client     *resty.Client
	host       string
	token      string
	publicHost string
	signEnable bool
	signSalt   string
}

func NewClient(host, token, publicHost string, signEnable bool, signSalt string) *Client {
	return &Client{
		client:     resty.New().SetTimeout(10 * time.Second),
		host:       strings.TrimRight(host, "/"),
		token:      token,
		publicHost: strings.TrimRight(publicHost, "/"),
		signEnable: signEnable,
		signSalt:   signSalt,
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
func (a *Client) GetFsGet(filePath string) (string, error) {
	if !strings.HasPrefix(filePath, "/") {
		filePath = "/" + filePath
	}

	reqBody := map[string]interface{}{
		"path":     filePath,
		"password": "",
	}

	resp, err := a.client.R().
		SetHeader("Authorization", a.token).
		SetHeader("Content-Type", "application/json").
		SetBody(reqBody).
		Post(a.host + "/api/fs/get")

	if err != nil {
		return "", err
	}

	var result fsGetResponse
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return "", fmt.Errorf("alist json parse fail: %v", err)
	}

	if result.Code != 200 {
		return "", fmt.Errorf("alist api error: %s", result.Message)
	}

	rawUrl := result.Data.RawURL
	if rawUrl == "" {
		return "", fmt.Errorf("alist returned empty raw_url")
	}

	// 替换公网地址
	if a.publicHost != "" {
		// 简单替换：如果 rawUrl 包含内网 Alist 地址，替换为公网
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

// signUrl 实现 Alist 的 HMAC 签名
func (a *Client) signUrl(rawUrl string, expireHours int) string {
	if a.signSalt == "" {
		return rawUrl
	}
	
	u, err := url.Parse(rawUrl)
	if err != nil {
		logrus.Warnf("Sign URL Parse Error: %v", err)
		return rawUrl
	}

	path := u.Path
	// 需要 decode，因为 Alist 签名是对原始字符签名的
	path, _ = url.QueryUnescape(path)
	
	timeVal := int64(0)
	if expireHours > 0 {
		timeVal = time.Now().Add(time.Duration(expireHours) * time.Hour).Unix()
	}
	
	// 构造签名源数据: path:time
	signData := fmt.Sprintf("%s:%d", path, timeVal)
	
	// HMAC-SHA256
	h := hmac.New(sha256.New, []byte(a.signSalt))
	h.Write([]byte(signData))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))
	
	// URL Safe 替换
	signature = strings.ReplaceAll(signature, "+", "-")
	signature = strings.ReplaceAll(signature, "/", "_")
	
	// 最终格式: sign=signature:time
	finalSign := fmt.Sprintf("%s:%d", signature, timeVal)

	q := u.Query()
	q.Set("sign", finalSign)
	u.RawQuery = q.Encode()
	
	return u.String()
}