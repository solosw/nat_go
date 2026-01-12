package common

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
)

var MineConfig *Config

// Config 应用配置结构
type Config struct {
	App          AppConfig          `yaml:"app"`
	Database     DatabaseConfig     `yaml:"database"`
	JWT          JWTConfig          `yaml:"jwt"`
	Server       ServerConfig       `yaml:"server"`
	Log          LogConfig          `yaml:"log"`
	TunnelServer TunnelServerConfig `yaml:"tunnel_server,omitempty"`
	TunnelClient TunnelClientConfig `yaml:"tunnel_client,omitempty"`
}

// AppConfig 应用基础配置
type AppConfig struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	Port    int    `yaml:"port"`
	Env     string `yaml:"env"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Driver          string `yaml:"driver"`
	Host            string `yaml:"host"`
	Port            int    `yaml:"port"`
	Username        string `yaml:"username"`
	Password        string `yaml:"password"`
	DBName          string `yaml:"dbname"`
	Charset         string `yaml:"charset"`
	SQLitePath      string `yaml:"sqlite_path"`
	MaxIdleConns    int    `yaml:"max_idle_conns"`
	MaxOpenConns    int    `yaml:"max_open_conns"`
	ConnMaxLifetime int    `yaml:"conn_max_lifetime"`
}

// JWTConfig JWT配置
type JWTConfig struct {
	SecretKey   string `yaml:"secret_key"`
	ExpireHours int    `yaml:"expire_hours"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	ReadTimeout   int    `yaml:"read_timeout"`
	WriteTimeout  int    `yaml:"write_timeout"`
	StaticPath    string `yaml:"static_path"`
	UploadMaxSize int    `yaml:"upload_max_size"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level      string `yaml:"level"`
	FilePath   string `yaml:"file_path"`
	MaxSize    int    `yaml:"max_size"`
	MaxAge     int    `yaml:"max_age"`
	MaxBackups int    `yaml:"max_backups"`
}

// TunnelServerConfig 内网穿透服务端配置
type TunnelServerConfig struct {
	Port         int  `yaml:"port"`          // 服务端监听端口
	ReadTimeout  int  `yaml:"read_timeout"`  // 读取超时（秒）
	WriteTimeout int  `yaml:"write_timeout"` // 写入超时（秒）
	PrivateUse   bool `yaml:"private_use"`   // 是否私人使用（true则禁用/tunnel前缀路由，只允许直接访问）
}

// TunnelClientConfig 内网穿透客户端配置
type TunnelClientConfig struct {
	ServerURL string `yaml:"server_url"` // 服务端WebSocket地址，如 ws://example.com:8080/ws
	TunnelID  string `yaml:"tunnel_id"`  // 隧道ID（可选，不提供则自动生成）
	TargetURL string `yaml:"target_url"` // 目标本地服务地址，如 http://localhost:8080
}

// LoadConfig 加载配置文件
func LoadConfig(configPath string) (*Config, error) {
	config := &Config{}

	// 读取配置文件
	file, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %v", err)
	}

	// 解析YAML
	err = yaml.Unmarshal(file, config)
	if err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %v", err)
	}

	// 设置全局配置
	MineConfig = config

	return config, nil
}
