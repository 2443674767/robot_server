package preset

import (
	"fmt"
	"strings"
	"time"

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
