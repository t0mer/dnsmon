package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds the full application configuration.
type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	DNS       DNSConfig       `mapstructure:"dns"`
	Resolvers ResolversConfig `mapstructure:"resolvers"`
	Storage   StorageConfig   `mapstructure:"storage"`
	Cache     CacheConfig     `mapstructure:"cache"`
	RateLimit RateLimitConfig `mapstructure:"ratelimit"`
	GeoIP     GeoIPConfig     `mapstructure:"geoip"`
	Log       LogConfig       `mapstructure:"log"`
	Metrics   MetricsConfig   `mapstructure:"metrics"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Listen       string        `mapstructure:"listen"`
	BaseURL      string        `mapstructure:"base_url"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

// DNSConfig holds DNS query settings.
type DNSConfig struct {
	QueryTimeout           time.Duration `mapstructure:"query_timeout"`
	PerResolverConcurrency int           `mapstructure:"per_resolver_concurrency"`
	DefaultProtocol        string        `mapstructure:"default_protocol"`
	Retry                  int           `mapstructure:"retry"`
}

// ResolversConfig holds resolver list settings.
type ResolversConfig struct {
	Builtin   bool     `mapstructure:"builtin"`
	File      string   `mapstructure:"file"`
	Countries []string `mapstructure:"countries"`
}

// StorageConfig holds persistence settings.
type StorageConfig struct {
	Driver        string `mapstructure:"driver"`
	DSN           string `mapstructure:"dsn"`
	RetentionDays int    `mapstructure:"retention_days"`
}

// CacheConfig holds cache settings.
type CacheConfig struct {
	Driver    string        `mapstructure:"driver"`
	TTL       time.Duration `mapstructure:"ttl"`
	Size      int           `mapstructure:"size"`
	RedisAddr string        `mapstructure:"redis_addr"`
}

// RateLimitConfig holds rate limiting settings.
type RateLimitConfig struct {
	Enabled    bool `mapstructure:"enabled"`
	AnonPerMin int  `mapstructure:"anon_per_min"`
	KeyPerMin  int  `mapstructure:"key_per_min"`
}

// GeoIPConfig holds GeoIP database settings.
type GeoIPConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	MMDBPath string `mapstructure:"mmdb_path"`
}

// LogConfig holds logging settings.
type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// MetricsConfig holds Prometheus metrics settings.
type MetricsConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Path    string `mapstructure:"path"`
}

// Load reads configuration from cfgFile (if non-empty) or from default search
// paths, merges environment variables with the GDNS_ prefix, and returns the
// parsed Config.
func Load(cfgFile string) (*Config, error) {
	v := viper.New()

	setDefaults(v)

	v.SetEnvPrefix("GDNS")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("$HOME/.gdns")
		v.AddConfigPath("/etc/gdns")
	}

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("reading config: %w", err)
		}
	}

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	return cfg, nil
}
