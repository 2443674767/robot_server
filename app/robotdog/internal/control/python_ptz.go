package control

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type PythonPTZDriver struct {
	Name       string
	ScriptPath string
	PythonBin  string
	LocalPort  string
}

func NewPythonPTZDriver() *PythonPTZDriver {
	cfg := LoadUDPConfig()
	return &PythonPTZDriver{
		Name:       "hy_gimbal_python",
		ScriptPath: "pysdk/hy_gimbal_control.py",
		PythonBin:  envDefault("ROBOTDOG_PYTHON_BIN", "python3"),
		LocalPort:  envDefault("ROBOTDOG_PTZ_LOCAL_PORT", cfg.PTZLocalPort),
	}
}

func (d *PythonPTZDriver) Up(ctx context.Context, target PTZTarget, opts PTZMoveOptions) (*CommandResult, error) {
	return d.runRate(ctx, target, "up", normalizePTZRate(opts.Speed), 0, opts.Duration)
}

func (d *PythonPTZDriver) Down(ctx context.Context, target PTZTarget, opts PTZMoveOptions) (*CommandResult, error) {
	return d.runRate(ctx, target, "down", -normalizePTZRate(opts.Speed), 0, opts.Duration)
}

func (d *PythonPTZDriver) Left(ctx context.Context, target PTZTarget, opts PTZMoveOptions) (*CommandResult, error) {
	return d.runRate(ctx, target, "left", 0, -normalizePTZRate(opts.Speed), opts.Duration)
}

func (d *PythonPTZDriver) Right(ctx context.Context, target PTZTarget, opts PTZMoveOptions) (*CommandResult, error) {
	return d.runRate(ctx, target, "right", 0, normalizePTZRate(opts.Speed), opts.Duration)
}

func (d *PythonPTZDriver) ZoomIn(ctx context.Context, target PTZTarget, opts ZoomOptions) (*CommandResult, error) {
	return d.runZoom(ctx, target, "zoom-in", opts)
}

func (d *PythonPTZDriver) ZoomOut(ctx context.Context, target PTZTarget, opts ZoomOptions) (*CommandResult, error) {
	return d.runZoom(ctx, target, "zoom-out", opts)
}

func (d *PythonPTZDriver) FocusNear(ctx context.Context, target PTZTarget, opts FocusOptions) (*CommandResult, error) {
	return d.runSimple(ctx, target, "focus_near", []string{"focus", "near"}, 0)
}

func (d *PythonPTZDriver) FocusFar(ctx context.Context, target PTZTarget, opts FocusOptions) (*CommandResult, error) {
	return d.runSimple(ctx, target, "focus_far", []string{"focus", "far"}, 0)
}

func (d *PythonPTZDriver) Home(ctx context.Context, target PTZTarget) (*CommandResult, error) {
	return d.runSimple(ctx, target, "home", []string{"center"}, 0)
}

func (d *PythonPTZDriver) SetAngle(ctx context.Context, target PTZTarget, opts PTZAngleOptions) (*CommandResult, error) {
	return d.runSimple(ctx, target, "angle_set", []string{"angle-abs", "--pitch", fmt.Sprintf("%.3f", opts.Tilt), "--yaw", fmt.Sprintf("%.3f", opts.Pan)}, 0)
}

func (d *PythonPTZDriver) TakePhoto(ctx context.Context, target PTZTarget, opts PhotoOptions) (*CommandResult, error) {
	return d.runSimple(ctx, target, "photo", []string{"photo"}, 0)
}

func (d *PythonPTZDriver) Stop(ctx context.Context, target PTZTarget) (*CommandResult, error) {
	return d.runRate(ctx, target, "stop", 0, 0, 0)
}

func (d *PythonPTZDriver) Realtime(ctx context.Context, target PTZTarget) (*RealtimeData, error) {
	target = FillPTZTargetDefaults(target)
	output, elapsed, err := d.execScript(ctx, target, []string{"status", "--gimbal-seconds", "3", "--camera-seconds", "3"}, 8*time.Second)
	if err != nil {
		return nil, err
	}
	return &RealtimeData{
		DeviceType: "ptz",
		TargetID:   target.ID,
		Driver:     d.Name,
		At:         time.Now(),
		Data: map[string]interface{}{
			"brand":    target.Brand,
			"model":    target.Model,
			"protocol": target.Protocol,
			"udp_host": target.UDPHost,
			"udp_port": target.UDPPort,
			"elapsed":  elapsed.String(),
			"output":   output,
		},
	}, nil
}

func (d *PythonPTZDriver) runRate(ctx context.Context, target PTZTarget, command string, pitchRate float64, yawRate float64, duration int) (*CommandResult, error) {
	args := []string{"rate", "--pitch-rate", fmt.Sprintf("%.3f", pitchRate), "--yaw-rate", fmt.Sprintf("%.3f", yawRate)}
	seconds := normalizePTZDuration(duration)
	if seconds > 0 {
		args = append(args, "--duration", fmt.Sprintf("%.3f", seconds))
	}
	return d.runSimple(ctx, target, command, args, seconds)
}

func (d *PythonPTZDriver) runZoom(ctx context.Context, target PTZTarget, command string, opts ZoomOptions) (*CommandResult, error) {
	step := int(opts.Step)
	if step <= 0 {
		step = 1
	}
	return d.runSimple(ctx, target, command, []string{command, "--step", strconv.Itoa(step)}, 0)
}

func (d *PythonPTZDriver) runSimple(ctx context.Context, target PTZTarget, command string, scriptArgs []string, duration float64) (*CommandResult, error) {
	target = FillPTZTargetDefaults(target)
	output, elapsed, err := d.execScript(ctx, target, scriptArgs, time.Duration((duration+6)*float64(time.Second)))
	if err != nil {
		return nil, err
	}
	return &CommandResult{
		DeviceType:  "ptz",
		Driver:      d.Name,
		TargetID:    target.ID,
		TargetAddr:  target.Addr(),
		Command:     command,
		SentAt:      time.Now(),
		Elapsed:     elapsed,
		Placeholder: false,
		Script:      d.scriptDisplayPath(),
		Output:      output,
	}, nil
}

func (d *PythonPTZDriver) execScript(ctx context.Context, target PTZTarget, scriptArgs []string, timeout time.Duration) (string, time.Duration, error) {
	target = FillPTZTargetDefaults(target)
	if target.UDPHost == "" || target.UDPPort == 0 {
		return "", 0, fmt.Errorf("云台UDP地址未配置")
	}
	scriptPath, err := d.resolveScriptPath()
	if err != nil {
		return "", 0, err
	}
	args := []string{
		scriptPath,
		"--ip", target.UDPHost,
		"--device-port", strconv.Itoa(int(target.UDPPort)),
		"--connect-seconds", "1",
		"--allow-ephemeral-port",
	}
	if d.LocalPort != "" {
		args = append(args, "--local-port", d.LocalPort)
	}
	args = append(args, scriptArgs...)
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

func (d *PythonPTZDriver) resolveScriptPath() (string, error) {
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

func (d *PythonPTZDriver) scriptDisplayPath() string {
	if filepath.IsAbs(d.ScriptPath) {
		return d.ScriptPath
	}
	if scriptPath, err := d.resolveScriptPath(); err == nil {
		return scriptPath
	}
	return d.ScriptPath
}

func normalizePTZRate(speed float64) float64 {
	if speed <= 0 {
		return 30
	}
	if speed <= 1 {
		return speed * 60
	}
	return speed
}

func normalizePTZDuration(duration int) float64 {
	if duration <= 0 {
		return 0
	}
	if duration > 10 {
		return float64(duration) / 1000
	}
	return float64(duration)
}
