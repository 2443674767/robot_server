package waypoint

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const (
	defaultDeviceAPIHost      = "10.21.31.100"
	defaultDeviceAPIPort      = 9000
	defaultDeviceAPITimeoutMS = 5000
)

type deviceExtraConfig struct {
	Host      string
	Port      int
	TimeoutMS int
}

func loadDeviceExtraConfig() deviceExtraConfig {
	cfg := deviceExtraConfig{
		Host:      defaultDeviceAPIHost,
		Port:      defaultDeviceAPIPort,
		TimeoutMS: defaultDeviceAPITimeoutMS,
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
		cfg.Host = v
	}
	if v := vip.GetInt("data.status_api_port"); v > 0 {
		cfg.Port = v
	}
	if v := vip.GetInt("data.status_api_timeout_ms"); v > 0 {
		cfg.TimeoutMS = v
	}
	return cfg
}

func (cfg deviceExtraConfig) baseURL() string {
	host := strings.TrimSpace(cfg.Host)
	if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
		return strings.TrimRight(host, "/")
	}
	return "http://" + host + ":" + strconv.Itoa(cfg.Port)
}

func (cfg deviceExtraConfig) timeout() time.Duration {
	return time.Duration(cfg.TimeoutMS) * time.Millisecond
}

func deviceExtraGet(ctx context.Context, path string, query url.Values) (map[string]interface{}, error) {
	cfg := loadDeviceExtraConfig()
	reqCtx, cancel := context.WithTimeout(ctx, cfg.timeout())
	defer cancel()
	target := cfg.baseURL() + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	return doDeviceExtraRequest(req)
}

func deviceExtraPost(ctx context.Context, path string, payload map[string]interface{}) (map[string]interface{}, error) {
	cfg := loadDeviceExtraConfig()
	reqCtx, cancel := context.WithTimeout(ctx, cfg.timeout())
	defer cancel()
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, cfg.baseURL()+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return doDeviceExtraRequest(req)
}

func doDeviceExtraRequest(req *http.Request) (map[string]interface{}, error) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("设备接口HTTP状态异常:%d", resp.StatusCode)
	}
	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	return data, nil
}
