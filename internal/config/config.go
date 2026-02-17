package config

import (
	"github.com/goccy/go-yaml"
	"os"
	"sync"
)

type Config struct {
	ServerPort       int           `yaml:"server_port"`
	BackendType      string        `yaml:"backend_type"` // emby 或 jellyfin
	EmbyHost         string        `yaml:"emby_host"`
	EmbyApiKey       string        `yaml:"emby_api_key"`
	AlistHost        string        `yaml:"alist_host"`
	AlistPublicHost  string        `yaml:"alist_public_host"`
	AlistToken       string        `yaml:"alist_token"`
	AlistSignEnable  bool          `yaml:"alist_sign_enable"`
	AlistSignSalt    string        `yaml:"alist_sign_salt"`
	MountPaths       []string      `yaml:"mount_paths"`
	RouteRules       []RouteRule   `yaml:"route_rules"`
	PathMappings     []PathMapping `yaml:"path_mappings"`
	DisableTranscode bool          `yaml:"disable_transcode"`
	mu               sync.RWMutex  `yaml:"-"`
}

type RouteRule struct {
	Group   string `yaml:"group"`   // 分组逻辑
	Mode    string `yaml:"mode"`    // proxy, redirect, block
	Target  string `yaml:"target"`  // filePath, alistRes, remote_addr, ua, userId
	Matcher string `yaml:"matcher"` // contains, startsWith, endsWith, regex, eq
	Value   string `yaml:"value"`
}

type PathMapping struct {
	Old string `yaml:"old"`
	New string `yaml:"new"`
}

func DefaultConfig() *Config {
	return &Config{
		ServerPort:       8091,
		BackendType:      "emby",
		EmbyHost:         "http://172.17.0.1:8096",
		EmbyApiKey:       "",
		AlistHost:        "http://172.17.0.1:5244",
		AlistPublicHost:  "",
		AlistToken:       "",
		AlistSignEnable:  false,
		MountPaths:       []string{"/mnt"},
		DisableTranscode: true,
		RouteRules:       []RouteRule{},
		PathMappings:     []PathMapping{},
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
	if cfg.ServerPort == 0 {
		cfg.ServerPort = 8091
	}
	if cfg.BackendType == "" {
		cfg.BackendType = "emby"
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

func (c *Config) Update(newCfg *Config) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 手动赋值所有字段
	c.ServerPort = newCfg.ServerPort
	c.BackendType = newCfg.BackendType
	c.EmbyHost = newCfg.EmbyHost
	c.EmbyApiKey = newCfg.EmbyApiKey
	c.AlistHost = newCfg.AlistHost
	c.AlistPublicHost = newCfg.AlistPublicHost
	c.AlistToken = newCfg.AlistToken
	c.AlistSignEnable = newCfg.AlistSignEnable
	c.AlistSignSalt = newCfg.AlistSignSalt
	c.MountPaths = newCfg.MountPaths
	c.RouteRules = newCfg.RouteRules
	c.PathMappings = newCfg.PathMappings
	c.DisableTranscode = newCfg.DisableTranscode
}