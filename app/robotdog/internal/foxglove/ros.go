package foxglove

import (
	"encoding/binary"
	"fmt"
	"math"
)

type Header struct {
	Seq       uint32 `json:"seq"`
	StampSec  uint32 `json:"stamp_sec"`
	StampNSec uint32 `json:"stamp_nsec"`
	FrameID   string `json:"frame_id"`
}

type Vector3 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

type Quaternion struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
	W float64 `json:"w"`
}

type Pose struct {
	Position    Vector3    `json:"position"`
	Orientation Quaternion `json:"orientation"`
}

type Twist struct {
	Linear  Vector3 `json:"linear"`
	Angular Vector3 `json:"angular"`
}

type NavCustomPayload struct {
	PositionX    float64 `json:"positionX"`
	PositionY    float64 `json:"positionY"`
	PositionZ    float64 `json:"positionZ"`
	OrientationX float64 `json:"orientationX"`
	OrientationY float64 `json:"orientationY"`
	OrientationZ float64 `json:"orientationZ"`
	OrientationW float64 `json:"orientationW"`
}

type Odometry struct {
	Header           Header           `json:"header"`
	ChildFrameID     string           `json:"child_frame_id"`
	Pose             Pose             `json:"pose"`
	Twist            Twist            `json:"twist"`
	NavCustomPayload NavCustomPayload `json:"navCustomPayload"`
}

func DecodeOdometry(payload []byte) (*Odometry, error) {
	r := rosReader{data: payload}
	header, err := r.header()
	if err != nil {
		return nil, err
	}
	childFrameID, err := r.string()
	if err != nil {
		return nil, err
	}
	position, err := r.vector3()
	if err != nil {
		return nil, err
	}
	orientation, err := r.quaternion()
	if err != nil {
		return nil, err
	}
	if err := r.skip(36 * 8); err != nil {
		return nil, err
	}
	linear, err := r.vector3()
	if err != nil {
		return nil, err
	}
	angular, err := r.vector3()
	if err != nil {
		return nil, err
	}
	return &Odometry{
		Header:       header,
		ChildFrameID: childFrameID,
		Pose: Pose{
			Position:    position,
			Orientation: orientation,
		},
		Twist: Twist{
			Linear:  linear,
			Angular: angular,
		},
		NavCustomPayload: NavCustomPayload{
			PositionX:    position.X,
			PositionY:    position.Y,
			PositionZ:    position.Z,
			OrientationX: orientation.X,
			OrientationY: orientation.Y,
			OrientationZ: orientation.Z,
			OrientationW: orientation.W,
		},
	}, nil
}

type rosReader struct {
	data []byte
	off  int
}

func (r *rosReader) header() (Header, error) {
	seq, err := r.uint32()
	if err != nil {
		return Header{}, err
	}
	sec, err := r.uint32()
	if err != nil {
		return Header{}, err
	}
	nsec, err := r.uint32()
	if err != nil {
		return Header{}, err
	}
	frameID, err := r.string()
	if err != nil {
		return Header{}, err
	}
	return Header{Seq: seq, StampSec: sec, StampNSec: nsec, FrameID: frameID}, nil
}

func (r *rosReader) vector3() (Vector3, error) {
	x, err := r.float64()
	if err != nil {
		return Vector3{}, err
	}
	y, err := r.float64()
	if err != nil {
		return Vector3{}, err
	}
	z, err := r.float64()
	if err != nil {
		return Vector3{}, err
	}
	return Vector3{X: x, Y: y, Z: z}, nil
}

func (r *rosReader) quaternion() (Quaternion, error) {
	x, err := r.float64()
	if err != nil {
		return Quaternion{}, err
	}
	y, err := r.float64()
	if err != nil {
		return Quaternion{}, err
	}
	z, err := r.float64()
	if err != nil {
		return Quaternion{}, err
	}
	w, err := r.float64()
	if err != nil {
		return Quaternion{}, err
	}
	return Quaternion{X: x, Y: y, Z: z, W: w}, nil
}

func (r *rosReader) string() (string, error) {
	n, err := r.uint32()
	if err != nil {
		return "", err
	}
	if err := r.need(int(n)); err != nil {
		return "", err
	}
	value := string(r.data[r.off : r.off+int(n)])
	r.off += int(n)
	return value, nil
}

func (r *rosReader) uint32() (uint32, error) {
	if err := r.need(4); err != nil {
		return 0, err
	}
	value := binary.LittleEndian.Uint32(r.data[r.off : r.off+4])
	r.off += 4
	return value, nil
}

func (r *rosReader) float64() (float64, error) {
	if err := r.need(8); err != nil {
		return 0, err
	}
	value := math.Float64frombits(binary.LittleEndian.Uint64(r.data[r.off : r.off+8]))
	r.off += 8
	return value, nil
}

func (r *rosReader) skip(n int) error {
	if err := r.need(n); err != nil {
		return err
	}
	r.off += n
	return nil
}

func (r *rosReader) need(n int) error {
	if n < 0 || r.off+n > len(r.data) {
		return fmt.Errorf("ROS payload长度不足: offset=%d need=%d total=%d", r.off, n, len(r.data))
	}
	return nil
}
