package waypoint

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gofly/dao"
	"gofly/dao/model"
	"gofly/utils/gf"

	"gorm.io/gorm"
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

func idList(v interface{}) []int64 {
	ids := gf.InterfaceToInt64(v)
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id > 0 {
			out = append(out, id)
		}
	}
	return out
}

func itemsFromParam(param map[string]interface{}, keys ...string) []interface{} {
	for _, key := range keys {
		if items := gf.Interfaces(param[key]); len(items) > 0 {
			return items
		}
	}
	return nil
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
			"status":       route.Status,
			"run_status":   route.RunStatus,
			"remark":       route.Remark,
			"waypoint_ids": wpMap[route.ID],
			"created_at":   route.CreatedAt,
			"updated_at":   route.UpdatedAt,
		})
	}
	return list
}

func (api *Index) GetList(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	dogDB := dao.Query().RobotdogDog
	tenant := tenantID(ctx, param)
	where := []dao.Condition{dogDB.TenantID.Eq(tenant)}
	if name := stringValue(param, "name", ""); name != "" {
		where = append(where, dogDB.Name.Like("%"+name+"%"))
	}
	if status := stringValue(param, "status", ""); status != "" {
		where = append(where, dogDB.Status.Eq(status))
	}
	offset, limit := pageArgs(param)
	list, total, err := dogDB.WithContext(ctx).Where(where...).Order(dogDB.ID.Desc()).FindByPage(offset, limit)
	if err != nil {
		gf.Failed().SetMsg("获取机械狗列表失败").SetData(err).Regin(ctx)
		return
	}
	gf.Success().SetMsg("获取机械狗列表").SetData(map[string]interface{}{"list": list, "total": total}).Regin(ctx)
}

func (api *Index) Save(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	dogDB := dao.Query().RobotdogDog
	tenant := tenantID(ctx, param)
	id := gf.Int64(param["id"])
	now := time.Now()
	if id == 0 {
		dog := &model.RobotdogDog{
			TenantID:  tenant,
			Name:      stringValue(param, "name", ""),
			Sn:        stringValue(param, "sn", ""),
			Model:     stringValue(param, "model", ""),
			Status:    stringValue(param, "status", "online"),
			MaxSpeed:  gf.Float64(param["max_speed"]),
			Battery:   gf.Int32(param["battery"]),
			StreamURL: stringValue(param, "stream_url", ""),
			RtspURL:   stringValue(param, "rtsp_url", ""),
			MapID:     gf.Int64(param["map_id"]),
			Remark:    stringValue(param, "remark", ""),
			CreatedAt: now,
			UpdatedAt: now,
		}
		if dog.Name == "" {
			gf.Failed().SetMsg("机械狗名称不能为空").Regin(ctx)
			return
		}
		if err := dogDB.WithContext(ctx).Create(dog); err != nil {
			gf.Failed().SetMsg("添加机械狗失败").SetData(err).Regin(ctx)
			return
		}
		gf.Success().SetMsg("添加机械狗成功").SetData(dog.ID).Regin(ctx)
		return
	}
	updates := map[string]interface{}{
		"name":       stringValue(param, "name", ""),
		"sn":         stringValue(param, "sn", ""),
		"model":      stringValue(param, "model", ""),
		"status":     stringValue(param, "status", "online"),
		"max_speed":  gf.Float64(param["max_speed"]),
		"battery":    gf.Int32(param["battery"]),
		"stream_url": stringValue(param, "stream_url", ""),
		"rtsp_url":   stringValue(param, "rtsp_url", ""),
		"map_id":     gf.Int64(param["map_id"]),
		"remark":     stringValue(param, "remark", ""),
		"updated_at": now,
	}
	res, err := dogDB.WithContext(ctx).Where(dogDB.ID.Eq(id), dogDB.TenantID.Eq(tenant)).Updates(updates)
	if err != nil {
		gf.Failed().SetMsg("更新机械狗失败").SetData(err).Regin(ctx)
		return
	}
	gf.Success().SetMsg("更新机械狗成功").SetData(res).Regin(ctx)
}

func (api *Index) Del(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	dogDB := dao.Query().RobotdogDog
	tenant := tenantID(ctx, param)
	ids := idList(param["ids"])
	if len(ids) == 0 {
		ids = idList(param["id"])
	}
	if len(ids) == 0 {
		gf.Failed().SetMsg("请选择要删除的机械狗").Regin(ctx)
		return
	}
	res, err := dogDB.WithContext(ctx).Where(dogDB.ID.In(ids...), dogDB.TenantID.Eq(tenant)).Delete()
	if err != nil {
		gf.Failed().SetMsg("删除机械狗失败").SetData(err).Regin(ctx)
		return
	}
	gf.Success().SetMsg("删除机械狗成功").SetData(res).Regin(ctx)
}

func (api *Index) GetDetail(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	dogDB := dao.Query().RobotdogDog
	dog, err := dogDB.WithContext(ctx).Where(dogDB.ID.Eq(gf.Int64(param["id"])), dogDB.TenantID.Eq(tenantID(ctx, param))).First()
	if err != nil {
		gf.Failed().SetMsg("获取机械狗详情失败").SetData(err).Regin(ctx)
		return
	}
	gf.Success().SetMsg("获取机械狗详情").SetData(dog).Regin(ctx)
}

func (api *Index) GetWaypointList(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	wpDB := dao.Query().RobotdogWaypoint
	tenant := tenantID(ctx, param)
	where := []dao.Condition{wpDB.TenantID.Eq(tenant)}
	if mapID := gf.Int64(param["map_id"]); mapID > 0 {
		where = append(where, wpDB.MapID.Eq(mapID))
	}
	if name := stringValue(param, "name", ""); name != "" {
		where = append(where, wpDB.Name.Like("%"+name+"%"))
	}
	offset, limit := pageArgs(param)
	list, total, err := wpDB.WithContext(ctx).Where(where...).Order(wpDB.ID.Desc()).FindByPage(offset, limit)
	if err != nil {
		gf.Failed().SetMsg("获取航点列表失败").SetData(err).Regin(ctx)
		return
	}
	gf.Success().SetMsg("获取航点列表").SetData(map[string]interface{}{"list": list, "total": total}).Regin(ctx)
}

func (api *Index) SaveWaypoint(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	wpDB := dao.Query().RobotdogWaypoint
	tenant := tenantID(ctx, param)
	id := gf.Int64(param["id"])
	now := time.Now()
	if id == 0 {
		wp := &model.RobotdogWaypoint{
			TenantID:  tenant,
			MapID:     gf.Int64(param["map_id"]),
			Name:      stringValue(param, "name", ""),
			X:         gf.Float64(param["x"]),
			Y:         gf.Float64(param["y"]),
			Z:         gf.Float64(param["z"]),
			Yaw:       gf.Float64(param["yaw"]),
			Remark:    stringValue(param, "remark", ""),
			CreatedAt: now,
			UpdatedAt: now,
		}
		if wp.Name == "" {
			gf.Failed().SetMsg("航点名称不能为空").Regin(ctx)
			return
		}
		if err := wpDB.WithContext(ctx).Create(wp); err != nil {
			gf.Failed().SetMsg("添加航点失败").SetData(err).Regin(ctx)
			return
		}
		gf.Success().SetMsg("添加航点成功").SetData(wp.ID).Regin(ctx)
		return
	}
	updates := map[string]interface{}{
		"map_id":     gf.Int64(param["map_id"]),
		"name":       stringValue(param, "name", ""),
		"x":          gf.Float64(param["x"]),
		"y":          gf.Float64(param["y"]),
		"z":          gf.Float64(param["z"]),
		"yaw":        gf.Float64(param["yaw"]),
		"remark":     stringValue(param, "remark", ""),
		"updated_at": now,
	}
	res, err := wpDB.WithContext(ctx).Where(wpDB.ID.Eq(id), wpDB.TenantID.Eq(tenant)).Updates(updates)
	if err != nil {
		gf.Failed().SetMsg("更新航点失败").SetData(err).Regin(ctx)
		return
	}
	gf.Success().SetMsg("更新航点成功").SetData(res).Regin(ctx)
}

func (api *Index) DelWaypoint(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	wpDB := dao.Query().RobotdogWaypoint
	rwDB := dao.Query().RobotdogRouteWaypoint
	tenant := tenantID(ctx, param)
	ids := idList(param["ids"])
	if len(ids) == 0 {
		ids = idList(param["id"])
	}
	if len(ids) == 0 {
		gf.Failed().SetMsg("请选择要删除的航点").Regin(ctx)
		return
	}
	count, err := rwDB.WithContext(ctx).Where(rwDB.TenantID.Eq(tenant), rwDB.WaypointID.In(ids...)).Count()
	if err != nil {
		gf.Failed().SetMsg("校验航点引用失败").SetData(err).Regin(ctx)
		return
	}
	if count > 0 {
		gf.Failed().SetMsg("航点已被航线引用，不能删除").Regin(ctx)
		return
	}
	res, err := wpDB.WithContext(ctx).Where(wpDB.TenantID.Eq(tenant), wpDB.ID.In(ids...)).Delete()
	if err != nil {
		gf.Failed().SetMsg("删除航点失败").SetData(err).Regin(ctx)
		return
	}
	gf.Success().SetMsg("删除航点成功").SetData(res).Regin(ctx)
}

func (api *Index) ImportWaypoint(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	tenant := tenantID(ctx, param)
	items := itemsFromParam(param, "items", "list", "waypoints")
	if len(items) == 0 {
		gf.Failed().SetMsg("请传入航点数组").Regin(ctx)
		return
	}
	now := time.Now()
	points := make([]*model.RobotdogWaypoint, 0, len(items))
	for i, item := range items {
		row, ok := item.(map[string]interface{})
		if !ok {
			gf.Failed().SetMsg(fmt.Sprintf("第%d个航点格式错误", i+1)).Regin(ctx)
			return
		}
		name := stringValue(row, "name", fmt.Sprintf("航点%d", i+1))
		points = append(points, &model.RobotdogWaypoint{
			TenantID:  tenant,
			MapID:     gf.Int64(row["map_id"]),
			Name:      name,
			X:         gf.Float64(row["x"]),
			Y:         gf.Float64(row["y"]),
			Z:         gf.Float64(row["z"]),
			Yaw:       gf.Float64(row["yaw"]),
			Remark:    stringValue(row, "remark", ""),
			CreatedAt: now,
			UpdatedAt: now,
		})
	}
	if err := dao.Query().RobotdogWaypoint.WithContext(ctx).CreateInBatches(points, 100); err != nil {
		gf.Failed().SetMsg("导入航点失败").SetData(err).Regin(ctx)
		return
	}
	gf.Success().SetMsg("导入航点成功").SetData(map[string]interface{}{"count": len(points)}).Regin(ctx)
}

func (api *Index) GetRouteList(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	routeDB := dao.Query().RobotdogRoute
	tenant := tenantID(ctx, param)
	where := []dao.Condition{routeDB.TenantID.Eq(tenant)}
	if dogID := gf.Int64(param["dog_id"]); dogID > 0 {
		where = append(where, routeDB.DogID.Eq(dogID))
	}
	if name := stringValue(param, "name", ""); name != "" {
		where = append(where, routeDB.Name.Like("%"+name+"%"))
	}
	if status := stringValue(param, "status", ""); status != "" {
		where = append(where, routeDB.Status.Eq(status))
	}
	offset, limit := pageArgs(param)
	routes, total, err := routeDB.WithContext(ctx).Where(where...).Order(routeDB.ID.Desc()).FindByPage(offset, limit)
	if err != nil {
		gf.Failed().SetMsg("获取航线列表失败").SetData(err).Regin(ctx)
		return
	}
	gf.Success().SetMsg("获取航线列表").SetData(map[string]interface{}{"list": routeListData(ctx, tenant, routes), "total": total}).Regin(ctx)
}

func (api *Index) SaveRoute(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	tenant := tenantID(ctx, param)
	routeID := gf.Int64(param["id"])
	waypointIDs := idList(param["waypoint_ids"])
	if len(waypointIDs) == 0 {
		waypointIDs = idList(param["waypoints"])
	}
	if len(waypointIDs) == 0 {
		gf.Failed().SetMsg("航线至少需要一个航点").Regin(ctx)
		return
	}
	name := stringValue(param, "name", "")
	if name == "" {
		gf.Failed().SetMsg("航线名称不能为空").Regin(ctx)
		return
	}
	now := time.Now()
	err := dao.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if routeID == 0 {
			route := &model.RobotdogRoute{
				TenantID:  tenant,
				DogID:     gf.Int64(param["dog_id"]),
				Name:      name,
				Status:    stringValue(param, "status", "draft"),
				RunStatus: stringValue(param, "run_status", "idle"),
				Remark:    stringValue(param, "remark", ""),
				CreatedAt: now,
				UpdatedAt: now,
			}
			if err := tx.Create(route).Error; err != nil {
				return err
			}
			routeID = route.ID
		} else {
			updates := map[string]interface{}{
				"dog_id":     gf.Int64(param["dog_id"]),
				"name":       name,
				"status":     stringValue(param, "status", "draft"),
				"run_status": stringValue(param, "run_status", "idle"),
				"remark":     stringValue(param, "remark", ""),
				"updated_at": now,
			}
			if err := tx.Model(&model.RobotdogRoute{}).Where("id = ? AND tenant_id = ?", routeID, tenant).Updates(updates).Error; err != nil {
				return err
			}
			if err := tx.Where("route_id = ? AND tenant_id = ?", routeID, tenant).Delete(&model.RobotdogRouteWaypoint{}).Error; err != nil {
				return err
			}
		}
		relations := make([]*model.RobotdogRouteWaypoint, 0, len(waypointIDs))
		for i, waypointID := range waypointIDs {
			relations = append(relations, &model.RobotdogRouteWaypoint{
				TenantID:   tenant,
				RouteID:    routeID,
				WaypointID: waypointID,
				Weigh:      int32(i + 1),
				CreatedAt:  now,
				UpdatedAt:  now,
			})
		}
		return tx.Create(&relations).Error
	})
	if err != nil {
		gf.Failed().SetMsg("保存航线失败").SetData(err).Regin(ctx)
		return
	}
	gf.Success().SetMsg("保存航线成功").SetData(routeID).Regin(ctx)
}

func (api *Index) PublishRoute(ctx *gf.GinCtx) {
	api.updateRouteStatus(ctx, "published", "发布航线成功")
}

func (api *Index) UnpublishRoute(ctx *gf.GinCtx) {
	api.updateRouteStatus(ctx, "draft", "取消发布航线成功")
}

func (api *Index) updateRouteStatus(ctx *gf.GinCtx, status string, msg string) {
	param, _ := gf.RequestParam(ctx)
	tenant := tenantID(ctx, param)
	routeID := gf.Int64(param["id"])
	if routeID == 0 {
		gf.Failed().SetMsg("航线ID不能为空").Regin(ctx)
		return
	}
	if status == "published" {
		rwDB := dao.Query().RobotdogRouteWaypoint
		count, err := rwDB.WithContext(ctx).Where(rwDB.TenantID.Eq(tenant), rwDB.RouteID.Eq(routeID)).Count()
		if err != nil {
			gf.Failed().SetMsg("校验航线航点失败").SetData(err).Regin(ctx)
			return
		}
		if count == 0 {
			gf.Failed().SetMsg("航线没有航点，不能发布").Regin(ctx)
			return
		}
	}
	routeDB := dao.Query().RobotdogRoute
	res, err := routeDB.WithContext(ctx).Where(routeDB.ID.Eq(routeID), routeDB.TenantID.Eq(tenant)).Updates(map[string]interface{}{"status": status, "updated_at": time.Now()})
	if err != nil {
		gf.Failed().SetMsg("更新航线状态失败").SetData(err).Regin(ctx)
		return
	}
	gf.Success().SetMsg(msg).SetData(res).Regin(ctx)
}

func (api *Index) DelRoute(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	tenant := tenantID(ctx, param)
	ids := idList(param["ids"])
	if len(ids) == 0 {
		ids = idList(param["id"])
	}
	if len(ids) == 0 {
		gf.Failed().SetMsg("请选择要删除的航线").Regin(ctx)
		return
	}
	err := dao.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("route_id IN ? AND tenant_id = ?", ids, tenant).Delete(&model.RobotdogRouteWaypoint{}).Error; err != nil {
			return err
		}
		return tx.Where("id IN ? AND tenant_id = ?", ids, tenant).Delete(&model.RobotdogRoute{}).Error
	})
	if err != nil {
		gf.Failed().SetMsg("删除航线失败").SetData(err).Regin(ctx)
		return
	}
	gf.Success().SetMsg("删除航线成功").SetData(true).Regin(ctx)
}

func (api *Index) GetRouteDetail(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	tenant := tenantID(ctx, param)
	routeDB := dao.Query().RobotdogRoute
	route, err := routeDB.WithContext(ctx).Where(routeDB.ID.Eq(gf.Int64(param["id"])), routeDB.TenantID.Eq(tenant)).First()
	if err != nil {
		gf.Failed().SetMsg("获取航线详情失败").SetData(err).Regin(ctx)
		return
	}
	rwDB := dao.Query().RobotdogRouteWaypoint
	relations, err := rwDB.WithContext(ctx).Where(rwDB.TenantID.Eq(tenant), rwDB.RouteID.Eq(route.ID)).Order(rwDB.Weigh.Asc()).Find()
	if err != nil {
		gf.Failed().SetMsg("获取航线航点关系失败").SetData(err).Regin(ctx)
		return
	}
	waypointIDs := make([]int64, 0, len(relations))
	for _, relation := range relations {
		waypointIDs = append(waypointIDs, relation.WaypointID)
	}
	wpRows := make([]*model.RobotdogWaypoint, 0)
	if len(waypointIDs) > 0 {
		wpDB := dao.Query().RobotdogWaypoint
		wpRows, err = wpDB.WithContext(ctx).Where(wpDB.TenantID.Eq(tenant), wpDB.ID.In(waypointIDs...)).Find()
		if err != nil {
			gf.Failed().SetMsg("获取航线航点失败").SetData(err).Regin(ctx)
			return
		}
	}
	wpMap := make(map[int64]*model.RobotdogWaypoint, len(wpRows))
	for _, waypoint := range wpRows {
		wpMap[waypoint.ID] = waypoint
	}
	waypoints := make([]*model.RobotdogWaypoint, 0, len(waypointIDs))
	for _, id := range waypointIDs {
		if waypoint, ok := wpMap[id]; ok {
			waypoints = append(waypoints, waypoint)
		}
	}
	gf.Success().SetMsg("获取航线详情").SetData(map[string]interface{}{"route": route, "waypoint_ids": waypointIDs, "waypoints": waypoints}).Regin(ctx)
}

func (api *Index) GetPointCloud(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	tenant := tenantID(ctx, param)
	mapDB := dao.Query().RobotdogMap
	var pointMap *model.RobotdogMap
	var err error
	if mapID := gf.Int64(param["map_id"]); mapID > 0 {
		pointMap, err = mapDB.WithContext(ctx).Where(mapDB.TenantID.Eq(tenant), mapDB.ID.Eq(mapID)).First()
	} else if dogID := gf.Int64(param["dog_id"]); dogID > 0 {
		dogDB := dao.Query().RobotdogDog
		dog, dogErr := dogDB.WithContext(ctx).Where(dogDB.TenantID.Eq(tenant), dogDB.ID.Eq(dogID)).First()
		if dogErr == nil && dog.MapID > 0 {
			pointMap, err = mapDB.WithContext(ctx).Where(mapDB.TenantID.Eq(tenant), mapDB.ID.Eq(dog.MapID)).First()
		}
	} else {
		pointMap, err = mapDB.WithContext(ctx).Where(mapDB.TenantID.Eq(tenant)).Order(mapDB.ID.Desc()).First()
	}
	if err != nil || pointMap == nil {
		gf.Success().SetMsg("获取点云地图").SetData(map[string]interface{}{"id": 0, "url": "", "format": "pcd", "preview_url": ""}).Regin(ctx)
		return
	}
	gf.Success().SetMsg("获取点云地图").SetData(pointMap).Regin(ctx)
}

func (api *Index) GetMapList(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	mapDB := dao.Query().RobotdogMap
	tenant := tenantID(ctx, param)
	offset, limit := pageArgs(param)
	list, total, err := mapDB.WithContext(ctx).Where(mapDB.TenantID.Eq(tenant)).Order(mapDB.ID.Desc()).FindByPage(offset, limit)
	if err != nil {
		gf.Failed().SetMsg("获取地图列表失败").SetData(err).Regin(ctx)
		return
	}
	gf.Success().SetMsg("获取地图列表").SetData(map[string]interface{}{"list": list, "total": total}).Regin(ctx)
}

func (api *Index) UploadMap(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	tenant := tenantID(ctx, param)
	now := time.Now()
	url := stringValue(param, "url", "")
	if file, err := ctx.FormFile("file"); err == nil && file != nil {
		dir := "resource/uploads/robotdog/maps"
		if err := os.MkdirAll(dir, 0755); err != nil {
			gf.Failed().SetMsg("创建地图目录失败").SetData(err).Regin(ctx)
			return
		}
		filename := fmt.Sprintf("%d_%s", now.UnixNano(), filepath.Base(file.Filename))
		dst := filepath.Join(dir, filename)
		if err := ctx.SaveUploadedFile(file, dst); err != nil {
			gf.Failed().SetMsg("保存地图文件失败").SetData(err).Regin(ctx)
			return
		}
		url = "/" + strings.TrimPrefix(dst, "/")
	}
	pointMap := &model.RobotdogMap{
		TenantID:   tenant,
		Name:       stringValue(param, "name", "点云地图"),
		Format:     stringValue(param, "format", "pcd"),
		URL:        url,
		PreviewURL: stringValue(param, "preview_url", ""),
		OriginX:    gf.Float64(param["origin_x"]),
		OriginY:    gf.Float64(param["origin_y"]),
		OriginZ:    gf.Float64(param["origin_z"]),
		Scale:      gf.Float64(param["scale"]),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if pointMap.Scale == 0 {
		pointMap.Scale = 1
	}
	if pointMap.URL == "" {
		gf.Failed().SetMsg("请上传地图文件或传入地图URL").Regin(ctx)
		return
	}
	if err := dao.Query().RobotdogMap.WithContext(ctx).Create(pointMap); err != nil {
		gf.Failed().SetMsg("上传地图失败").SetData(err).Regin(ctx)
		return
	}
	gf.Success().SetMsg("上传地图成功").SetData(pointMap).Regin(ctx)
}
