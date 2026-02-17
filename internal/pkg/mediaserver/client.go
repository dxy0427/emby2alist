package mediaserver

import (
	"fmt"
	"strings"
)

type MediaServerClient interface {
	GetItemInfo(itemId string, mediaSourceId string) (string, *ItemInfo, error)
}

type ItemInfo struct {
	Name         string
	Path         string
	MediaSources []MediaSource
}

type MediaSource struct {
	Id       string
	Path     string
	Protocol string
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
		Name         string        `json:"Name"`
		Path         string        `json:"Path"`
		MediaSources []MediaSource `json:"MediaSources"`
	} `json:"Items"`
}

func commonGetItemPath(res commonItemsResponse, mediaSourceId string) (string, *ItemInfo, error) {
	if len(res.Items) == 0 {
		return "", nil, fmt.Errorf("item not found")
	}

	rawItem := res.Items[0]
	info := &ItemInfo{
		Name:         rawItem.Name,
		Path:         rawItem.Path,
		MediaSources: rawItem.MediaSources,
	}

	targetPath := rawItem.Path
	if len(rawItem.MediaSources) > 0 {
		for _, ms := range rawItem.MediaSources {
			if mediaSourceId == "" || ms.Id == mediaSourceId {
				targetPath = ms.Path
				if ms.Protocol == "Http" && ms.Path != "" {
					return ms.Path, info, nil
				}
				break
			}
		}
	}
	return targetPath, info, nil
}