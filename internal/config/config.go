package config

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all application configuration
type Config struct {
	Anna      AnnaConfig     `mapstructure:"anna"`
	Downloads DownloadConfig `mapstructure:"downloads"`
	Files     FileConfig     `mapstructure:"files"`
	Network   NetworkConfig  `mapstructure:"network"`
}

// AnnaConfig holds Anna's Archive settings
type AnnaConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
}

// DownloadConfig holds download settings
type DownloadConfig struct {
	Path          string `mapstructure:"path"`
	ChunkSize     int64  `mapstructure:"chunk_size"`
	MaxConcurrent int    `mapstructure:"max_concurrent"`
	AutoResume    bool   `mapstructure:"auto_resume"`
}

// FileConfig holds file preferences
type FileConfig struct {
	PreferredFormats []string `mapstructure:"preferred_formats"`
	OrganizeMode     string   `mapstructure:"organize_mode"`     // flat, author, format, year, custom
	OrganizePattern  string   `mapstructure:"organize_pattern"`  // custom pattern like {author}/{year}/{title}
	RenameFiles      bool     `mapstructure:"rename_files"`      // rename files based on metadata
}

// NetworkConfig holds network settings
type NetworkConfig struct {
	Timeout       time.Duration `mapstructure:"timeout"`
	RetryAttempts int           `mapstructure:"retry_attempts"`
	UserAgent     string        `mapstructure:"user_agent"`
}

var cfg *Config

// GetConfigDir returns the configuration directory path
func GetConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "bookdl")
}

// GetDBPath returns the database file path
func GetDBPath() string {
	return filepath.Join(GetConfigDir(), "bookdl.db")
}

// GetConfigPath returns the config file path
func GetConfigPath() string {
	return filepath.Join(GetConfigDir(), "config.yaml")
}

// Init initializes the configuration
func Init(cfgFile string) error {
	// Set defaults
	viper.SetDefault("anna.base_url", "annas-archive.li")
	viper.SetDefault("downloads.path", "~/Downloads/books")
	viper.SetDefault("downloads.chunk_size", 5*1024*1024) // 5MB
	viper.SetDefault("downloads.max_concurrent", 2)
	viper.SetDefault("downloads.auto_resume", true)
	viper.SetDefault("files.preferred_formats", []string{"epub", "pdf"})
	viper.SetDefault("files.organize_mode", "flat")
	viper.SetDefault("files.organize_pattern", "{author}/{title}")
	viper.SetDefault("files.rename_files", false)
	viper.SetDefault("network.timeout", 30*time.Second)
	viper.SetDefault("network.retry_attempts", 3)
	viper.SetDefault("network.user_agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(GetConfigDir())
	}

	// Environment variable overrides
	viper.SetEnvPrefix("BOOKDL")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Read config file (ignore if not found)
	_ = viper.ReadInConfig()

	return nil
}

// Get returns the current configuration
func Get() *Config {
	if cfg == nil {
		cfg = &Config{}
		viper.Unmarshal(cfg)
		cfg.Downloads.Path = expandPath(cfg.Downloads.Path)
	}
	return cfg
}

// Set sets a configuration value
func Set(key, value string) error {
	viper.Set(key, value)

	// Ensure config directory exists
	configDir := GetConfigDir()
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	// Reset cached config
	cfg = nil

	return viper.WriteConfigAs(GetConfigPath())
}

// GetValue retrieves a configuration value
func GetValue(key string) interface{} {
	return viper.Get(key)
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}
