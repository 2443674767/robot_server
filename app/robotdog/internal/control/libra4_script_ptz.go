package control

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Libra4ScriptPTZDriver struct {
	Name       string
	ScriptPath string
	PythonBin  string
	Legacy     *Libra4PTZDriver
}

type libra4ScriptOutput struct {
	OK      bool                   `json:"ok"`
	Command string                 `json:"command"`
	Status  libra4ScriptStatus     `json:"status"`
	Result  map[string]interface{} `json:"result"`
	Error   string                 `json:"error"`
}

type libra4ScriptStatus struct {
	Yaw         *float64 `json:"yaw"`
	Pitch       *float64 `json:"pitch"`
	PitchOffset *float64 `json:"pitch_offset"`
	Roll        *float64 `json:"roll"`
	Zoom        *float64 `json:"zoom"`
	AttitudeAt  *float64 `json:"attitude_at"`
	CameraAt    *float64 `json:"camera_at"`
}

func NewLibra4ScriptPTZDriver() *Libra4ScriptPTZDriver {
	return &Libra4ScriptPTZDriver{
		Name:       "libra4_udp_script",
		ScriptPath: "pysdk/libra4_ptz_control.py",
		PythonBin:  envDefault("ROBOTDOG_PYTHON_BIN", "python3"),
		Legacy:     NewLibra4PTZDriver(),
	}
}

func (d *Libra4ScriptPTZDriver) Up(ctx context.Context, target PTZTarget, opts PTZMoveOptions) (*CommandResult, error) {
	return d.runSimple(ctx, target, "up", []string{"up", "--step", fmtFloat(libra4PitchStep(opts))}, 7*time.Second)
}

func (d *Libra4ScriptPTZDriver) Down(ctx context.Context, target PTZTarget, opts PTZMoveOptions) (*CommandResult, error) {
	return d.runSimple(ctx, target, "down", []string{"down", "--step", fmtFloat(libra4PitchStep(opts))}, 7*time.Second)
}

func (d *Libra4ScriptPTZDriver) Left(ctx context.Context, target PTZTarget, opts PTZMoveOptions) (*CommandResult, error) {
	return d.runSimple(ctx, target, "left", []string{"left", "--step", fmtFloat(libra4YawStep(opts))}, 7*time.Second)
}

func (d *Libra4ScriptPTZDriver) Right(ctx context.Context, target PTZTarget, opts PTZMoveOptions) (*CommandResult, error) {
	return d.runSimple(ctx, target, "right", []string{"right", "--step", fmtFloat(libra4YawStep(opts))}, 7*time.Second)
}

func (d *Libra4ScriptPTZDriver) ZoomIn(ctx context.Context, target PTZTarget, opts ZoomOptions) (*CommandResult, error) {
	return d.runSimple(ctx, target, "zoom_in", []string{"zoom-in", "--step", fmtFloat(libra4ZoomStep(opts))}, 7*time.Second)
}

func (d *Libra4ScriptPTZDriver) ZoomOut(ctx context.Context, target PTZTarget, opts ZoomOptions) (*CommandResult, error) {
	return d.runSimple(ctx, target, "zoom_out", []string{"zoom-out", "--step", fmtFloat(libra4ZoomStep(opts))}, 7*time.Second)
}

func (d *Libra4ScriptPTZDriver) Nudge(ctx context.Context, target PTZTarget, axis string, delta float64, zoomMax float64) (*CommandResult, error) {
	if zoomMax <= 0 {
		zoomMax = 30
	}
	args := []string{"nudge", "--axis", strings.ToLower(strings.TrimSpace(axis)), "--delta", fmtFloat(delta), "--zoom-max", fmtFloat(zoomMax)}
	return d.runSimple(ctx, target, "nudge", args, 7*time.Second)
}

func (d *Libra4ScriptPTZDriver) FocusNear(ctx context.Context, target PTZTarget, opts FocusOptions) (*CommandResult, error) {
	return d.runLegacy(ctx, target, func() (*CommandResult, error) {
		return d.Legacy.FocusNear(ctx, target, opts)
	})
}

func (d *Libra4ScriptPTZDriver) FocusFar(ctx context.Context, target PTZTarget, opts FocusOptions) (*CommandResult, error) {
	return d.runLegacy(ctx, target, func() (*CommandResult, error) {
		return d.Legacy.FocusFar(ctx, target, opts)
	})
}

func (d *Libra4ScriptPTZDriver) Home(ctx context.Context, target PTZTarget) (*CommandResult, error) {
	return d.runSimple(ctx, target, "home", []string{"home"}, 6*time.Second)
}

func (d *Libra4ScriptPTZDriver) ZoomHome(ctx context.Context, target PTZTarget) (*CommandResult, error) {
	return d.runSimple(ctx, target, "zoom_home", []string{"zoom-home"}, 6*time.Second)
}

func (d *Libra4ScriptPTZDriver) SetAngle(ctx context.Context, target PTZTarget, opts PTZAngleOptions) (*CommandResult, error) {
	args := []string{"angle-set", "--yaw", fmtFloat(opts.Pan), "--pitch", fmtFloat(opts.Tilt)}
	return d.runSimple(ctx, target, "angle_set", args, 6*time.Second)
}

func (d *Libra4ScriptPTZDriver) TakePhoto(ctx context.Context, target PTZTarget, opts PhotoOptions) (*CommandResult, error) {
	return d.runLegacy(ctx, target, func() (*CommandResult, error) {
		return d.Legacy.TakePhoto(ctx, target, opts)
	})
}

func (d *Libra4ScriptPTZDriver) Stop(ctx context.Context, target PTZTarget) (*CommandResult, error) {
	return d.runSimple(ctx, target, "stop", []string{"stop"}, 5*time.Second)
}

func (d *Libra4ScriptPTZDriver) Refresh(ctx context.Context, target PTZTarget) (*CommandResult, error) {
	return d.runSimple(ctx, target, "refresh", []string{"refresh"}, 6*time.Second)
}

func (d *Libra4ScriptPTZDriver) Realtime(ctx context.Context, target PTZTarget) (*RealtimeData, error) {
	target = FillPTZTargetDefaults(target)
	output, parsed, elapsed, err := d.execScript(ctx, target, []string{"status", "--require", "attitude"}, 6*time.Second)
	if err != nil {
		return nil, err
	}
	return &RealtimeData{
		DeviceType: "ptz",
		TargetID:   target.ID,
		Driver:     d.Name,
		At:         time.Now(),
		Pitch:      parsed.Status.Pitch,
		Yaw:        parsed.Status.Yaw,
		Roll:       parsed.Status.Roll,
		Zoom:       parsed.Status.Zoom,
		Data: map[string]interface{}{
			"brand":        target.Brand,
			"model":        target.Model,
			"protocol":     target.Protocol,
			"udp_host":     target.UDPHost,
			"udp_port":     target.UDPPort,
			"local_port":   target.LocalPort,
			"pitch_offset": parsed.Status.PitchOffset,
			"elapsed":      elapsed.String(),
			"script":       d.scriptDisplayPath(),
			"output":       output,
		},
	}, nil
}

func (d *Libra4ScriptPTZDriver) runSimple(ctx context.Context, target PTZTarget, command string, scriptArgs []string, timeout time.Duration) (*CommandResult, error) {
	target = FillPTZTargetDefaults(target)
	output, _, elapsed, err := d.execScript(ctx, target, scriptArgs, timeout)
	if err != nil {
		return nil, err
	}
	sentBytes := 0
	if parsed, parseErr := parseLibra4Output(output); parseErr == nil && parsed.Result != nil {
		sentBytes = intFromInterface(parsed.Result["sent_bytes"])
	}
	return &CommandResult{
		DeviceType:  "ptz",
		Driver:      d.Name,
		TargetID:    target.ID,
		TargetAddr:  target.Addr(),
		Command:     command,
		SentBytes:   sentBytes,
		SentAt:      time.Now(),
		Elapsed:     elapsed,
		Placeholder: false,
		Script:      d.scriptDisplayPath(),
		Output:      output,
	}, nil
}

func (d *Libra4ScriptPTZDriver) runLegacy(ctx context.Context, target PTZTarget, call func() (*CommandResult, error)) (*CommandResult, error) {
	if d.Legacy == nil {
		return nil, fmt.Errorf("LIBRA4兼容驱动未初始化")
	}
	result, err := call()
	if result != nil {
		result.Driver = d.Name
	}
	return result, err
}

func (d *Libra4ScriptPTZDriver) execScript(ctx context.Context, target PTZTarget, scriptArgs []string, timeout time.Duration) (string, *libra4ScriptOutput, time.Duration, error) {
	target = FillPTZTargetDefaults(target)
	if target.UDPHost == "" || target.UDPPort == 0 {
		return "", nil, 0, fmt.Errorf("云台UDP地址未配置")
	}
	scriptPath, err := d.resolveScriptPath()
	if err != nil {
		return "", nil, 0, err
	}
	args := []string{
		scriptPath,
		"--host", target.UDPHost,
		"--port", strconv.Itoa(int(target.UDPPort)),
	}
	if target.LocalPort > 0 {
		args = append(args, "--local-port", strconv.Itoa(int(target.LocalPort)))
	}
	args = append(args, scriptArgs...)
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
	errText := strings.TrimSpace(stderr.String())
	if errText != "" {
		output = strings.TrimSpace(output + "\n" + errText)
	}
	parsed, parseErr := parseLibra4Output(output)
	if err != nil {
		if parsed != nil && parsed.Error != "" {
			return output, parsed, elapsed, errors.New(parsed.Error)
		}
		if errText != "" {
			return output, parsed, elapsed, fmt.Errorf("%w: %s", err, errText)
		}
		return output, parsed, elapsed, err
	}
	if parseErr != nil {
		return output, nil, elapsed, parseErr
	}
	if parsed != nil && !parsed.OK {
		if parsed.Error != "" {
			return output, parsed, elapsed, errors.New(parsed.Error)
		}
		return output, parsed, elapsed, fmt.Errorf("LIBRA4脚本返回失败")
	}
	return output, parsed, elapsed, nil
}

func (d *Libra4ScriptPTZDriver) resolveScriptPath() (string, error) {
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
	return "", fmt.Errorf("未找到LIBRA4 Python控制脚本: %s", d.ScriptPath)
}

func (d *Libra4ScriptPTZDriver) scriptDisplayPath() string {
	if filepath.IsAbs(d.ScriptPath) {
		return d.ScriptPath
	}
	if scriptPath, err := d.resolveScriptPath(); err == nil {
		return scriptPath
	}
	return d.ScriptPath
}

func parseLibra4Output(output string) (*libra4ScriptOutput, error) {
	output = strings.TrimSpace(output)
	if output == "" {
		return nil, fmt.Errorf("LIBRA4脚本无输出")
	}
	line := output
	if idx := strings.LastIndex(output, "\n"); idx >= 0 {
		line = strings.TrimSpace(output[idx+1:])
	}
	var parsed libra4ScriptOutput
	if err := json.Unmarshal([]byte(line), &parsed); err != nil {
		return nil, fmt.Errorf("解析LIBRA4脚本输出失败: %w", err)
	}
	return &parsed, nil
}

func libra4YawStep(opts PTZMoveOptions) float64 {
	return firstPositive(opts.Pan, opts.Speed, 5)
}

func libra4PitchStep(opts PTZMoveOptions) float64 {
	return firstPositive(opts.Tilt, opts.Speed, 2)
}

func libra4ZoomStep(opts ZoomOptions) float64 {
	return firstPositive(opts.Step, 0.5)
}

func firstPositive(values ...float64) float64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func fmtFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', 3, 64)
}

func intFromInterface(value interface{}) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case json.Number:
		i, _ := v.Int64()
		return int(i)
	default:
		return 0
	}
}
