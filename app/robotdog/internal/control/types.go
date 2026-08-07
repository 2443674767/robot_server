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
	ID                int64  `json:"id"`
	Name              string `json:"name"`
	DeviceUID         string `json:"device_uid"`
	Username          string `json:"username"`
	Password          string `json:"password"`
	Brand             string `json:"brand"`
	Model             string `json:"model"`
	Protocol          string `json:"protocol"`
	UDPHost           string `json:"udp_host"`
	UDPPort           int32  `json:"udp_port"`
	LocalPort         int32  `json:"local_port"`
	TargetSystemID    byte   `json:"target_system_id"`
	TargetComponentID byte   `json:"target_component_id"`
	SourceSystemID    byte   `json:"source_system_id"`
	SourceComponentID byte   `json:"source_component_id"`
	RTSPURL           string `json:"rtsp_url"`
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

type PTZAngleOptions struct {
	Pan      float64 `json:"pan"`
	Tilt     float64 `json:"tilt"`
	Roll     float64 `json:"roll"`
	Duration int     `json:"duration"`
}

type PhotoOptions struct {
	Mode     string `json:"mode"`
	Folder   string `json:"folder"`
	Filename string `json:"filename"`
}

type RealtimeData struct {
	DeviceType string                 `json:"device_type"`
	TargetID   int64                  `json:"target_id"`
	Driver     string                 `json:"driver"`
	At         time.Time              `json:"at"`
	Battery    *int                   `json:"battery,omitempty"`
	NavStatus  string                 `json:"nav_status,omitempty"`
	Pitch      *float64               `json:"pitch,omitempty"`
	Yaw        *float64               `json:"yaw,omitempty"`
	Roll       *float64               `json:"roll,omitempty"`
	Zoom       *float64               `json:"zoom,omitempty"`
	Focus      string                 `json:"focus_status,omitempty"`
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
	Home(ctx context.Context, target PTZTarget) (*CommandResult, error)
	SetAngle(ctx context.Context, target PTZTarget, opts PTZAngleOptions) (*CommandResult, error)
	TakePhoto(ctx context.Context, target PTZTarget, opts PhotoOptions) (*CommandResult, error)
	Stop(ctx context.Context, target PTZTarget) (*CommandResult, error)
	Realtime(ctx context.Context, target PTZTarget) (*RealtimeData, error)
}

type PTZNudgeController interface {
	Nudge(ctx context.Context, target PTZTarget, axis string, delta float64, zoomMax float64) (*CommandResult, error)
}

type PTZZoomHomeController interface {
	ZoomHome(ctx context.Context, target PTZTarget) (*CommandResult, error)
}

type PTZRefreshController interface {
	Refresh(ctx context.Context, target PTZTarget) (*CommandResult, error)
}

func (t DogTarget) Addr() string {
	return fmt.Sprintf("%s:%d", strings.TrimSpace(t.UDPHost), t.UDPPort)
}

func (t PTZTarget) Addr() string {
	return fmt.Sprintf("%s:%d", strings.TrimSpace(t.UDPHost), t.UDPPort)
}
