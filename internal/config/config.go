package config

import (
	"github.com/goccy/go-yaml"
	"os"
)

type Config struct {
	Port     int          `yaml:"port"`
	Server   ServerConf   `yaml:"server"`
	HttpStrm HttpStrmConf `yaml:"http_strm"`
	Cache    CacheConf    `yaml:"cache"`
	Client   ClientConf   `yaml:"client"`
}

type ServerConf struct {
	Type string `yaml:"type"`
	Addr string `yaml:"addr"`
	Auth string `yaml:"auth"`
}

type HttpStrmConf struct {
	Enable           bool          `yaml:"enable"`
	DisableTranscode bool          `yaml:"disable_transcode"`
	ResolveStrmLinks bool          `yaml:"resolve_strm_links"`
	UaPassthrough    bool          `yaml:"alist_ua_passthrough"`
	PathMappings     []PathMapping `yaml:"path_mappings"`
}

type PathMapping struct {
	Old string `yaml:"old"`
	New string `yaml:"new"`
}

type CacheConf struct {
	Enable      bool   `yaml:"enable"`
	HttpStrmTTL string `yaml:"http_strm_ttl"`
}

type ClientConf struct {
	Enable bool     `yaml:"enable"`
	Mode   string   `yaml:"mode"` // BlackList or WhiteList
	List   []string `yaml:"list"`
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
		cfg.Port = 8091
	}
	// 默认值处理
	if cfg.Cache.HttpStrmTTL == "" {
		cfg.Cache.HttpStrmTTL = "10m"
	}
	return &cfg, nil
}