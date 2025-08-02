package config

import (
	"encoding/json"
	"os"
	"time"
)

type Config struct {
	Server struct {
		Host string `json:"host"`
		Port string `json:"port"`
	} `json:"server"`
	Auth struct {
		Password string `json:"password"`
	} `json:"auth"`
	Persistence struct {
		AtdInterval string `json:"atd_interval"`
		AclInterval string `json:"acl_interval"`
	} `json:"persistence"`
	Compression struct {
		Enabled     bool   `json:"enabled"`
		Type        string `json:"type"`         // "gzip" or "zlib"
		Level       string `json:"level"`        // "default", "best_speed", "best_compression"
		MinSize     int    `json:"min_size"`     // 最小压缩大小（字节）
		StringsOnly bool   `json:"strings_only"` // 是否只压缩字符串
	} `json:"compression"`
}

// GetAtdInterval returns the ATD interval as time.Duration
func (c *Config) GetAtdInterval() time.Duration {
	if c.Persistence.AtdInterval == "" {
		return time.Hour // 默认每小时
	}
	duration, err := time.ParseDuration(c.Persistence.AtdInterval)
	if err != nil {
		// If parsing fails, return the default value
		return time.Hour
	}
	return duration
}

// GetAclInterval returns the ACL interval as time.Duration
func (c *Config) GetAclInterval() time.Duration {
	if c.Persistence.AclInterval == "" {
		return time.Second
	}
	duration, err := time.ParseDuration(c.Persistence.AclInterval)
	if err != nil {
		// If parsing fails, return the default value
		return time.Second
	}
	return duration
}

func LoadConfig(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	config := &Config{}
	decoder := json.NewDecoder(file)
	err = decoder.Decode(config)
	if err != nil {
		return nil, err
	}

	// Set default values
	if config.Server.Host == "" {
		config.Server.Host = "localhost"
	}
	if config.Server.Port == "" {
		config.Server.Port = "8890"
	}
	if config.Persistence.AtdInterval == "" {
		config.Persistence.AtdInterval = "1h"
	}
	if config.Persistence.AclInterval == "" {
		config.Persistence.AclInterval = "1s"
	}
	// Set default compression values
	if config.Compression.Type == "" {
		config.Compression.Type = "gzip"
	}
	if config.Compression.Level == "" {
		config.Compression.Level = "default"
	}
	if config.Compression.MinSize == 0 {
		config.Compression.MinSize = 1024 // 默认1KB
	}

	return config, nil
}

// SaveConfig saves the configuration to a file
func (c *Config) SaveConfig(filename string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0644)
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		Server: struct {
			Host string `json:"host"`
			Port string `json:"port"`
		}{
			Host: "localhost",
			Port: "8890",
		},
		Auth: struct {
			Password string `json:"password"`
		}{
			Password: "",
		},
		Persistence: struct {
			AtdInterval string `json:"atd_interval"`
			AclInterval string `json:"acl_interval"`
		}{
			AtdInterval: "1h",
			AclInterval: "1s",
		},
		Compression: struct {
			Enabled     bool   `json:"enabled"`
			Type        string `json:"type"`
			Level       string `json:"level"`
			MinSize     int    `json:"min_size"`
			StringsOnly bool   `json:"strings_only"`
		}{
			Enabled:     false,
			Type:        "gzip",
			Level:       "default",
			MinSize:     1024,  // 默认1KB以上的值才压缩
			StringsOnly: false, // 默认压缩所有类型
		},
	}
}
