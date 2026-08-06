package control

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type DogTarget struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Model   string `json:"model"`
	UDPHost string `json:"udp_host"`
	UDPPort int32  `json:"udp_port"`
}

type PTZTarget struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Brand    string `json:"brand"`
	Model    string `json:"model"`
	Protocol string `json:"protocol"`
	UDPHost  string `json:"udp_host"`
	UDPPort  int32  `json:"udp_port"`
}

type MoveOptions struct {
	Direction string  `json:"direction"`
	Speed     float64 `json:"speed"`
	Duration  int     `json:"duration"`
}

type PTZMoveOptions struct {
	Direction string  `json:"direction"`
	Speed     float64 `json:"speed"`
	Duration  int     `json:"duration"`
	Pan       float64 `json:"pan"`
	Tilt      float64 `json:"tilt"`
}

type ZoomOptions struct {
	Direction string  `json:"direction"`
	Step      float64 `json:"step"`
	Duration  int     `json:"duration"`
}

type FocusOptions struct {
	Direction string  `json:"direction"`
	Step      float64 `json:"step"`
	Duration  int     `json:"duration"`
}

type RealtimeData struct {
	DeviceType string                 `json:"device_type"`
	TargetID   int64                  `json:"target_id"`
	Driver     string                 `json:"driver"`
	At         time.Time              `json:"at"`
	Battery    *int                   `json:"battery,omitempty"`
	NavStatus  string                 `json:"nav_status,omitempty"`
	Data       map[string]interface{} `json:"data"`
}

type CommandResult struct {
	DeviceType  string        `json:"device_type"`
	Driver      string        `json:"driver"`
	TargetID    int64         `json:"target_id"`
	TargetAddr  string        `json:"target_addr"`
	Command     string        `json:"command"`
	PayloadHex  string        `json:"payload_hex"`
	SentBytes   int           `json:"sent_bytes"`
	SentAt      time.Time     `json:"sent_at"`
	Elapsed     time.Duration `json:"elapsed"`
	Placeholder bool          `json:"placeholder"`
	Script      string        `json:"script,omitempty"`
	Output      string        `json:"output,omitempty"`
}

type DogController interface {
	MoveLeft(ctx context.Context, target DogTarget, opts MoveOptions) (*CommandResult, error)
	MoveRight(ctx context.Context, target DogTarget, opts MoveOptions) (*CommandResult, error)
	MoveForward(ctx context.Context, target DogTarget, opts MoveOptions) (*CommandResult, error)
	MoveBackward(ctx context.Context, target DogTarget, opts MoveOptions) (*CommandResult, error)
	Stop(ctx context.Context, target DogTarget) (*CommandResult, error)
	SetGait(ctx context.Context, target DogTarget, gait string) (*CommandResult, error)
	Charge(ctx context.Context, target DogTarget, action string) (*CommandResult, error)
	Realtime(ctx context.Context, target DogTarget) (*RealtimeData, error)
}

type PTZController interface {
	Up(ctx context.Context, target PTZTarget, opts PTZMoveOptions) (*CommandResult, error)
	Down(ctx context.Context, target PTZTarget, opts PTZMoveOptions) (*CommandResult, error)
	Left(ctx context.Context, target PTZTarget, opts PTZMoveOptions) (*CommandResult, error)
	Right(ctx context.Context, target PTZTarget, opts PTZMoveOptions) (*CommandResult, error)
	ZoomIn(ctx context.Context, target PTZTarget, opts ZoomOptions) (*CommandResult, error)
	ZoomOut(ctx context.Context, target PTZTarget, opts ZoomOptions) (*CommandResult, error)
	FocusNear(ctx context.Context, target PTZTarget, opts FocusOptions) (*CommandResult, error)
	FocusFar(ctx context.Context, target PTZTarget, opts FocusOptions) (*CommandResult, error)
	Stop(ctx context.Context, target PTZTarget) (*CommandResult, error)
	Realtime(ctx context.Context, target PTZTarget) (*RealtimeData, error)
}

func (t DogTarget) Addr() string {
	return fmt.Sprintf("%s:%d", strings.TrimSpace(t.UDPHost), t.UDPPort)
}

func (t PTZTarget) Addr() string {
	return fmt.Sprintf("%s:%d", strings.TrimSpace(t.UDPHost), t.UDPPort)
}
