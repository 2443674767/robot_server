package user

import (
	"gofly/dao"
	"gofly/dao/model"
	"gofly/utils/gf"
	"strings"
)

// 获取权限菜单
func GetMenuArray(ctx *gf.GinCtx, pdata []*model.AdminAuthRule, parent_id int32, roles []int32) []map[string]interface{} {
	var returnList []map[string]interface{}
	ruleDB := dao.Query().AdminAuthRule
	var one int8 = 1
	for _, v := range pdata {
		if v.Pid == parent_id {
			mid_item := map[string]interface{}{
				"path":      v.Routepath,
				"name":      v.Routename,
				"component": v.Component,
			}
			children := GetMenuArray(ctx, pdata, v.ID, roles)
			if children != nil {
				mid_item["children"] = children
			}
			//1.标题
			// var Menu_title interface{}
			// if v["locale"] != nil && v["locale"].String() != "" {
			// 	Menu_title = v["locale"]
			// } else {
			// 	Menu_title = v["title"]
			// }
			meta := map[string]interface{}{
				"locale": v.Locale,
				"title":  v.Title,
				"id":     v.ID,
			}
			//2.重定向
			if !gf.IsEmpty(v.Redirect) {
				mid_item["redirect"] = v.Redirect
			}
			//3.隐藏子菜单
			if v.Hidechildreninmenu == one {
				meta["hideChildrenInMenu"] = true
			}
			//3.图标
			if !gf.IsEmpty(v.Icon) {
				meta["icon"] = v.Icon
			}
			//4.缓存
			if v.Keepalive == one { //设置为true页面将不会被缓存 false=缓存
				meta["ignoreCache"] = false
			} else {
				meta["ignoreCache"] = true
			}
			//5.隐藏菜单
			if v.Hideinmenu == one {
				meta["hideInMenu"] = true
			}
			//6.在标签隐藏
			if v.Noaffix == one {
				meta["noAffix"] = true
			}
			//7.详情页在本业打开-用于配置详情页时左侧激活的菜单路径
			if v.Activemenu == one {
				meta["activeMenu"] = true
			}
			//8.是否需要登录鉴权
			if v.Requiresauth == one {
				meta["requiresAuth"] = true
			} else {
				meta["requiresAuth"] = false
			}
			//9.是否需要登录鉴权
			if v.Isext == one {
				meta["isExt"] = true
			}
			//10.是否需要登录鉴权
			if v.Onlypage == one {
				meta["onlypage"] = true
			} else {
				meta["onlypage"] = false
			}
			//11.按钮权限
			if len(roles) == 0 { //超级权限
				var permission []string
				ruleDB.WithContext(ctx).Where(ruleDB.Status.Eq(0), ruleDB.Type.Eq(2), ruleDB.Pid.Eq(v.ID), ruleDB.Permission.IsNotNull()).Pluck(ruleDB.Permission, &permission)
				if len(permission) > 0 {
					meta["btnroles"] = permission
				} else {
					meta["btnroles"] = [1]string{"*"}
				}
			} else { //选择路由
				var permission []string
				ruleDB.WithContext(ctx).Where(ruleDB.Status.Eq(0), ruleDB.Type.Eq(2), ruleDB.Pid.Eq(v.ID), ruleDB.Pid.In(roles...), ruleDB.Permission.IsNotNull()).Pluck(ruleDB.Permission, &permission)
				if len(permission) > 0 {
					meta["btnroles"] = permission
				} else {
					var hasepermission []string
					ruleDB.WithContext(ctx).Where(ruleDB.Status.Eq(0), ruleDB.Type.Eq(2), ruleDB.Pid.Eq(v.ID), ruleDB.Permission.IsNotNull()).Pluck(ruleDB.Permission, &hasepermission)
					if hasepermission == nil {
						meta["btnroles"] = make([]interface{}, 0)
					} else {
						meta["btnroles"] = [1]string{"*"}
					}
				}
			}
			//赋值
			mid_item["meta"] = meta
			returnList = append(returnList, mid_item)
		}
	}
	return returnList
}

// 多维数组合并-权限
func ArrayMergeInt32(data []string) []int32 {
	var rule_ids_arr []int32
	for _, mainv := range data {
		ids_arr := strings.Split(mainv, `,`)
		for _, intv := range ids_arr {
			rule_ids_arr = append(rule_ids_arr, gf.Int32(intv))
		}
	}
	return rule_ids_arr
}
