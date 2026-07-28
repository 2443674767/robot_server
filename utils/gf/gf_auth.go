package gf

import (
	"gofly/dao"
	"gofly/dao/model"
	"gofly/utils/plugin"
	"strings"
	"time"
)

/**
* 后台RBAC的权限管理
 */

// 检查权限
// path是当前请求路径
func CheckAuth(ctx *GinCtx, modelname string) bool {
	ARADB := dao.Query().AdminAuthRoleAccess
	var role_id []int32
	acerr := ARADB.WithContext(ctx).Where(ARADB.UID.Eq(ctx.GetInt64("uid"))).Pluck(ARADB.RoleID, &role_id)
	if acerr != nil || role_id == nil {
		return false
	}
	//1.判断是否有超级角色
	ARODB := dao.Query().AdminAuthRole
	super_role, rerr := ARODB.WithContext(ctx).Where(ARODB.ID.In(role_id...), ARODB.Rules.Eq("*")).Count()
	if rerr != nil {
		return false
	}
	ARUDB := dao.Query().AdminAuthRule
	if super_role != 0 { //超级角色
		if Bool(appConf_arr["superRoleAuth"]) {
			//1.需要查找是否已经把权限接口添加到数据库
			hasepath, ruerr := ARUDB.WithContext(ctx).Where(ARUDB.Status.Eq(0), ARUDB.Type.Eq(2), ARUDB.Path.Eq(ctx.FullPath())).Count()
			if ruerr == nil && hasepath != 0 {
				return true
			}
		} else {
			//2.不需添加权限数据直接返回
			return true
		}
	} else { //普通角色
		var rulesStr []string
		rerr := ARODB.WithContext(ctx).Where(ARODB.ID.In(role_id...)).Pluck(ARODB.Rules, &rulesStr)
		if rerr != nil {
			return false
		}
		var menu_ids []int32
		for _, s := range rulesStr {
			for _, id := range strings.Split(s, ",") {
				id = strings.TrimSpace(id)
				if id != "" {
					menu_ids = append(menu_ids, Int32(id))
				}
			}
		}
		if len(menu_ids) == 0 {
			return false
		}
		hasepath, ruerr := ARUDB.WithContext(ctx).Where(ARUDB.Status.Eq(0), ARUDB.Type.Eq(2), ARUDB.ID.In(menu_ids...), ARUDB.Path.Eq(ctx.FullPath())).Count()
		if ruerr == nil && hasepath != 0 {
			return true
		}
	}
	return false
}

// 添加登录日志
func AddloginLog(ctx *GinCtx, savedata Map) {
	ip := ctx.ClientIP()
	savedata["ip"] = ip
	savedata["type"] = "admin"
	savedata["created_at"] = time.Now().Format("2006-01-02 15:04:05")
	address, err := plugin.NewIpRegion(ip)
	if err == nil {
		savedata["address"] = address
	}
	ua := ctx.Request.Header.Get("User-Agent")
	// 判断操作系统
	if strings.Contains(ua, "Windows") {
		savedata["os"] = "Windows"
	} else if strings.Contains(ua, "Mac OS") {
		savedata["os"] = "macOS"
	} else if strings.Contains(ua, "OpenHarmony") {
		savedata["os"] = "OpenHarmony"
	} else if strings.Contains(ua, "Android") {
		savedata["os"] = "Android"
	} else if strings.Contains(ua, "Linux") {
		savedata["os"] = "Linux"
	} else if strings.Contains(ua, "iPhone") || strings.Contains(ua, "iPad") {
		savedata["os"] = "iOS"
	}
	// 判断浏览器
	if strings.Contains(ua, "Chrome") {
		savedata["browser"] = "Chrome"
	} else if strings.Contains(ua, "Safari") && !strings.Contains(ua, "Chrome") {
		savedata["browser"] = "Safari"
	} else if strings.Contains(ua, "Edge") {
		savedata["browser"] = "Edge"
	} else if strings.Contains(ua, "Firefox") {
		savedata["browser"] = "Firefox"
	}
	dao.Query().LoginLog.WithContext(ctx).UnderlyingDB().Model(&model.LoginLog{}).Create(savedata)
}
