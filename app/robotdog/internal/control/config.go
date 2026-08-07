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
	defaultPTZUDPHost      = "10.21.31.64"
	defaultPTZUDPPort      = 1030
	defaultPTZUDPLocalPort = "0"
)

type UDPConfig struct {
	DogHost              string
	DogPort              int
	DogLocalPort         string
	PTZHost              string
	PTZPort              int
	PTZLocalPort         string
	PTZTargetSystemID    int
	PTZTargetComponentID int
	PTZSourceSystemID    int
	PTZSourceComponentID int
}

func LoadUDPConfig() UDPConfig {
	cfg := UDPConfig{
		DogHost:              defaultDogUDPHost,
		DogPort:              defaultDogUDPPort,
		DogLocalPort:         defaultDogUDPLocalPort,
		PTZHost:              defaultPTZUDPHost,
		PTZPort:              defaultPTZUDPPort,
		PTZLocalPort:         defaultPTZUDPLocalPort,
		PTZTargetSystemID:    3,
		PTZTargetComponentID: 1,
		PTZSourceSystemID:    1,
		PTZSourceComponentID: 1,
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
	if v := vip.GetInt("data.ptz_target_system_id"); v > 0 {
		cfg.PTZTargetSystemID = v
	}
	if v := vip.GetInt("data.ptz_target_component_id"); v > 0 {
		cfg.PTZTargetComponentID = v
	}
	if v := vip.GetInt("data.ptz_source_system_id"); v > 0 {
		cfg.PTZSourceSystemID = v
	}
	if v := vip.GetInt("data.ptz_source_component_id"); v > 0 {
		cfg.PTZSourceComponentID = v
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
	if target.LocalPort == 0 {
		target.LocalPort = int32(parsePositiveInt(cfg.PTZLocalPort))
	}
	if target.TargetSystemID == 0 {
		target.TargetSystemID = byte(cfg.PTZTargetSystemID)
	}
	if target.TargetComponentID == 0 {
		target.TargetComponentID = byte(cfg.PTZTargetComponentID)
	}
	if target.SourceSystemID == 0 {
		target.SourceSystemID = byte(cfg.PTZSourceSystemID)
	}
	if target.SourceComponentID == 0 {
		target.SourceComponentID = byte(cfg.PTZSourceComponentID)
	}
	return target
}

func parsePositiveInt(v string) int {
	n := 0
	for _, ch := range strings.TrimSpace(v) {
		if ch < '0' || ch > '9' {
			return 0
		}
		n = n*10 + int(ch-'0')
	}
	return n
}
