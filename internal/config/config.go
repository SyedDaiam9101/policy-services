// internal/config/config.go
package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all configuration for the service
type Config struct {
	// Server configuration
	Port        int    `mapstructure:"port"`
	MetricsPort int    `mapstructure:"metrics_port"`
	Model       string `mapstructure:"model"`
	Redis       string `mapstructure:"redis"`

	// OpenTelemetry configuration
	OTELEnabled  bool   `mapstructure:"otel_enabled"`
	OTELEndpoint string `mapstructure:"otel_endpoint"`

	// Feature flags
	UseMockInference bool `mapstructure:"use_mock_inference"`
}

// Load loads configuration from flags, environment variables, and optional config file.
// Priority (highest to lowest): flags > env vars > config file > defaults
func Load() (*Config, error) {
	v := viper.New()

	// Set defaults
	v.SetDefault("port", 50051)
	v.SetDefault("metrics_port", 9100)
	v.SetDefault("model", "policy_cpu.onnx")
	v.SetDefault("redis", "localhost:6379")
	v.SetDefault("otel_enabled", false)
	v.SetDefault("otel_endpoint", "")
	v.SetDefault("use_mock_inference", false)

	// Environment variable configuration
	v.SetEnvPrefix("POLICY_SERVICE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Also read OTEL standard env vars
	if otelEndpoint := viper.GetString("OTEL_EXPORTER_OTLP_ENDPOINT"); otelEndpoint != "" {
		v.Set("otel_endpoint", otelEndpoint)
		v.Set("otel_enabled", true)
	}

	// Bind specific environment variables
	v.BindEnv("port", "POLICY_SERVICE_PORT")
	v.BindEnv("metrics_port", "POLICY_SERVICE_METRICS_PORT")
	v.BindEnv("model", "POLICY_SERVICE_MODEL")
	v.BindEnv("redis", "POLICY_SERVICE_REDIS")
	v.BindEnv("otel_enabled", "POLICY_SERVICE_OTEL_ENABLED")
	v.BindEnv("otel_endpoint", "POLICY_SERVICE_OTEL_ENDPOINT", "OTEL_EXPORTER_OTLP_ENDPOINT")
	v.BindEnv("use_mock_inference", "POLICY_SERVICE_USE_MOCK")

	// Config file (optional)
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("/etc/policy-service/")
	v.AddConfigPath("$HOME/.policy-service")

	// Read config file if present (ignore error if not found)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// Config file was found but another error occurred
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
		// Config file not found; ignore
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

// LoadWithConfigFile loads configuration from a specific config file
func LoadWithConfigFile(configPath string) (*Config, error) {
	v := viper.New()

	// Set defaults (same as Load)
	v.SetDefault("port", 50051)
	v.SetDefault("metrics_port", 9100)
	v.SetDefault("model", "policy_cpu.onnx")
	v.SetDefault("redis", "localhost:6379")
	v.SetDefault("otel_enabled", false)
	v.SetDefault("otel_endpoint", "")
	v.SetDefault("use_mock_inference", false)

	// Environment variable configuration
	v.SetEnvPrefix("POLICY_SERVICE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Read specific config file
	v.SetConfigFile(configPath)
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("error reading config file %s: %w", configPath, err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("invalid port: %d", c.Port)
	}
	if c.MetricsPort <= 0 || c.MetricsPort > 65535 {
		return fmt.Errorf("invalid metrics port: %d", c.MetricsPort)
	}
	if c.Port == c.MetricsPort {
		return fmt.Errorf("port and metrics_port must be different")
	}
	if c.Model == "" && !c.UseMockInference {
		return fmt.Errorf("model path is required when not using mock inference")
	}
	return nil
}
