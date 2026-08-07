#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import socket
import struct
import sys
import time
from dataclasses import dataclass, asdict

DEFAULT_IP = "10.21.31.64"
DEFAULT_UDP_PORT = 1030
STX = 0xFD

SYS_ID_APP = 0x01
COMP_ID_APP = 0x01
SYS_ID_POD = 0x04
COMP_ID_POD = 0x01

MSG_PTZ_CONTROL = 0x000010
MSG_PTZ_ANGLE = 0x000012
MSG_ZOOM_SPECIFIC = 0x000304
MSG_PTZ_ATTITUDE = 0x020002
MSG_CAMERA_STATUS = 0x020005
ACK_MSG_PREFIX = 0x010000
ACK_OK = 0

YAW_MIN = -150.0
YAW_MAX = 150.0
PITCH_OFFSET_MIN = -90.0
PITCH_OFFSET_MAX = 50.0
ZOOM_MIN = 1.0
ZOOM_MAX_DEFAULT = 30.0
COMMAND_ACCUMULATE_WINDOW = 0.5


def clamp(value: float, min_value: float, max_value: float) -> float:
    return max(min_value, min(max_value, value))


def normalize_yaw_feedback(yaw: float) -> float:
    yaw = float(yaw)
    if yaw < -180.0:
        yaw += 360.0
    elif yaw > 180.0:
        yaw -= 360.0
    return yaw


def normalize_pitch_feedback(pitch: float) -> float:
    pitch = float(pitch)
    if abs(abs(pitch) - 180.0) < 1.0:
        return 180.0
    return pitch


def pitch_abs_to_offset(abs_pitch: float) -> float:
    pitch = normalize_pitch_feedback(abs_pitch)
    if pitch > 0 and abs(pitch - 180.0) < 1.0:
        return 0.0
    if pitch < 0 and abs(pitch + 180.0) < 1.0:
        return 0.0
    if 90.0 <= pitch <= 180.0:
        return pitch - 180.0
    if -180.0 <= pitch <= -130.0:
        return pitch + 180.0
    return clamp(pitch, PITCH_OFFSET_MIN, PITCH_OFFSET_MAX)


def pitch_offset_to_abs(offset: float) -> float:
    value = clamp(offset, PITCH_OFFSET_MIN, PITCH_OFFSET_MAX)
    if abs(value) < 0.05:
        return 180.0
    if value < 0:
        return 180.0 + value
    return -180.0 + value


def normalize_pitch_target(pitch: float) -> float:
    pitch_value = float(pitch)
    if 90.0 <= pitch_value <= 180.0 or -180.0 <= pitch_value <= -130.0:
        return pitch_value
    return pitch_offset_to_abs(pitch_value)


def encode_pitch_target_angle(pitch: float) -> tuple[int, int]:
    pitch_value = normalize_pitch_target(pitch)
    if 90.0 <= pitch_value <= 180.0:
        return 0, int(round((180.0 - pitch_value) * 100.0))
    if -180.0 <= pitch_value <= -130.0:
        return 1, int(round((180.0 + pitch_value) * 100.0))
    raise ValueError(f"pitch out of protocol range: {pitch_value:.3f}")


def encode_yaw_target_angle(yaw: float) -> tuple[int, int]:
    yaw_value = clamp(float(yaw), YAW_MIN, YAW_MAX)
    return (0 if yaw_value <= 0 else 1), int(round(abs(yaw_value) * 100.0))


def crc_accumulate(data: int, crc: int) -> int:
    tmp = data ^ (crc & 0xFF)
    tmp ^= (tmp << 4) & 0xFF
    crc = (crc >> 8) ^ (tmp << 8) ^ (tmp << 3) ^ (tmp >> 4)
    return crc & 0xFFFF


def crc_calculate(data: bytes) -> int:
    crc = 0xFFFF
    for b in data:
        crc = crc_accumulate(b, crc)
    return crc


def build_frame(seq: int, msg_id: int, payload: bytes) -> bytes:
    header = bytes(
        [
            STX,
            len(payload),
            SYS_ID_POD,
            COMP_ID_POD,
            seq & 0xFF,
            SYS_ID_APP,
            COMP_ID_APP,
        ]
    )
    body = header + struct.pack("<I", msg_id)[:3] + payload
    return body + struct.pack("<H", crc_calculate(body[1:]))


def parse_frame(data: bytes):
    if len(data) < 12 or data[0] != STX:
        return None
    payload_len = data[1]
    expected = 10 + payload_len + 2
    if len(data) < expected:
        return None
    body = data[: 10 + payload_len]
    given_crc = struct.unpack("<H", data[10 + payload_len : expected])[0]
    if crc_calculate(body[1:]) != given_crc:
        return None
    msg_id = struct.unpack("<I", data[7:10] + b"\x00")[0]
    payload = data[10 : 10 + payload_len]
    return msg_id, payload


def build_stop_payload() -> bytes:
    return bytes([0x00, 0x02, 0x02, 0x00])


def build_home_payload() -> bytes:
    return bytes([0x10, 0x02, 0x02, 0x00])


def build_goto_angle_payload(pitch_deg: float, yaw_deg: float) -> bytes:
    pitch_dir, pitch_value = encode_pitch_target_angle(pitch_deg)
    yaw_dir, yaw_value = encode_yaw_target_angle(yaw_deg)
    return (
        bytes([pitch_dir])
        + struct.pack("<H", pitch_value)
        + bytes([yaw_dir])
        + struct.pack("<H", yaw_value)
        + b"\x00"
    )


def build_zoom_payload(ratio: float) -> bytes:
    ratio_int = int(round(float(ratio) * 10.0)) & 0xFFFF
    return bytes([0x00]) + struct.pack("<H", ratio_int)


@dataclass
class Status:
    yaw: float = 0.0
    pitch: float = 180.0
    pitch_offset: float = 0.0
    roll: float = 0.0
    attitude_yaw: float = 0.0
    attitude_roll: float = 0.0
    attitude_pitch: float = 0.0
    yaw_speed: float = 0.0
    pitch_speed: float = 0.0
    roll_speed: float = 0.0
    zoom: float = 0.0
    updated_at: float = 0.0
    attitude_at: float = 0.0
    camera_at: float = 0.0
    connected: bool = False
    last_error: str = ""
    last_command: str = ""


class Libra4Client:
    def __init__(self, host: str, port: int, local_port: int, timeout: float):
        self.addr = (host, port)
        self.timeout = timeout
        self.seq = 0
        self.status = Status()
        self.command_accumulator: dict[str, dict[str, float]] = {}
        self.sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        self.sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        self.sock.bind(("", local_port))
        self.sock.settimeout(0.2)

    def close(self) -> None:
        self.sock.close()

    def next_seq(self) -> int:
        seq = self.seq & 0xFF
        self.seq = (self.seq + 1) & 0xFF
        return seq

    def send_message(self, msg_id: int, payload: bytes, label: str = "") -> int:
        packet = build_frame(self.next_seq(), msg_id, payload)
        self.status.last_command = label
        self.status.last_error = ""
        return self.sock.sendto(packet, self.addr)

    def recv_for(self, seconds: float) -> None:
        deadline = time.time() + seconds
        while time.time() < deadline:
            remaining = max(0.02, min(0.2, deadline - time.time()))
            self.sock.settimeout(remaining)
            try:
                data, _ = self.sock.recvfrom(4096)
            except socket.timeout:
                continue
            frame = parse_frame(data)
            if frame is None:
                continue
            try:
                self.process_frame(*frame)
            except Exception as exc:
                self.status.last_error = str(exc)

    def process_frame(self, msg_id: int, payload: bytes) -> None:
        now = time.time()
        if msg_id == MSG_PTZ_ATTITUDE and len(payload) >= 18:
            values = struct.unpack("<hhhhhhhhh", payload[:18])
            self.status.yaw = normalize_yaw_feedback(values[0] / 100.0)
            self.status.roll = values[1] / 100.0
            self.status.pitch = normalize_pitch_feedback(values[2] / 100.0)
            self.status.pitch_offset = pitch_abs_to_offset(self.status.pitch)
            self.status.attitude_yaw = normalize_yaw_feedback(values[3] / 100.0)
            self.status.attitude_roll = values[4] / 100.0
            self.status.attitude_pitch = values[5] / 100.0
            self.status.yaw_speed = values[6] / 100.0
            self.status.pitch_speed = values[7] / 100.0
            self.status.roll_speed = values[8] / 100.0
            self.status.updated_at = now
            self.status.attitude_at = now
            self.status.connected = True
        elif msg_id == MSG_CAMERA_STATUS and len(payload) >= 14:
            self.status.zoom = struct.unpack("<h", payload[3:5])[0] / 10.0
            self.status.updated_at = now
            self.status.camera_at = now
            self.status.connected = True
        elif (msg_id & 0xFF0000) == ACK_MSG_PREFIX and len(payload) >= 2:
            code = struct.unpack("<H", payload[:2])[0]
            if code != ACK_OK:
                self.status.last_error = f"ACK 0x{msg_id:06X} code=0x{code:04X}"

    def snapshot(self) -> dict:
        return status_snapshot(self.status)

    def accumulator_target(self, axis: str, fallback: float, now: float) -> float:
        acc = self.command_accumulator.get(axis)
        if acc and now - acc["time"] <= COMMAND_ACCUMULATE_WINDOW:
            return acc["target"]
        return fallback

    def remember_target(self, axis: str, target: float, now: float) -> None:
        self.command_accumulator[axis] = {"target": target, "time": now}

    def refresh(self, require: str) -> None:
        before = self.status.updated_at
        self.send_message(MSG_PTZ_CONTROL, build_stop_payload(), "refresh")
        deadline = time.time() + self.timeout
        while time.time() < deadline:
            self.recv_for(min(0.25, max(0.02, deadline - time.time())))
            if self.status.updated_at and self.status.updated_at != before:
                break
        if require == "attitude" and not self.status.attitude_at:
            raise RuntimeError("未收到实时位姿，已取消发送")
        if require == "camera" and not self.status.camera_at:
            raise RuntimeError("未收到实时倍数，已取消发送")
        if require == "any" and not self.status.updated_at:
            raise RuntimeError("未收到实时位姿/倍数，已取消发送")

    def nudge(self, axis: str, delta: float, zoom_max: float) -> dict:
        now = time.time()
        active_axis = self.command_accumulator.get(axis)
        can_accumulate = bool(active_axis and now - active_axis["time"] <= COMMAND_ACCUMULATE_WINDOW)
        if axis in ("yaw", "pitch"):
            if not can_accumulate:
                self.refresh("attitude")
                now = time.time()
            yaw_base = self.accumulator_target("yaw", self.status.yaw, now)
            pitch_base = self.accumulator_target("pitch", self.status.pitch_offset, now)
            if axis == "yaw":
                target_yaw = clamp(yaw_base + delta, YAW_MIN, YAW_MAX)
                target_pitch_offset = pitch_base
            else:
                target_yaw = clamp(yaw_base, YAW_MIN, YAW_MAX)
                target_pitch_offset = clamp(pitch_base + delta, PITCH_OFFSET_MIN, PITCH_OFFSET_MAX)
            target_pitch = pitch_offset_to_abs(target_pitch_offset)
            sent = self.send_message(
                MSG_PTZ_ANGLE,
                build_goto_angle_payload(target_pitch, target_yaw),
                f"yaw {target_yaw:.2f}, pitch {target_pitch:.2f}",
            )
            self.remember_target(axis, target_yaw if axis == "yaw" else target_pitch_offset, now)
            self.recv_for(0.25)
            return {
                "axis": axis,
                "delta": delta,
                "target_yaw": target_yaw,
                "target_pitch": target_pitch,
                "target_pitch_offset": target_pitch_offset,
                "sent_bytes": sent,
            }
        if axis == "zoom":
            if not can_accumulate:
                self.refresh("camera")
                now = time.time()
            current_zoom = self.status.zoom if self.status.zoom > 0 else ZOOM_MIN
            zoom_base = self.accumulator_target("zoom", current_zoom, now)
            target_zoom = clamp(zoom_base + delta, ZOOM_MIN, max(ZOOM_MIN, zoom_max))
            sent = self.send_message(MSG_ZOOM_SPECIFIC, build_zoom_payload(target_zoom), f"zoom {target_zoom:.1f}x")
            self.remember_target("zoom", target_zoom, now)
            self.recv_for(0.25)
            return {"axis": axis, "delta": delta, "target_zoom": target_zoom, "sent_bytes": sent}
        raise ValueError(f"unsupported axis: {axis}")

    def angle_set(self, yaw: float, pitch: float) -> dict:
        sent = self.send_message(MSG_PTZ_ANGLE, build_goto_angle_payload(pitch, yaw), f"yaw {yaw:.2f}, pitch {pitch:.2f}")
        self.recv_for(0.25)
        return {"target_yaw": clamp(yaw, YAW_MIN, YAW_MAX), "target_pitch": normalize_pitch_target(pitch), "sent_bytes": sent}

    def home(self) -> dict:
        self.command_accumulator.clear()
        sent = self.send_message(MSG_PTZ_CONTROL, build_home_payload(), "home")
        self.recv_for(0.25)
        return {"sent_bytes": sent}

    def zoom_home(self) -> dict:
        self.command_accumulator.pop("zoom", None)
        sent = self.send_message(MSG_ZOOM_SPECIFIC, build_zoom_payload(1.0), "zoom 1.0x")
        self.recv_for(0.25)
        return {"target_zoom": 1.0, "sent_bytes": sent}

    def stop(self) -> dict:
        sent = self.send_message(MSG_PTZ_CONTROL, build_stop_payload(), "refresh")
        self.recv_for(0.25)
        return {"sent_bytes": sent}


def command_to_nudge(args) -> tuple[str, float]:
    mapping = {
        "left": ("yaw", -abs(args.step)),
        "right": ("yaw", abs(args.step)),
        "up": ("pitch", -abs(args.step)),
        "down": ("pitch", abs(args.step)),
        "left-fast": ("yaw", -abs(args.step) * 5),
        "right-fast": ("yaw", abs(args.step) * 5),
        "up-fast": ("pitch", -abs(args.step) * 5),
        "down-fast": ("pitch", abs(args.step) * 5),
        "zoom-in": ("zoom", abs(args.step)),
        "zoom-out": ("zoom", -abs(args.step)),
        "zoom-in-fast": ("zoom", abs(args.step) * 5),
        "zoom-out-fast": ("zoom", -abs(args.step) * 5),
    }
    if args.command in mapping:
        return mapping[args.command]
    return args.axis, args.delta


def status_snapshot(status: Status) -> dict:
    data = asdict(status)
    data["age"] = time.time() - status.updated_at if status.updated_at else None
    data["limits"] = {
        "yaw": [YAW_MIN, YAW_MAX],
        "pitchOffset": [PITCH_OFFSET_MIN, PITCH_OFFSET_MAX],
        "zoom": [ZOOM_MIN, ZOOM_MAX_DEFAULT],
    }
    return data


def emit(ok: bool, command: str, status: Status, **extra) -> None:
    payload = {"ok": ok, "command": command, "status": status_snapshot(status)}
    payload.update(extra)
    print(json.dumps(payload, ensure_ascii=False, separators=(",", ":")))


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="LIBRA4 UDP PTZ control without UI")
    parser.add_argument("--host", "--ip", dest="host", default=DEFAULT_IP)
    parser.add_argument("--port", "--udp-port", "--device-port", dest="port", type=int, default=DEFAULT_UDP_PORT)
    parser.add_argument("--local-port", type=int, default=0)
    parser.add_argument("--timeout", type=float, default=1.2)
    sub = parser.add_subparsers(dest="command", required=True)

    for name, default_step in (
        ("left", 5.0),
        ("right", 5.0),
        ("up", 2.0),
        ("down", 2.0),
        ("left-fast", 5.0),
        ("right-fast", 5.0),
        ("up-fast", 2.0),
        ("down-fast", 2.0),
        ("zoom-in", 0.5),
        ("zoom-out", 0.5),
        ("zoom-in-fast", 0.5),
        ("zoom-out-fast", 0.5),
    ):
        p = sub.add_parser(name)
        p.add_argument("--step", type=float, default=default_step)
        p.add_argument("--zoom-max", type=float, default=ZOOM_MAX_DEFAULT)

    p = sub.add_parser("nudge")
    p.add_argument("--axis", choices=("yaw", "pitch", "zoom"), required=True)
    p.add_argument("--delta", type=float, required=True)
    p.add_argument("--zoom-max", type=float, default=ZOOM_MAX_DEFAULT)

    p = sub.add_parser("angle-set")
    p.add_argument("--yaw", type=float, required=True)
    p.add_argument("--pitch", type=float, required=True)

    sub.add_parser("home")
    sub.add_parser("zoom-home")
    sub.add_parser("stop")
    p = sub.add_parser("status")
    p.add_argument("--require", choices=("any", "attitude", "camera"), default="any")
    sub.add_parser("refresh")
    return parser


def main(argv=None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)
    client = None
    try:
        client = Libra4Client(args.host, args.port, args.local_port, args.timeout)
        if args.command == "status":
            client.recv_for(args.timeout)
            if (
                (args.require == "attitude" and not client.status.attitude_at)
                or (args.require == "camera" and not client.status.camera_at)
                or (args.require == "any" and not client.status.updated_at)
            ):
                client.refresh(args.require)
            result = client.snapshot()
        elif args.command == "refresh":
            client.refresh("any")
            result = client.snapshot()
        elif args.command in (
            "left",
            "right",
            "up",
            "down",
            "left-fast",
            "right-fast",
            "up-fast",
            "down-fast",
            "zoom-in",
            "zoom-out",
            "zoom-in-fast",
            "zoom-out-fast",
            "nudge",
        ):
            axis, delta = command_to_nudge(args)
            result = client.nudge(axis, delta, args.zoom_max)
        elif args.command == "angle-set":
            result = client.angle_set(args.yaw, args.pitch)
        elif args.command == "home":
            result = client.home()
        elif args.command == "zoom-home":
            result = client.zoom_home()
        elif args.command == "stop":
            result = client.stop()
        else:
            raise RuntimeError(f"unsupported command: {args.command}")
        emit(True, args.command, client.status, result=result)
        return 0
    except Exception as exc:
        status = client.status if client is not None else Status()
        emit(False, getattr(args, "command", ""), status, error=str(exc))
        return 1
    finally:
        if client is not None:
            client.close()


if __name__ == "__main__":
    sys.exit(main())
