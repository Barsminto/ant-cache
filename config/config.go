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
			Password: "", // 默认无密码，无密码则不启用认证
		},
		Persistence: struct {
			AtdInterval string `json:"atd_interval"`
			AclInterval string `json:"acl_interval"`
		}{
			AtdInterval: "1h",
			AclInterval: "1s",
		},
	}
}
