package control

import (
	"context"
	"fmt"
	"math"
	"time"
)

type DefaultDogDriver struct {
	Name string
}

func NewDefaultDogDriver() *DefaultDogDriver {
	return &DefaultDogDriver{Name: "default"}
}

func (d *DefaultDogDriver) MoveLeft(ctx context.Context, target DogTarget, opts MoveOptions) (*CommandResult, error) {
	return d.sendMove(ctx, target, "left", 0x03, opts)
}

func (d *DefaultDogDriver) MoveRight(ctx context.Context, target DogTarget, opts MoveOptions) (*CommandResult, error) {
	return d.sendMove(ctx, target, "right", 0x04, opts)
}

func (d *DefaultDogDriver) MoveForward(ctx context.Context, target DogTarget, opts MoveOptions) (*CommandResult, error) {
	return d.sendMove(ctx, target, "forward", 0x01, opts)
}

func (d *DefaultDogDriver) MoveBackward(ctx context.Context, target DogTarget, opts MoveOptions) (*CommandResult, error) {
	return d.sendMove(ctx, target, "backward", 0x02, opts)
}

func (d *DefaultDogDriver) Stop(ctx context.Context, target DogTarget) (*CommandResult, error) {
	return d.send(ctx, target, "stop", buildFrame(0x01, 0x00, 0, 0))
}

func (d *DefaultDogDriver) SetGait(ctx context.Context, target DogTarget, gait string) (*CommandResult, error) {
	switch gait {
	case "basic":
		return d.send(ctx, target, "gait-basic", buildFrame(0x01, 0x23, 0x01, 0x1001))
	case "stair":
		return d.send(ctx, target, "gait-stair", buildFrame(0x01, 0x23, 0x03, 0x1003))
	default:
		return nil, fmt.Errorf("不支持的步态类型")
	}
}

func (d *DefaultDogDriver) Charge(ctx context.Context, target DogTarget, action string) (*CommandResult, error) {
	switch action {
	case "enter":
		return d.send(ctx, target, "charge-enter", buildFrame(0x01, 0x24, 1, 0))
	case "exit":
		return d.send(ctx, target, "charge-exit", buildFrame(0x01, 0x24, 0, 0))
	case "clear":
		return d.send(ctx, target, "charge-clear", buildFrame(0x01, 0x24, 2, 0))
	default:
		return nil, fmt.Errorf("不支持的充电桩动作")
	}
}

func (d *DefaultDogDriver) Realtime(ctx context.Context, target DogTarget) (*RealtimeData, error) {
	return &RealtimeData{
		DeviceType: "dog",
		TargetID:   target.ID,
		Driver:     d.Name,
		At:         time.Now(),
		Data: map[string]interface{}{
			"model":    target.Model,
			"udp_host": target.UDPHost,
			"udp_port": target.UDPPort,
		},
	}, nil
}

func (d *DefaultDogDriver) sendMove(ctx context.Context, target DogTarget, command string, commandCode byte, opts MoveOptions) (*CommandResult, error) {
	return d.send(ctx, target, command, buildFrame(0x01, commandCode, scaleToByte(opts.Speed, 100), durationToUint16(opts.Duration)))
}

func (d *DefaultDogDriver) send(ctx context.Context, target DogTarget, command string, payload []byte) (*CommandResult, error) {
	if target.UDPHost == "" || target.UDPPort == 0 {
		return nil, fmt.Errorf("机械狗UDP地址未配置")
	}
	n, elapsed, err := SendUDP(ctx, target.Addr(), payload)
	if err != nil {
		return nil, err
	}
	return &CommandResult{
		DeviceType:  "dog",
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

func scaleToByte(v float64, fallback byte) byte {
	if v <= 0 {
		return fallback
	}
	if v > 255 {
		return 255
	}
	return byte(math.Round(v))
}

func durationToUint16(v int) uint16 {
	if v <= 0 {
		return 0
	}
	if v > 65535 {
		return 65535
	}
	return uint16(v)
}
