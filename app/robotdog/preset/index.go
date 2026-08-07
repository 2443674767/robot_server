package preset

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	internalcontrol "gofly/app/robotdog/internal/control"
	"gofly/dao"
	"gofly/dao/model"
	"gofly/utils/gf"
)

const (
	dogAPIHost = "10.21.31.103"
	dogAPIPort = 30000
	ptzAPIHost = "10.21.31.64"
)

type Index struct{ NoNeedAuths []string }

func init() {
	gf.Register(&Index{})
}

func tenantID(ctx *gf.GinCtx, param map[string]interface{}) int32 {
	id := ctx.GetInt32("tenant_id")
	if id == 0 {
		id = gf.Int32(param["tenant_id"])
	}
	if id == 0 {
		id = 1
	}
	return id
}

func pageArgs(param map[string]interface{}) (int, int) {
	page := gf.Int(param["page"])
	limit := gf.Int(param["limit"])
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}
	return (page - 1) * limit, limit
}

func stringValue(param map[string]interface{}, key string, def string) string {
	if v, ok := param[key]; ok {
		if s := strings.TrimSpace(gf.String(v)); s != "" {
			return s
		}
	}
	return def
}

func hasParam(param map[string]interface{}, key string) bool {
	_, ok := param[key]
	return ok
}

func routeWaypointMap(ctx *gf.GinCtx, tenantID int32, routeIDs []int64) map[int64][]int64 {
	result := make(map[int64][]int64)
	if len(routeIDs) == 0 {
		return result
	}
	rwDB := dao.Query().RobotdogRouteWaypoint
	rows, err := rwDB.WithContext(ctx).Where(rwDB.TenantID.Eq(tenantID), rwDB.RouteID.In(routeIDs...)).Order(rwDB.RouteID.Asc(), rwDB.Weigh.Asc()).Find()
	if err != nil {
		return result
	}
	for _, row := range rows {
		result[row.RouteID] = append(result[row.RouteID], row.WaypointID)
	}
	return result
}

func routeListData(ctx *gf.GinCtx, tenantID int32, routes []*model.RobotdogRoute) []map[string]interface{} {
	ids := make([]int64, 0, len(routes))
	for _, route := range routes {
		ids = append(ids, route.ID)
	}
	wpMap := routeWaypointMap(ctx, tenantID, ids)
	list := make([]map[string]interface{}, 0, len(routes))
	for _, route := range routes {
		list = append(list, map[string]interface{}{
			"id":           route.ID,
			"tenant_id":    route.TenantID,
			"dog_id":       route.DogID,
			"name":         route.Name,
			"status":       route.RunStatus,
			"route_status": route.Status,
			"remark":       route.Remark,
			"waypoint_ids": wpMap[route.ID],
			"created_at":   route.CreatedAt,
			"updated_at":   route.UpdatedAt,
		})
	}
	return list
}

func newTaskID(prefix string) string {
	return fmt.Sprintf("%s-%s-%03d", prefix, time.Now().Format("20060102150405"), time.Now().UnixNano()%1000)
}

func (api *Index) GetRouteList(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	routeDB := dao.Query().RobotdogRoute
	tenant := tenantID(ctx, param)
	where := []dao.Condition{routeDB.TenantID.Eq(tenant)}
	routeStatus := stringValue(param, "route_status", "published")
	if routeStatus != "" {
		where = append(where, routeDB.Status.Eq(routeStatus))
	}
	if dogID := gf.Int64(param["dog_id"]); dogID > 0 {
		where = append(where, routeDB.DogID.Eq(dogID))
	}
	if status := stringValue(param, "status", ""); status != "" {
		where = append(where, routeDB.RunStatus.Eq(status))
	}
	if name := stringValue(param, "name", ""); name != "" {
		where = append(where, routeDB.Name.Like("%"+name+"%"))
	}
	offset, limit := pageArgs(param)
	routes, total, err := routeDB.WithContext(ctx).Where(where...).Order(routeDB.ID.Desc()).FindByPage(offset, limit)
	if err != nil {
		gf.Failed().SetMsg("获取预置位航线列表失败").SetData(err).Regin(ctx)
		return
	}
	gf.Success().SetMsg("获取预置位航线列表").SetData(map[string]interface{}{"list": routeListData(ctx, tenant, routes), "total": total}).Regin(ctx)
}

func (api *Index) GetPlayUrl(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	dogID := gf.Int64(param["dog_id"])
	if dogID == 0 {
		gf.Failed().SetMsg("机械狗ID不能为空").Regin(ctx)
		return
	}
	dogDB := dao.Query().RobotdogDog
	dog, err := dogDB.WithContext(ctx).Where(dogDB.ID.Eq(dogID), dogDB.TenantID.Eq(tenantID(ctx, param))).First()
	if err != nil {
		gf.Failed().SetMsg("获取视频地址失败").SetData(err).Regin(ctx)
		return
	}
	playURL := dog.StreamURL
	rtspURL := dog.RtspURL
	if playURL == "" {
		playURL = fmt.Sprintf("http://%s:%d/live/dog%d.live.flv", dogAPIHost, dogAPIPort, dog.ID)
	}
	if rtspURL == "" {
		rtspURL = fmt.Sprintf("rtsp://%s:%d/dog/%d/stream", dogAPIHost, dogAPIPort, dog.ID)
	}
	gf.Success().SetMsg("获取视频地址").SetData(map[string]interface{}{
		"dog_id":    dog.ID,
		"play_url":  playURL,
		"rtsp_url":  rtspURL,
		"protocol":  "flv",
		"api_host":  dogAPIHost,
		"api_port":  dogAPIPort,
		"connected": false,
	}).Regin(ctx)
}

func (api *Index) DogCmd(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	dogID := gf.Int64(param["dog_id"])
	cmd := stringValue(param, "cmd", "")
	if dogID == 0 || cmd == "" {
		gf.Failed().SetMsg("机械狗ID和命令不能为空").Regin(ctx)
		return
	}
	gf.Success().SetMsg("机械狗命令已接收").SetData(map[string]interface{}{
		"dog_id":      dogID,
		"cmd":         cmd,
		"speed":       gf.Float64(param["speed"]),
		"duration":    gf.Int(param["duration"]),
		"accepted":    true,
		"placeholder": true,
		"device_host": dogAPIHost,
		"device_port": dogAPIPort,
	}).Regin(ctx)
}

func (api *Index) PtzCmd(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	cmd := stringValue(param, "cmd", "")
	if cmd == "" {
		gf.Failed().SetMsg("云台命令不能为空").Regin(ctx)
		return
	}
	gf.Success().SetMsg("云台命令已接收").SetData(map[string]interface{}{
		"cmd":         cmd,
		"pan":         gf.Float64(param["pan"]),
		"tilt":        gf.Float64(param["tilt"]),
		"zoom":        gf.Float64(param["zoom"]),
		"accepted":    true,
		"placeholder": true,
		"device_host": ptzAPIHost,
	}).Regin(ctx)
}

func (api *Index) PtzMove(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	ptz, err := findPTZ(ctx, param)
	if err != nil {
		gf.Failed().SetMsg("云台不存在").SetData(err).Regin(ctx)
		return
	}
	command := normalizePTZCommand(stringValue(param, "cmd", ""))
	if command == "" {
		command = normalizePTZCommand(stringValue(param, "direction", ""))
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
	target := internalcontrol.FillPTZTargetDefaults(ptzTarget(ptz))
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
		Step:      firstPositiveFloat(gf.Float64(param["step"]), gf.Float64(param["speed"])),
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
		axis := stringValue(param, "axis", "")
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
		if !hasParam(param, "yaw") {
			yaw = gf.Float64(param["pan"])
		}
		pitch := gf.Float64(param["pitch"])
		if !hasParam(param, "pitch") {
			pitch = gf.Float64(param["tilt"])
		}
		result, err = driver.SetAngle(ctx, target, internalcontrol.PTZAngleOptions{
			Pan:      yaw,
			Tilt:     pitch,
			Roll:     gf.Float64(param["roll"]),
			Duration: gf.Int(param["duration"]),
		})
	case "photo":
		result, err = driver.TakePhoto(ctx, target, photoOptions(param))
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

func (api *Index) PtzPhoto(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	tenant := tenantID(ctx, param)
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
	opts := photoOptions(param)
	result, err := driver.TakePhoto(ctx, internalcontrol.FillPTZTargetDefaults(ptzTarget(ptz)), opts)
	if err != nil {
		gf.Failed().SetMsg("云台拍照失败: " + err.Error()).SetData(map[string]interface{}{"error": err.Error()}).Regin(ctx)
		return
	}
	result.Driver = driverName
	rawData, _ := json.Marshal(result)
	now := time.Now()
	filePath := photoFilePath(opts.Folder, opts.Filename)
	photo := &model.RobotdogPtzPhoto{
		TenantID:   tenant,
		WaypointID: gf.Int64(param["waypoint_id"]),
		PtzID:      ptz.ID,
		Filename:   opts.Filename,
		FilePath:   filePath,
		Mode:       opts.Mode,
		RawData:    string(rawData),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := dao.DB().WithContext(ctx).Create(photo).Error; err != nil {
		gf.Failed().SetMsg("保存云台拍照记录失败").SetData(err).Regin(ctx)
		return
	}
	gf.Success().SetMsg("拍照成功").SetData(photoResponse(photo)).Regin(ctx)
}

func (api *Index) GetPtzPhotoList(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	tenant := tenantID(ctx, param)
	offset, limit := pageArgs(param)
	db := dao.DB().WithContext(ctx).Model(&model.RobotdogPtzPhoto{}).Where("tenant_id = ? AND deleted_at IS NULL", tenant)
	if waypointID := gf.Int64(param["waypoint_id"]); waypointID > 0 {
		db = db.Where("waypoint_id = ?", waypointID)
	}
	if ptzID := gf.Int64(param["ptz_id"]); ptzID > 0 {
		db = db.Where("ptz_id = ?", ptzID)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		gf.Failed().SetMsg("获取云台拍照记录总数失败").SetData(err).Regin(ctx)
		return
	}
	var list []model.RobotdogPtzPhoto
	if err := db.Order("id DESC").Offset(offset).Limit(limit).Find(&list).Error; err != nil {
		gf.Failed().SetMsg("获取云台拍照记录列表失败").SetData(err).Regin(ctx)
		return
	}
	rows := make([]map[string]interface{}, 0, len(list))
	for i := range list {
		rows = append(rows, photoResponse(&list[i]))
	}
	gf.Success().SetMsg("获取云台拍照记录列表").SetData(map[string]interface{}{"list": rows, "total": total}).Regin(ctx)
}

func (api *Index) PtzPhotoDel(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	id := gf.Int64(param["id"])
	if id == 0 {
		gf.Failed().SetMsg("云台拍照记录ID不能为空").Regin(ctx)
		return
	}
	if err := dao.DB().WithContext(ctx).Where("id = ? AND tenant_id = ?", id, tenantID(ctx, param)).Delete(&model.RobotdogPtzPhoto{}).Error; err != nil {
		gf.Failed().SetMsg("删除云台拍照记录失败").SetData(err).Regin(ctx)
		return
	}
	gf.Success().SetMsg("删除云台拍照记录成功").SetData(nil).Regin(ctx)
}

func (api *Index) GetPtzGetRealtime(ctx *gf.GinCtx) {
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
	target := internalcontrol.FillPTZTargetDefaults(ptzTarget(ptz))
	data, err := driver.Realtime(ctx, target)
	if err != nil {
		gf.Failed().SetMsg("无法获取云台实时姿态").SetData(map[string]interface{}{
			"error":      err.Error(),
			"ptz_id":     ptz.ID,
			"ptz_name":   ptz.Name,
			"model":      ptz.Model,
			"udp_host":   target.UDPHost,
			"udp_port":   target.UDPPort,
			"local_port": target.LocalPort,
		}).Regin(ctx)
		return
	}
	data.Driver = driverName
	gf.Success().SetMsg("获取云台实时数据").SetData(data).Regin(ctx)
}

func (api *Index) PtzSetPreset(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	tenant := tenantID(ctx, param)
	waypointID := gf.Int64(param["waypoint_id"])
	if waypointID == 0 {
		gf.Failed().SetMsg("航点ID不能为空").Regin(ctx)
		return
	}
	waypoint, err := dao.Query().RobotdogWaypoint.WithContext(ctx).Where(dao.Query().RobotdogWaypoint.ID.Eq(waypointID), dao.Query().RobotdogWaypoint.TenantID.Eq(tenant)).First()
	if err != nil {
		gf.Failed().SetMsg("特殊航点不存在").SetData(err).Regin(ctx)
		return
	}
	if waypoint.IsTask != 1 {
		gf.Failed().SetMsg("当前航点不是任务航点").Regin(ctx)
		return
	}
	if gf.Int64(param["ptz_id"]) == 0 && waypoint.DogID > 0 {
		param["dog_id"] = waypoint.DogID
	}
	ptz, err := findPTZ(ctx, param)
	if err != nil {
		gf.Failed().SetMsg("云台不存在").SetData(err).Regin(ctx)
		return
	}
	driver, _, err := internalcontrol.PTZDriver(ptz.Model)
	if err != nil {
		gf.Failed().SetMsg("获取云台驱动失败").SetData(err).Regin(ctx)
		return
	}
	realtime, err := driver.Realtime(ctx, internalcontrol.FillPTZTargetDefaults(ptzTarget(ptz)))
	if err != nil {
		gf.Failed().SetMsg("无法获取云台实时姿态").SetData(err).Regin(ctx)
		return
	}
	if realtime.Pitch == nil || realtime.Yaw == nil || realtime.Roll == nil {
		gf.Failed().SetMsg("无法获取云台实时姿态").SetData(realtime).Regin(ctx)
		return
	}
	servoPhoto, ok := optionalBinaryInt8(param, "servo_photo")
	if !ok {
		gf.Failed().SetMsg("配置值无效").Regin(ctx)
		return
	}
	autoHome, ok := optionalBinaryInt8(param, "auto_home")
	if !ok {
		gf.Failed().SetMsg("配置值无效").Regin(ctx)
		return
	}
	rawData, _ := json.Marshal(realtime)
	now := time.Now()
	preset := &model.RobotdogPtzPreset{
		TenantID:   tenant,
		WaypointID: waypointID,
		PtzID:      ptz.ID,
		Name:       stringValue(param, "name", fmt.Sprintf("航点%d云台预置位", waypointID)),
		SortNo:     positiveInt32(gf.Int32(param["sort_no"]), 1),
		ServoPhoto: servoPhoto,
		AutoHome:   autoHome,
		Pitch:      ptrFloat64Value(realtime.Pitch),
		Yaw:        ptrFloat64Value(realtime.Yaw),
		Roll:       ptrFloat64Value(realtime.Roll),
		Zoom:       ptrFloat64Value(realtime.Zoom),
		Focus:      realtime.Focus,
		RawData:    string(rawData),
		Remark:     stringValue(param, "remark", ""),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if id := gf.Int64(param["id"]); id > 0 {
		preset.ID = id
		res := dao.DB().WithContext(ctx).Model(&model.RobotdogPtzPreset{}).Where("id = ? AND tenant_id = ?", id, tenant).Updates(presetFullUpdates(preset, now))
		if res.Error != nil {
			gf.Failed().SetMsg("更新云台预置位失败").SetData(res.Error).Regin(ctx)
			return
		}
		if res.RowsAffected == 0 {
			gf.Failed().SetMsg("预置位不存在").Regin(ctx)
			return
		}
		if err := dao.DB().WithContext(ctx).Where("id = ? AND tenant_id = ?", id, tenant).First(preset).Error; err != nil {
			gf.Failed().SetMsg("获取云台预置位失败").SetData(err).Regin(ctx)
			return
		}
		gf.Success().SetMsg("保存云台预置位成功").SetData(preset).Regin(ctx)
		return
	}
	var existing model.RobotdogPtzPreset
	if err := dao.DB().WithContext(ctx).Where("tenant_id = ? AND waypoint_id = ? AND deleted_at IS NULL", tenant, waypointID).Order("id ASC").First(&existing).Error; err == nil {
		preset.ID = existing.ID
		if err := dao.DB().WithContext(ctx).Model(&model.RobotdogPtzPreset{}).Where("id = ? AND tenant_id = ?", existing.ID, tenant).Updates(presetFullUpdates(preset, now)).Error; err != nil {
			gf.Failed().SetMsg("更新云台预置位失败").SetData(err).Regin(ctx)
			return
		}
		if err := dao.DB().WithContext(ctx).Where("id = ? AND tenant_id = ?", existing.ID, tenant).First(preset).Error; err != nil {
			gf.Failed().SetMsg("获取云台预置位失败").SetData(err).Regin(ctx)
			return
		}
		gf.Success().SetMsg("保存云台预置位成功").SetData(preset).Regin(ctx)
		return
	}
	if err := dao.DB().WithContext(ctx).Create(preset).Error; err != nil {
		gf.Failed().SetMsg("保存云台预置位失败").SetData(err).Regin(ctx)
		return
	}
	gf.Success().SetMsg("保存云台预置位成功").SetData(preset).Regin(ctx)
}

func presetFullUpdates(preset *model.RobotdogPtzPreset, now time.Time) map[string]interface{} {
	return map[string]interface{}{
		"waypoint_id":  preset.WaypointID,
		"ptz_id":       preset.PtzID,
		"name":         preset.Name,
		"sort_no":      preset.SortNo,
		"servo_photo":  preset.ServoPhoto,
		"auto_home":    preset.AutoHome,
		"pitch":        preset.Pitch,
		"yaw":          preset.Yaw,
		"roll":         preset.Roll,
		"zoom":         preset.Zoom,
		"focus_status": preset.Focus,
		"raw_data":     preset.RawData,
		"remark":       preset.Remark,
		"updated_at":   now,
		"deleted_at":   nil,
	}
}

func (api *Index) GetPtzPresetList(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	tenant := tenantID(ctx, param)
	offset, limit := pageArgs(param)
	db := dao.DB().WithContext(ctx).Model(&model.RobotdogPtzPreset{}).Where("tenant_id = ? AND deleted_at IS NULL", tenant)
	if id := gf.Int64(param["id"]); id > 0 {
		db = db.Where("id = ?", id)
	}
	if waypointID := gf.Int64(param["waypoint_id"]); waypointID > 0 {
		db = db.Where("waypoint_id = ?", waypointID)
	}
	if ptzID := gf.Int64(param["ptz_id"]); ptzID > 0 {
		db = db.Where("ptz_id = ?", ptzID)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		gf.Failed().SetMsg("获取云台预置位总数失败").SetData(err).Regin(ctx)
		return
	}
	var list []model.RobotdogPtzPreset
	if err := db.Order("sort_no ASC, id ASC").Offset(offset).Limit(limit).Find(&list).Error; err != nil {
		gf.Failed().SetMsg("获取云台预置位列表失败").SetData(err).Regin(ctx)
		return
	}
	gf.Success().SetMsg("获取云台预置位列表").SetData(map[string]interface{}{"list": list, "total": total}).Regin(ctx)
}

func (api *Index) GetPtzPresetDetail(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	id := gf.Int64(param["id"])
	if id == 0 {
		gf.Failed().SetMsg("预置位ID不能为空").Regin(ctx)
		return
	}
	var preset model.RobotdogPtzPreset
	if err := dao.DB().WithContext(ctx).Where("id = ? AND tenant_id = ? AND deleted_at IS NULL", id, tenantID(ctx, param)).First(&preset).Error; err != nil {
		gf.Failed().SetMsg("预置位不存在").SetData(err).Regin(ctx)
		return
	}
	gf.Success().SetMsg("获取云台预置位详情").SetData(preset).Regin(ctx)
}

func (api *Index) PtzUpdatePresetBase(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	id := gf.Int64(param["id"])
	if id == 0 {
		gf.Failed().SetMsg("预置位ID不能为空").Regin(ctx)
		return
	}
	if !hasParam(param, "servo_photo") || !hasParam(param, "auto_home") {
		gf.Failed().SetMsg("配置值无效").Regin(ctx)
		return
	}
	servoPhoto, ok := validBinaryInt8(param["servo_photo"])
	if !ok {
		gf.Failed().SetMsg("配置值无效").Regin(ctx)
		return
	}
	autoHome, ok := validBinaryInt8(param["auto_home"])
	if !ok {
		gf.Failed().SetMsg("配置值无效").Regin(ctx)
		return
	}
	tenant := tenantID(ctx, param)
	res := dao.DB().WithContext(ctx).Model(&model.RobotdogPtzPreset{}).Where("id = ? AND tenant_id = ? AND deleted_at IS NULL", id, tenant).Updates(map[string]interface{}{
		"servo_photo": servoPhoto,
		"auto_home":   autoHome,
		"updated_at":  time.Now(),
	})
	if res.Error != nil {
		gf.Failed().SetMsg("更新预置位基础配置失败").SetData(res.Error).Regin(ctx)
		return
	}
	if res.RowsAffected == 0 {
		gf.Failed().SetMsg("预置位不存在").Regin(ctx)
		return
	}
	var preset model.RobotdogPtzPreset
	if err := dao.DB().WithContext(ctx).Where("id = ? AND tenant_id = ?", id, tenant).First(&preset).Error; err != nil {
		gf.Failed().SetMsg("获取云台预置位失败").SetData(err).Regin(ctx)
		return
	}
	gf.Success().SetMsg("更新预置位基础配置成功").SetData(preset).Regin(ctx)
}

func (api *Index) PtzPresetDel(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	id := gf.Int64(param["id"])
	if id == 0 {
		gf.Failed().SetMsg("云台预置位ID不能为空").Regin(ctx)
		return
	}
	if err := dao.DB().WithContext(ctx).Where("id = ? AND tenant_id = ?", id, tenantID(ctx, param)).Delete(&model.RobotdogPtzPreset{}).Error; err != nil {
		gf.Failed().SetMsg("删除云台预置位失败").SetData(err).Regin(ctx)
		return
	}
	gf.Success().SetMsg("删除云台预置位成功").SetData(nil).Regin(ctx)
}

func (api *Index) GotoWaypoint(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	tenant := tenantID(ctx, param)
	dogID := gf.Int64(param["dog_id"])
	waypointID := gf.Int64(param["waypoint_id"])
	if dogID == 0 || waypointID == 0 {
		gf.Failed().SetMsg("机械狗ID和航点ID不能为空").Regin(ctx)
		return
	}
	wpDB := dao.Query().RobotdogWaypoint
	if _, err := wpDB.WithContext(ctx).Where(wpDB.TenantID.Eq(tenant), wpDB.ID.Eq(waypointID)).First(); err != nil {
		gf.Failed().SetMsg("航点不存在").SetData(err).Regin(ctx)
		return
	}
	now := time.Now()
	task := &model.RobotdogTask{
		TenantID:   tenant,
		TaskID:     newTaskID("nav"),
		DogID:      dogID,
		WaypointID: waypointID,
		Type:       "nav",
		Action:     "goto",
		Status:     "running",
		Progress:   0,
		Message:    "goto waypoint accepted",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := dao.Query().RobotdogTask.WithContext(ctx).Create(task); err != nil {
		gf.Failed().SetMsg("创建导航任务失败").SetData(err).Regin(ctx)
		return
	}
	gf.Success().SetMsg("导航任务已创建").SetData(task).Regin(ctx)
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

func ptzTarget(ptz *model.RobotdogPtz) internalcontrol.PTZTarget {
	return internalcontrol.PTZTarget{
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

func photoOptions(param map[string]interface{}) internalcontrol.PhotoOptions {
	folder := stringValue(param, "folder", "robotdog_ptz")
	filename := stringValue(param, "filename", "")
	if filename == "" {
		filename = "ptz_" + time.Now().Format("20060102150405") + ".jpg"
	}
	return internalcontrol.PhotoOptions{
		Mode:     stringValue(param, "mode", "default"),
		Folder:   folder,
		Filename: filename,
	}
}

func photoResponse(photo *model.RobotdogPtzPhoto) map[string]interface{} {
	filePath := "/" + strings.TrimLeft(photo.FilePath, "/")
	return map[string]interface{}{
		"id":          photo.ID,
		"tenant_id":   photo.TenantID,
		"waypoint_id": photo.WaypointID,
		"ptz_id":      photo.PtzID,
		"filename":    photo.Filename,
		"file_path":   filePath,
		"url":         gf.GetFullUrl(filePath),
		"mode":        photo.Mode,
		"raw_data":    photo.RawData,
		"created_at":  photo.CreatedAt,
	}
}

func photoFilePath(folder string, filename string) string {
	folder = strings.Trim(folder, "/")
	filename = strings.Trim(filename, "/")
	if folder == "" {
		folder = "robotdog_ptz"
	}
	return "resource/uploads/" + folder + "/" + filename
}

func positiveInt32(v int32, fallback int32) int32 {
	if v > 0 {
		return v
	}
	return fallback
}

func optionalBinaryInt8(param map[string]interface{}, key string) (int8, bool) {
	if !hasParam(param, key) {
		return 0, true
	}
	return validBinaryInt8(param[key])
}

func validBinaryInt8(v interface{}) (int8, bool) {
	n := gf.Int8(v)
	if n == 0 || n == 1 {
		return n, true
	}
	return 0, false
}

func ptrFloat64Value(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
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

func (api *Index) RunRoute(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	tenant := tenantID(ctx, param)
	routeID := gf.Int64(param["route_id"])
	action := stringValue(param, "action", "start")
	if routeID == 0 {
		gf.Failed().SetMsg("航线ID不能为空").Regin(ctx)
		return
	}
	routeDB := dao.Query().RobotdogRoute
	route, err := routeDB.WithContext(ctx).Where(routeDB.TenantID.Eq(tenant), routeDB.ID.Eq(routeID)).First()
	if err != nil {
		gf.Failed().SetMsg("航线不存在").SetData(err).Regin(ctx)
		return
	}
	runStatus := "running"
	taskStatus := "running"
	if action == "stop" {
		runStatus = "idle"
		taskStatus = "stopped"
	} else if action == "pause" {
		runStatus = "running"
		taskStatus = "paused"
	} else if action == "complete" {
		runStatus = "done"
		taskStatus = "done"
	}
	now := time.Now()
	task := &model.RobotdogTask{
		TenantID:  tenant,
		TaskID:    newTaskID("route"),
		DogID:     route.DogID,
		RouteID:   route.ID,
		Type:      "route",
		Action:    action,
		Status:    taskStatus,
		Progress:  0,
		Message:   "route command accepted",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if taskStatus == "done" {
		task.Progress = 100
	}
	if action == "stop" || action == "pause" || action == "complete" {
		updates := map[string]interface{}{
			"status":     taskStatus,
			"message":    "route command " + action,
			"updated_at": now,
		}
		if taskStatus == "done" {
			updates["progress"] = 100
		}
		if _, err := dao.Query().RobotdogTask.WithContext(ctx).Where(
			dao.Query().RobotdogTask.TenantID.Eq(tenant),
			dao.Query().RobotdogTask.RouteID.Eq(route.ID),
			dao.Query().RobotdogTask.Type.Eq("route"),
			dao.Query().RobotdogTask.Status.Eq("running"),
		).Updates(updates); err != nil {
			gf.Failed().SetMsg("更新运行中航线任务失败").SetData(err).Regin(ctx)
			return
		}
	}
	if err := dao.Query().RobotdogTask.WithContext(ctx).Create(task); err != nil {
		gf.Failed().SetMsg("创建航线任务失败").SetData(err).Regin(ctx)
		return
	}
	if _, err := routeDB.WithContext(ctx).Where(routeDB.TenantID.Eq(tenant), routeDB.ID.Eq(route.ID)).Updates(map[string]interface{}{"run_status": runStatus, "updated_at": now}); err != nil {
		gf.Failed().SetMsg("更新航线执行状态失败").SetData(err).Regin(ctx)
		return
	}
	gf.Success().SetMsg("航线命令已接收").SetData(task).Regin(ctx)
}

func (api *Index) GetTaskStatus(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	taskDB := dao.Query().RobotdogTask
	tenant := tenantID(ctx, param)
	query := taskDB.WithContext(ctx).Where(taskDB.TenantID.Eq(tenant))
	if taskID := stringValue(param, "task_id", ""); taskID != "" {
		query = query.Where(taskDB.TaskID.Eq(taskID))
	} else {
		if routeID := gf.Int64(param["route_id"]); routeID > 0 {
			query = query.Where(taskDB.RouteID.Eq(routeID))
		}
		if dogID := gf.Int64(param["dog_id"]); dogID > 0 {
			query = query.Where(taskDB.DogID.Eq(dogID))
		}
	}
	task, err := query.Order(taskDB.ID.Desc()).First()
	if err != nil {
		gf.Failed().SetMsg("获取任务状态失败").SetData(err).Regin(ctx)
		return
	}
	gf.Success().SetMsg("获取任务状态").SetData(task).Regin(ctx)
}
