package control

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

const (
	defaultDogUDPHost      = "10.21.31.103"
	defaultDogUDPPort      = 30000
	defaultDogUDPLocalPort = "30002"
	defaultPTZUDPHost      = "10.21.31.111"
	defaultPTZUDPPort      = 3000
	defaultPTZUDPLocalPort = "3000"
)

type UDPConfig struct {
	DogHost      string
	DogPort      int
	DogLocalPort string
	PTZHost      string
	PTZPort      int
	PTZLocalPort string
}

func LoadUDPConfig() UDPConfig {
	cfg := UDPConfig{
		DogHost:      defaultDogUDPHost,
		DogPort:      defaultDogUDPPort,
		DogLocalPort: defaultDogUDPLocalPort,
		PTZHost:      defaultPTZUDPHost,
		PTZPort:      defaultPTZUDPPort,
		PTZLocalPort: defaultPTZUDPLocalPort,
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
	if v := strings.TrimSpace(vip.GetString("data.dog_udp_host")); v != "" {
		cfg.DogHost = v
	}
	if v := vip.GetInt("data.dog_udp_port"); v > 0 {
		cfg.DogPort = v
	}
	if v := strings.TrimSpace(vip.GetString("data.dog_udp_local_port")); v != "" {
		cfg.DogLocalPort = v
	}
	if v := strings.TrimSpace(vip.GetString("data.ptz_udp_host")); v != "" {
		cfg.PTZHost = v
	}
	if v := vip.GetInt("data.ptz_udp_port"); v > 0 {
		cfg.PTZPort = v
	}
	if v := strings.TrimSpace(vip.GetString("data.ptz_udp_local_port")); v != "" {
		cfg.PTZLocalPort = v
	}
	return cfg
}

func FillDogTargetDefaults(target DogTarget) DogTarget {
	cfg := LoadUDPConfig()
	if strings.TrimSpace(target.UDPHost) == "" {
		target.UDPHost = cfg.DogHost
	}
	if target.UDPPort == 0 {
		target.UDPPort = int32(cfg.DogPort)
	}
	return target
}

func FillPTZTargetDefaults(target PTZTarget) PTZTarget {
	cfg := LoadUDPConfig()
	if strings.TrimSpace(target.UDPHost) == "" {
		target.UDPHost = cfg.PTZHost
	}
	if target.UDPPort == 0 {
		target.UDPPort = int32(cfg.PTZPort)
	}
	return target
}
