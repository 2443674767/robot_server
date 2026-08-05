package task

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const (
	defaultStatusAPIHost      = "10.21.31.100"
	defaultStatusAPIPort      = 9000
	defaultStatusAPITimeoutMS = 5000
	defaultStatusPollSec      = 2
	defaultStatusMaxWaitSec   = 180
	defaultAlignWaitSec       = 10
)

type robotdogTaskConfig struct {
	StatusAPIHost      string
	StatusAPIPort      int
	StatusAPITimeoutMS int
	StatusPollSec      int
	StatusMaxWaitSec   int
	AlignWaitSec       int
}

type dogStatus struct {
	Status         string  `json:"status"`
	MapName        string  `json:"map_name"`
	Package        string  `json:"package"`
	ControlStatus  string  `json:"control_status"`
	TaskStatus     string  `json:"task_status"`
	PositionMsg    string  `json:"position_msg"`
	PubVelMsg      string  `json:"pub_vel_msg"`
	Battery        string  `json:"battery"`
	CPUTemperature float64 `json:"cpu_temperature"`
	CPUUsage       float64 `json:"cpu_usage"`
	MemoryPercent  float64 `json:"memory_percent"`
	Version        string  `json:"version"`
}

type navCustomRequest struct {
	InputValue   string  `json:"inputValue"`
	OrientationW float64 `json:"orientationW"`
	OrientationX float64 `json:"orientationX"`
	OrientationY float64 `json:"orientationY"`
	OrientationZ float64 `json:"orientationZ"`
	PositionX    float64 `json:"positionX"`
	PositionY    float64 `json:"positionY"`
	PositionZ    float64 `json:"positionZ"`
}

type navCustomResponse struct {
	Msg     string `json:"msg"`
	Success bool   `json:"success"`
}

func loadTaskConfig() robotdogTaskConfig {
	cfg := robotdogTaskConfig{
		StatusAPIHost:      defaultStatusAPIHost,
		StatusAPIPort:      defaultStatusAPIPort,
		StatusAPITimeoutMS: defaultStatusAPITimeoutMS,
		StatusPollSec:      defaultStatusPollSec,
		StatusMaxWaitSec:   defaultStatusMaxWaitSec,
		AlignWaitSec:       defaultAlignWaitSec,
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
	if v := strings.TrimSpace(vip.GetString("data.status_api_host")); v != "" {
		cfg.StatusAPIHost = v
	}
	if v := vip.GetInt("data.status_api_port"); v > 0 {
		cfg.StatusAPIPort = v
	}
	if v := vip.GetInt("data.status_api_timeout_ms"); v > 0 {
		cfg.StatusAPITimeoutMS = v
	}
	if v := vip.GetInt("data.status_poll_interval_sec"); v > 0 {
		cfg.StatusPollSec = v
	}
	if v := vip.GetInt("data.status_max_wait_sec"); v > 0 {
		cfg.StatusMaxWaitSec = v
	}
	if v := vip.GetInt("data.align_wait_sec"); v > 0 {
		cfg.AlignWaitSec = v
	}
	return cfg
}

func (cfg robotdogTaskConfig) statusURL() string {
	return cfg.extraURL("/api/extra/get_status")
}

func (cfg robotdogTaskConfig) navCustomURL() string {
	return cfg.extraURL("/api/extra/nav_custom")
}

func (cfg robotdogTaskConfig) extraURL(path string) string {
	host := strings.TrimSpace(cfg.StatusAPIHost)
	if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
		return strings.TrimRight(host, "/") + path
	}
	return "http://" + host + ":" + strconv.Itoa(cfg.StatusAPIPort) + path
}

func (cfg robotdogTaskConfig) timeout() time.Duration {
	return time.Duration(cfg.StatusAPITimeoutMS) * time.Millisecond
}

func (cfg robotdogTaskConfig) pollInterval() time.Duration {
	return time.Duration(cfg.StatusPollSec) * time.Second
}

func (cfg robotdogTaskConfig) maxWait() time.Duration {
	return time.Duration(cfg.StatusMaxWaitSec) * time.Second
}

func (cfg robotdogTaskConfig) alignWait() time.Duration {
	return time.Duration(cfg.AlignWaitSec) * time.Second
}

func fetchDogStatus(ctx context.Context, cfg robotdogTaskConfig) (*dogStatus, error) {
	reqCtx, cancel := context.WithTimeout(ctx, cfg.timeout())
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, cfg.statusURL(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("状态接口HTTP状态异常:%d", resp.StatusCode)
	}
	var data dogStatus
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	return &data, nil
}

func postNavCustom(ctx context.Context, cfg robotdogTaskConfig, payload navCustomRequest) (*navCustomResponse, error) {
	reqCtx, cancel := context.WithTimeout(ctx, cfg.timeout())
	defer cancel()
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, cfg.navCustomURL(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("导航接口HTTP状态异常:%d", resp.StatusCode)
	}
	var data navCustomResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	if !data.Success {
		if strings.TrimSpace(data.Msg) == "" {
			data.Msg = "导航接口返回失败"
		}
		return &data, fmt.Errorf("%s", data.Msg)
	}
	return &data, nil
}
