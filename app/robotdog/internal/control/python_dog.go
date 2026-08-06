package control

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type PythonDogDriver struct {
	Name       string
	ScriptPath string
	PythonBin  string
	LocalPort  string
}

func NewPythonDogDriver() *PythonDogDriver {
	cfg := LoadUDPConfig()
	return &PythonDogDriver{
		Name:       "yunshenchu_m20_python",
		ScriptPath: "pysdk/robotdog_m20.py",
		PythonBin:  envDefault("ROBOTDOG_PYTHON_BIN", "python3"),
		LocalPort:  envDefault("ROBOTDOG_PYSDK_LOCAL_PORT", cfg.DogLocalPort),
	}
}

func (d *PythonDogDriver) MoveLeft(ctx context.Context, target DogTarget, opts MoveOptions) (*CommandResult, error) {
	return d.run(ctx, target, "left", opts)
}

func (d *PythonDogDriver) MoveRight(ctx context.Context, target DogTarget, opts MoveOptions) (*CommandResult, error) {
	return d.run(ctx, target, "right", opts)
}

func (d *PythonDogDriver) MoveForward(ctx context.Context, target DogTarget, opts MoveOptions) (*CommandResult, error) {
	return d.run(ctx, target, "forward", opts)
}

func (d *PythonDogDriver) MoveBackward(ctx context.Context, target DogTarget, opts MoveOptions) (*CommandResult, error) {
	return d.run(ctx, target, "backward", opts)
}

func (d *PythonDogDriver) Stop(ctx context.Context, target DogTarget) (*CommandResult, error) {
	return d.run(ctx, target, "stop", MoveOptions{})
}

func (d *PythonDogDriver) SetGait(ctx context.Context, target DogTarget, gait string) (*CommandResult, error) {
	switch strings.TrimSpace(strings.ToLower(gait)) {
	case "basic":
		return d.run(ctx, target, "gait-basic", MoveOptions{})
	case "stair":
		return d.run(ctx, target, "gait-stair", MoveOptions{})
	default:
		return nil, fmt.Errorf("不支持的步态类型")
	}
}

func (d *PythonDogDriver) Charge(ctx context.Context, target DogTarget, action string) (*CommandResult, error) {
	switch strings.TrimSpace(strings.ToLower(action)) {
	case "enter":
		return d.run(ctx, target, "charge-enter", MoveOptions{})
	case "exit":
		return d.run(ctx, target, "charge-exit", MoveOptions{})
	case "clear":
		return d.run(ctx, target, "charge-clear", MoveOptions{})
	default:
		return nil, fmt.Errorf("不支持的充电桩动作")
	}
}

func (d *PythonDogDriver) Realtime(ctx context.Context, target DogTarget) (*RealtimeData, error) {
	target = FillDogTargetDefaults(target)
	output, elapsed, err := d.execScript(ctx, target, "status", 0, 1)
	if err != nil {
		return nil, err
	}
	battery, navStatus := parseDogRealtimeOutput(output)
	data := map[string]interface{}{
		"model":    target.Model,
		"udp_host": target.UDPHost,
		"udp_port": target.UDPPort,
		"elapsed":  elapsed.String(),
		"output":   output,
	}
	if battery != nil {
		data["battery"] = *battery
	}
	if navStatus != "" {
		data["nav_status"] = navStatus
		data["control_status"] = navStatus
	}
	return &RealtimeData{
		DeviceType: "dog",
		TargetID:   target.ID,
		Driver:     d.Name,
		At:         time.Now(),
		Battery:    battery,
		NavStatus:  navStatus,
		Data:       data,
	}, nil
}

func (d *PythonDogDriver) run(ctx context.Context, target DogTarget, action string, opts MoveOptions) (*CommandResult, error) {
	target = FillDogTargetDefaults(target)
	speed := normalizePythonSpeed(opts.Speed)
	duration := normalizePythonDuration(opts.Duration)
	if action == "stop" {
		duration = 0
	}
	output, elapsed, err := d.execScript(ctx, target, action, speed, duration)
	if err != nil {
		return nil, err
	}
	return &CommandResult{
		DeviceType:  "dog",
		Driver:      d.Name,
		TargetID:    target.ID,
		TargetAddr:  target.Addr(),
		Command:     action,
		PayloadHex:  "",
		SentBytes:   0,
		SentAt:      time.Now(),
		Elapsed:     elapsed,
		Placeholder: false,
		Script:      d.scriptDisplayPath(),
		Output:      output,
	}, nil
}

func (d *PythonDogDriver) execScript(ctx context.Context, target DogTarget, action string, speed float64, duration float64) (string, time.Duration, error) {
	target = FillDogTargetDefaults(target)
	if target.UDPHost == "" || target.UDPPort == 0 {
		return "", 0, fmt.Errorf("机械狗UDP地址未配置")
	}
	scriptPath, err := d.resolveScriptPath()
	if err != nil {
		return "", 0, err
	}
	args := []string{
		scriptPath,
		action,
		"--host", target.UDPHost,
		"--port", strconv.Itoa(int(target.UDPPort)),
	}
	if action != "stop" && action != "status" {
		args = append(args, "--speed", fmt.Sprintf("%.3f", speed), "--duration", fmt.Sprintf("%.3f", duration))
	}
	if action == "status" {
		args = append(args, "--duration", fmt.Sprintf("%.3f", duration))
	}
	if d.LocalPort != "" {
		args = append(args, "--local-port", d.LocalPort)
	}
	timeout := time.Duration((duration + 3) * float64(time.Second))
	if timeout < 5*time.Second {
		timeout = 5 * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	start := time.Now()
	cmd := exec.CommandContext(runCtx, d.PythonBin, args...)
	cmd.Dir = filepath.Dir(scriptPath)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	elapsed := time.Since(start)
	output := strings.TrimSpace(stdout.String())
	if err != nil {
		errText := strings.TrimSpace(stderr.String())
		if errText != "" {
			return output, elapsed, fmt.Errorf("%w: %s", err, errText)
		}
		return output, elapsed, err
	}
	if stderr.Len() > 0 {
		output = strings.TrimSpace(output + "\n" + stderr.String())
	}
	return output, elapsed, nil
}

func (d *PythonDogDriver) resolveScriptPath() (string, error) {
	if filepath.IsAbs(d.ScriptPath) {
		return d.ScriptPath, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(cwd, d.ScriptPath)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			break
		}
		cwd = parent
	}
	return "", fmt.Errorf("未找到Python控制脚本: %s", d.ScriptPath)
}

func (d *PythonDogDriver) scriptDisplayPath() string {
	if filepath.IsAbs(d.ScriptPath) {
		return d.ScriptPath
	}
	if scriptPath, err := d.resolveScriptPath(); err == nil {
		return scriptPath
	}
	return d.ScriptPath
}

func normalizePythonSpeed(speed float64) float64 {
	if speed <= 0 {
		return 0.5
	}
	if speed > 1 {
		speed = speed / 100
	}
	if speed > 1 {
		return 1
	}
	return speed
}

func normalizePythonDuration(duration int) float64 {
	if duration <= 0 {
		return 1
	}
	if duration > 10 {
		return float64(duration) / 1000
	}
	return float64(duration)
}

func envDefault(key string, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func parseDogRealtimeOutput(output string) (*int, string) {
	battery := parseDogBattery(output)
	mode := firstRealtimeLabel(output, `ControlUsageMode[^\n:]*:\s*[^\(\n]*\(([^)]+)\)`)
	motion := firstRealtimeLabel(output, `MotionState[^\n:]*:\s*[^\(\n]*\(([^)]+)\)`)
	navStatus := strings.TrimSpace(strings.Join(nonEmptyStrings(mode, motion), " "))
	return battery, navStatus
}

func parseDogBattery(output string) *int {
	matches := regexp.MustCompile(`BatteryLevel(?:Left|Right)?[^\n:]*:\s*([0-9]+(?:\.[0-9]+)?)%?`).FindAllStringSubmatch(output, -1)
	if len(matches) == 0 {
		return nil
	}
	minValue := 101
	for _, match := range matches {
		value, err := strconv.ParseFloat(match[1], 64)
		if err != nil {
			continue
		}
		n := int(value + 0.5)
		if n < minValue {
			minValue = n
		}
	}
	if minValue < 0 {
		minValue = 0
	}
	if minValue > 100 {
		minValue = 100
	}
	if minValue == 101 {
		return nil
	}
	return &minValue
}

func firstRealtimeLabel(output string, pattern string) string {
	match := regexp.MustCompile(pattern).FindStringSubmatch(output)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func nonEmptyStrings(values ...string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && value != "未知" {
			result = append(result, value)
		}
	}
	return result
}
