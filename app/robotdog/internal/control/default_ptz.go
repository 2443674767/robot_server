package control

import (
	"context"
	"fmt"
	"time"
)

type DefaultPTZDriver struct {
	Name string
}

func NewDefaultPTZDriver() *DefaultPTZDriver {
	return &DefaultPTZDriver{Name: "default"}
}

func (d *DefaultPTZDriver) Up(ctx context.Context, target PTZTarget, opts PTZMoveOptions) (*CommandResult, error) {
	return d.sendMove(ctx, target, "up", 0x11, opts)
}

func (d *DefaultPTZDriver) Down(ctx context.Context, target PTZTarget, opts PTZMoveOptions) (*CommandResult, error) {
	return d.sendMove(ctx, target, "down", 0x12, opts)
}

func (d *DefaultPTZDriver) Left(ctx context.Context, target PTZTarget, opts PTZMoveOptions) (*CommandResult, error) {
	return d.sendMove(ctx, target, "left", 0x13, opts)
}

func (d *DefaultPTZDriver) Right(ctx context.Context, target PTZTarget, opts PTZMoveOptions) (*CommandResult, error) {
	return d.sendMove(ctx, target, "right", 0x14, opts)
}

func (d *DefaultPTZDriver) ZoomIn(ctx context.Context, target PTZTarget, opts ZoomOptions) (*CommandResult, error) {
	return d.send(ctx, target, "zoom_in", buildFrame(0x02, 0x21, scaleToByte(opts.Step, 1), durationToUint16(opts.Duration)))
}

func (d *DefaultPTZDriver) ZoomOut(ctx context.Context, target PTZTarget, opts ZoomOptions) (*CommandResult, error) {
	return d.send(ctx, target, "zoom_out", buildFrame(0x02, 0x22, scaleToByte(opts.Step, 1), durationToUint16(opts.Duration)))
}

func (d *DefaultPTZDriver) FocusNear(ctx context.Context, target PTZTarget, opts FocusOptions) (*CommandResult, error) {
	return d.send(ctx, target, "focus_near", buildFrame(0x02, 0x31, scaleToByte(opts.Step, 1), durationToUint16(opts.Duration)))
}

func (d *DefaultPTZDriver) FocusFar(ctx context.Context, target PTZTarget, opts FocusOptions) (*CommandResult, error) {
	return d.send(ctx, target, "focus_far", buildFrame(0x02, 0x32, scaleToByte(opts.Step, 1), durationToUint16(opts.Duration)))
}

func (d *DefaultPTZDriver) Home(ctx context.Context, target PTZTarget) (*CommandResult, error) {
	return d.send(ctx, target, "home", buildFrame(0x02, 0x40, 0, 0))
}

func (d *DefaultPTZDriver) SetAngle(ctx context.Context, target PTZTarget, opts PTZAngleOptions) (*CommandResult, error) {
	return d.send(ctx, target, "angle_set", buildFrame(0x02, 0x41, scaleToByte(opts.Pan, 0), durationToUint16(opts.Duration)))
}

func (d *DefaultPTZDriver) TakePhoto(ctx context.Context, target PTZTarget, opts PhotoOptions) (*CommandResult, error) {
	return d.send(ctx, target, "photo", buildFrame(0x02, 0x50, 1, 0))
}

func (d *DefaultPTZDriver) Stop(ctx context.Context, target PTZTarget) (*CommandResult, error) {
	return d.send(ctx, target, "stop", buildFrame(0x02, 0x00, 0, 0))
}

func (d *DefaultPTZDriver) Realtime(ctx context.Context, target PTZTarget) (*RealtimeData, error) {
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
		},
	}, nil
}

func (d *DefaultPTZDriver) sendMove(ctx context.Context, target PTZTarget, command string, commandCode byte, opts PTZMoveOptions) (*CommandResult, error) {
	return d.send(ctx, target, command, buildFrame(0x02, commandCode, scaleToByte(opts.Speed, 50), durationToUint16(opts.Duration)))
}

func (d *DefaultPTZDriver) send(ctx context.Context, target PTZTarget, command string, payload []byte) (*CommandResult, error) {
	if target.Protocol != "" && target.Protocol != "udp" {
		return nil, fmt.Errorf("云台协议暂不支持: %s", target.Protocol)
	}
	if target.UDPHost == "" || target.UDPPort == 0 {
		return nil, fmt.Errorf("云台UDP地址未配置")
	}
	n, elapsed, err := SendUDP(ctx, target.Addr(), payload)
	if err != nil {
		return nil, err
	}
	return &CommandResult{
		DeviceType:  "ptz",
		Driver:      d.Name,
		TargetID:    target.ID,
		TargetAddr:  target.Addr(),
		Command:     command,
		PayloadHex:  payloadHex(payload),
		SentBytes:   n,
		SentAt:      time.Now(),
		Elapsed:     elapsed,
		Placeholder: true,
	}, nil
}
