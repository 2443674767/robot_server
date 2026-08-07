package waypoint

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	internalfoxglove "gofly/app/robotdog/internal/foxglove"
	"gofly/dao"
	"gofly/dao/model"
	"gofly/utils/gf"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

type Index struct {
	NoNeedAuths []string
	NoNeedLogin []string
}

func init() {
	gf.Register(&Index{NoNeedLogin: []string{"getPcdFile"}})
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

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
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

func hasParam(param map[string]interface{}, key string) bool {
	_, ok := param[key]
	return ok
}

func taskItemsFromParam(param map[string]interface{}) []interface{} {
	if !hasParam(param, "tasks") {
		return nil
	}
	return gf.Interfaces(param["tasks"])
}

func allowedRouteTaskAction(action string) bool {
	switch action {
	case "lie", "stand", "navigate", "line_navigate", "photo", "switch_map", "relocalize", "voice":
		return true
	default:
		return false
	}
}

func normalizeParamsJSON(v interface{}) string {
	if v == nil || gf.String(v) == "" {
		return "{}"
	}
	if s, ok := v.(string); ok {
		s = strings.TrimSpace(s)
		if s == "" {
			return "{}"
		}
		if json.Valid([]byte(s)) {
			return s
		}
	}
	if raw, err := json.Marshal(v); err == nil && json.Valid(raw) {
		return string(raw)
	}
	return "{}"
}

func parseRouteTasks(param map[string]interface{}, tenantID int32, routeID int64, now time.Time) ([]*model.RobotdogRouteTask, bool, error) {
	if !hasParam(param, "tasks") {
		return nil, false, nil
	}
	items := taskItemsFromParam(param)
	tasks := make([]*model.RobotdogRouteTask, 0, len(items))
	for i, item := range items {
		row, ok := item.(map[string]interface{})
		if !ok {
			return nil, true, fmt.Errorf("第%d个子任务格式错误", i+1)
		}
		action := stringValue(row, "action", "")
		if action == "" {
			return nil, true, fmt.Errorf("第%d个子任务动作不能为空", i+1)
		}
		if !allowedRouteTaskAction(action) {
			return nil, true, fmt.Errorf("第%d个子任务动作不支持: %s", i+1, action)
		}
		seq := gf.Int32(row["seq"])
		if seq <= 0 {
			seq = int32(i + 1)
		}
		tasks = append(tasks, &model.RobotdogRouteTask{
			TenantID:  tenantID,
			RouteID:   routeID,
			Seq:       seq,
			Action:    action,
			WaitSec:   gf.Float64(row["wait_sec"]),
			Params:    normalizeParamsJSON(row["params"]),
			CreatedAt: now,
			UpdatedAt: now,
		})
	}
	return tasks, true, nil
}

type navigateWaypointRef struct {
	Seq        int32
	WaypointID int64
}

func navigateWaypointRefs(tasks []*model.RobotdogRouteTask) ([]navigateWaypointRef, error) {
	refs := make([]navigateWaypointRef, 0)
	for _, task := range tasks {
		if task.Action != "navigate" {
			continue
		}
		var params map[string]interface{}
		if err := json.Unmarshal([]byte(task.Params), &params); err != nil {
			return nil, fmt.Errorf("第%d个navigate子任务params格式错误", task.Seq)
		}
		waypointID := gf.Int64(params["waypoint_id"])
		if waypointID <= 0 {
			return nil, fmt.Errorf("第%d个navigate子任务必须传params.waypoint_id", task.Seq)
		}
		refs = append(refs, navigateWaypointRef{Seq: task.Seq, WaypointID: waypointID})
	}
	sort.SliceStable(refs, func(i, j int) bool {
		return refs[i].Seq < refs[j].Seq
	})
	return refs, nil
}

func routeEdgesFromNavigateRefs(tenantID int32, routeID int64, refs []navigateWaypointRef, now time.Time) []*model.RobotdogRouteEdge {
	if len(refs) < 2 {
		return []*model.RobotdogRouteEdge{}
	}
	edges := make([]*model.RobotdogRouteEdge, 0, len(refs)-1)
	for i := 0; i < len(refs)-1; i++ {
		edges = append(edges, &model.RobotdogRouteEdge{
			TenantID:       tenantID,
			RouteID:        routeID,
			FromWaypointID: refs[i].WaypointID,
			ToWaypointID:   refs[i+1].WaypointID,
			FromTaskSeq:    refs[i].Seq,
			ToTaskSeq:      refs[i+1].Seq,
			EdgeSeq:        int32(i + 1),
			CreatedAt:      now,
			UpdatedAt:      now,
		})
	}
	return edges
}

func waypointIDsFromNavigateRefs(refs []navigateWaypointRef) []int64 {
	ids := make([]int64, 0, len(refs))
	for _, ref := range refs {
		if ref.WaypointID > 0 {
			ids = append(ids, ref.WaypointID)
		}
	}
	return ids
}

func validateRouteEdgeWaypoints(tx *gorm.DB, tenantID int32, refs []navigateWaypointRef) error {
	seen := make(map[int64]struct{}, len(refs))
	ids := make([]int64, 0, len(refs))
	for _, ref := range refs {
		if _, ok := seen[ref.WaypointID]; ok {
			continue
		}
		seen[ref.WaypointID] = struct{}{}
		ids = append(ids, ref.WaypointID)
	}
	if len(ids) == 0 {
		return nil
	}
	var count int64
	if err := tx.Model(&model.RobotdogWaypoint{}).Where("tenant_id = ? AND id IN ?", tenantID, ids).Count(&count).Error; err != nil {
		return err
	}
	if count != int64(len(ids)) {
		return fmt.Errorf("navigate子任务中存在无效航点ID")
	}
	return nil
}

func decodeTaskParams(params string) interface{} {
	params = strings.TrimSpace(params)
	if params == "" {
		return map[string]interface{}{}
	}
	var data interface{}
	if err := json.Unmarshal([]byte(params), &data); err != nil {
		return map[string]interface{}{}
	}
	return data
}

func routeTasksData(ctx *gf.GinCtx, tenantID int32, routeID int64) []map[string]interface{} {
	taskDB := dao.Query().RobotdogRouteTask
	rows, err := taskDB.WithContext(ctx).Where(taskDB.TenantID.Eq(tenantID), taskDB.RouteID.Eq(routeID)).Order(taskDB.Seq.Asc(), taskDB.ID.Asc()).Find()
	if err != nil {
		return []map[string]interface{}{}
	}
	tasks := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		tasks = append(tasks, map[string]interface{}{
			"id":       row.ID,
			"seq":      row.Seq,
			"action":   row.Action,
			"wait_sec": row.WaitSec,
			"params":   decodeTaskParams(row.Params),
		})
	}
	return tasks
}

func routeTaskMap(ctx *gf.GinCtx, tenantID int32, routeIDs []int64) map[int64][]map[string]interface{} {
	result := make(map[int64][]map[string]interface{})
	if len(routeIDs) == 0 {
		return result
	}
	taskDB := dao.Query().RobotdogRouteTask
	rows, err := taskDB.WithContext(ctx).Where(taskDB.TenantID.Eq(tenantID), taskDB.RouteID.In(routeIDs...)).Order(taskDB.RouteID.Asc(), taskDB.Seq.Asc(), taskDB.ID.Asc()).Find()
	if err != nil {
		return result
	}
	for _, row := range rows {
		result[row.RouteID] = append(result[row.RouteID], map[string]interface{}{
			"id":       row.ID,
			"seq":      row.Seq,
			"action":   row.Action,
			"wait_sec": row.WaitSec,
			"params":   decodeTaskParams(row.Params),
		})
	}
	return result
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

func routeWaypointBriefMap(ctx *gf.GinCtx, tenantID int32, routeIDs []int64) map[int64][]map[string]interface{} {
	if len(routeIDs) == 0 {
		return map[int64][]map[string]interface{}{}
	}
	idsMap := routeWaypointMap(ctx, tenantID, routeIDs)
	seen := map[int64]struct{}{}
	waypointIDs := make([]int64, 0)
	for _, ids := range idsMap {
		for _, id := range ids {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			waypointIDs = append(waypointIDs, id)
		}
	}
	if len(waypointIDs) == 0 {
		return map[int64][]map[string]interface{}{}
	}
	wpDB := dao.Query().RobotdogWaypoint
	rows, err := wpDB.WithContext(ctx).Where(wpDB.TenantID.Eq(tenantID), wpDB.ID.In(waypointIDs...)).Find()
	if err != nil {
		return map[int64][]map[string]interface{}{}
	}
	wpMap := make(map[int64]*model.RobotdogWaypoint, len(rows))
	for _, row := range rows {
		wpMap[row.ID] = row
	}
	presetMap := waypointPresetIDMap(ctx, tenantID, waypointIDs)
	briefResult := make(map[int64][]map[string]interface{}, len(idsMap))
	for routeID, ids := range idsMap {
		list := make([]map[string]interface{}, 0, len(ids))
		for i, id := range ids {
			waypoint, ok := wpMap[id]
			if !ok {
				continue
			}
			var presetID interface{}
			if waypoint.IsTask == 1 {
				if id := presetMap[waypoint.ID]; id > 0 {
					presetID = id
				}
			}
			list = append(list, map[string]interface{}{
				"seq":       i + 1,
				"id":        waypoint.ID,
				"name":      waypoint.Name,
				"is_task":   waypoint.IsTask,
				"preset_id": presetID,
			})
		}
		briefResult[routeID] = list
	}
	return briefResult
}

func waypointPresetIDMap(ctx *gf.GinCtx, tenantID int32, waypointIDs []int64) map[int64]int64 {
	result := make(map[int64]int64)
	if len(waypointIDs) == 0 {
		return result
	}
	var presets []model.RobotdogPtzPreset
	if err := dao.DB().WithContext(ctx).
		Where("tenant_id = ? AND waypoint_id IN ? AND deleted_at IS NULL", tenantID, waypointIDs).
		Order("sort_no ASC, id ASC").
		Find(&presets).Error; err != nil {
		return result
	}
	for _, preset := range presets {
		if _, ok := result[preset.WaypointID]; ok {
			continue
		}
		result[preset.WaypointID] = preset.ID
	}
	return result
}

func routeEdgeMap(ctx *gf.GinCtx, tenantID int32, routeIDs []int64) map[int64][]map[string]interface{} {
	result := make(map[int64][]map[string]interface{})
	if len(routeIDs) == 0 {
		return result
	}
	edgeDB := dao.Query().RobotdogRouteEdge
	rows, err := edgeDB.WithContext(ctx).Where(edgeDB.TenantID.Eq(tenantID), edgeDB.RouteID.In(routeIDs...)).Order(edgeDB.RouteID.Asc(), edgeDB.EdgeSeq.Asc()).Find()
	if err != nil {
		return result
	}
	for _, row := range rows {
		result[row.RouteID] = append(result[row.RouteID], map[string]interface{}{
			"id":               row.ID,
			"route_id":         row.RouteID,
			"from_waypoint_id": row.FromWaypointID,
			"to_waypoint_id":   row.ToWaypointID,
			"from_task_seq":    row.FromTaskSeq,
			"to_task_seq":      row.ToTaskSeq,
			"edge_seq":         row.EdgeSeq,
		})
	}
	return result
}

func routeListData(ctx *gf.GinCtx, tenantID int32, routes []*model.RobotdogRoute) []map[string]interface{} {
	ids := make([]int64, 0, len(routes))
	for _, route := range routes {
		ids = append(ids, route.ID)
	}
	wpMap := routeWaypointMap(ctx, tenantID, ids)
	taskMap := routeTaskMap(ctx, tenantID, ids)
	edgeMap := routeEdgeMap(ctx, tenantID, ids)
	list := make([]map[string]interface{}, 0, len(routes))
	for _, route := range routes {
		tasks := taskMap[route.ID]
		edges := edgeMap[route.ID]
		list = append(list, map[string]interface{}{
			"id":           route.ID,
			"tenant_id":    route.TenantID,
			"dog_id":       route.DogID,
			"name":         route.Name,
			"status":       route.Status,
			"run_status":   route.RunStatus,
			"remark":       route.Remark,
			"waypoint_ids": wpMap[route.ID],
			"edges":        edges,
			"edge_count":   len(edges),
			"task_count":   len(tasks),
			"tasks":        tasks,
			"created_at":   route.CreatedAt,
			"updated_at":   route.UpdatedAt,
		})
	}
	return list
}

func routeWaypointAllData(ctx *gf.GinCtx, tenantID int32, routes []*model.RobotdogRoute) []map[string]interface{} {
	ids := make([]int64, 0, len(routes))
	for _, route := range routes {
		ids = append(ids, route.ID)
	}
	wpBriefMap := routeWaypointBriefMap(ctx, tenantID, ids)
	list := make([]map[string]interface{}, 0, len(routes))
	for _, route := range routes {
		waypoints := wpBriefMap[route.ID]
		if waypoints == nil {
			waypoints = []map[string]interface{}{}
		}
		list = append(list, map[string]interface{}{
			"route_id":   route.ID,
			"route_name": route.Name,
			"waypoints":  waypoints,
		})
	}
	return list
}

type robotdogMinioConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
}

func loadRobotdogMinioConfig() robotdogMinioConfig {
	cfg := robotdogMinioConfig{
		Endpoint:  "localhost:9000",
		AccessKey: "admin",
		SecretKey: "12345678",
		Bucket:    "robotdog",
		UseSSL:    false,
	}
	wd, err := os.Getwd()
	if err != nil {
		return cfg
	}
	vip := viper.New()
	vip.AddConfigPath(filepath.Join(wd, "resource/config"))
	vip.SetConfigName("upload")
	vip.SetConfigType("yaml")
	if err := vip.ReadInConfig(); err != nil {
		return cfg
	}
	if v := strings.TrimSpace(vip.GetString("minio.endpoint")); v != "" {
		cfg.Endpoint = v
	}
	if v := strings.TrimSpace(vip.GetString("minio.accessKey")); v != "" {
		cfg.AccessKey = v
	}
	if v := strings.TrimSpace(vip.GetString("minio.secretKey")); v != "" {
		cfg.SecretKey = v
	}
	if v := strings.TrimSpace(vip.GetString("minio.bucket")); v != "" {
		cfg.Bucket = v
	}
	cfg.UseSSL = vip.GetBool("minio.useSSL")
	return cfg
}

func (cfg robotdogMinioConfig) objectBaseURL() string {
	endpoint := strings.TrimRight(cfg.Endpoint, "/")
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		return endpoint + "/" + cfg.Bucket
	}
	scheme := "http"
	if cfg.UseSSL {
		scheme = "https"
	}
	return scheme + "://" + endpoint + "/" + cfg.Bucket
}

func (cfg robotdogMinioConfig) client() (*minio.Client, error) {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "https://")
	return minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
}

func ensureRobotdogMinioBucket(ctx context.Context, client *minio.Client, cfg robotdogMinioConfig) error {
	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return err
	}
	if !exists {
		if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{}); err != nil {
			return err
		}
	}
	policy := fmt.Sprintf(`{
		"Version": "2012-10-17",
		"Statement": [
			{
				"Effect": "Allow",
				"Principal": {"AWS": ["*"]},
				"Action": ["s3:GetObject"],
				"Resource": ["arn:aws:s3:::%s/*"]
			}
		]
	}`, cfg.Bucket)
	return client.SetBucketPolicy(ctx, cfg.Bucket, policy)
}

func cleanMinioObjectPath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "/")
	path = filepath.ToSlash(filepath.Clean(path))
	if path == "." || strings.Contains(path, "..") {
		return ""
	}
	return path
}

func minioObjectURL(cfg robotdogMinioConfig, objectPath string) string {
	objectPath = cleanMinioObjectPath(objectPath)
	if objectPath == "" {
		return ""
	}
	return cfg.objectBaseURL() + "/" + objectPath
}

func pcdFileProxyURL(objectPath string) string {
	objectPath = cleanMinioObjectPath(objectPath)
	if objectPath == "" {
		return ""
	}
	return "/robotdog/waypoint/getPcdFile?path=" + url.QueryEscape(objectPath)
}

func cleanUploadURL(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	path = filepath.ToSlash(filepath.Clean(path))
	path = strings.TrimPrefix(path, "./")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}

func fullUploadURL(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	if len(path) < 4 {
		return path
	}
	return gf.GetFullUrl(path)
}

func pcdMapObjectPrefix(mapURL string) (string, bool) {
	mapURL = strings.TrimSpace(mapURL)
	if mapURL == "" || strings.HasPrefix(mapURL, "http://") || strings.HasPrefix(mapURL, "https://") {
		return "", false
	}
	prefix := cleanMinioObjectPath(mapURL)
	if prefix == "" || strings.HasPrefix(prefix, "resource/uploads/") {
		return "", false
	}
	return strings.TrimSuffix(prefix, "/"), true
}

func pcdLocalPath(url string) (string, bool) {
	url = strings.TrimSpace(url)
	if url == "" || strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") || strings.Contains(url, "..") {
		return "", false
	}
	url = strings.TrimPrefix(filepath.ToSlash(url), "/")
	if !strings.HasPrefix(url, "resource/uploads/") {
		return "", false
	}
	return filepath.Clean(url), true
}

func pcdLayerKey(filename string) (string, bool) {
	name := strings.TrimSuffix(filename, filepath.Ext(filename))
	if name == "" {
		return "", false
	}
	if strings.HasSuffix(name, "_downsize") {
		name = strings.TrimSuffix(name, "_downsize")
		return name, true
	}
	return name, false
}

func pcdMapMinioLayers(mapURL string) []map[string]interface{} {
	prefix, ok := pcdMapObjectPrefix(mapURL)
	if !ok {
		return nil
	}
	cfg := loadRobotdogMinioConfig()
	client, err := cfg.client()
	if err != nil {
		return []map[string]interface{}{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := ensureRobotdogMinioBucket(ctx, client, cfg); err != nil {
		return []map[string]interface{}{}
	}
	layerMap := make(map[string]map[string]interface{})
	for object := range client.ListObjects(ctx, cfg.Bucket, minio.ListObjectsOptions{Prefix: prefix + "/", Recursive: true}) {
		if object.Err != nil || strings.ToLower(filepath.Ext(object.Key)) != ".pcd" {
			continue
		}
		filename := filepath.Base(object.Key)
		key, downsize := pcdLayerKey(filename)
		if key == "" {
			continue
		}
		layer, ok := layerMap[key]
		if !ok {
			layer = map[string]interface{}{"key": key, "name": key}
			layerMap[key] = layer
		}
		fileURL := minioObjectURL(cfg, object.Key)
		proxyURL := pcdFileProxyURL(object.Key)
		if downsize {
			layer["downsize_path"] = object.Key
			layer["downsize_url"] = proxyURL
			layer["downsize_file_url"] = fileURL
		} else {
			layer["path"] = object.Key
			layer["url"] = proxyURL
			layer["file_url"] = fileURL
		}
	}
	keys := make([]string, 0, len(layerMap))
	for key := range layerMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	layers := make([]map[string]interface{}, 0, len(keys))
	for _, key := range keys {
		layers = append(layers, layerMap[key])
	}
	return layers
}

func pcdMapLayers(mapURL string) []map[string]interface{} {
	if layers := pcdMapMinioLayers(mapURL); layers != nil {
		return layers
	}
	localPath, ok := pcdLocalPath(mapURL)
	if !ok {
		return []map[string]interface{}{}
	}
	info, err := os.Stat(localPath)
	if err != nil {
		return []map[string]interface{}{}
	}
	layerMap := make(map[string]map[string]interface{})
	addFile := func(filePath string) {
		if strings.ToLower(filepath.Ext(filePath)) != ".pcd" {
			return
		}
		filename := filepath.Base(filePath)
		key, downsize := pcdLayerKey(filename)
		if key == "" {
			return
		}
		layer, ok := layerMap[key]
		if !ok {
			layer = map[string]interface{}{"key": key, "name": key}
			layerMap[key] = layer
		}
		url := "/" + filepath.ToSlash(filePath)
		if downsize {
			layer["downsize_url"] = url
			layer["downsize_file_url"] = fullUploadURL(url)
		} else {
			layer["url"] = url
			layer["file_url"] = fullUploadURL(url)
		}
	}
	if info.IsDir() {
		entries, err := os.ReadDir(localPath)
		if err != nil {
			return []map[string]interface{}{}
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			addFile(filepath.Join(localPath, entry.Name()))
		}
	} else {
		addFile(localPath)
	}
	keys := make([]string, 0, len(layerMap))
	for key := range layerMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	layers := make([]map[string]interface{}, 0, len(keys))
	for _, key := range keys {
		layers = append(layers, layerMap[key])
	}
	return layers
}

func pcdMapData(pointMap *model.RobotdogMap) map[string]interface{} {
	if pointMap == nil {
		return map[string]interface{}{"id": 0, "url": "", "file_url": "", "format": "pcd", "preview_url": "", "layers": []map[string]interface{}{}}
	}
	data := map[string]interface{}{
		"id":          pointMap.ID,
		"tenant_id":   pointMap.TenantID,
		"name":        pointMap.Name,
		"format":      pointMap.Format,
		"url":         pointMap.URL,
		"preview_url": pointMap.PreviewURL,
		"origin_x":    pointMap.OriginX,
		"origin_y":    pointMap.OriginY,
		"origin_z":    pointMap.OriginZ,
		"scale":       pointMap.Scale,
		"created_at":  pointMap.CreatedAt,
		"updated_at":  pointMap.UpdatedAt,
		"layers":      pcdMapLayers(pointMap.URL),
	}
	if pointMap.URL != "" {
		if prefix, ok := pcdMapObjectPrefix(pointMap.URL); ok {
			data["file_url"] = minioObjectURL(loadRobotdogMinioConfig(), prefix)
		} else {
			data["file_url"] = fullUploadURL(pointMap.URL)
		}
	} else {
		data["file_url"] = ""
	}
	return data
}

func pcdUploadFiles(ctx *gf.GinCtx) []*multipart.FileHeader {
	if err := ctx.Request.ParseMultipartForm(32 << 20); err != nil {
		return nil
	}
	if ctx.Request.MultipartForm == nil {
		return nil
	}
	files := make([]*multipart.FileHeader, 0)
	for _, key := range []string{"files", "files[]", "file"} {
		files = append(files, ctx.Request.MultipartForm.File[key]...)
	}
	return files
}

func (api *Index) GetList(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	dogDB := dao.Query().RobotdogDog
	tenant := tenantID(ctx, param)
	where := []dao.Condition{dogDB.TenantID.Eq(tenant)}
	// mine=1 is accepted for the frontend contract. The current dog table has no
	// user owner column, so tenant isolation is the effective scope for now.
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
	gf.Success().SetMsg("获取列表成功").SetData(map[string]interface{}{"list": list, "total": total}).Regin(ctx)
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
			PtzID:     gf.Int64(param["ptz_id"]),
			UdpHost:   stringValue(param, "udp_host", ""),
			UdpPort:   gf.Int32(param["udp_port"]),
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
		"ptz_id":     gf.Int64(param["ptz_id"]),
		"udp_host":   stringValue(param, "udp_host", ""),
		"udp_port":   gf.Int32(param["udp_port"]),
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
	if dogID := gf.Int64(param["dog_id"]); dogID > 0 {
		where = append(where, wpDB.DogID.Eq(dogID))
	}
	if mapID := gf.Int64(param["map_id"]); mapID > 0 {
		where = append(where, wpDB.MapID.Eq(mapID))
	}
	if name := stringValue(param, "name", ""); name != "" {
		where = append(where, wpDB.Name.Like("%"+name+"%"))
	}
	if hasParam(param, "is_task") {
		where = append(where, wpDB.IsTask.Eq(gf.Int8(param["is_task"])))
	}
	offset, limit := pageArgs(param)
	list, total, err := wpDB.WithContext(ctx).Where(where...).Order(wpDB.ID.Desc()).FindByPage(offset, limit)
	if err != nil {
		gf.Failed().SetMsg("获取航点列表失败").SetData(err).Regin(ctx)
		return
	}
	gf.Success().SetMsg("获取航点列表").SetData(map[string]interface{}{"list": list, "total": total}).Regin(ctx)
}

func (api *Index) GetRouteWaypointAll(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	tenant := tenantID(ctx, param)
	routeDB := dao.Query().RobotdogRoute
	routeWhere := []dao.Condition{routeDB.TenantID.Eq(tenant)}
	if routeID := gf.Int64(param["route_id"]); routeID > 0 {
		routeWhere = append(routeWhere, routeDB.ID.Eq(routeID))
	} else if routeID := gf.Int64(param["id"]); routeID > 0 {
		routeWhere = append(routeWhere, routeDB.ID.Eq(routeID))
	}
	if dogID := gf.Int64(param["dog_id"]); dogID > 0 {
		routeWhere = append(routeWhere, routeDB.DogID.Eq(dogID))
	}
	if status := stringValue(param, "status", ""); status != "" {
		routeWhere = append(routeWhere, routeDB.Status.Eq(status))
	}
	if runStatus := stringValue(param, "run_status", ""); runStatus != "" {
		routeWhere = append(routeWhere, routeDB.RunStatus.Eq(runStatus))
	}
	if name := firstNonEmptyString(stringValue(param, "route_name", ""), stringValue(param, "name", "")); name != "" {
		routeWhere = append(routeWhere, routeDB.Name.Like("%"+name+"%"))
	}
	routes, err := routeDB.WithContext(ctx).Where(routeWhere...).Order(routeDB.ID.Desc()).Find()
	if err != nil {
		gf.Failed().SetMsg("获取航线数据失败").SetData(err).Regin(ctx)
		return
	}

	gf.Success().SetMsg("获取航线航点数据").SetData(map[string]interface{}{
		"list":  routeWaypointAllData(ctx, tenant, routes),
		"total": len(routes),
	}).Regin(ctx)
}

func (api *Index) SaveWaypoint(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	wpDB := dao.Query().RobotdogWaypoint
	tenant := tenantID(ctx, param)
	id := gf.Int64(param["id"])
	now := time.Now()
	name := stringValue(param, "name", "")
	if name == "" {
		gf.Failed().SetMsg("航点名称不能为空").Regin(ctx)
		return
	}
	latest, err := internalfoxglove.FetchLatestOdometry(ctx.Request.Context(), internalfoxglove.FetchOptions{
		WSURL: gf.String(param["ws_url"]),
		Topic: gf.String(param["topic"]),
	})
	if err != nil {
		gf.Failed().SetMsg("获取机械狗当前位置失败").SetData(err.Error()).Regin(ctx)
		return
	}
	rawData, err := json.Marshal(latest)
	if err != nil {
		gf.Failed().SetMsg("序列化机械狗当前位置失败").SetData(err.Error()).Regin(ctx)
		return
	}
	poseValues := waypointValuesFromPose(latest, string(rawData), now)
	if id == 0 {
		wp := &model.RobotdogWaypoint{
			TenantID:           tenant,
			MapID:              gf.Int64(param["map_id"]),
			DogID:              gf.Int64(param["dog_id"]),
			Name:               name,
			IsTask:             gf.Int8(param["is_task"]),
			X:                  poseValues["x"].(float64),
			Y:                  poseValues["y"].(float64),
			Z:                  poseValues["z"].(float64),
			Yaw:                poseValues["yaw"].(float64),
			Source:             poseValues["source"].(string),
			FoxgloveWsURL:      poseValues["foxglove_ws_url"].(string),
			FoxgloveTopic:      poseValues["foxglove_topic"].(string),
			FoxgloveSchemaName: poseValues["foxglove_schema_name"].(string),
			FoxgloveTimestamp:  poseValues["foxglove_timestamp"].(int64),
			FrameID:            poseValues["frame_id"].(string),
			ChildFrameID:       poseValues["child_frame_id"].(string),
			OrientationX:       poseValues["orientation_x"].(float64),
			OrientationY:       poseValues["orientation_y"].(float64),
			OrientationZ:       poseValues["orientation_z"].(float64),
			OrientationW:       poseValues["orientation_w"].(float64),
			TwistLinearX:       poseValues["twist_linear_x"].(float64),
			TwistLinearY:       poseValues["twist_linear_y"].(float64),
			TwistLinearZ:       poseValues["twist_linear_z"].(float64),
			TwistAngularX:      poseValues["twist_angular_x"].(float64),
			TwistAngularY:      poseValues["twist_angular_y"].(float64),
			TwistAngularZ:      poseValues["twist_angular_z"].(float64),
			RawData:            poseValues["raw_data"].(string),
			Remark:             stringValue(param, "remark", ""),
			CreatedAt:          now,
			UpdatedAt:          now,
		}
		if err := wpDB.WithContext(ctx).Create(wp); err != nil {
			gf.Failed().SetMsg("添加航点失败").SetData(err).Regin(ctx)
			return
		}
		gf.Success().SetMsg("添加航点成功").SetData(wp).Regin(ctx)
		return
	}
	updates := map[string]interface{}{
		"map_id":     gf.Int64(param["map_id"]),
		"dog_id":     gf.Int64(param["dog_id"]),
		"name":       name,
		"is_task":    gf.Int8(param["is_task"]),
		"remark":     stringValue(param, "remark", ""),
		"updated_at": now,
	}
	for k, v := range poseValues {
		updates[k] = v
	}
	res, err := wpDB.WithContext(ctx).Where(wpDB.ID.Eq(id), wpDB.TenantID.Eq(tenant)).Updates(updates)
	if err != nil {
		gf.Failed().SetMsg("更新航点失败").SetData(err).Regin(ctx)
		return
	}
	if res.RowsAffected == 0 {
		gf.Failed().SetMsg("航点不存在").Regin(ctx)
		return
	}
	wp, err := wpDB.WithContext(ctx).Where(wpDB.ID.Eq(id), wpDB.TenantID.Eq(tenant)).First()
	if err != nil {
		gf.Failed().SetMsg("获取更新后航点失败").SetData(err).Regin(ctx)
		return
	}
	gf.Success().SetMsg("更新航点成功").SetData(wp).Regin(ctx)
}

func waypointValuesFromPose(latest *internalfoxglove.PoseMessage, rawData string, now time.Time) map[string]interface{} {
	decoded := latest.Decoded
	q := decoded.Pose.Orientation
	return map[string]interface{}{
		"x":                    decoded.NavCustomPayload.PositionX,
		"y":                    decoded.NavCustomPayload.PositionY,
		"z":                    decoded.NavCustomPayload.PositionZ,
		"yaw":                  yawFromQuaternion(q.X, q.Y, q.Z, q.W),
		"source":               "foxglove",
		"foxglove_ws_url":      latest.WSURL,
		"foxglove_topic":       latest.Topic,
		"foxglove_schema_name": latest.SchemaName,
		"foxglove_timestamp":   int64(latest.Timestamp),
		"frame_id":             decoded.Header.FrameID,
		"child_frame_id":       decoded.ChildFrameID,
		"orientation_x":        decoded.NavCustomPayload.OrientationX,
		"orientation_y":        decoded.NavCustomPayload.OrientationY,
		"orientation_z":        decoded.NavCustomPayload.OrientationZ,
		"orientation_w":        decoded.NavCustomPayload.OrientationW,
		"twist_linear_x":       decoded.Twist.Linear.X,
		"twist_linear_y":       decoded.Twist.Linear.Y,
		"twist_linear_z":       decoded.Twist.Linear.Z,
		"twist_angular_x":      decoded.Twist.Angular.X,
		"twist_angular_y":      decoded.Twist.Angular.Y,
		"twist_angular_z":      decoded.Twist.Angular.Z,
		"raw_data":             rawData,
		"updated_at":           now,
	}
}

func yawFromQuaternion(x, y, z, w float64) float64 {
	sinyCosp := 2 * (w*z + x*y)
	cosyCosp := 1 - 2*(y*y+z*z)
	return math.Atan2(sinyCosp, cosyCosp) * 180 / math.Pi
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
			TenantID:     tenant,
			MapID:        gf.Int64(row["map_id"]),
			Name:         name,
			IsTask:       gf.Int8(row["is_task"]),
			X:            gf.Float64(row["x"]),
			Y:            gf.Float64(row["y"]),
			Z:            gf.Float64(row["z"]),
			Yaw:          gf.Float64(row["yaw"]),
			Source:       "manual",
			OrientationW: 1,
			Remark:       stringValue(row, "remark", ""),
			CreatedAt:    now,
			UpdatedAt:    now,
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
	name := stringValue(param, "name", "")
	if name == "" {
		gf.Failed().SetMsg("航线名称不能为空").Regin(ctx)
		return
	}
	now := time.Now()
	routeTasks, hasTasks, taskErr := parseRouteTasks(param, tenant, routeID, now)
	if taskErr != nil {
		gf.Failed().SetMsg(taskErr.Error()).Regin(ctx)
		return
	}
	navigateRefs, edgeErr := navigateWaypointRefs(routeTasks)
	if edgeErr != nil {
		gf.Failed().SetMsg(edgeErr.Error()).Regin(ctx)
		return
	}
	if len(navigateRefs) > 0 {
		waypointIDs = waypointIDsFromNavigateRefs(navigateRefs)
	}
	if len(waypointIDs) == 0 && (!hasTasks || len(routeTasks) == 0) {
		gf.Failed().SetMsg("航线至少需要一个航点或子任务").Regin(ctx)
		return
	}
	err := dao.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := validateRouteEdgeWaypoints(tx, tenant, navigateRefs); err != nil {
			return err
		}
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
			res := tx.Model(&model.RobotdogRoute{}).Where("id = ? AND tenant_id = ?", routeID, tenant).Updates(updates)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				return fmt.Errorf("航线不存在")
			}
			if err := tx.Where("route_id = ? AND tenant_id = ?", routeID, tenant).Delete(&model.RobotdogRouteWaypoint{}).Error; err != nil {
				return err
			}
			if hasTasks {
				if err := tx.Where("route_id = ? AND tenant_id = ?", routeID, tenant).Delete(&model.RobotdogRouteEdge{}).Error; err != nil {
					return err
				}
				if err := tx.Where("route_id = ? AND tenant_id = ?", routeID, tenant).Delete(&model.RobotdogRouteTask{}).Error; err != nil {
					return err
				}
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
		if len(relations) > 0 {
			if err := tx.Create(&relations).Error; err != nil {
				return err
			}
		}
		if hasTasks {
			edges := routeEdgesFromNavigateRefs(tenant, routeID, navigateRefs, now)
			if len(edges) > 0 {
				if err := tx.Create(&edges).Error; err != nil {
					return err
				}
			}
		}
		if hasTasks && len(routeTasks) > 0 {
			for _, task := range routeTasks {
				task.RouteID = routeID
			}
			if err := tx.Create(&routeTasks).Error; err != nil {
				return err
			}
		}
		return nil
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
		waypointCount, err := rwDB.WithContext(ctx).Where(rwDB.TenantID.Eq(tenant), rwDB.RouteID.Eq(routeID)).Count()
		if err != nil {
			gf.Failed().SetMsg("校验航线航点失败").SetData(err).Regin(ctx)
			return
		}
		taskDB := dao.Query().RobotdogRouteTask
		taskCount, err := taskDB.WithContext(ctx).Where(taskDB.TenantID.Eq(tenant), taskDB.RouteID.Eq(routeID)).Count()
		if err != nil {
			gf.Failed().SetMsg("校验航线子任务失败").SetData(err).Regin(ctx)
			return
		}
		if waypointCount == 0 && taskCount == 0 {
			gf.Failed().SetMsg("航线没有航点或子任务，不能发布").Regin(ctx)
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
		if err := tx.Where("route_id IN ? AND tenant_id = ?", ids, tenant).Delete(&model.RobotdogRouteEdge{}).Error; err != nil {
			return err
		}
		if err := tx.Where("route_id IN ? AND tenant_id = ?", ids, tenant).Delete(&model.RobotdogRouteTask{}).Error; err != nil {
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
	tasks := routeTasksData(ctx, tenant, route.ID)
	edgeMap := routeEdgeMap(ctx, tenant, []int64{route.ID})
	gf.Success().SetMsg("获取航线详情").SetData(map[string]interface{}{"route": route, "waypoint_ids": waypointIDs, "waypoints": waypoints, "tasks": tasks, "edges": edgeMap[route.ID]}).Regin(ctx)
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
		gf.Success().SetMsg("获取点云地图").SetData(pcdMapData(nil)).Regin(ctx)
		return
	}
	gf.Success().SetMsg("获取点云地图").SetData(pcdMapData(pointMap)).Regin(ctx)
}

func (api *Index) GetPcdMap(ctx *gf.GinCtx) {
	api.GetPointCloud(ctx)
}

func (api *Index) GetPcdFile(ctx *gf.GinCtx) {
	objectPath := cleanMinioObjectPath(ctx.Query("path"))
	if objectPath == "" || strings.ToLower(filepath.Ext(objectPath)) != ".pcd" {
		gf.Failed().SetMsg("PCD文件路径不合法").Regin(ctx)
		return
	}
	cfg := loadRobotdogMinioConfig()
	client, err := cfg.client()
	if err != nil {
		gf.Failed().SetMsg("连接MinIO失败").SetData(err.Error()).Regin(ctx)
		return
	}
	object, err := client.GetObject(ctx.Request.Context(), cfg.Bucket, objectPath, minio.GetObjectOptions{})
	if err != nil {
		gf.Failed().SetMsg("读取PCD文件失败").SetData(err.Error()).Regin(ctx)
		return
	}
	defer object.Close()
	stat, err := object.Stat()
	if err != nil {
		gf.Failed().SetMsg("PCD文件不存在").SetData(err.Error()).Regin(ctx)
		return
	}
	ctx.Header("Content-Type", "application/octet-stream")
	ctx.Header("Content-Disposition", "inline; filename="+filepath.Base(objectPath))
	ctx.Header("Content-Transfer-Encoding", "binary")
	ctx.Header("Cache-Control", "public, max-age=3600")
	ctx.Header("Content-Length", fmt.Sprintf("%d", stat.Size))
	ctx.Status(200)
	if _, err := io.Copy(ctx.Writer, object); err != nil {
		return
	}
}

func (api *Index) GetNavData(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	page := gf.Int(param["page"])
	if page <= 0 {
		page = 1
	}
	data, err := deviceExtraGet(ctx.Request.Context(), "/api/extra/get_nav_data", url.Values{"page": []string{fmt.Sprintf("%d", page)}})
	if err != nil {
		gf.Failed().SetMsg("获取导航点位列表失败").SetData(err.Error()).Regin(ctx)
		return
	}
	gf.Success().SetMsg("获取导航点位列表成功").SetData(data).Regin(ctx)
}

func (api *Index) ResetLocation(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	id := gf.Int(param["id"])
	if id <= 0 {
		gf.Failed().SetMsg("重定位点位ID不能为空").Regin(ctx)
		return
	}
	data, err := deviceExtraPost(ctx.Request.Context(), "/api/extra/reset_location", map[string]interface{}{"id": id})
	if err != nil {
		gf.Failed().SetMsg("重定位失败").SetData(err.Error()).Regin(ctx)
		return
	}
	gf.Success().SetMsg("重定位请求已发送").SetData(data).Regin(ctx)
}

func (api *Index) GetMapList(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	page := gf.Int(param["page"])
	if page <= 0 {
		page = 1
	}
	data, err := deviceExtraGet(ctx.Request.Context(), "/api/extra/get_map_data", url.Values{"page": []string{fmt.Sprintf("%d", page)}})
	if err != nil {
		gf.Failed().SetMsg("获取设备地图列表失败").SetData(err.Error()).Regin(ctx)
		return
	}
	gf.Success().SetMsg("获取设备地图列表成功").SetData(data).Regin(ctx)
}

func (api *Index) GetAllMapNavData(ctx *gf.GinCtx) {
	data, err := deviceExtraGet(ctx.Request.Context(), "/api/task/get_all_map_nav_data", nil)
	if err != nil {
		gf.Failed().SetMsg("获取地图导航数据失败").SetData(err.Error()).Regin(ctx)
		return
	}
	gf.Success().SetMsg("获取地图导航数据成功").SetData(data).Regin(ctx)
}

func (api *Index) GetPcdMapList(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	tenant := tenantID(ctx, param)
	mapDB := dao.Query().RobotdogMap
	where := []dao.Condition{mapDB.TenantID.Eq(tenant)}
	if name := stringValue(param, "name", ""); name != "" {
		where = append(where, mapDB.Name.Like("%"+name+"%"))
	}
	offset, limit := pageArgs(param)
	rows, total, err := mapDB.WithContext(ctx).Where(where...).Order(mapDB.ID.Desc()).FindByPage(offset, limit)
	if err != nil {
		gf.Failed().SetMsg("获取点云地图列表失败").SetData(err).Regin(ctx)
		return
	}
	list := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		list = append(list, pcdMapData(row))
	}
	gf.Success().SetMsg("获取点云地图列表成功").SetData(map[string]interface{}{"list": list, "total": total}).Regin(ctx)
}

func (api *Index) UploadMap(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	tenant := tenantID(ctx, param)
	now := time.Now()
	mapURL := cleanMinioObjectPath(stringValue(param, "url", ""))
	files := pcdUploadFiles(ctx)
	if len(files) > 0 {
		cfg := loadRobotdogMinioConfig()
		client, err := cfg.client()
		if err != nil {
			gf.Failed().SetMsg("连接MinIO失败").SetData(err.Error()).Regin(ctx)
			return
		}
		if err := ensureRobotdogMinioBucket(ctx.Request.Context(), client, cfg); err != nil {
			gf.Failed().SetMsg("初始化MinIO桶失败").SetData(err.Error()).Regin(ctx)
			return
		}
		prefix := fmt.Sprintf("maps/%d", now.UnixNano())
		for i, file := range files {
			if strings.ToLower(filepath.Ext(file.Filename)) != ".pcd" {
				gf.Failed().SetMsg("地图文件仅支持.pcd格式").SetData(file.Filename).Regin(ctx)
				return
			}
			filename := filepath.Base(file.Filename)
			if filename == "." || filename == string(filepath.Separator) || strings.Contains(filename, "..") {
				filename = fmt.Sprintf("layer_%d.pcd", i+1)
			}
			src, err := file.Open()
			if err != nil {
				gf.Failed().SetMsg("读取地图文件失败").SetData(err.Error()).Regin(ctx)
				return
			}
			objectName := prefix + "/" + filename
			_, err = client.PutObject(ctx.Request.Context(), cfg.Bucket, objectName, src, file.Size, minio.PutObjectOptions{ContentType: "application/octet-stream"})
			_ = src.Close()
			if err != nil {
				gf.Failed().SetMsg("上传地图文件到MinIO失败").SetData(err.Error()).Regin(ctx)
				return
			}
		}
		mapURL = prefix
	}
	pointMap := &model.RobotdogMap{
		TenantID:   tenant,
		Name:       stringValue(param, "name", "点云地图"),
		Format:     stringValue(param, "format", "pcd"),
		URL:        mapURL,
		PreviewURL: cleanUploadURL(stringValue(param, "preview_url", "")),
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
		gf.Failed().SetMsg("请上传PCD地图文件或传入地图目录URL").Regin(ctx)
		return
	}
	if err := dao.Query().RobotdogMap.WithContext(ctx).Create(pointMap); err != nil {
		gf.Failed().SetMsg("上传地图失败").SetData(err).Regin(ctx)
		return
	}
	gf.Success().SetMsg("上传地图成功").SetData(pcdMapData(pointMap)).Regin(ctx)
}

func (api *Index) UploadPcdMap(ctx *gf.GinCtx) {
	api.UploadMap(ctx)
}
