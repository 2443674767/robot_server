package system

import (
	"gofly/dao"
	"gofly/dao/model"
	"gofly/utils/gf"
	"strings"
	"time"
)

// 菜单管理
type Rule struct{ NoNeedAuths []string }

func init() {
	gf.Register(&Rule{NoNeedAuths: []string{"getParent", "getRoutes", "getContent"}})
}

// 1获取列表
func (api *Rule) GetList(ctx *gf.GinCtx) {
	ruleDB := dao.Query().AdminAuthRule
	var menuList gf.List
	err := ruleDB.WithContext(ctx).Select(ruleDB.ID, ruleDB.Pid, ruleDB.Type, ruleDB.Title, ruleDB.Locale, ruleDB.Icon, ruleDB.Permission, ruleDB.Path, ruleDB.Component, ruleDB.Weigh, ruleDB.Status, ruleDB.CreatedAt).Order(ruleDB.Weigh.Asc()).Scan(&menuList)
	if err != nil {
		gf.Failed().SetMsg("获取数据失败").SetData(err).Regin(ctx)
		return
	}
	if menuList == nil {
		menuList = make(gf.List, 0)
	}
	for _, val := range menuList {
		if val["title"] == "" {
			val["title"] = val["locale"]
		}
	}
	menuList = gf.GetTreeArray(menuList, 0, "")
	gf.Success().SetMsg("获取全部菜单列表").SetData(menuList).Regin(ctx)
}

// 2获取列表-获取选项列表
func (api *Rule) GetParent(ctx *gf.GinCtx) {
	id := ctx.DefaultQuery("id", "0")
	ruleDB := dao.Query().AdminAuthRule
	var menuList gf.List
	err := ruleDB.WithContext(ctx).Where(ruleDB.Type.In(0, 1), ruleDB.ID.Neq(gf.Int32(id))).Select(ruleDB.ID, ruleDB.Pid, ruleDB.Title, ruleDB.Locale, ruleDB.Routepath).Order(ruleDB.Weigh.Asc()).Scan(&menuList)
	if err != nil {
		gf.Failed().SetMsg("获取数据失败").SetData(err).Regin(ctx)
	} else {
		if menuList == nil {
			menuList = make(gf.List, 0)
		}
		for _, val := range menuList {
			if gf.String(val["title"]) == "" {
				val["title"] = val["locale"]
			}
		}
		menuTree := gf.GetMenuChildrenArray(menuList, 0, "pid")
		gf.Success().SetMsg("菜单父级数据！").SetData(gf.Map{"tree": menuTree, "list": menuList}).Regin(ctx)
	}
}

// 获取权限选择的路由列表
func (api *Rule) GetRoutes(ctx *gf.GinCtx) {
	filePath := "runtime/app/routers.txt"
	list, err := gf.ReaderFileByline(filePath)
	if err != nil {
		gf.Failed().SetMsg("获取数据失败").SetData(err).Regin(ctx)
		return
	}
	var nlist []interface{} = make([]interface{}, 0)
	for _, val := range list {
		item := strings.Split(gf.String(val), ":")
		if len(item) == 2 && !strings.HasPrefix(item[1], "/.js") && !strings.Contains(item[1], "/developer/") && !strings.HasPrefix(item[1], "/common/") {
			nlist = append(nlist, gf.Map{"method": item[0], "path": item[1]})
		}
	}
	gf.Success().SetMsg("获取权限选择的路由列表").SetData(nlist).Regin(ctx)
}

// 3保存、编辑菜单
func (api *Rule) Save(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	ruleDB := dao.Query().AdminAuthRule
	var f_id = gf.GetEditId(param["id"])
	if f_id == 0 {
		param["uid"] = ctx.GetInt64("uid") //当前用户ID
		param["created_at"] = time.Now()
		txDB := ruleDB.WithContext(ctx).UnderlyingDB().Model(&model.AdminAuthRule{}).Create(param)
		if txDB.Error != nil {
			gf.Failed().SetMsg("添加菜单失败").SetData(txDB.Error).Regin(ctx)
		} else {
			ruleDB.WithContext(ctx).Where(ruleDB.ID.Eq(gf.Int32(param["id"]))).Update(ruleDB.Weigh, param["id"])
			gf.Success().SetMsg("添加成功！").SetData(param["id"]).Regin(ctx)
		}
	} else {
		res, err := ruleDB.WithContext(ctx).Where(ruleDB.ID.Eq(gf.Int32(f_id))).Select(
			// 使用Select设置需要更新字段
			ruleDB.Title,
			ruleDB.Des,
			ruleDB.Locale,
			ruleDB.Weigh,
			ruleDB.Type,
			ruleDB.Pid,
			ruleDB.Icon,
			ruleDB.Routepath,
			ruleDB.Routename,
			ruleDB.Component,
			ruleDB.Redirect,
			ruleDB.Path,
			ruleDB.Permission,
			ruleDB.Status,
			ruleDB.Isext,
			ruleDB.Keepalive,
			ruleDB.Requiresauth,
			ruleDB.Hideinmenu,
			ruleDB.Hidechildreninmenu,
			ruleDB.Activemenu,
			ruleDB.Noaffix,
			ruleDB.Onlypage,
		).Updates(param)
		if err != nil {
			gf.Failed().SetMsg("更新菜单失败").SetData(err).Regin(ctx)
		} else {
			gf.Success().SetMsg("更新成功！").SetData(res).Regin(ctx)
		}
	}
}

// 4更新状态
func (api *Rule) UpStatus(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	ruleDB := dao.Query().AdminAuthRule
	res, err := ruleDB.WithContext(ctx).Where(ruleDB.ID.Eq(gf.Int32(param["id"]))).Update(ruleDB.Status, param["status"])
	if err != nil {
		gf.Failed().SetMsg("更新失败！").SetData(err).Regin(ctx)
	} else {
		msg := "更新成功！"
		if res.RowsAffected == 0 {
			msg = "暂无数据更新"
		}
		gf.Success().SetMsg(msg).Regin(ctx)
	}
}

// 删除菜单
func (api *Rule) Del(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	ruleDB := dao.Query().AdminAuthRule
	res, err := ruleDB.WithContext(ctx).Where(ruleDB.ID.In(gf.InterfaceToInt32(param["ids"])...)).Delete()
	if err != nil {
		gf.Failed().SetMsg("删除菜单失败").SetData(err).Regin(ctx)
	} else {
		//删除子类数据
		ruleDB.WithContext(ctx).Where(ruleDB.Pid.In(gf.InterfaceToInt32(param["ids"])...)).Delete()
		gf.Success().SetMsg("删除成功！").SetData(res).Regin(ctx)
	}
}

// 永久删除已经软删除菜单
func (api *Rule) EmptyRecyclebin(ctx *gf.GinCtx) {
	ruleDB := dao.Query().AdminAuthRule
	res, err := ruleDB.WithContext(ctx).Unscoped().Where(ruleDB.DeletedAt.IsNotNull()).Delete()
	if err != nil {
		gf.Failed().SetMsg("清空回收站失败").SetData(err).Regin(ctx)
	} else {
		gf.Success().SetMsg("清空回收站成功").SetData(res).Regin(ctx)
	}
}

// 获取内容
func (api *Rule) GetContent(ctx *gf.GinCtx) {
	id := ctx.DefaultQuery("id", "")
	ruleDB := dao.Query().AdminAuthRule
	if id == "" {
		gf.Failed().SetMsg("请传参数id").Regin(ctx)
	} else {
		data, err := ruleDB.WithContext(ctx).Where(ruleDB.ID.Eq(gf.Int32(id))).First()
		if err != nil {
			gf.Failed().SetMsg("获取内容失败").SetData(err).Regin(ctx)
		} else {
			gf.Success().SetMsg("获取内容成功！").SetData(data).Regin(ctx)
		}
	}
}
