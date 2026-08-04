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
