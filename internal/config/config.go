package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	LogLevel    string `mapstructure:"log_level"`
	DatabaseDir string `mapstructure:"database_dir"`
	GRPC        GRPC   `mapstructure:"grpc"`
	HTTP        HTTP   `mapstructure:"http"`
}

type GRPC struct {
	ListenAddress string `mapstructure:"listen_address"`
}

type HTTP struct {
	ListenAddress string `mapstructure:"listen_address"`
}

func Load(path string) (*Config, error) {
	if path != "" {
		viper.SetConfigFile(path)
	} else {
		viper.SetConfigName("config")
		viper.SetConfigType("toml")
		viper.AddConfigPath(".")
		viper.AddConfigPath("configs")
	}

	viper.SetDefault("log_level", "INFO")
	viper.SetDefault("database_dir", ".")
	viper.SetDefault("grpc.listen_address", "[::1]:50052")
	viper.SetDefault("http.listen_address", "127.0.0.1:8090")

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	return &cfg, nil
}
