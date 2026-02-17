package config

import (
	"github.com/goccy/go-yaml"
	"os"
)

type Config struct {
	Port   int        `yaml:"port"`
	Server ServerConf `yaml:"server"`
	
	HttpStrm  HttpStrmConf  `yaml:"http_strm"`
	AlistStrm AlistStrmConf `yaml:"alist_strm"`
}

type ServerConf struct {
	Type string `yaml:"type"` // Emby / Jellyfin
	Addr string `yaml:"addr"`
	Auth string `yaml:"auth"` // ApiKey
}

type HttpStrmConf struct {
	Enable             bool          `yaml:"enable"`
	DisableTranscode   bool          `yaml:"disable_transcode"`
	ResolveStrmLinks   bool          `yaml:"resolve_strm_links"`
	UaPassthrough      bool          `yaml:"alist_ua_passthrough"` // 其实HTTP模式不需要alist UA，但为了兼容保留
	PathMappings       []PathMapping `yaml:"path_mappings"`
}

type AlistStrmConf struct {
	Enable           bool          `yaml:"enable"`
	DisableTranscode bool          `yaml:"disable_transcode"`
	AlistHost        string        `yaml:"alist_host"`
	AlistPublicHost  string        `yaml:"alist_public_host"`
	AlistToken       string        `yaml:"alist_token"`
	UaPassthrough    bool          `yaml:"alist_ua_passthrough"`
	PathMappings     []PathMapping `yaml:"path_mappings"`
}

type PathMapping struct {
	Old string `yaml:"old"`
	New string `yaml:"new"`
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
	// 默认值
	if cfg.Port == 0 {
		cfg.Port = 8091
	}
	return &cfg, nil
}