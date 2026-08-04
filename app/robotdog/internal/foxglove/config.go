package foxglove

import (
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
)

const (
	fallbackWSURL     = "ws://10.21.31.100:8765"
	fallbackTopic     = "/base_link/odom"
	fallbackTimeoutMS = 5000
)

type Config struct {
	FoxgloveWSURL     string
	FoxgloveTopic     string
	FoxgloveTimeoutMS int
}

func LoadConfig() Config {
	cfg := Config{
		FoxgloveWSURL:     fallbackWSURL,
		FoxgloveTopic:     fallbackTopic,
		FoxgloveTimeoutMS: fallbackTimeoutMS,
	}
	wd, err := os.Getwd()
	if err != nil {
		return cfg
	}
	vip := viper.New()
	vip.AddConfigPath(filepath.Join(wd, "resource/config"))
	vip.SetConfigName("robotdog")
	vip.SetConfigType("yaml")
	if err := vip.ReadInConfig(); err != nil {
		return cfg
	}
	if v := vip.GetString("data.foxglove_ws_url"); v != "" {
		cfg.FoxgloveWSURL = v
	}
	if v := vip.GetString("data.foxglove_topic"); v != "" {
		cfg.FoxgloveTopic = v
	}
	if v := vip.GetInt("data.foxglove_timeout_ms"); v > 0 {
		cfg.FoxgloveTimeoutMS = v
	}
	return cfg
}

func (cfg Config) Timeout() time.Duration {
	if cfg.FoxgloveTimeoutMS <= 0 {
		return time.Duration(fallbackTimeoutMS) * time.Millisecond
	}
	return time.Duration(cfg.FoxgloveTimeoutMS) * time.Millisecond
}
