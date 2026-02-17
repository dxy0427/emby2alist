package mediaserver

import (
	"encoding/json"
	"fmt"
	"github.com/go-resty/resty/v2"
	"strings"
	"time"
)

type EmbyClient struct {
	client *resty.Client
	host   string
	apiKey string
}

func NewEmbyClient(host, apiKey string) *EmbyClient {
	return &EmbyClient{
		client: resty.New().SetTimeout(5 * time.Second),
		host:   strings.TrimRight(host, "/"),
		apiKey: apiKey,
	}
}

func (e *EmbyClient) GetItemInfo(itemId string, mediaSourceId string) (string, *ItemInfo, error) {
	url := fmt.Sprintf("%s/Items", e.host)
	req := e.client.R().
		SetQueryParam("Ids", itemId).
		SetQueryParam("Fields", "Path,MediaSources").
		SetQueryParam("Limit", "1")

	if e.apiKey != "" {
		req.SetQueryParam("api_key", e.apiKey)
	}

	resp, err := req.Get(url)
	if err != nil {
		return "", nil, err
	}
	if resp.StatusCode() != 200 {
		return "", nil, fmt.Errorf("emby api error: %d", resp.StatusCode())
	}

	var res commonItemsResponse
	if err := json.Unmarshal(resp.Body(), &res); err != nil {
		return "", nil, err
	}

	return commonGetItemPath(res, mediaSourceId)
}