package config

import (
	"github.com/goccy/go-yaml"
	"os"
	"time"
)

type Config struct {
	Port     int              `yaml:"port"`
	Server   ServerConf       `yaml:"server"`
	Cache    CacheConf        `yaml:"cache"`
	Client   ClientFilterConf `yaml:"client"`
	HttpStrm HttpStrmConf     `yaml:"http_strm"`
	Logging  LoggingConf      `yaml:"logging"`
}

type ServerConf struct {
	Type string `yaml:"type"`
	Addr string `yaml:"addr"`
	Auth string `yaml:"auth"`
}

type CacheConf struct {
	Enable      bool          `yaml:"enable"`
	HttpStrmTTL time.Duration `yaml:"http_strm_ttl"`
}

type ClientFilterConf struct {
	Enable bool     `yaml:"enable"`
	Mode   string   `yaml:"mode"` // WhiteList or BlackList
	List   []string `yaml:"list"`
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

type LoggingConf struct {
	Verbose bool `yaml:"verbose"` // 是否启用详细日志（包括所有 HTTP 请求）
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
	// 设置默认缓存时间
	if cfg.Cache.HttpStrmTTL == 0 {
		cfg.Cache.HttpStrmTTL = 1 * time.Minute
	}
	return &cfg, nil
}