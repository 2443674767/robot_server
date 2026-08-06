#!/usr/bin/env python3
"""HY-DZ230F 云台/相机 MAVLink UDP 控制与状态读取 CLI。

通信方式见厂商文档（汇云 MavLink 接口）：
- 发送：HY_REQUEST / HY_GIMBAL_CONTROL / HY_CAMERA_CONFIG 等
- 接收：HY_GIMBAL_REPORT（角度）、HY_CAMERA_REPORT（倍率/拍照状态）等

注意：设备通常把状态包回发到本机 UDP 3000，因此 local_port 必须能绑定 3000，
否则 status 等读状态命令会失败。
"""

import argparse
import importlib
import socket
import subprocess
import threading
import time
from collections import Counter
from dataclasses import dataclass


# --- 默认网络参数 ---
DEFAULT_DEVICE_IP = "10.21.31.111"
DEFAULT_DEVICE_PORT = 3000
# 设备侧配置为向客户端 3000 端口推送 HY_GIMBAL_REPORT / HY_CAMERA_REPORT
DEFAULT_LOCAL_PORT = 3000

# --- HY_REQUEST.request 取值（见 hy_gimbal.py / 厂商 PDF）---
CONNECT_REQUEST = 1
CHANNEL_REQUEST = 4

# --- HY_GIMBAL_CONTROL 模式：1=速率(deg/s)，2=绝对角度(deg) ---
GIMBAL_RATE_CONTROL = 1
GIMBAL_ANGLE_CONTROL = 2

# --- HY_CAMERA_CONFIG.config_type 常用取值 ---
CAMERA_HEARTBEAT_FUNCTION = 0
CAMERA_ZOOM = 1
CAMERA_TAKE_PHOTO = 2
CAMERA_FOCUS_VALUE = 12

# --- 标准 MAVLink 命令/消息 ID（用于 request_message 主动拉取相机状态）---
MAV_CMD_REQUEST_MESSAGE = 512
MAV_CMD_REQUEST_VIDEO_STREAM_INFORMATION = 2504
MAV_CMD_REQUEST_VIDEO_STREAM_STATUS = 2505
MAVLINK_MSG_ID_CAMERA_SETTINGS = 260
MAVLINK_MSG_ID_CAMERA_CAPTURE_STATUS = 262
MAVLINK_MSG_ID_CAMERA_IMAGE_CAPTURED = 263
MAVLINK_MSG_ID_VIDEO_STREAM_INFORMATION = 269
MAVLINK_MSG_ID_VIDEO_STREAM_STATUS = 270
MAVLINK_MSG_ID_CAMERA_FOV_STATUS = 271
MAVLINK_MSG_ID_HY_CAMERA_REPORT = 11061
MAVLINK_MSG_ID_HY_CAMERA_CONFIG = 11060

FOCUS_VALUES = {
    "far": 1,
    "near": 2,
    "auto": 3,
}


def get_zoom_times_from_status_value(status_value):
    # HY_CAMERA_REPORT.status_value 低 7 位为光学倍率整数部分
    return int(status_value) & 0x7F


# --- 从 MAVLink 报文解析出的结构化状态（便于 CLI 打印与业务脚本复用）---
@dataclass
class GimbalStatus:
    pitch_deg: float
    yaw_deg: float
    laser_distance: float
    temperature: int
    src_system: int
    src_component: int
    raw: object


@dataclass
class CameraStatus:
    zoom_times: int
    taking_photo: bool
    recording: bool
    digital_zoom: bool
    raw_status_value: int
    src_system: int
    src_component: int
    raw: object


@dataclass
class CameraAck:
    target_msg_id: int
    stage: int
    result: int
    ext_val: int
    message: str
    raw: object


@dataclass
class StandardCameraSettings:
    zoom_level: float
    focus_level: float
    mode_id: int
    camera_device_id: int
    raw: object


@dataclass
class CameraCaptureStatus:
    image_status: int
    video_status: int
    image_count: int
    raw: object


@dataclass
class CameraImageCaptured:
    image_index: int
    capture_result: int
    file_url: str
    raw: object


@dataclass
class VideoStreamInformation:
    stream_id: int
    count: int
    flags: int
    framerate: float
    resolution_h: int
    resolution_v: int
    rotation: int
    hfov: float
    name: str
    uri: str
    camera_device_id: int
    src_system: int
    src_component: int
    raw: object


@dataclass
class VideoStreamStatus:
    stream_id: int
    flags: int
    framerate: float
    resolution_h: int
    resolution_v: int
    rotation: int
    hfov: float
    camera_device_id: int
    src_system: int
    src_component: int
    raw: object


@dataclass
class CameraFovStatus:
    hfov: float
    vfov: float
    camera_device_id: int
    src_system: int
    src_component: int
    raw: object


class UdpSender:
    """把 pymavlink 期望的 write() 接口适配到 UDP sendto。"""

    def __init__(self, sock, target):
        self.sock = sock
        self.target = target

    def write(self, data):
        return self.sock.sendto(data, self.target)


class HyGimbalClient:
    """HY-DZ230F UDP/MAVLink 客户端：后台线程收包，主线程发控制指令。"""

    def __init__(self, ip, device_port, local_port, src_system=1, src_component=1):
        self.hy_gimbal = self._load_hy_gimbal()
        self.target = (ip, device_port)
        self.sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        self.sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        self.sock.bind(("0.0.0.0", local_port))
        self.sock.settimeout(0.2)
        # local_port=0 时系统分配临时端口；requested_local_port 保留用户原始意图
        self.local_port = self.sock.getsockname()[1]
        self.requested_local_port = local_port

        self.mav = self.hy_gimbal.MAVLink(
            UdpSender(self.sock, self.target),
            srcSystem=src_system,
            srcComponent=src_component,
        )
        self.last_gimbal = None
        self.last_camera = None
        self.last_camera_settings = None
        self.last_camera_ack = None
        self.last_capture_status = None
        self.last_image_captured = None
        self.last_camera_fov = None
        self.video_stream_information = {}
        self.video_stream_statuses = {}
        self.debug_messages = False
        self.verbose_reports = False
        self.seen_sources = set()
        self.message_counts = Counter()
        self._running = False
        self._thread = None

    @staticmethod
    def _load_hy_gimbal():
        try:
            return importlib.import_module("hy_gimbal")
        except ModuleNotFoundError:
            raise SystemExit(
                "Missing generated SDK file: hy_gimbal.py\n"
                "Put the vendor-generated hy_gimbal.py in this directory, or add its "
                "folder to PYTHONPATH, then run again.\n\n"
                "The PDF says Python control depends on: import hy_gimbal."
            )

    def start(self):
        self._running = True
        self._thread = threading.Thread(target=self._recv_loop, daemon=True)
        self._thread.start()

    def close(self):
        self._running = False
        if self._thread:
            self._thread.join(timeout=1)
        self.sock.close()

    def _recv_loop(self):
        """后台循环解析 MAVLink 报文，更新 last_gimbal / last_camera 等缓存。"""
        while self._running:
            try:
                data, _addr = self.sock.recvfrom(4096)
            except socket.timeout:
                continue
            except OSError:
                return

            try:
                msgs = self.mav.parse_buffer(data) or []
            except Exception as exc:
                print(f"Parse error: {exc}")
                continue

            for msg in msgs:
                msg_type = msg.get_type()
                src_system = msg.get_srcSystem()
                src_component = msg.get_srcComponent()
                self.seen_sources.add((src_system, src_component))
                self.message_counts[msg_type] += 1
                if self.debug_messages:
                    print(f"RX {msg_type} from sys={src_system} comp={src_component}")
                if msg_type == "HY_GIMBAL_REPORT":
                    # pitch_angle / yaw_angle 单位为 0.01 度（centi-degrees）
                    self.last_gimbal = GimbalStatus(
                        pitch_deg=msg.pitch_angle / 100.0,
                        yaw_deg=msg.yaw_angle / 100.0,
                        laser_distance=msg.laser_distance / 10.0,
                        temperature=msg.temperature,
                        src_system=src_system,
                        src_component=src_component,
                        raw=msg,
                    )
                    if self.verbose_reports:
                        print(
                            "GIMBAL "
                            f"pitch={self.last_gimbal.pitch_deg:.2f} "
                            f"yaw={self.last_gimbal.yaw_deg:.2f} "
                            f"distance={self.last_gimbal.laser_distance:.1f} "
                            f"temp={self.last_gimbal.temperature}"
                        )
                elif msg_type == "HY_CAMERA_REPORT":
                    val = int(msg.status_value)
                    # status_value 位域：bit0-6 倍率，bit7 拍照中，bit8 录像中，bit28 电子变倍
                    self.last_camera = CameraStatus(
                        zoom_times=get_zoom_times_from_status_value(val),
                        taking_photo=bool((val >> 7) & 1),
                        recording=bool((val >> 8) & 1),
                        digital_zoom=bool((val >> 28) & 1),
                        raw_status_value=val,
                        src_system=src_system,
                        src_component=src_component,
                        raw=msg,
                    )
                    if self.verbose_reports:
                        print(
                            "CAMERA "
                            f"zoom={self.last_camera.zoom_times}x "
                            f"taking_photo={self.last_camera.taking_photo} "
                            f"recording={self.last_camera.recording} "
                            f"digital_zoom={self.last_camera.digital_zoom}"
                        )
                elif msg_type == "HY_CAMERA_GENERAL_ACK":
                    self.last_camera_ack = CameraAck(
                        target_msg_id=msg.target_msg_id,
                        stage=msg.stage,
                        result=msg.result,
                        ext_val=msg.ext_val,
                        message=msg.message,
                        raw=msg,
                    )
                    print(
                        "CAMERA_ACK "
                        f"target_msg_id={msg.target_msg_id} "
                        f"stage={msg.stage} "
                        f"result={msg.result} "
                        f"ext_val={msg.ext_val} "
                        f"message={msg.message}"
                    )
                elif msg_type == "CAMERA_SETTINGS":
                    self.last_camera_settings = StandardCameraSettings(
                        zoom_level=msg.zoomLevel,
                        focus_level=msg.focusLevel,
                        mode_id=msg.mode_id,
                        camera_device_id=msg.camera_device_id,
                        raw=msg,
                    )
                    if self.verbose_reports:
                        print(
                            "CAMERA_SETTINGS "
                            f"zoomLevel={msg.zoomLevel} "
                            f"focusLevel={msg.focusLevel} "
                            f"mode_id={msg.mode_id} "
                            f"camera_device_id={msg.camera_device_id}"
                        )
                elif msg_type == "CAMERA_CAPTURE_STATUS":
                    self.last_capture_status = CameraCaptureStatus(
                        image_status=msg.image_status,
                        video_status=msg.video_status,
                        image_count=msg.image_count,
                        raw=msg,
                    )
                    print(
                        "CAMERA_CAPTURE_STATUS "
                        f"image_status={msg.image_status} "
                        f"video_status={msg.video_status} "
                        f"image_count={msg.image_count}"
                    )
                elif msg_type == "CAMERA_IMAGE_CAPTURED":
                    self.last_image_captured = CameraImageCaptured(
                        image_index=msg.image_index,
                        capture_result=msg.capture_result,
                        file_url=msg.file_url,
                        raw=msg,
                    )
                    print(
                        "CAMERA_IMAGE_CAPTURED "
                        f"image_index={msg.image_index} "
                        f"capture_result={msg.capture_result} "
                        f"file_url={msg.file_url}"
                    )
                elif msg_type == "VIDEO_STREAM_INFORMATION":
                    info = VideoStreamInformation(
                        stream_id=msg.stream_id,
                        count=msg.count,
                        flags=msg.flags,
                        framerate=msg.framerate,
                        resolution_h=msg.resolution_h,
                        resolution_v=msg.resolution_v,
                        rotation=msg.rotation,
                        hfov=float(msg.hfov),
                        name=msg.name,
                        uri=msg.uri,
                        camera_device_id=msg.camera_device_id,
                        src_system=src_system,
                        src_component=src_component,
                        raw=msg,
                    )
                    self.video_stream_information[
                        (src_system, src_component, msg.stream_id, msg.camera_device_id)
                    ] = info
                    if self.verbose_reports:
                        print(
                            "VIDEO_STREAM_INFORMATION "
                            f"stream_id={info.stream_id} "
                            f"resolution={info.resolution_h}x{info.resolution_v} "
                            f"hfov={info.hfov:.2f} "
                            f"name={info.name}"
                        )
                elif msg_type == "VIDEO_STREAM_STATUS":
                    status = VideoStreamStatus(
                        stream_id=msg.stream_id,
                        flags=msg.flags,
                        framerate=msg.framerate,
                        resolution_h=msg.resolution_h,
                        resolution_v=msg.resolution_v,
                        rotation=msg.rotation,
                        hfov=float(msg.hfov),
                        camera_device_id=msg.camera_device_id,
                        src_system=src_system,
                        src_component=src_component,
                        raw=msg,
                    )
                    self.video_stream_statuses[
                        (src_system, src_component, msg.stream_id, msg.camera_device_id)
                    ] = status
                    if self.verbose_reports:
                        print(
                            "VIDEO_STREAM_STATUS "
                            f"stream_id={status.stream_id} "
                            f"resolution={status.resolution_h}x{status.resolution_v} "
                            f"hfov={status.hfov:.2f}"
                        )
                elif msg_type == "CAMERA_FOV_STATUS":
                    self.last_camera_fov = CameraFovStatus(
                        hfov=float(msg.hfov),
                        vfov=float(msg.vfov),
                        camera_device_id=msg.camera_device_id,
                        src_system=src_system,
                        src_component=src_component,
                        raw=msg,
                    )
                    if self.verbose_reports:
                        print(
                            "CAMERA_FOV_STATUS "
                            f"hfov={self.last_camera_fov.hfov:.2f} "
                            f"vfov={self.last_camera_fov.vfov:.2f} "
                            f"camera_device_id={self.last_camera_fov.camera_device_id}"
                        )
                elif msg_type == "COMMAND_ACK":
                    print(
                        "COMMAND_ACK "
                        f"command={msg.command} "
                        f"result={msg.result} "
                        f"progress={msg.progress} "
                        f"result_param2={msg.result_param2}"
                    )

    def connect_until_report(self, seconds=5):
        """首次连接时周期性发送 CONNECT/CHANNEL 请求，直到收到任意状态包。"""
        deadline = time.time() + seconds
        while time.time() < deadline:
            self.mav.hy_request_send(request=CONNECT_REQUEST)
            self.mav.hy_request_send(request=CHANNEL_REQUEST)
            if self.last_gimbal or self.last_camera:
                return True
            time.sleep(0.2)
        return bool(self.last_gimbal or self.last_camera)

    def request_camera_status_until_report(self, seconds=5):
        deadline = time.time() + seconds
        while time.time() < deadline:
            self.mav.hy_request_send(request=CONNECT_REQUEST)
            self.mav.hy_request_send(request=CHANNEL_REQUEST)
            self.mav.hy_camera_config_send(
                config_type=CAMERA_HEARTBEAT_FUNCTION,
                cmd_value=1,
            )
            if self.last_camera:
                return True
            time.sleep(0.2)
        return bool(self.last_camera)

    def request_message(
        self,
        message_id,
        target_system=0,
        target_component=0,
        param2=0,
        param3=0,
        param4=0,
        param5=0,
        param6=0,
        param7=0,
    ):
        self.mav.command_long_send(
            target_system,
            target_component,
            MAV_CMD_REQUEST_MESSAGE,
            0,
            float(message_id),
            float(param2),
            float(param3),
            float(param4),
            float(param5),
            float(param6),
            float(param7),
        )

    def build_zoom_request_targets(self, target_system=0, target_component=0):
        """构造 MAV_CMD_REQUEST_MESSAGE 的目标列表。

        设备 sys/comp ID 因固件而异，未指定时向广播及已观测到的源多种组合尝试。
        """
        targets = []
        if target_system or target_component:
            targets.append((target_system, target_component))
        else:
            targets.append((0, 0))
            for sys_id, comp_id in sorted(self.seen_sources):
                targets.append((sys_id, comp_id))
                targets.append((sys_id, 0))
                targets.append((sys_id, 1))
                targets.append((sys_id, 100))
            targets.extend([(1, 1), (1, 100)])

        deduped = []
        seen = set()
        for target in targets:
            if target not in seen:
                seen.add(target)
                deduped.append(target)
        return deduped

    def test_current_zoom(
        self,
        seconds=10,
        target_system=0,
        target_component=0,
        passive_seconds=3,
        heartbeat_values=None,
    ):
        if heartbeat_values is None:
            heartbeat_values = [1]
        self.last_camera = None
        self.last_camera_settings = None
        self.last_camera_ack = None

        passive_deadline = time.time() + passive_seconds
        while time.time() < passive_deadline:
            if self.last_camera or self.last_camera_settings:
                break
            time.sleep(0.1)

        deadline = time.time() + seconds
        while time.time() < deadline:
            self.mav.hy_request_send(request=CONNECT_REQUEST)
            self.mav.hy_request_send(request=CHANNEL_REQUEST)

            # Vendor custom path: request/trigger camera status report.
            for heartbeat_value in heartbeat_values:
                if self.debug_messages:
                    print(f"TX CAMERA_HEARTBEAT_FUNCTION cmd_value={heartbeat_value}")
                self.mav.hy_camera_config_send(
                    config_type=CAMERA_HEARTBEAT_FUNCTION,
                    cmd_value=int(heartbeat_value),
                )
            for sys_id, comp_id in self.build_zoom_request_targets(
                target_system=target_system,
                target_component=target_component,
            ):
                if self.debug_messages:
                    print(f"TX request HY_CAMERA_REPORT to sys={sys_id} comp={comp_id}")
                self.request_message(
                    MAVLINK_MSG_ID_HY_CAMERA_REPORT,
                    target_system=sys_id,
                    target_component=comp_id,
                )

                # Standard MAVLink camera path: CAMERA_SETTINGS.zoomLevel.
                if self.debug_messages:
                    print(f"TX request CAMERA_SETTINGS to sys={sys_id} comp={comp_id}")
                self.request_message(
                    MAVLINK_MSG_ID_CAMERA_SETTINGS,
                    target_system=sys_id,
                    target_component=comp_id,
                )

            if self.last_camera or self.last_camera_settings:
                break
            time.sleep(0.5)

        print("Zoom status test result:")
        if self.seen_sources:
            sources = ", ".join(f"sys={s} comp={c}" for s, c in sorted(self.seen_sources))
            print(f"  Seen message sources: {sources}")
        if self.message_counts:
            counts = ", ".join(f"{name}={count}" for name, count in self.message_counts.most_common())
            print(f"  Message counts: {counts}")
        if self.last_camera:
            print(f"  HY_CAMERA_REPORT raw status_value: {self.last_camera.raw_status_value}")
            print("  zoom_times bits 0-6: status_value & 0x7F")
            print(f"  HY_CAMERA_REPORT zoom_times: {self.last_camera.zoom_times}x")
        else:
            print("  HY_CAMERA_REPORT: not received")

        if self.last_camera_settings:
            print(f"  CAMERA_SETTINGS zoomLevel: {self.last_camera_settings.zoom_level}")
            print(f"  CAMERA_SETTINGS focusLevel: {self.last_camera_settings.focus_level}")
            print(f"  CAMERA_SETTINGS mode_id: {self.last_camera_settings.mode_id}")
            print(f"  CAMERA_SETTINGS camera_device_id: {self.last_camera_settings.camera_device_id}")
        else:
            print("  CAMERA_SETTINGS: not received")

        if self.last_camera_ack:
            print(
                "  Last camera ACK: "
                f"target_msg_id={self.last_camera_ack.target_msg_id}, "
                f"stage={self.last_camera_ack.stage}, "
                f"result={self.last_camera_ack.result}, "
                f"ext_val={self.last_camera_ack.ext_val}, "
                f"message={self.last_camera_ack.message}"
            )

    def wait_status(self, seconds=5):
        deadline = time.time() + seconds
        while time.time() < deadline:
            if self.last_gimbal or self.last_camera:
                return True
            time.sleep(0.1)
        return False

    def wait_gimbal_report(self, seconds=8):
        """主动拉取云台角度；比 wait_status 多发送 CONNECT/CHANNEL 请求。"""
        deadline = time.time() + seconds
        while time.time() < deadline:
            self.mav.hy_request_send(request=CONNECT_REQUEST)
            self.mav.hy_request_send(request=CHANNEL_REQUEST)
            if self.last_gimbal:
                return True
            time.sleep(0.2)
        return bool(self.last_gimbal)

    def print_status(self, baseline_pitch=0.0, baseline_yaw=0.0):
        if self.last_gimbal:
            relative_pitch = self.last_gimbal.pitch_deg - float(baseline_pitch)
            relative_yaw = self.last_gimbal.yaw_deg - float(baseline_yaw)
            print(
                "Current absolute angle: "
                f"pitch={self.last_gimbal.pitch_deg:.2f}, "
                f"yaw={self.last_gimbal.yaw_deg:.2f}"
            )
            print(
                "Current relative angle: "
                f"pitch={relative_pitch:.2f}, "
                f"yaw={relative_yaw:.2f} "
                f"(baseline pitch={float(baseline_pitch):.2f}, yaw={float(baseline_yaw):.2f})"
            )
        else:
            print("Current angle: no HY_GIMBAL_REPORT received yet")

        if self.last_camera:
            print(f"Current zoom: {self.last_camera.zoom_times}x")
            print(f"Camera raw status_value: {self.last_camera.raw_status_value}")
        else:
            print("Current zoom: no HY_CAMERA_REPORT received yet")

    def set_absolute_angle(self, pitch, yaw):
        """发送 HY_GIMBAL_CONTROL，pitch/yaw 均为绝对角度模式。"""
        self.mav.hy_gimbal_control_send(
            pitch_mode=GIMBAL_ANGLE_CONTROL,
            yaw_mode=GIMBAL_ANGLE_CONTROL,
            pitch_value=float(pitch),
            yaw_value=float(yaw),
        )

    def set_absolute_directions(self, up=None, down=None, left=None, right=None):
        """按上/下/左/右语义设置绝对角度，并映射到 pitch/yaw。

        方向约定（与实测及厂商示例一致）：
        - up/down   -> pitch  正/负
        - left/right -> yaw   负/正

        未指定的轴：有 HY_GIMBAL_REPORT 时保持当前值，否则回退为 0。
        返回 (pitch, yaw, fallback_axes)，fallback_axes 记录哪些轴因无上报而用了 0。
        """
        if up is not None and down is not None:
            raise ValueError("Specify either up or down, not both.")
        if left is not None and right is not None:
            raise ValueError("Specify either left or right, not both.")
        if up is None and down is None and left is None and right is None:
            raise ValueError("Specify at least one of up, down, left, or right.")

        pitch_specified = up is not None or down is not None
        yaw_specified = left is not None or right is not None

        if self.last_gimbal:
            pitch = self.last_gimbal.pitch_deg
            yaw = self.last_gimbal.yaw_deg
        else:
            pitch = 0.0
            yaw = 0.0

        if up is not None:
            pitch = float(up)
        elif down is not None:
            pitch = -abs(float(down))

        if left is not None:
            yaw = -abs(float(left))
        elif right is not None:
            yaw = abs(float(right))

        fallback_axes = []
        if not pitch_specified and self.last_gimbal is None:
            fallback_axes.append("pitch")
        if not yaw_specified and self.last_gimbal is None:
            fallback_axes.append("yaw")

        self.set_absolute_angle(pitch, yaw)
        return pitch, yaw, fallback_axes

    def set_absolute_up(self, degrees):
        pitch, yaw, _fallback = self.set_absolute_directions(up=degrees)
        return pitch, yaw

    def set_absolute_down(self, degrees):
        pitch, yaw, _fallback = self.set_absolute_directions(down=degrees)
        return pitch, yaw

    def set_absolute_left(self, degrees):
        pitch, yaw, _fallback = self.set_absolute_directions(left=degrees)
        return pitch, yaw

    def set_absolute_right(self, degrees):
        pitch, yaw, _fallback = self.set_absolute_directions(right=degrees)
        return pitch, yaw

    def set_relative_angle(self, delta_pitch, delta_yaw):
        if not self.last_gimbal:
            raise RuntimeError("Need current angle first. Wait for HY_GIMBAL_REPORT before relative control.")

        # The document only defines absolute angle mode and rate mode.
        # Relative angle is implemented as current_angle + delta, then sent as absolute target.
        self.set_absolute_angle(
            self.last_gimbal.pitch_deg + float(delta_pitch),
            self.last_gimbal.yaw_deg + float(delta_yaw),
        )

    def set_rate(self, pitch_rate, yaw_rate):
        """速率模式转动；stop() 通过发送 0 deg/s 停止。"""
        self.mav.hy_gimbal_control_send(
            pitch_mode=GIMBAL_RATE_CONTROL,
            yaw_mode=GIMBAL_RATE_CONTROL,
            pitch_value=float(pitch_rate),
            yaw_value=float(yaw_rate),
        )

    def stop(self):
        self.set_rate(0.0, 0.0)

    def center(self):
        # The documents do not define a dedicated "home/center" command.
        # This uses documented absolute angle control to command pitch=0, yaw=0.
        self.set_absolute_angle(0.0, 0.0)

    def set_zoom(self, zoom_times):
        self.mav.hy_camera_config_send(
            config_type=CAMERA_ZOOM,
            cmd_value=int(zoom_times),
        )

    def set_focus(self, mode):
        self.mav.hy_camera_config_send(
            config_type=CAMERA_FOCUS_VALUE,
            cmd_value=FOCUS_VALUES[mode],
        )

    def take_photo(self):
        self.mav.hy_camera_config_send(
            config_type=CAMERA_TAKE_PHOTO,
            cmd_value=1,
        )

    def take_visible_photo(self):
        self.take_photo()

    def wait_photo_feedback(self, seconds=5):
        saw_taking_photo = False
        deadline = time.time() + seconds
        next_request = 0.0

        while time.time() < deadline:
            if self.last_camera and self.last_camera.taking_photo:
                saw_taking_photo = True

            now = time.time()
            if now >= next_request:
                self.mav.hy_request_send(request=CONNECT_REQUEST)
                self.mav.hy_request_send(request=CHANNEL_REQUEST)
                self.mav.hy_camera_config_send(
                    config_type=CAMERA_HEARTBEAT_FUNCTION,
                    cmd_value=1,
                )
                self.request_message(MAVLINK_MSG_ID_HY_CAMERA_REPORT)
                self.request_message(MAVLINK_MSG_ID_CAMERA_CAPTURE_STATUS)
                self.request_message(MAVLINK_MSG_ID_CAMERA_IMAGE_CAPTURED)
                next_request = now + 0.5

            time.sleep(0.05)

        return saw_taking_photo

    def print_photo_feedback(self, saw_taking_photo=False):
        print("Photo command feedback:")
        if self.last_camera_ack:
            print(
                "  HY_CAMERA_GENERAL_ACK: "
                f"target_msg_id={self.last_camera_ack.target_msg_id}, "
                f"stage={self.last_camera_ack.stage}, "
                f"result={self.last_camera_ack.result}, "
                f"ext_val={self.last_camera_ack.ext_val}, "
                f"message={self.last_camera_ack.message}"
            )
        else:
            print("  HY_CAMERA_GENERAL_ACK: not received")

        if self.last_camera:
            print(
                "  HY_CAMERA_REPORT: "
                f"taking_photo={self.last_camera.taking_photo}, "
                f"raw_status_value={self.last_camera.raw_status_value}"
            )
        else:
            print("  HY_CAMERA_REPORT: not received after command")

        if saw_taking_photo:
            print("  take_photo bit toggled: yes")
        else:
            print("  take_photo bit toggled: not observed")

        if self.last_capture_status:
            print(
                "  CAMERA_CAPTURE_STATUS: "
                f"image_status={self.last_capture_status.image_status}, "
                f"video_status={self.last_capture_status.video_status}, "
                f"image_count={self.last_capture_status.image_count}"
            )
        else:
            print("  CAMERA_CAPTURE_STATUS: not received")

        if self.last_image_captured:
            print(
                "  CAMERA_IMAGE_CAPTURED: "
                f"image_index={self.last_image_captured.image_index}, "
                f"capture_result={self.last_image_captured.capture_result}, "
                f"file_url={self.last_image_captured.file_url}"
            )
        else:
            print("  CAMERA_IMAGE_CAPTURED: not received")

    def wait_gimbal_update(self, seconds=3):
        start_status = self.last_gimbal
        deadline = time.time() + seconds
        while time.time() < deadline:
            if self.last_gimbal and self.last_gimbal is not start_status:
                return self.last_gimbal
            time.sleep(0.05)
        return self.last_gimbal

    def wait_angle_near(self, target_pitch, target_yaw, tolerance=1.0, seconds=8):
        deadline = time.time() + seconds
        last = self.last_gimbal
        while time.time() < deadline:
            if self.last_gimbal:
                last = self.last_gimbal
                if (
                    abs(last.pitch_deg - target_pitch) <= tolerance
                    and abs(last.yaw_deg - target_yaw) <= tolerance
                ):
                    return last, True
            time.sleep(0.1)
        return last, False

    def scan_yaw_range(self, start, stop, step, pitch=0.0, wait_seconds=8, tolerance=1.0):
        if step == 0:
            raise ValueError("step must not be 0")
        if (stop - start) * step < 0:
            raise ValueError("step direction does not move from start to stop")

        results = []
        current = start
        while (step > 0 and current <= stop) or (step < 0 and current >= stop):
            target_yaw = float(current)
            print(f"\nTarget yaw={target_yaw:.2f}, pitch={pitch:.2f}")
            self.set_absolute_angle(pitch, target_yaw)
            actual, reached = self.wait_angle_near(
                pitch,
                target_yaw,
                tolerance=tolerance,
                seconds=wait_seconds,
            )
            if actual:
                yaw_error = actual.yaw_deg - target_yaw
                print(
                    f"  actual yaw={actual.yaw_deg:.2f}, "
                    f"pitch={actual.pitch_deg:.2f}, "
                    f"error={yaw_error:.2f}, "
                    f"reached={reached}"
                )
                results.append((target_yaw, actual.yaw_deg, actual.pitch_deg, yaw_error, reached))
            else:
                print("  no HY_GIMBAL_REPORT received")
                results.append((target_yaw, None, None, None, False))
            current += step

        print("\nYaw range scan summary:")
        reachable = [item for item in results if item[4]]
        if reachable:
            print(f"  reachable target min: {min(item[0] for item in reachable):.2f}")
            print(f"  reachable target max: {max(item[0] for item in reachable):.2f}")
        else:
            print("  no reachable target detected")
        print("  target_yaw,actual_yaw,actual_pitch,error,reached")
        for item in results:
            target_yaw, actual_yaw, actual_pitch, error, reached = item
            actual_yaw_text = "" if actual_yaw is None else f"{actual_yaw:.2f}"
            actual_pitch_text = "" if actual_pitch is None else f"{actual_pitch:.2f}"
            error_text = "" if error is None else f"{error:.2f}"
            print(f"  {target_yaw:.2f},{actual_yaw_text},{actual_pitch_text},{error_text},{reached}")


def build_parser():
    """构建 argparse：全局网络参数 + 各子命令（status / move-abs / zoom 等）。"""
    parser = argparse.ArgumentParser(description="HY-DZ230F MavLink control test demo.")
    parser.add_argument("--ip", default=DEFAULT_DEVICE_IP)
    parser.add_argument("--device-port", type=int, default=DEFAULT_DEVICE_PORT)
    parser.add_argument("--local-port", type=int, default=DEFAULT_LOCAL_PORT)
    parser.add_argument("--connect-seconds", type=float, default=5)
    parser.add_argument(
        "--allow-ephemeral-port",
        action="store_true",
        help=(
            "If local UDP port 3000 is busy, bind a temporary port instead of exiting. "
            "Status reads usually still require port 3000."
        ),
    )
    parser.add_argument(
        "--verbose-reports",
        action="store_true",
        help="Print every decoded GIMBAL/CAMERA report.",
    )

    sub = parser.add_subparsers(dest="command", required=True)
    status_p = sub.add_parser("status", help="Get current angle and zoom from reports.")
    status_p.add_argument(
        "--baseline-pitch",
        type=float,
        default=0.0,
        help="Reference pitch used to calculate relative angle.",
    )
    status_p.add_argument(
        "--baseline-yaw",
        type=float,
        default=0.0,
        help="Reference yaw used to calculate relative angle.",
    )
    status_p.add_argument(
        "--gimbal-seconds",
        type=float,
        default=10.0,
        help="Seconds to actively request HY_GIMBAL_REPORT for angle status.",
    )
    status_p.add_argument(
        "--camera-seconds",
        type=float,
        default=8.0,
        help="Seconds to actively request HY_CAMERA_REPORT for zoom status.",
    )
    status_p.add_argument(
        "--debug-messages",
        action="store_true",
        help="Print every received MAVLink message type.",
    )

    abs_p = sub.add_parser("angle-abs", help="Set absolute pitch/yaw angle in degrees.")
    abs_p.add_argument("--pitch", type=float, required=True)
    abs_p.add_argument("--yaw", type=float, required=True)

    # --- 绝对角度：按方向语义控制（可组合 move-abs --up N --left M）---
    def add_direction_parser(name, help_text):
        direction_p = sub.add_parser(name, help=help_text)
        direction_p.add_argument(
            "--degrees",
            type=float,
            required=True,
            help="Absolute angle in degrees for this direction.",
        )
        return direction_p

    add_direction_parser("up", "Set absolute pitch upward; keep current yaw.")
    add_direction_parser("down", "Set absolute pitch downward; keep current yaw.")
    add_direction_parser("left", "Set absolute yaw to the left; keep current pitch.")
    add_direction_parser("right", "Set absolute yaw to the right; keep current pitch.")

    move_abs_p = sub.add_parser(
        "move-abs",
        help="Set absolute angle by direction; combine up/down with left/right in one command.",
    )
    move_abs_p.add_argument("--up", type=float, default=None, help="Absolute upward pitch in degrees.")
    move_abs_p.add_argument("--down", type=float, default=None, help="Absolute downward pitch in degrees.")
    move_abs_p.add_argument("--left", type=float, default=None, help="Absolute left yaw in degrees.")
    move_abs_p.add_argument("--right", type=float, default=None, help="Absolute right yaw in degrees.")

    yaw_scan_p = sub.add_parser("yaw-scan", help="Scan absolute yaw range with step targets.")
    yaw_scan_p.add_argument("--start", type=float, required=True, help="Start yaw angle.")
    yaw_scan_p.add_argument("--stop", type=float, required=True, help="Stop yaw angle.")
    yaw_scan_p.add_argument("--step", type=float, required=True, help="Yaw step angle.")
    yaw_scan_p.add_argument("--pitch", type=float, default=0.0, help="Pitch angle to keep during scan.")
    yaw_scan_p.add_argument(
        "--wait-seconds",
        type=float,
        default=8.0,
        help="Seconds to wait for each target.",
    )
    yaw_scan_p.add_argument(
        "--tolerance",
        type=float,
        default=1.0,
        help="Angle tolerance used to mark a target as reached.",
    )

    rel_p = sub.add_parser("angle-rel", help="Relative angle = current angle + delta.")
    rel_p.add_argument("--delta-pitch", type=float, required=True)
    rel_p.add_argument("--delta-yaw", type=float, required=True)

    left_p = sub.add_parser("left-30", help="Rotate camera left by 30 degrees.")
    left_p.add_argument(
        "--yaw-delta",
        type=float,
        default=-30.0,
        help="Yaw delta in degrees. Default -30 is treated as left.",
    )

    rate_p = sub.add_parser("rate", help="Set gimbal movement speed in deg/s using rate mode.")
    rate_p.add_argument("--pitch-rate", type=float, required=True)
    rate_p.add_argument("--yaw-rate", type=float, required=True)
    rate_p.add_argument("--duration", type=float, default=0.0)

    sub.add_parser("stop", help="Stop gimbal movement in rate mode.")
    sub.add_parser("center", help="Return to pitch=0,yaw=0 using absolute angle control.")

    zoom_p = sub.add_parser("zoom", help="Set visible-light zoom target.")
    zoom_p.add_argument("--times", type=int, required=True)
    zoom_p.add_argument(
        "--wait-ack",
        type=float,
        default=2.0,
        help="Seconds to wait for HY_CAMERA_GENERAL_ACK after sending zoom.",
    )

    zoom_status_p = sub.add_parser("zoom-status", help="Test current zoom reading only.")
    zoom_status_p.add_argument(
        "--seconds",
        type=float,
        default=10.0,
        help="Seconds to wait for zoom status messages.",
    )
    zoom_status_p.add_argument(
        "--passive-seconds",
        type=float,
        default=3.0,
        help="Seconds to passively listen before sending camera status requests.",
    )
    zoom_status_p.add_argument(
        "--heartbeat-values",
        default="1",
        help="Comma-separated CAMERA_HEARTBEAT_FUNCTION cmd_value list. Default: 1.",
    )
    zoom_status_p.add_argument(
        "--target-system",
        type=int,
        default=0,
        help="Target system for MAV_CMD_REQUEST_MESSAGE. 0 means broadcast.",
    )
    zoom_status_p.add_argument(
        "--target-component",
        type=int,
        default=0,
        help="Target component for MAV_CMD_REQUEST_MESSAGE. 0 means broadcast.",
    )
    zoom_status_p.add_argument(
        "--debug-messages",
        action="store_true",
        help="Print every received MAVLink message type.",
    )

    focus_p = sub.add_parser("focus", help="Control focus.")
    focus_p.add_argument("mode", choices=sorted(FOCUS_VALUES))

    sub.add_parser("autofocus", help="Run camera autofocus. Alias of: focus auto.")

    def add_photo_parser(name, help_text):
        photo_p = sub.add_parser(name, help=help_text)
        photo_p.add_argument(
            "--wait-feedback",
            type=float,
            default=5.0,
            help="Seconds to wait for ACK, camera status, or image-captured feedback.",
        )
        photo_p.add_argument(
            "--repeat",
            type=int,
            default=1,
            help="Number of CAMERA_TAKE_PHOTO commands to send.",
        )
        photo_p.add_argument(
            "--interval",
            type=float,
            default=0.5,
            help="Seconds between repeated photo commands.",
        )

    add_photo_parser("photo", "Take photo via HY_CAMERA_CONFIG/CAMERA_TAKE_PHOTO.")
    add_photo_parser("photo-visible", "Alias of photo; sends generic CAMERA_TAKE_PHOTO.")
    sub.add_parser("photo-thermal-664", help="Not defined in the provided documents.")
    sub.add_parser("reboot", help="Not defined in the provided documents.")

    return parser


def describe_udp_port_owner(port):
    """用 lsof 查占用指定 UDP 端口的进程，便于提示用户释放 3000。"""
    try:
        result = subprocess.run(
            ["lsof", "-nP", f"-iUDP:{port}"],
            capture_output=True,
            text=True,
            timeout=5,
            check=False,
        )
        lines = [line for line in result.stdout.strip().splitlines() if line.strip()]
        if len(lines) > 1:
            return "\n".join(lines)
    except (FileNotFoundError, subprocess.TimeoutExpired, OSError):
        pass
    return None


def create_client(ip, device_port, local_port, allow_ephemeral_port=False):
    """创建客户端；默认要求绑定 local_port，避免静默降级导致读不到状态。"""
    try:
        return HyGimbalClient(ip, device_port, local_port)
    except OSError as exc:
        if local_port == 0:
            raise
        owner = describe_udp_port_owner(local_port)
        print(f"Local port {local_port} is unavailable ({exc}).")
        if owner:
            print("Process currently using the port:")
            print(owner)
        print(f"Check with: lsof -nP -iUDP:{local_port}")
        if not allow_ephemeral_port:
            raise SystemExit(
                f"Cannot bind local UDP port {local_port}. "
                "This device sends HY_GIMBAL_REPORT/HY_CAMERA_REPORT back to local port 3000. "
                "Stop the process above and retry, or pass --allow-ephemeral-port to force a temporary port."
            ) from exc
        print("Retrying with an ephemeral local port (--allow-ephemeral-port).")
        print(
            "Warning: status reads usually fail on a temporary port because the device "
            "replies to UDP 3000."
        )
        return HyGimbalClient(ip, device_port, 0)


def print_status_read_failure(client, device_ip):
    print()
    print("Failed to read device status.")
    if client.local_port != DEFAULT_LOCAL_PORT:
        print(
            f"  Local bind port is {client.local_port}, not {DEFAULT_LOCAL_PORT}. "
            "The device usually sends telemetry back to UDP 3000."
        )
        owner = describe_udp_port_owner(DEFAULT_LOCAL_PORT)
        if owner:
            print("  Process currently using UDP 3000:")
            for line in owner.splitlines()[1:]:
                print(f"    {line}")
    print(f"  Device target: {device_ip}:{DEFAULT_DEVICE_PORT}")
    print("  Try:")
    print("    1. Stop the process using UDP 3000, then run status again.")
    print("    2. Confirm the device IP with --ip if it is not 10.21.31.111.")
    print("    3. Run with --verbose-reports --debug-messages to inspect incoming packets.")


def ensure_gimbal_report(client, connect_seconds):
    """方向控制前尽量拿到当前角度，用于保留未指定轴的目标值。"""
    if client.last_gimbal:
        return True
    wait_seconds = max(float(connect_seconds), 8.0)
    return client.wait_gimbal_report(wait_seconds)


def print_direction_fallback_warning(fallback_axes):
    if not fallback_axes:
        return
    axes = ", ".join(fallback_axes)
    print(
        f"Warning: no HY_GIMBAL_REPORT; unspecified axis defaults to 0 ({axes}). "
        "Stop other processes on UDP 3000 if reports never arrive."
    )


def run_direction_command(client, connect_seconds, command_fn):
    """执行 up/down/left/right/move-abs：先等云台上报，再发 HY_GIMBAL_CONTROL。"""
    ensure_gimbal_report(client, connect_seconds)
    try:
        pitch, yaw, fallback_axes = command_fn()
    except ValueError as exc:
        raise SystemExit(str(exc)) from exc
    print_direction_fallback_warning(fallback_axes)
    return pitch, yaw


def run(args):
    """CLI 入口：建连 -> 执行子命令 -> 短暂等待后关闭 socket。"""
    if args.command in {"photo-thermal-664", "reboot"}:
        raise SystemExit(
            f"{args.command} is not defined in the provided PDF/DOCX. "
            "Ask the vendor for the exact MavLink message/config_type/cmd_value."
        )

    client = create_client(
        args.ip,
        args.device_port,
        args.local_port,
        allow_ephemeral_port=args.allow_ephemeral_port,
    )
    client.debug_messages = getattr(args, "debug_messages", False)
    client.verbose_reports = args.verbose_reports
    client.start()
    try:
        connected = client.connect_until_report(args.connect_seconds)
        if not connected:
            print("No status report received yet; command will still be sent.")

        if args.command == "status":
            gimbal_seconds = max(float(args.gimbal_seconds), float(args.connect_seconds))
            if not client.last_gimbal:
                client.wait_gimbal_report(gimbal_seconds)
            if not client.last_camera:
                client.request_camera_status_until_report(args.camera_seconds)
            client.print_status(
                baseline_pitch=args.baseline_pitch,
                baseline_yaw=args.baseline_yaw,
            )
            if not client.last_gimbal and not client.last_camera:
                print_status_read_failure(client, args.ip)
                raise SystemExit(1)
        elif args.command == "angle-abs":
            client.set_absolute_angle(args.pitch, args.yaw)
            print(f"Sent absolute angle: pitch={args.pitch}, yaw={args.yaw}")
        elif args.command in {"up", "down", "left", "right"}:
            if args.command == "up":
                pitch, yaw = run_direction_command(
                    client,
                    args.connect_seconds,
                    lambda: client.set_absolute_directions(up=args.degrees),
                )
            elif args.command == "down":
                pitch, yaw = run_direction_command(
                    client,
                    args.connect_seconds,
                    lambda: client.set_absolute_directions(down=args.degrees),
                )
            elif args.command == "left":
                pitch, yaw = run_direction_command(
                    client,
                    args.connect_seconds,
                    lambda: client.set_absolute_directions(left=args.degrees),
                )
            else:
                pitch, yaw = run_direction_command(
                    client,
                    args.connect_seconds,
                    lambda: client.set_absolute_directions(right=args.degrees),
                )
            print(f"Sent absolute {args.command}: pitch={pitch:.2f}, yaw={yaw:.2f}")
        elif args.command == "move-abs":
            pitch, yaw = run_direction_command(
                client,
                args.connect_seconds,
                lambda: client.set_absolute_directions(
                    up=args.up,
                    down=args.down,
                    left=args.left,
                    right=args.right,
                ),
            )
            parts = []
            if args.up is not None:
                parts.append(f"up={args.up}")
            if args.down is not None:
                parts.append(f"down={args.down}")
            if args.left is not None:
                parts.append(f"left={args.left}")
            if args.right is not None:
                parts.append(f"right={args.right}")
            print(f"Sent move-abs ({', '.join(parts)}): pitch={pitch:.2f}, yaw={yaw:.2f}")
        elif args.command == "yaw-scan":
            if not client.last_gimbal:
                client.wait_status(3)
            client.scan_yaw_range(
                start=args.start,
                stop=args.stop,
                step=args.step,
                pitch=args.pitch,
                wait_seconds=args.wait_seconds,
                tolerance=args.tolerance,
            )
        elif args.command == "angle-rel":
            if not client.last_gimbal:
                client.wait_status(3)
            client.set_relative_angle(args.delta_pitch, args.delta_yaw)
            print(f"Sent relative angle delta: pitch={args.delta_pitch}, yaw={args.delta_yaw}")
        elif args.command == "left-30":
            if not client.last_gimbal:
                client.wait_status(3)
            client.set_relative_angle(0.0, args.yaw_delta)
            print(f"Sent left rotation: yaw_delta={args.yaw_delta}")
        elif args.command == "rate":
            client.set_rate(args.pitch_rate, args.yaw_rate)
            print(f"Sent rate: pitch={args.pitch_rate} deg/s, yaw={args.yaw_rate} deg/s")
            if args.duration > 0:
                time.sleep(args.duration)
                client.stop()
                print("Stopped after duration.")
        elif args.command == "stop":
            client.stop()
            print("Sent stop.")
        elif args.command == "center":
            client.center()
            print("Sent center: pitch=0, yaw=0.")
        elif args.command == "zoom":
            client.set_zoom(args.times)
            print(f"Sent zoom target: {args.times}x")
            time.sleep(args.wait_ack)
            if client.last_camera_ack:
                print(
                    "Last camera ACK: "
                    f"target_msg_id={client.last_camera_ack.target_msg_id}, "
                    f"stage={client.last_camera_ack.stage}, "
                    f"result={client.last_camera_ack.result}, "
                    f"ext_val={client.last_camera_ack.ext_val}, "
                    f"message={client.last_camera_ack.message}"
                )
            else:
                print("No HY_CAMERA_GENERAL_ACK received for zoom command.")
        elif args.command == "zoom-status":
            heartbeat_values = [
                int(value.strip())
                for value in args.heartbeat_values.split(",")
                if value.strip()
            ]
            client.test_current_zoom(
                seconds=args.seconds,
                target_system=args.target_system,
                target_component=args.target_component,
                passive_seconds=args.passive_seconds,
                heartbeat_values=heartbeat_values,
            )
        elif args.command == "focus":
            client.set_focus(args.mode)
            print(f"Sent focus: {args.mode}")
        elif args.command == "autofocus":
            client.set_focus("auto")
            print("Sent autofocus.")
        elif args.command in {"photo", "photo-visible"}:
            client.last_camera_ack = None
            client.last_capture_status = None
            client.last_image_captured = None
            for index in range(max(1, args.repeat)):
                client.take_photo()
                print(
                    "Sent HY_CAMERA_CONFIG photo command: "
                    f"config_type={CAMERA_TAKE_PHOTO}, cmd_value=1"
                )
                if index + 1 < max(1, args.repeat):
                    time.sleep(max(0.0, args.interval))
            saw_taking_photo = client.wait_photo_feedback(args.wait_feedback)
            client.print_photo_feedback(saw_taking_photo=saw_taking_photo)

        time.sleep(0.5)
    finally:
        client.close()


if __name__ == "__main__":
    run(build_parser().parse_args())
