package control

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"strings"
	"sync/atomic"
	"time"
)

const (
	libra4MsgGimbalStatus = 0x000002
	libra4MsgCameraStatus = 0x000005
	libra4MsgGimbalMove   = 0x000010
	libra4MsgAngleSet     = 0x000012
	libra4MsgWheelSpeed   = 0x000017
	libra4MsgZoomSet      = 0x000304
	libra4MsgZoomControl  = 0x000306
	libra4MsgFocus        = 0x000313
	libra4MsgPhoto        = 0x000302
)

type Libra4PTZDriver struct {
	Name string
	seq  uint32
}

func NewLibra4PTZDriver() *Libra4PTZDriver {
	return &Libra4PTZDriver{Name: "libra4_udp"}
}

func (d *Libra4PTZDriver) Up(ctx context.Context, target PTZTarget, opts PTZMoveOptions) (*CommandResult, error) {
	return d.move(ctx, target, "up", opts.Speed, 2, 0)
}

func (d *Libra4PTZDriver) Down(ctx context.Context, target PTZTarget, opts PTZMoveOptions) (*CommandResult, error) {
	return d.move(ctx, target, "down", opts.Speed, 2, 1)
}

func (d *Libra4PTZDriver) Left(ctx context.Context, target PTZTarget, opts PTZMoveOptions) (*CommandResult, error) {
	return d.move(ctx, target, "left", opts.Speed, 0, 2)
}

func (d *Libra4PTZDriver) Right(ctx context.Context, target PTZTarget, opts PTZMoveOptions) (*CommandResult, error) {
	return d.move(ctx, target, "right", opts.Speed, 1, 2)
}

func (d *Libra4PTZDriver) ZoomIn(ctx context.Context, target PTZTarget, opts ZoomOptions) (*CommandResult, error) {
	return d.sendCommand(ctx, target, "zoom_in", libra4MsgZoomControl, []byte{0x03, 0x00})
}

func (d *Libra4PTZDriver) ZoomOut(ctx context.Context, target PTZTarget, opts ZoomOptions) (*CommandResult, error) {
	return d.sendCommand(ctx, target, "zoom_out", libra4MsgZoomControl, []byte{0x04, 0x00})
}

func (d *Libra4PTZDriver) FocusNear(ctx context.Context, target PTZTarget, opts FocusOptions) (*CommandResult, error) {
	return d.sendCommand(ctx, target, "focus_near", libra4MsgFocus, libra4FocusPayload(0x01))
}

func (d *Libra4PTZDriver) FocusFar(ctx context.Context, target PTZTarget, opts FocusOptions) (*CommandResult, error) {
	return d.sendCommand(ctx, target, "focus_far", libra4MsgFocus, libra4FocusPayload(0x02))
}

func (d *Libra4PTZDriver) Home(ctx context.Context, target PTZTarget) (*CommandResult, error) {
	return d.SetAngle(ctx, target, PTZAngleOptions{Pan: 0, Tilt: 0, Roll: 0})
}

func (d *Libra4PTZDriver) SetAngle(ctx context.Context, target PTZTarget, opts PTZAngleOptions) (*CommandResult, error) {
	payload := make([]byte, 7)
	pitchDir, pitch := libra4AngleDirection(opts.Tilt, 0, 1)
	yawDir, yaw := libra4AngleDirection(opts.Pan, 1, 0)
	payload[0] = pitchDir
	binary.LittleEndian.PutUint16(payload[1:3], uint16(math.Round(pitch*100)))
	payload[3] = yawDir
	binary.LittleEndian.PutUint16(payload[4:6], uint16(math.Round(yaw*100)))
	return d.sendCommand(ctx, target, "angle_set", libra4MsgAngleSet, payload)
}

func (d *Libra4PTZDriver) TakePhoto(ctx context.Context, target PTZTarget, opts PhotoOptions) (*CommandResult, error) {
	payload := make([]byte, 54)
	payload[0] = libra4PhotoMode(opts.Mode)
	payload[1] = 0x00
	copyFixedASCII(payload[2:22], opts.Folder)
	copyFixedASCII(payload[22:54], opts.Filename)
	return d.sendCommand(ctx, target, "photo", libra4MsgPhoto, payload)
}

func (d *Libra4PTZDriver) Stop(ctx context.Context, target PTZTarget) (*CommandResult, error) {
	return d.sendCommand(ctx, target, "stop", libra4MsgGimbalMove, []byte{0x00, 0x02, 0x02, 0x00})
}

func (d *Libra4PTZDriver) Realtime(ctx context.Context, target PTZTarget) (*RealtimeData, error) {
	target = FillPTZTargetDefaults(target)
	data := map[string]interface{}{
		"brand":      target.Brand,
		"model":      target.Model,
		"protocol":   target.Protocol,
		"udp_host":   target.UDPHost,
		"udp_port":   target.UDPPort,
		"local_port": target.LocalPort,
	}
	state, err := d.listenRealtime(ctx, target, 1200*time.Millisecond)
	if err == nil {
		for k, v := range state {
			data[k] = v
		}
	}
	rt := &RealtimeData{
		DeviceType: "ptz",
		TargetID:   target.ID,
		Driver:     d.Name,
		At:         time.Now(),
		Data:       data,
	}
	if v, ok := state["pitch"].(float64); ok {
		rt.Pitch = &v
	}
	if v, ok := state["yaw"].(float64); ok {
		rt.Yaw = &v
	}
	if v, ok := state["roll"].(float64); ok {
		rt.Roll = &v
	}
	if v, ok := state["zoom"].(float64); ok {
		rt.Zoom = &v
	}
	if v, ok := state["focus_status"].(string); ok {
		rt.Focus = v
	}
	return rt, nil
}

func (d *Libra4PTZDriver) move(ctx context.Context, target PTZTarget, command string, speed float64, yawDir byte, pitchDir byte) (*CommandResult, error) {
	var totalBytes int
	var lastPayload []byte
	var elapsed time.Duration
	if speed > 0 {
		payload := []byte{libra4ProtocolSpeed(speed), libra4ProtocolSpeed(speed), 0x00}
		result, err := d.sendCommand(ctx, target, "speed", libra4MsgWheelSpeed, payload)
		if err != nil {
			return nil, err
		}
		totalBytes += result.SentBytes
		elapsed += result.Elapsed
		lastPayload = payload
	}
	result, err := d.sendCommand(ctx, target, command, libra4MsgGimbalMove, []byte{0x00, yawDir, pitchDir, 0x00})
	if err != nil {
		return nil, err
	}
	result.SentBytes += totalBytes
	result.Elapsed += elapsed
	if len(lastPayload) > 0 {
		result.Output = "speed command sent before move"
	}
	return result, nil
}

func (d *Libra4PTZDriver) sendCommand(ctx context.Context, target PTZTarget, command string, msgID uint32, payload []byte) (*CommandResult, error) {
	target = FillPTZTargetDefaults(target)
	if target.UDPHost == "" || target.UDPPort == 0 {
		return nil, fmt.Errorf("云台UDP地址未配置")
	}
	if msgID >= 0x000100 {
		target.TargetSystemID = 0x04
	}
	frame := d.frame(target, msgID, payload)
	n, elapsed, err := SendUDP(ctx, target.Addr(), frame)
	if err != nil {
		return nil, err
	}
	return &CommandResult{
		DeviceType:  "ptz",
		Driver:      d.Name,
		TargetID:    target.ID,
		TargetAddr:  target.Addr(),
		Command:     command,
		PayloadHex:  payloadHex(frame),
		SentBytes:   n,
		SentAt:      time.Now(),
		Elapsed:     elapsed,
		Placeholder: false,
	}, nil
}

func (d *Libra4PTZDriver) frame(target PTZTarget, msgID uint32, payload []byte) []byte {
	header := []byte{
		0xFD,
		byte(len(payload)),
		target.TargetSystemID,
		target.TargetComponentID,
		byte(atomic.AddUint32(&d.seq, 1)),
		target.SourceSystemID,
		target.SourceComponentID,
		byte(msgID),
		byte(msgID >> 8),
		byte(msgID >> 16),
	}
	frame := append(append([]byte{}, header...), payload...)
	crc := x25CRC(frame[1:])
	frame = append(frame, byte(crc), byte(crc>>8))
	return frame
}

func (d *Libra4PTZDriver) listenRealtime(ctx context.Context, target PTZTarget, timeout time.Duration) (map[string]interface{}, error) {
	if target.LocalPort <= 0 {
		return map[string]interface{}{}, nil
	}
	addr := &net.UDPAddr{IP: net.IPv4zero, Port: int(target.LocalPort)}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	buf := make([]byte, 2048)
	state := map[string]interface{}{}
	for {
		select {
		case <-ctx.Done():
			return state, ctx.Err()
		default:
		}
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			if len(state) > 0 {
				return state, nil
			}
			return state, err
		}
		parseLibra4Frame(state, buf[:n])
		if _, ok := state["pitch"]; ok {
			if _, ok := state["zoom"]; ok {
				return state, nil
			}
		}
	}
}

func parseLibra4Frame(state map[string]interface{}, frame []byte) {
	if len(frame) < 12 || frame[0] != 0xFD {
		return
	}
	payloadLen := int(frame[1])
	if len(frame) < 12+payloadLen {
		return
	}
	msgID := uint32(frame[7]) | uint32(frame[8])<<8 | uint32(frame[9])<<16
	payload := frame[10 : 10+payloadLen]
	switch msgID & 0x00FFFF {
	case libra4MsgGimbalStatus:
		if len(payload) < 18 {
			return
		}
		state["yaw"] = float64(int16(binary.LittleEndian.Uint16(payload[0:2]))) / 100
		state["roll"] = float64(int16(binary.LittleEndian.Uint16(payload[2:4]))) / 100
		state["pitch"] = float64(int16(binary.LittleEndian.Uint16(payload[4:6]))) / 100
	case libra4MsgCameraStatus:
		if len(payload) < 12 {
			return
		}
		state["zoom"] = float64(binary.LittleEndian.Uint16(payload[3:5])) / 10
		if payload[11] == 0x01 {
			state["focus_status"] = "focusing"
		} else {
			state["focus_status"] = "done"
		}
	}
}

func libra4AngleDirection(v float64, positiveDir byte, negativeDir byte) (byte, float64) {
	if v > 0 {
		return positiveDir, math.Abs(v)
	}
	if v < 0 {
		return negativeDir, math.Abs(v)
	}
	return positiveDir, 0
}

func libra4ProtocolSpeed(speed float64) byte {
	if speed <= 0 {
		return 50
	}
	if speed <= 30 {
		speed = speed * 5
	}
	if speed < 5 {
		speed = 5
	}
	if speed > 150 {
		speed = 150
	}
	return byte(math.Round(speed))
}

func libra4FocusPayload(kind byte) []byte {
	payload := make([]byte, 10)
	payload[0] = kind
	return payload
}

func libra4PhotoMode(mode string) byte {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case "ir", "infrared":
		return 0x01
	case "visible", "rgb":
		return 0x02
	case "both":
		return 0x03
	default:
		return 0x00
	}
}

func copyFixedASCII(dst []byte, value string) {
	value = strings.TrimSpace(value)
	for i := 0; i < len(dst) && i < len(value); i++ {
		if value[i] < 0x80 {
			dst[i] = value[i]
		}
	}
}

func x25CRC(buf []byte) uint16 {
	crc := uint16(0xffff)
	for _, b := range buf {
		tmp := b ^ byte(crc&0xff)
		tmp ^= tmp << 4
		crc = (crc >> 8) ^ (uint16(tmp) << 8) ^ (uint16(tmp) << 3) ^ (uint16(tmp) >> 4)
	}
	return crc
}
