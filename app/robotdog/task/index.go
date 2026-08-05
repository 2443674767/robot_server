package task

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"gofly/dao"
	"gofly/dao/model"
	"gofly/utils/gf"
)

type Index struct{ NoNeedAuths []string }

func init() {
	gf.Register(&Index{})
}

type routeStep struct {
	ID         int64
	Seq        int32
	Action     string
	WaitSec    float64
	Params     map[string]interface{}
	ParamsJSON string
}

func (api *Index) StartRoute(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	tenant := tenantID(ctx, param)
	routeID := gf.Int64(param["route_id"])
	if routeID == 0 {
		routeID = gf.Int64(param["id"])
	}
	if routeID == 0 {
		gf.Failed().SetMsg("航线ID不能为空").Regin(ctx)
		return
	}
	route, steps, err := loadExecutableRoute(ctx.Request.Context(), tenant, routeID)
	if err != nil {
		gf.Failed().SetMsg(err.Error()).Regin(ctx)
		return
	}
	if err := ensureRouteStartCondition(ctx.Request.Context(), tenant, route); err != nil {
		gf.Failed().SetMsg(err.Error()).Regin(ctx)
		return
	}

	now := time.Now()
	taskID := "route_" + uuid.NewString()
	task := &model.RobotdogTask{
		TenantID:   tenant,
		TaskID:     taskID,
		DogID:      route.DogID,
		RouteID:    route.ID,
		WaypointID: 0,
		Type:       "route",
		Action:     "execute_route",
		Status:     "running",
		Progress:   0,
		Message:    "航线任务已创建，等待执行",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := dao.Query().RobotdogTask.WithContext(ctx).Create(task); err != nil {
		gf.Failed().SetMsg("创建航线执行任务失败").SetData(err).Regin(ctx)
		return
	}
	if _, err := dao.Query().RobotdogRoute.WithContext(ctx).Where(dao.Query().RobotdogRoute.ID.Eq(route.ID), dao.Query().RobotdogRoute.TenantID.Eq(tenant)).Updates(map[string]interface{}{"run_status": "running", "updated_at": now}); err != nil {
		gf.Failed().SetMsg("更新航线执行状态失败").SetData(err).Regin(ctx)
		return
	}

	go executeRouteTask(context.Background(), tenant, taskID, route, steps)

	gf.Success().SetMsg("航线执行任务已启动").SetData(map[string]interface{}{
		"task_id":    taskID,
		"route_id":   route.ID,
		"dog_id":     route.DogID,
		"status":     "running",
		"step_count": len(steps),
	}).Regin(ctx)
}

func (api *Index) GetStatus(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	tenant := tenantID(ctx, param)
	taskID := strings.TrimSpace(gf.String(param["task_id"]))
	taskDB := dao.Query().RobotdogTask
	var task *model.RobotdogTask
	var err error
	if taskID != "" {
		task, err = taskDB.WithContext(ctx).Where(taskDB.TenantID.Eq(tenant), taskDB.TaskID.Eq(taskID)).First()
	} else {
		routeID := gf.Int64(param["route_id"])
		if routeID == 0 {
			routeID = gf.Int64(param["id"])
		}
		if routeID == 0 {
			gf.Failed().SetMsg("task_id或route_id不能为空").Regin(ctx)
			return
		}
		task, err = taskDB.WithContext(ctx).Where(taskDB.TenantID.Eq(tenant), taskDB.RouteID.Eq(routeID), taskDB.Type.Eq("route")).Order(taskDB.ID.Desc()).First()
	}
	if err != nil {
		gf.Failed().SetMsg("获取航线执行任务失败").SetData(err).Regin(ctx)
		return
	}
	gf.Success().SetMsg("获取航线执行任务状态").SetData(task).Regin(ctx)
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

func loadExecutableRoute(ctx context.Context, tenant int32, routeID int64) (*model.RobotdogRoute, []routeStep, error) {
	routeDB := dao.Query().RobotdogRoute
	route, err := routeDB.WithContext(ctx).Where(routeDB.ID.Eq(routeID), routeDB.TenantID.Eq(tenant)).First()
	if err != nil {
		return nil, nil, fmt.Errorf("航线不存在")
	}
	taskDB := dao.Query().RobotdogRouteTask
	rows, err := taskDB.WithContext(ctx).Where(taskDB.TenantID.Eq(tenant), taskDB.RouteID.Eq(routeID)).Order(taskDB.Seq.Asc(), taskDB.ID.Asc()).Find()
	if err != nil {
		return nil, nil, fmt.Errorf("获取航线子任务失败: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil, fmt.Errorf("航线没有子任务，不能执行")
	}
	steps := make([]routeStep, 0, len(rows))
	for _, row := range rows {
		params := map[string]interface{}{}
		if strings.TrimSpace(row.Params) != "" {
			_ = json.Unmarshal([]byte(row.Params), &params)
		}
		steps = append(steps, routeStep{ID: row.ID, Seq: row.Seq, Action: row.Action, WaitSec: row.WaitSec, Params: params, ParamsJSON: row.Params})
	}
	return route, steps, nil
}

func ensureRouteStartCondition(ctx context.Context, tenant int32, route *model.RobotdogRoute) error {
	if route.DogID <= 0 {
		return fmt.Errorf("航线未绑定机械狗，不能执行")
	}
	if route.RunStatus == "running" {
		return fmt.Errorf("航线正在执行中")
	}
	taskDB := dao.Query().RobotdogTask
	count, err := taskDB.WithContext(ctx).Where(taskDB.TenantID.Eq(tenant), taskDB.RouteID.Eq(route.ID), taskDB.Type.Eq("route"), taskDB.Status.Eq("running")).Count()
	if err != nil {
		return fmt.Errorf("校验任务执行状态失败: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("当前航线已有运行中的任务")
	}
	return nil
}

func executeRouteTask(ctx context.Context, tenant int32, taskID string, route *model.RobotdogRoute, steps []routeStep) {
	cfg := loadTaskConfig()
	status := "done"
	message := "航线任务执行完成"
	for i, step := range steps {
		checkResult, err := ensureStepStartCondition(ctx, tenant, route, step, cfg)
		if err != nil {
			status = "failed"
			message = err.Error()
			updateTaskStatus(ctx, tenant, taskID, status, progress(i, len(steps)), message)
			break
		}
		if checkResult.Skip {
			updateTaskStatus(ctx, tenant, taskID, "running", progress(i+1, len(steps)), checkResult.Message)
			continue
		}
		updateTaskStatus(ctx, tenant, taskID, "running", progress(i, len(steps)), fmt.Sprintf("正在执行第%d个子任务:%s", step.Seq, step.Action))
		if err := executeStep(ctx, tenant, route, step, cfg); err != nil {
			status = "failed"
			message = err.Error()
			updateTaskStatus(ctx, tenant, taskID, status, progress(i, len(steps)), message)
			break
		}
		if err := ensureStepEndCondition(ctx, tenant, route, step, cfg); err != nil {
			status = "failed"
			message = err.Error()
			updateTaskStatus(ctx, tenant, taskID, status, progress(i+1, len(steps)), message)
			break
		}
		if step.WaitSec > 0 {
			time.Sleep(time.Duration(step.WaitSec * float64(time.Second)))
		}
		updateTaskStatus(ctx, tenant, taskID, "running", progress(i+1, len(steps)), fmt.Sprintf("第%d个子任务执行完成:%s", step.Seq, step.Action))
	}
	if status == "done" {
		updateTaskStatus(ctx, tenant, taskID, "done", 100, message)
	}
	runStatus := "done"
	if status == "failed" {
		runStatus = "idle"
	}
	routeDB := dao.Query().RobotdogRoute
	_, _ = routeDB.WithContext(ctx).Where(routeDB.ID.Eq(route.ID), routeDB.TenantID.Eq(tenant)).Updates(map[string]interface{}{"run_status": runStatus, "updated_at": time.Now()})
}

type stepCheckResult struct {
	Skip    bool
	Message string
}

func ensureStepStartCondition(ctx context.Context, tenant int32, route *model.RobotdogRoute, step routeStep, cfg robotdogTaskConfig) (stepCheckResult, error) {
	if route.DogID <= 0 {
		return stepCheckResult{}, fmt.Errorf("第%d个子任务执行前校验失败:航线未绑定机械狗", step.Seq)
	}
	if step.Action == "" {
		return stepCheckResult{}, fmt.Errorf("第%d个子任务执行前校验失败:动作不能为空", step.Seq)
	}
	status, err := fetchDogStatus(ctx, cfg)
	if err != nil {
		return stepCheckResult{}, fmt.Errorf("第%d个子任务执行前获取机械狗状态失败:%w", step.Seq, err)
	}
	if strings.Contains(status.ControlStatus, "导航模式请重定位初始化") && step.Action != "relocalize" {
		return stepCheckResult{}, fmt.Errorf("第%d个子任务执行前校验失败:需要先重定位初始化", step.Seq)
	}
	if step.Action == "navigate" {
		waypointID := gf.Int64(step.Params["waypoint_id"])
		if waypointID <= 0 {
			return stepCheckResult{}, fmt.Errorf("第%d个navigate子任务缺少params.waypoint_id", step.Seq)
		}
		wpDB := dao.Query().RobotdogWaypoint
		if _, err := wpDB.WithContext(ctx).Where(wpDB.TenantID.Eq(tenant), wpDB.ID.Eq(waypointID)).First(); err != nil {
			return stepCheckResult{}, fmt.Errorf("第%d个navigate子任务航点不存在:%d", step.Seq, waypointID)
		}
		result, err := waitNavigateReady(ctx, cfg, step, status)
		if err != nil {
			return stepCheckResult{}, err
		}
		return result, nil
	}
	return stepCheckResult{}, nil
}

func waitNavigateReady(ctx context.Context, cfg robotdogTaskConfig, step routeStep, firstStatus *dogStatus) (stepCheckResult, error) {
	start := time.Now()
	current := firstStatus
	for {
		controlStatus := strings.TrimSpace(current.ControlStatus)
		switch {
		case controlStatus == "导航模式 空闲" || controlStatus == "":
			return stepCheckResult{}, nil
		case strings.Contains(controlStatus, "导航模式请重定位初始化"):
			return stepCheckResult{}, fmt.Errorf("第%d个navigate子任务执行前校验失败:需要先重定位初始化", step.Seq)
		case strings.Contains(controlStatus, "导航模式 对准中"):
			time.Sleep(cfg.alignWait())
			return stepCheckResult{Skip: true, Message: fmt.Sprintf("第%d个navigate子任务处于导航模式 对准中，等待%d秒后跳过该航点", step.Seq, cfg.AlignWaitSec)}, nil
		case strings.Contains(controlStatus, "导航模式 进度："), strings.Contains(controlStatus, "导航模式 进度:"), strings.Contains(controlStatus, "导航模式 避障中"):
			if time.Since(start) >= cfg.maxWait() {
				return stepCheckResult{}, fmt.Errorf("第%d个navigate子任务等待机械狗空闲超时，当前状态:%s", step.Seq, controlStatus)
			}
			time.Sleep(cfg.pollInterval())
			next, err := fetchDogStatus(ctx, cfg)
			if err != nil {
				return stepCheckResult{}, fmt.Errorf("第%d个navigate子任务重新获取机械狗状态失败:%w", step.Seq, err)
			}
			current = next
		default:
			return stepCheckResult{}, fmt.Errorf("第%d个navigate子任务执行前校验失败:机械狗当前状态不允许导航:%s", step.Seq, controlStatus)
		}
	}
}

func executeStep(ctx context.Context, tenant int32, route *model.RobotdogRoute, step routeStep, cfg robotdogTaskConfig) error {
	switch step.Action {
	case "navigate":
		waypointID := gf.Int64(step.Params["waypoint_id"])
		waypoint, err := dao.Query().RobotdogWaypoint.WithContext(ctx).Where(dao.Query().RobotdogWaypoint.TenantID.Eq(tenant), dao.Query().RobotdogWaypoint.ID.Eq(waypointID)).First()
		if err != nil {
			return fmt.Errorf("第%d个navigate子任务航点不存在:%d", step.Seq, waypointID)
		}
		payload := navCustomRequest{
			InputValue:   "null",
			OrientationW: waypoint.OrientationW,
			OrientationX: waypoint.OrientationX,
			OrientationY: waypoint.OrientationY,
			OrientationZ: waypoint.OrientationZ,
			PositionX:    waypoint.X,
			PositionY:    waypoint.Y,
			PositionZ:    waypoint.Z,
		}
		resp, err := postNavCustom(ctx, cfg, payload)
		if err != nil {
			return fmt.Errorf("第%d个navigate子任务发送导航目标失败:%w", step.Seq, err)
		}
		if resp != nil && strings.TrimSpace(resp.Msg) != "" {
			return nil
		}
		return nil
	case "relocalize":
		pointID := stepParamID(step.Params, "id", "point_id", "location_id", "nav_id")
		if pointID <= 0 {
			return fmt.Errorf("第%d个relocalize子任务缺少params.id", step.Seq)
		}
		if _, err := postResetLocation(ctx, cfg, pointID); err != nil {
			return fmt.Errorf("第%d个relocalize子任务发送重定位失败:%w", step.Seq, err)
		}
		return nil
	case "lie", "stand", "line_navigate", "photo", "switch_map", "voice":
		return nil
	default:
		return fmt.Errorf("第%d个子任务动作不支持:%s", step.Seq, step.Action)
	}
}

func stepParamID(params map[string]interface{}, names ...string) int64 {
	for _, name := range names {
		if id := gf.Int64(params[name]); id > 0 {
			return id
		}
	}
	return 0
}

func ensureStepEndCondition(ctx context.Context, tenant int32, route *model.RobotdogRoute, step routeStep, cfg robotdogTaskConfig) error {
	if step.Action != "navigate" {
		return nil
	}
	start := time.Now()
	for {
		status, err := fetchDogStatus(ctx, cfg)
		if err != nil {
			return fmt.Errorf("第%d个navigate子任务结束检查获取机械狗状态失败:%w", step.Seq, err)
		}
		controlStatus := strings.TrimSpace(status.ControlStatus)
		switch {
		case controlStatus == "导航模式 空闲" || controlStatus == "":
			return nil
		case strings.Contains(controlStatus, "导航模式请重定位初始化"):
			return fmt.Errorf("第%d个navigate子任务结束检查失败:需要先重定位初始化", step.Seq)
		case strings.Contains(controlStatus, "导航模式 进度："), strings.Contains(controlStatus, "导航模式 进度:"), strings.Contains(controlStatus, "导航模式 避障中"), strings.Contains(controlStatus, "导航模式 对准中"):
			if time.Since(start) >= cfg.maxWait() {
				return fmt.Errorf("第%d个navigate子任务等待导航结束超时，当前状态:%s", step.Seq, controlStatus)
			}
			time.Sleep(cfg.pollInterval())
		default:
			return fmt.Errorf("第%d个navigate子任务结束检查失败:机械狗当前状态异常:%s", step.Seq, controlStatus)
		}
	}
	return nil
}

func updateTaskStatus(ctx context.Context, tenant int32, taskID string, status string, progress int32, message string) {
	taskDB := dao.Query().RobotdogTask
	_, _ = taskDB.WithContext(ctx).Where(taskDB.TenantID.Eq(tenant), taskDB.TaskID.Eq(taskID)).Updates(map[string]interface{}{
		"status":     status,
		"progress":   progress,
		"message":    trimMessage(message),
		"updated_at": time.Now(),
	})
}

func progress(done int, total int) int32 {
	if total <= 0 {
		return 0
	}
	v := int32(done * 100 / total)
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func trimMessage(message string) string {
	message = strings.TrimSpace(message)
	if len([]rune(message)) <= 250 {
		return message
	}
	return string([]rune(message)[:250])
}
