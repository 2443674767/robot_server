package control

import (
	"strings"

	internalcontrol "gofly/app/robotdog/internal/control"
	"gofly/dao"
	"gofly/dao/model"
	"gofly/utils/gf"
)

type Ptz struct{ NoNeedAuths []string }

func init() {
	gf.Register(&Ptz{})
}

func (api *Ptz) Move(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	ptz, err := findPTZ(ctx, param)
	if err != nil {
		gf.Failed().SetMsg("云台不存在").SetData(err).Regin(ctx)
		return
	}
	command := normalizePTZCommand(gf.String(param["cmd"]))
	if command == "" {
		command = normalizePTZCommand(gf.String(param["direction"]))
	}
	if command == "" {
		gf.Failed().SetMsg("云台命令不能为空").Regin(ctx)
		return
	}
	driver, driverName, err := internalcontrol.PTZDriver(ptz.Model)
	if err != nil {
		gf.Failed().SetMsg("获取云台驱动失败").SetData(err).Regin(ctx)
		return
	}
	target := internalcontrol.PTZTarget{
		ID:                ptz.ID,
		Name:              ptz.Name,
		DeviceUID:         ptz.DeviceUID,
		Username:          ptz.Username,
		Password:          ptz.Password,
		Brand:             ptz.Brand,
		Model:             ptz.Model,
		Protocol:          strings.ToLower(ptz.Protocol),
		UDPHost:           firstNonEmpty(ptz.UdpHost, ptz.IPAddr),
		UDPPort:           ptz.UdpPort,
		LocalPort:         ptz.LocalPort,
		TargetSystemID:    byte(ptz.TargetSystemID),
		TargetComponentID: byte(ptz.TargetComponentID),
		SourceSystemID:    byte(ptz.SourceSystemID),
		SourceComponentID: byte(ptz.SourceComponentID),
		RTSPURL:           ptz.RTSPURL,
	}
	target = internalcontrol.FillPTZTargetDefaults(target)
	moveOpts := internalcontrol.PTZMoveOptions{
		Direction: command,
		Speed:     gf.Float64(param["speed"]),
		Duration:  gf.Int(param["duration"]),
		Pan:       firstPositiveFloat(gf.Float64(param["step"]), gf.Float64(param["pan"]), gf.Float64(param["yaw"])),
		Tilt:      firstPositiveFloat(gf.Float64(param["step"]), gf.Float64(param["tilt"]), gf.Float64(param["pitch"])),
	}
	zoomOpts := internalcontrol.ZoomOptions{
		Direction: command,
		Step:      firstPositiveFloat(gf.Float64(param["step"]), gf.Float64(param["speed"]), gf.Float64(param["zoom"])),
		Duration:  gf.Int(param["duration"]),
	}
	focusOpts := internalcontrol.FocusOptions{
		Direction: command,
		Step:      gf.Float64(param["step"]),
		Duration:  gf.Int(param["duration"]),
	}
	var result *internalcontrol.CommandResult
	switch command {
	case "up":
		result, err = driver.Up(ctx, target, moveOpts)
	case "down":
		result, err = driver.Down(ctx, target, moveOpts)
	case "left":
		result, err = driver.Left(ctx, target, moveOpts)
	case "right":
		result, err = driver.Right(ctx, target, moveOpts)
	case "up_fast":
		moveOpts.Tilt = ptzControlStep(param, 2) * 5
		result, err = driver.Up(ctx, target, moveOpts)
	case "down_fast":
		moveOpts.Tilt = ptzControlStep(param, 2) * 5
		result, err = driver.Down(ctx, target, moveOpts)
	case "left_fast":
		moveOpts.Pan = ptzControlStep(param, 5) * 5
		result, err = driver.Left(ctx, target, moveOpts)
	case "right_fast":
		moveOpts.Pan = ptzControlStep(param, 5) * 5
		result, err = driver.Right(ctx, target, moveOpts)
	case "zoom_in":
		result, err = driver.ZoomIn(ctx, target, zoomOpts)
	case "zoom_out":
		result, err = driver.ZoomOut(ctx, target, zoomOpts)
	case "zoom_in_fast":
		zoomOpts.Step = ptzControlStep(param, 0.5) * 5
		result, err = driver.ZoomIn(ctx, target, zoomOpts)
	case "zoom_out_fast":
		zoomOpts.Step = ptzControlStep(param, 0.5) * 5
		result, err = driver.ZoomOut(ctx, target, zoomOpts)
	case "nudge":
		nudgeDriver, ok := driver.(internalcontrol.PTZNudgeController)
		if !ok {
			gf.Failed().SetMsg("当前云台驱动不支持nudge控制").Regin(ctx)
			return
		}
		axis := strings.TrimSpace(gf.String(param["axis"]))
		delta := gf.Float64(param["delta"])
		if axis == "" || delta == 0 {
			gf.Failed().SetMsg("nudge控制需要axis和delta").Regin(ctx)
			return
		}
		result, err = nudgeDriver.Nudge(ctx, target, axis, delta, firstPositiveFloat(gf.Float64(param["zoom_max"]), gf.Float64(param["zoomMax"]), 30))
	case "focus_near":
		result, err = driver.FocusNear(ctx, target, focusOpts)
	case "focus_far":
		result, err = driver.FocusFar(ctx, target, focusOpts)
	case "home":
		result, err = driver.Home(ctx, target)
	case "zoom_home":
		zoomHomeDriver, ok := driver.(internalcontrol.PTZZoomHomeController)
		if !ok {
			gf.Failed().SetMsg("当前云台驱动不支持变倍回到1x").Regin(ctx)
			return
		}
		result, err = zoomHomeDriver.ZoomHome(ctx, target)
	case "angle_set":
		yaw := gf.Float64(param["yaw"])
		if yaw == 0 {
			yaw = gf.Float64(param["pan"])
		}
		pitch := gf.Float64(param["pitch"])
		if pitch == 0 {
			pitch = gf.Float64(param["tilt"])
		}
		result, err = driver.SetAngle(ctx, target, internalcontrol.PTZAngleOptions{
			Pan:      yaw,
			Tilt:     pitch,
			Roll:     gf.Float64(param["roll"]),
			Duration: gf.Int(param["duration"]),
		})
	case "photo":
		result, err = driver.TakePhoto(ctx, target, internalcontrol.PhotoOptions{
			Mode:     gf.String(param["mode"]),
			Folder:   gf.String(param["folder"]),
			Filename: gf.String(param["filename"]),
		})
	case "stop":
		result, err = driver.Stop(ctx, target)
	case "refresh":
		refreshDriver, ok := driver.(internalcontrol.PTZRefreshController)
		if !ok {
			gf.Failed().SetMsg("当前云台驱动不支持刷新控制").Regin(ctx)
			return
		}
		result, err = refreshDriver.Refresh(ctx, target)
	default:
		gf.Failed().SetMsg("不支持的云台命令: " + command).Regin(ctx)
		return
	}
	if err != nil {
		gf.Failed().SetMsg("发送云台UDP指令失败: " + err.Error()).SetData(map[string]interface{}{"error": err.Error()}).Regin(ctx)
		return
	}
	result.Driver = driverName
	gf.Success().SetMsg("云台UDP指令已发送").SetData(result).Regin(ctx)
}

func (api *Ptz) GetRealtime(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	ptz, err := findPTZ(ctx, param)
	if err != nil {
		gf.Failed().SetMsg("云台不存在").SetData(err).Regin(ctx)
		return
	}
	driver, driverName, err := internalcontrol.PTZDriver(ptz.Model)
	if err != nil {
		gf.Failed().SetMsg("获取云台驱动失败").SetData(err).Regin(ctx)
		return
	}
	data, err := driver.Realtime(ctx, internalcontrol.PTZTarget{
		ID:                ptz.ID,
		Name:              ptz.Name,
		DeviceUID:         ptz.DeviceUID,
		Username:          ptz.Username,
		Password:          ptz.Password,
		Brand:             ptz.Brand,
		Model:             ptz.Model,
		Protocol:          strings.ToLower(ptz.Protocol),
		UDPHost:           firstNonEmpty(ptz.UdpHost, ptz.IPAddr),
		UDPPort:           ptz.UdpPort,
		LocalPort:         ptz.LocalPort,
		TargetSystemID:    byte(ptz.TargetSystemID),
		TargetComponentID: byte(ptz.TargetComponentID),
		SourceSystemID:    byte(ptz.SourceSystemID),
		SourceComponentID: byte(ptz.SourceComponentID),
		RTSPURL:           ptz.RTSPURL,
	})
	if err != nil {
		gf.Failed().SetMsg("获取云台实时数据失败").SetData(err).Regin(ctx)
		return
	}
	data.Driver = driverName
	gf.Success().SetMsg("获取云台实时数据").SetData(data).Regin(ctx)
}

func findPTZ(ctx *gf.GinCtx, param map[string]interface{}) (*model.RobotdogPtz, error) {
	ptzDB := dao.Query().RobotdogPtz
	tenant := tenantID(ctx, param)
	if ptzID := gf.Int64(param["ptz_id"]); ptzID > 0 {
		return ptzDB.WithContext(ctx).Where(ptzDB.ID.Eq(ptzID), ptzDB.TenantID.Eq(tenant)).First()
	}
	if dogID := gf.Int64(param["dog_id"]); dogID > 0 {
		dogDB := dao.Query().RobotdogDog
		if dog, err := dogDB.WithContext(ctx).Where(dogDB.ID.Eq(dogID), dogDB.TenantID.Eq(tenant)).First(); err == nil && dog.PtzID > 0 {
			return ptzDB.WithContext(ctx).Where(ptzDB.ID.Eq(dog.PtzID), ptzDB.TenantID.Eq(tenant)).First()
		}
	}
	return ptzDB.WithContext(ctx).Where(ptzDB.TenantID.Eq(tenant), ptzDB.Status.Eq("online")).Order(ptzDB.ID.Asc()).First()
}

func normalizePTZCommand(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	switch v {
	case "up", "u", "tilt_up":
		return "up"
	case "down", "d", "tilt_down":
		return "down"
	case "left", "l", "pan_left":
		return "left"
	case "right", "r", "pan_right":
		return "right"
	case "up_fast", "upfast", "tilt_up_fast":
		return "up_fast"
	case "down_fast", "downfast", "tilt_down_fast":
		return "down_fast"
	case "left_fast", "leftfast", "pan_left_fast":
		return "left_fast"
	case "right_fast", "rightfast", "pan_right_fast":
		return "right_fast"
	case "zoom_in", "zoomin", "zoom+":
		return "zoom_in"
	case "zoom_out", "zoomout", "zoom-":
		return "zoom_out"
	case "zoom_in_fast", "zoomin_fast", "zoominfast", "zoom+_fast":
		return "zoom_in_fast"
	case "zoom_out_fast", "zoomout_fast", "zoomoutfast", "zoom-_fast":
		return "zoom_out_fast"
	case "zoom_home", "zoomhome", "zoom_reset", "zoom_zero":
		return "zoom_home"
	case "nudge":
		return "nudge"
	case "focus_near", "focusnear", "focus+":
		return "focus_near"
	case "focus_far", "focusfar", "focus-":
		return "focus_far"
	case "home", "center", "reset", "zero":
		return "home"
	case "angle_set", "angleset", "set_angle":
		return "angle_set"
	case "photo", "take_photo", "capture":
		return "photo"
	case "stop", "halt":
		return "stop"
	case "refresh", "status":
		return "refresh"
	default:
		return v
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstPositiveFloat(values ...float64) float64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func ptzControlStep(param map[string]interface{}, fallback float64) float64 {
	return firstPositiveFloat(gf.Float64(param["step"]), gf.Float64(param["speed"]), gf.Float64(param["pan"]), gf.Float64(param["tilt"]), gf.Float64(param["yaw"]), gf.Float64(param["pitch"]), fallback)
}
