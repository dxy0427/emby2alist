package config

import (
	"github.com/goccy/go-yaml"
	"os"
	"sync"
)

type Config struct {
	Port         int              `yaml:"port" json:"port"`
	Server       ServerConfig     `yaml:"server" json:"server"`
	HttpStrm     HttpStrmConfig   `yaml:"http_strm" json:"http_strm"`
	PathMappings []PathMapping    `yaml:"path_mappings" json:"path_mappings"` // 全局/HTTP地址替换
	AlistStrm    AlistStrmConfig  `yaml:"alist_strm" json:"alist_strm"`
	mu           sync.RWMutex     `yaml:"-" json:"-"`
}

type ServerConfig struct {
	Type string `yaml:"type" json:"type"` // Emby / Jellyfin
	Addr string `yaml:"addr" json:"addr"`
	Auth string `yaml:"auth" json:"auth"` // ApiKey
}

type HttpStrmConfig struct {
	Enable            bool `yaml:"enable" json:"enable"`
	DisableTranscode  bool `yaml:"disable_transcode" json:"disable_transcode"`
	ResolveStrmLinks  bool `yaml:"resolve_strm_links" json:"resolve_strm_links"`
	AlistUaPassthrough bool `yaml:"alist_ua_passthrough" json:"alist_ua_passthrough"`
}

type AlistStrmConfig struct {
	Enable            bool          `yaml:"enable" json:"enable"`
	DisableTranscode  bool          `yaml:"disable_transcode" json:"disable_transcode"`
	AlistHost         string        `yaml:"alist_host" json:"alist_host"`
	AlistPublicHost   string        `yaml:"alist_public_host" json:"alist_public_host"`
	AlistToken        string        `yaml:"alist_token" json:"alist_token"`
	AlistUaPassthrough bool         `yaml:"alist_ua_passthrough" json:"alist_ua_passthrough"`
	PathMappings      []PathMapping `yaml:"path_mappings" json:"path_mappings"` // Alist 专用路径映射
}

type PathMapping struct {
	Old string `yaml:"old" json:"old"`
	New string `yaml:"new" json:"new"`
}

func DefaultConfig() *Config {
	return &Config{
		Port: 9000,
		Server: ServerConfig{
			Type: "Emby",
			Addr: "http://localhost:8096",
			Auth: "",
		},
		HttpStrm: HttpStrmConfig{
			Enable:            true,
			DisableTranscode:  true,
			ResolveStrmLinks:  true,
			AlistUaPassthrough: false,
		},
		PathMappings: []PathMapping{},
		AlistStrm: AlistStrmConfig{
			Enable:            true,
			DisableTranscode:  true,
			AlistHost:         "http://172.17.0.1:5244",
			AlistPublicHost:   "",
			AlistToken:        "",
			AlistUaPassthrough: false,
			PathMappings:      []PathMapping{},
		},
	}
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.Port == 0 {
		cfg.Port = 9000
	}
	if cfg.Server.Type == "" {
		cfg.Server.Type = "Emby"
	}
	return &cfg, nil
}

func (c *Config) Save(path string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}