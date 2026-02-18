package mediaserver

import (
	"encoding/json"
	"fmt"
	"github.com/go-resty/resty/v2"
	"strings"
	"time"
)

type JellyfinClient struct {
	client *resty.Client
	host   string
	apiKey string
}

func NewJellyfinClient(host, apiKey string) *JellyfinClient {
	return &JellyfinClient{
		client: resty.New().SetTimeout(5 * time.Second),
		host:   strings.TrimRight(host, "/"),
		apiKey: apiKey,
	}
}

func (j *JellyfinClient) GetItemInfo(itemId string, mediaSourceId string) (string, error) {
	url := fmt.Sprintf("%s/Items", j.host)
	req := j.client.R().
		SetQueryParam("Ids", itemId).
		SetQueryParam("Fields", "Path,MediaSources").
		SetQueryParam("Limit", "1")

	if j.apiKey != "" {
		req.SetHeader("X-Emby-Token", j.apiKey)
		req.SetQueryParam("api_key", j.apiKey)
	}

	resp, err := req.Get(url)
	if err != nil {
		return "", err
	}
	if resp.StatusCode() != 200 {
		return "", fmt.Errorf("jellyfin api error: %d", resp.StatusCode())
	}

	var res commonItemsResponse
	if err := json.Unmarshal(resp.Body(), &res); err != nil {
		return "", err
	}

	return commonGetItemPath(res, mediaSourceId)
}