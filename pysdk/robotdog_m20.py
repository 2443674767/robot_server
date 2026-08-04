#!/usr/bin/env python3
"""Small UDP test tool for the robot dog body-control protocol."""

from __future__ import annotations

import argparse
import json
import socket
import time
from datetime import datetime
from typing import Any


DEFAULT_HOST = "10.21.31.103"
DEFAULT_PORT = 30000
DEFAULT_LOCAL_PORT = 30002
SEND_HZ = 20
STOP_FRAMES = 5
SYNC = b"\xeb\x91\xeb\x90"

MOTION_ACTIONS = {
    "left": {"y": 1.0, "desc": "向左横移"},
    "right": {"y": -1.0, "desc": "向右横移"},
    "forward": {"x": 1.0, "desc": "前进"},
    "backward": {"x": -1.0, "desc": "后退"},
    "turn-left": {"yaw": 1.0, "desc": "原地左转/逆时针"},
    "turn-right": {"yaw": -1.0, "desc": "原地右转/顺时针"},
}

MOTION_STATE = {
    0: "空闲",
    1: "站立",
    2: "关节阻尼/软急停",
    3: "开机阻尼",
    4: "趴下",
    17: "RL控制",
}
GAIT = {
    0x1001: "基础步态(标准运动模式)",
    0x1002: "高台步态(标准运动模式)",
    0x1003: "楼梯步态(标准运动模式)",
    0x3002: "平地步态(敏捷运动模式)",
    0x3003: "楼梯步态(敏捷运动模式)",
}
CHARGE = {
    0: "空闲",
    1: "前往充电桩过程中",
    2: "充电中",
    3: "退出充电桩过程中",
    4: "机器人异常",
    5: "机器人在桩上但未充电",
}
HES = {0: "未触发", 1: "已触发"}
MODE = {0: "常规模式", 1: "导航模式", 2: "辅助模式"}
DIRECTION = {0: "正向为前进正方向", 1: "后向为前进正方向"}
OOA = {0: "未启动", 1: "空闲中", 2: "未触发避障", 3: "主动避障中"}
POWER_MANAGEMENT = {0: "常规电源模式", 1: "单电池模式"}
SLEEP = {0: "未休眠", 1: "已休眠", 2: "进入休眠中"}
VERSION = {"STD": "山猫M20", "PRO": "山猫M20 Pro"}
ERROR_CODE = {
    0x0000: "成功",
    0xE001: "数据格式不支持",
    0xE002: "数据解析失败",
    0xE003: "不支持的协议",
    0xE004: "缺少必要字段",
    0xE005: "字段类型不匹配",
    0xE006: "请求客户端不匹配，轴指令要求短时间内来自同一个客户端",
    0xE007: "无操作权限",
    0xE008: "不允许的操作，通常是当前模式/运动状态不满足",
    0xE009: "操作失败",
    0xE00A: "不支持的功能",
}


class RobotUdpClient:
    def __init__(self, host: str, port: int, local_port: int | None = DEFAULT_LOCAL_PORT) -> None:
        self.address = (host, port)
        self.message_id = 0
        self.sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        if local_port:
            self.sock.bind(("", local_port))

    def close(self) -> None:
        self.sock.close()

    def send_payload(self, msg_type: int, command: int, items: dict[str, Any] | None = None) -> None:
        payload = {
            "PatrolDevice": {
                "Type": msg_type,
                "Command": command,
                "Time": datetime.now().strftime("%Y-%m-%d %H:%M:%S"),
                "Items": items or {},
            }
        }
        self.sock.sendto(self._pack(payload), self.address)

    def send_heartbeat(self) -> None:
        self.send_payload(100, 100)

    def send_mode(self, mode: int) -> None:
        self.send_payload(1101, 5, {"Mode": mode})

    def send_motion_state(self, state: int) -> None:
        self.send_payload(2, 22, {"MotionParam": state})

    def send_gait(self, gait: int) -> None:
        self.send_payload(2, 23, {"GaitParam": gait})

    def send_light(self, front: int, back: int) -> None:
        self.send_payload(1101, 2, {"Front": front, "Back": back})

    def send_motion(self, *, x: float = 0.0, y: float = 0.0, yaw: float = 0.0) -> None:
        self.send_payload(
            2,
            21,
            {
                "X": float(x),
                "Y": float(y),
                "Z": 0.0,
                "Roll": 0.0,
                "Pitch": 0.0,
                "Yaw": float(yaw),
            },
        )

    def receive_payload(self, timeout: float = 0.2) -> dict[str, Any] | None:
        self.sock.settimeout(timeout)
        try:
            data, _ = self.sock.recvfrom(65535)
        except socket.timeout:
            return None

        if len(data) < 16 or data[:4] != SYNC:
            return None
        asdu_len = int.from_bytes(data[4:6], "little")
        asdu = data[16 : 16 + asdu_len]
        try:
            return json.loads(asdu.decode("utf-8"))
        except (UnicodeDecodeError, json.JSONDecodeError):
            return None

    def _pack(self, payload: dict[str, Any]) -> bytes:
        asdu = json.dumps(payload, separators=(",", ":"), ensure_ascii=False).encode("utf-8")
        header = bytearray()
        header.extend(SYNC)
        header.extend(len(asdu).to_bytes(2, "little"))
        header.extend(self.message_id.to_bytes(2, "little"))
        header.append(0x01)
        header.extend(b"\x00" * 7)
        self.message_id = (self.message_id + 1) & 0xFFFF
        return bytes(header) + asdu


def clamp_speed(speed: float) -> float:
    return max(0.0, min(1.0, speed))


def send_motion_for_duration(client: RobotUdpClient, action: str, speed: float, duration: float) -> None:
    factors = MOTION_ACTIONS[action]
    interval = 1.0 / SEND_HZ
    deadline = time.monotonic() + duration

    while time.monotonic() < deadline:
        client.send_motion(
            x=float(factors.get("x", 0.0)) * speed,
            y=float(factors.get("y", 0.0)) * speed,
            yaw=float(factors.get("yaw", 0.0)) * speed,
        )
        time.sleep(interval)

    send_stop(client)


def send_stop(client: RobotUdpClient) -> None:
    interval = 1.0 / SEND_HZ
    for _ in range(STOP_FRAMES):
        client.send_motion()
        time.sleep(interval)


def label(value: Any, mapping: dict[Any, str]) -> str:
    return f"{value}({mapping.get(value, '未知')})"


def line(name: str, value: Any, desc: str = "", unit: str = "") -> None:
    suffix = f"{unit}" if value is not None and unit else ""
    note = f" - {desc}" if desc else ""
    print(f"  {name}: {value}{suffix}{note}")


def print_raw_json(payload: dict[str, Any]) -> None:
    print(json.dumps(payload, ensure_ascii=False, indent=2))


def print_status(payload: dict[str, Any], raw: bool = False) -> None:
    patrol = payload.get("PatrolDevice", {})
    msg_type = patrol.get("Type")
    command = patrol.get("Command")
    msg_time = patrol.get("Time")
    items = patrol.get("Items", {})

    if msg_type == 1002 and command == 6:
        basic = items.get("BasicStatus", {})
        print(f"\n[{msg_time}] 基础状态 Type=1002 Command=6")
        line("MotionState 运动状态", label(basic.get("MotionState"), MOTION_STATE))
        line("Gait 当前步态", label(basic.get("Gait"), GAIT))
        line("Charge 充电流程状态", label(basic.get("Charge"), CHARGE))
        line("HES 硬急停状态", label(basic.get("HES"), HES))
        line("ControlUsageMode 使用模式", label(basic.get("ControlUsageMode"), MODE))
        line("Direction 前进正方向", label(basic.get("Direction"), DIRECTION))
        line("OOA 辅助模式避障", label(basic.get("OOA"), OOA))
        line("PowerManagement 电源管理", label(basic.get("PowerManagement"), POWER_MANAGEMENT))
        line("Sleep 休眠状态", label(basic.get("Sleep"), SLEEP))
        version = basic.get("Version")
        line("Version 设备版本", f"{version}({VERSION.get(version, '未知')})")
    elif msg_type == 1002 and command == 5:
        battery = items.get("BatteryStatus", {})
        battery_list = items.get("BatteryList", [])
        print(f"\n[{msg_time}] 设备/电池状态 Type=1002 Command=5")
        print("  BatteryStatus 左右电池: Right 表示靠近硬急停按钮一侧")
        line("BatteryLevelLeft 左侧电量", battery.get("BatteryLevelLeft"), unit="%")
        line("BatteryLevelRight 右侧电量", battery.get("BatteryLevelRight"), unit="%")
        line("VoltageLeft 左侧电压", battery.get("VoltageLeft"), unit="V")
        line("VoltageRight 右侧电压", battery.get("VoltageRight"), unit="V")
        line("battery_temperatureLeft 左侧温度", battery.get("battery_temperatureLeft"), unit="C")
        line("battery_temperatureRight 右侧温度", battery.get("battery_temperatureRight"), unit="C")
        line("chargeLeft 左侧是否充电", battery.get("chargeLeft"), desc="true=充电中, false=未充电")
        line("chargeRight 右侧是否充电", battery.get("chargeRight"), desc="true=充电中, false=未充电")
        for index, item in enumerate(battery_list):
            position = "后电池" if index == 0 else "前电池" if index == 1 else "未知位置电池"
            print(f"  BatteryList[{index}] {position}:")
            line("  BatteryLevel 剩余电量", item.get("BatteryLevel"), unit="%")
            line("  Voltage 电压", item.get("Voltage"), unit="V")
            line("  battery_temperature 温度", item.get("battery_temperature"), unit="C")
            line("  charge 是否充电", item.get("charge"), desc="true=充电中, false=未充电")
            line("  serial 序列号", item.get("serial"))

        led = items.get("Led", {}).get("Fill", {})
        if led:
            print("  Led 照明灯:")
            line("  Front 前灯", led.get("Front"), desc="1=开, 0=关")
            line("  Back 后灯", led.get("Back"), desc="1=开, 0=关")

        gps = items.get("GPS", {})
        if gps:
            print("  GPS:")
            line("  Latitude 纬度", gps.get("Latitude"))
            line("  Longitude 经度", gps.get("Longitude"))
            line("  Speed 地面速度", gps.get("Speed"))
            line("  NumSatellites 卫星数量", gps.get("NumSatellites"))
    elif msg_type == 1002 and command == 4:
        motion = items.get("MotionStatus", {})
        print(f"\n[{msg_time}] 运控状态 Type=1002 Command=4")
        line("Roll 横滚角", motion.get("Roll"), unit="rad")
        line("Pitch 俯仰角", motion.get("Pitch"), unit="rad")
        line("Yaw 偏航角", motion.get("Yaw"), unit="rad")
        line("OmegaZ Z方向角速度", motion.get("OmegaZ"), unit="rad/s")
        line("LinearX 当前X方向线速度", motion.get("LinearX"), unit="m/s")
        line("LinearY 当前Y方向线速度", motion.get("LinearY"), unit="m/s")
        line("Height 当前机身高度", motion.get("Height"), unit="m")
        line("RemainMile 预计剩余续航", motion.get("RemainMile"), unit="km")
    elif "ErrorCode" in items:
        code = items.get("ErrorCode")
        message = items.get("ErrorMessage")
        try:
            code_int = int(code)
        except (TypeError, ValueError):
            code_int = None
        desc = ERROR_CODE.get(code_int, "未知错误码")
        print(f"\n[{msg_time}] 指令响应 Type={msg_type} Command={command}")
        line("ErrorCode 错误码", f"0x{code_int:04X}" if code_int is not None else code)
        line("ErrorMessage 原始消息", message)
        line("含义", desc)
    else:
        print(f"\n[{msg_time}] 收到未专门解析消息 Type={msg_type} Command={command}")
        print_raw_json(payload)

    if raw and msg_type is not None:
        print("  原始JSON:")
        print_raw_json(payload)


def listen_status(client: RobotUdpClient, duration: float, raw: bool = False) -> None:
    next_heartbeat = 0.0
    deadline = time.monotonic() + duration
    while time.monotonic() < deadline:
        now = time.monotonic()
        if now >= next_heartbeat:
            client.send_heartbeat()
            next_heartbeat = now + 1.0

        payload = client.receive_payload(timeout=0.2)
        if payload:
            print_status(payload, raw=raw)


def read_responses(client: RobotUdpClient, duration: float = 0.5) -> None:
    deadline = time.monotonic() + duration
    while time.monotonic() < deadline:
        payload = client.receive_payload(timeout=0.1)
        if payload:
            print_status(payload)


def prepare_for_axis_motion(client: RobotUdpClient) -> None:
    print("准备轴指令运动: 切常规模式 -> 站立 -> RL控制 -> 基础步态")
    client.send_mode(0)
    read_responses(client, 0.3)
    time.sleep(0.5)
    client.send_motion_state(1)
    read_responses(client, 0.3)
    time.sleep(1.0)
    client.send_motion_state(17)
    read_responses(client, 0.3)
    time.sleep(0.5)
    client.send_gait(0x1001)
    read_responses(client, 0.3)


def parse_args() -> argparse.Namespace:
    actions = sorted(
        list(MOTION_ACTIONS)
        + [
            "stop",
            "status",
            "heartbeat",
            "mode-normal",
            "mode-nav",
            "mode-assist",
            "stand",
            "rl",
            "damp",
            "lie",
            "gait-basic",
            "gait-stair",
            "gait-flat-fast",
            "light-on",
            "light-off",
        ]
    )
    parser = argparse.ArgumentParser(description="Robot dog UDP control/status test tool.")
    parser.add_argument("action", choices=actions, help="test action to send")
    parser.add_argument("--speed", type=float, default=0.5, help="motion speed ratio in [0, 1], default: 0.5")
    parser.add_argument("--duration", type=float, default=1.0, help="motion/status seconds, default: 1.0")
    parser.add_argument("--host", default=DEFAULT_HOST, help=f"robot UDP host, default: {DEFAULT_HOST}")
    parser.add_argument("--port", type=int, default=DEFAULT_PORT, help=f"robot UDP port, default: {DEFAULT_PORT}")
    parser.add_argument(
        "--local-port",
        type=int,
        default=DEFAULT_LOCAL_PORT,
        help=f"fixed local UDP source port, default: {DEFAULT_LOCAL_PORT}; use 0 for random",
    )
    parser.add_argument("--prepare", action="store_true", help="before motion, send mode-normal + stand + rl + gait-basic")
    parser.add_argument("--raw", action="store_true", help="also print raw JSON when action=status")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    local_port = args.local_port if args.local_port > 0 else None
    client = RobotUdpClient(args.host, args.port, local_port=local_port)
    try:
        if args.action in MOTION_ACTIONS:
            if args.prepare:
                prepare_for_axis_motion(client)
            send_motion_for_duration(client, args.action, clamp_speed(args.speed), max(0.1, args.duration))
            read_responses(client, 0.5)
        elif args.action == "stop":
            send_stop(client)
            read_responses(client, 0.5)
        elif args.action == "status":
            listen_status(client, max(1.0, args.duration), raw=args.raw)
        elif args.action == "heartbeat":
            client.send_heartbeat()
            read_responses(client, 0.5)
        elif args.action == "mode-normal":
            client.send_mode(0)
            read_responses(client, 0.5)
        elif args.action == "mode-nav":
            client.send_mode(1)
            read_responses(client, 0.5)
        elif args.action == "mode-assist":
            client.send_mode(2)
            read_responses(client, 0.5)
        elif args.action == "stand":
            client.send_motion_state(1)
            read_responses(client, 0.5)
        elif args.action == "rl":
            client.send_motion_state(17)
            read_responses(client, 0.5)
        elif args.action == "damp":
            client.send_motion_state(2)
            read_responses(client, 0.5)
        elif args.action == "lie":
            client.send_motion_state(4)
            read_responses(client, 0.5)
        elif args.action == "gait-basic":
            client.send_gait(0x1001)
            read_responses(client, 0.5)
        elif args.action == "gait-stair":
            client.send_gait(0x1003)
            read_responses(client, 0.5)
        elif args.action == "gait-flat-fast":
            client.send_gait(0x3002)
            read_responses(client, 0.5)
        elif args.action == "light-on":
            client.send_light(1, 1)
            read_responses(client, 0.5)
        elif args.action == "light-off":
            client.send_light(0, 0)
            read_responses(client, 0.5)
    finally:
        client.close()


if __name__ == "__main__":
    main()
