package mediaserver

import (
	"fmt"
	"strings"
)

type MediaServerClient interface {
	GetItemInfo(itemId string, mediaSourceId string) (string, error)
}

func NewClient(backendType, host, apiKey string) MediaServerClient {
	bt := strings.ToLower(backendType)
	if bt == "jellyfin" {
		return NewJellyfinClient(host, apiKey)
	}
	return NewEmbyClient(host, apiKey)
}

type commonItemsResponse struct {
	Items []struct {
		Path         string `json:"Path"`
		MediaSources []struct {
			Id       string `json:"Id"`
			Path     string `json:"Path"`
			Protocol string `json:"Protocol"`
		} `json:"MediaSources"`
	} `json:"Items"`
}

func commonGetItemPath(res commonItemsResponse, mediaSourceId string) (string, error) {
	if len(res.Items) == 0 {
		return "", fmt.Errorf("item not found")
	}

	rawItem := res.Items[0]
	targetPath := rawItem.Path

	if len(rawItem.MediaSources) > 0 {
		for _, ms := range rawItem.MediaSources {
			if mediaSourceId == "" || ms.Id == mediaSourceId {
				targetPath = ms.Path
				if ms.Protocol == "Http" && ms.Path != "" {
					return ms.Path, nil
				}
				break
			}
		}
	}
	return targetPath, nil
}