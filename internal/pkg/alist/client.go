package alist

import (
	"encoding/json"
	"fmt"
	"github.com/go-resty/resty/v2"
	"github.com/sirupsen/logrus"
	"strings"
	"time"
)

type Client struct {
	client        *resty.Client
	host          string
	token         string
	publicHost    string
	uaPassthrough bool
}

func NewClient(host, token, publicHost string, uaPassthrough bool) *Client {
	return &Client{
		client:        resty.New().SetTimeout(10 * time.Second).SetRetryCount(2),
		host:          strings.TrimRight(host, "/"),
		token:         token,
		publicHost:    strings.TrimRight(publicHost, "/"),
		uaPassthrough: uaPassthrough,
	}
}

type fsGetResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		RawURL string `json:"raw_url"`
	} `json:"data"`
}

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
		return "", fmt.Errorf("alist api error: %s (code %d)", result.Message, result.Code)
	}

	rawUrl := result.Data.RawURL
	if rawUrl == "" {
		return "", fmt.Errorf("alist returned empty raw_url")
	}

	if a.publicHost != "" {
		if strings.HasPrefix(rawUrl, a.host) {
			rawUrl = strings.Replace(rawUrl, a.host, a.publicHost, 1)
		}
	}

	return rawUrl, nil
}