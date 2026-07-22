package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	MySQL     MySQLConfig     `yaml:"mysql"`
	Redis     RedisConfig     `yaml:"redis"`
	JWT       JWTConfig       `yaml:"jwt"`
	FlashSale FlashSaleConfig `yaml:"
	
	
	"`
}

type ServerConfig struct {
	Port int    `yaml:"port"`
	Mode string `yaml:"mode"`
}

type MySQLConfig struct {
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	User         string `yaml:"user"`
	Password     string `yaml:"password"`
	Database     string `yaml:"database"`
	Charset      string `yaml:"charset"`
	MaxOpenConns int    `yaml:"max_open_conns"`
	MaxIdleConns int    `yaml:"max_idle_conns"`
}

func (m MySQLConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=true&loc=Local&interpolateParams=true",
		m.User, m.Password, m.Host, m.Port, m.Database, m.Charset)
}

type RedisConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
	PoolSize int    `yaml:"pool_size"`
}

func (r RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

type JWTConfig struct {
	Secret      string `yaml:"secret"`
	ExpireHours int    `yaml:"expire_hours"`
}

type FlashSaleConfig struct {
	MaxPerUser     int `yaml:"max_per_user"`
	WorkerCount    int `yaml:"worker_count"`
	BatchSize      int `yaml:"batch_size"`
	BatchIntervalMs int `yaml:"batch_interval_ms"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}
	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}
	// 默认值
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Server.Mode == "" {
		cfg.Server.Mode = "debug"
	}
	if cfg.MySQL.MaxOpenConns == 0 {
		cfg.MySQL.MaxOpenConns = 200
	}
	if cfg.MySQL.MaxIdleConns == 0 {
		cfg.MySQL.MaxIdleConns = 50
	}
	if cfg.Redis.PoolSize == 0 {
		cfg.Redis.PoolSize = 500
	}
	if cfg.JWT.ExpireHours == 0 {
		cfg.JWT.ExpireHours = 2
	}
	if cfg.FlashSale.MaxPerUser == 0 {
		cfg.FlashSale.MaxPerUser = 1
	}
	if cfg.FlashSale.WorkerCount == 0 {
		cfg.FlashSale.WorkerCount = 10
	}
	if cfg.FlashSale.BatchSize == 0 {
		cfg.FlashSale.BatchSize = 100
	}
	if cfg.FlashSale.BatchIntervalMs == 0 {
		cfg.FlashSale.BatchIntervalMs = 1000
	}
	return cfg, nil
}
