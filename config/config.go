package config

import (
	"time"

	"github.com/spf13/viper"
)

// Config 配置结构体
type Config struct {
	PortRange    PortRangeConfig    `mapstructure:"port_range"`
	UPnP         UPnPConfig         `mapstructure:"upnp"`
	Network      NetworkConfig      `mapstructure:"network"`
	Log          LogConfig          `mapstructure:"log"`
	Monitor      MonitorConfig      `mapstructure:"monitor"`
	Admin        AdminConfig        `mapstructure:"admin"`
	NATTraversal NATTraversalConfig `mapstructure:"nat_traversal"`
}

// PortRangeConfig 端口范围配置
type PortRangeConfig struct {
	Start int `mapstructure:"start"`
	End   int `mapstructure:"end"`
	Step  int `mapstructure:"step"`
}

// UPnPConfig UPnP配置
type UPnPConfig struct {
	DiscoveryTimeout    time.Duration `mapstructure:"discovery_timeout"`
	MappingDuration     time.Duration `mapstructure:"mapping_duration"`
	RetryAttempts       int           `mapstructure:"retry_attempts"`
	RetryDelay          time.Duration `mapstructure:"retry_delay"`
	HealthCheckInterval time.Duration `mapstructure:"health_check_interval"`
	MaxFailCount        int           `mapstructure:"max_fail_count"`
	KeepAliveInterval   time.Duration `mapstructure:"keep_alive_interval"`
}

// NetworkConfig 网络配置
type NetworkConfig struct {
	PreferredInterfaces []string `mapstructure:"preferred_interfaces"`
	ExcludeInterfaces   []string `mapstructure:"exclude_interfaces"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level       string `mapstructure:"level"`
	Format      string `mapstructure:"format"`
	File        string `mapstructure:"file"`
	MaxSize     int64  `mapstructure:"max_size"`
	BackupCount int    `mapstructure:"backup_count"`
}

// MonitorConfig 监控配置
type MonitorConfig struct {
	CheckInterval   time.Duration `mapstructure:"check_interval"`
	CleanupInterval time.Duration `mapstructure:"cleanup_interval"`
}

// AdminConfig 管理服务配置
type AdminConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Host     string `mapstructure:"host"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	DataDir  string `mapstructure:"data_dir"`
}

// NATTraversalConfig NAT穿透配置
type NATTraversalConfig struct {
	Enabled     bool            `mapstructure:"enabled"`
	UseTURN     bool            `mapstructure:"use_turn"`
	TURNServers []TURNServer    `mapstructure:"turn_servers"`
	PortRange   PortRangeConfig `mapstructure:"port_range"`
}

// TURNServer TURN服务器配置
type TURNServer struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Realm    string `mapstructure:"realm"`
}

// LoadConfig 加载配置文件
func LoadConfig(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	// 设置默认值
	setDefaults()

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

// setDefaults 设置默认配置值
func setDefaults() {
	// 端口范围默认值
	viper.SetDefault("port_range.start", 8000)
	viper.SetDefault("port_range.end", 9000)
	viper.SetDefault("port_range.step", 1)

	// UPnP默认值
	viper.SetDefault("upnp.discovery_timeout", 10)
	viper.SetDefault("upnp.mapping_duration", "1h")
	viper.SetDefault("upnp.retry_attempts", 3)
	viper.SetDefault("upnp.retry_delay", "5s")
	viper.SetDefault("upnp.health_check_interval", "1m")
	viper.SetDefault("upnp.max_fail_count", 3)
	viper.SetDefault("upnp.keep_alive_interval", "2m")

	// 网络默认值
	viper.SetDefault("network.preferred_interfaces", []string{"eth0", "wlan0"})
	viper.SetDefault("network.exclude_interfaces", []string{"lo", "docker"})

	// 日志默认值
	viper.SetDefault("log.level", "info")
	viper.SetDefault("log.format", "json")
	viper.SetDefault("log.file", "auto_upnp.log")
	viper.SetDefault("log.max_size", 10*1024*1024) // 10MB
	viper.SetDefault("log.backup_count", 5)

	// 监控默认值
	viper.SetDefault("monitor.check_interval", "30s")
	viper.SetDefault("monitor.cleanup_interval", "5m")
	viper.SetDefault("monitor.max_mappings", 100)

	// 管理服务默认值
	viper.SetDefault("admin.enabled", true)
	viper.SetDefault("admin.host", "0.0.0.0")
	viper.SetDefault("admin.username", "admin")
	viper.SetDefault("admin.password", "admin")
	viper.SetDefault("admin.data_dir", "data")

	// NAT穿透默认值
	viper.SetDefault("nat_traversal.enabled", false)
	viper.SetDefault("nat_traversal.use_turn", true)
	viper.SetDefault("nat_traversal.turn_servers", []map[string]interface{}{
		{
			"host":     "47.104.139.35",
			"port":     3478,
			"username": "admin",
			"password": "Flzx@2025",
			"realm":    "turn.ealine.cn",
		},
	})
	viper.SetDefault("nat_traversal.port_range.start", 49152)
	viper.SetDefault("nat_traversal.port_range.end", 65535)
}

// GetPortRange 获取端口范围列表
func (c *Config) GetPortRange() []int {
	var ports []int
	for i := c.PortRange.Start; i <= c.PortRange.End; i += c.PortRange.Step {
		ports = append(ports, i)
	}
	return ports
}

// GetPortPairs 获取端口对列表 (内部端口, 外部端口)
func (c *Config) GetPortPairs() [][2]int {
	ports := c.GetPortRange()
	pairs := make([][2]int, len(ports))
	for i, port := range ports {
		pairs[i] = [2]int{port, port}
	}
	return pairs
}
