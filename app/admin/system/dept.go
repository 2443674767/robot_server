package system

import (
	"gofly/dao"
	"gofly/dao/model"
	"gofly/utils/gf"
	"time"
)

// 部门管理
type Dept struct{ NoNeedAuths []string }

func init() {
	gf.Register(&Dept{NoNeedAuths: []string{"getParent"}})
}

// 获取部门列表
func (api *Dept) GetList(ctx *gf.GinCtx) {
	deptDB := dao.Query().AdminAuthDept
	//查找条件
	param, _ := gf.RequestParam(ctx)
	var whereMap []dao.Condition
	whereMap = append(whereMap, deptDB.TenantID.Eq(ctx.GetInt32("tenant_id")))
	if name, ok := param["name"]; ok && name != "" {
		whereMap = append(whereMap, deptDB.Name.Like("%"+gf.String(name)+"%"))
	}
	if status, ok := param["status"]; ok && status != "" {
		whereMap = append(whereMap, deptDB.Status.Eq(gf.Int8(status)))
	}
	if createtime, ok := param["createtime"]; ok && createtime != "" {
		datetime_arr := gf.SplitAndStr(gf.String(createtime), ",")
		whereMap = append(whereMap, deptDB.CreatedAt.Between(gf.StrToTime(datetime_arr[0]+" 00:00"), gf.StrToTime(datetime_arr[1]+" 23:59")))
	}
	var list gf.List
	err := deptDB.WithContext(ctx).Where(whereMap...).Order(deptDB.Weigh.Asc()).Scan(&list)
	if err != nil {
		gf.Failed().SetMsg("获取失败！" + err.Error()).Regin(ctx)
		return
	}
	if len(list) > 0 {
		list = gf.GetTreeArray(list, 0, "")
	}
	gf.Success().SetMsg("获取部门列表").SetData(list).Regin(ctx)
}

// 获取部门列表-表单
func (api *Dept) GetParent(ctx *gf.GinCtx) {
	tenantID := ctx.GetInt32("tenant_id") //当前商户ID
	deptDB := dao.Query().AdminAuthDept
	var list gf.List
	err := deptDB.WithContext(ctx).Where(deptDB.Status.Eq(0), deptDB.TenantID.Eq(tenantID)).Select(deptDB.ID, deptDB.Pid, deptDB.Name).Order(deptDB.Weigh.Asc()).Scan(&list)
	if err != nil {
		gf.Failed().SetMsg("获取失败！" + err.Error()).Regin(ctx)
		return
	}
	if len(list) > 0 {
		list = gf.GetMenuChildrenArray(list, 0, "pid")
	}
	gf.Success().SetMsg("获取部门列表").SetData(list).Regin(ctx)
}

// 保存
func (api *Dept) Save(ctx *gf.GinCtx) {
	deptDB := dao.Query().AdminAuthDept
	param, _ := gf.RequestParam(ctx)
	var f_id = gf.GetEditId(param["id"])
	if f_id == 0 {
		param["account_id"] = ctx.GetInt64("uid")      //当前用户ID
		param["tenant_id"] = ctx.GetInt32("tenant_id") //当前租户
		param["created_at"] = time.Now()
		txDB := deptDB.WithContext(ctx).UnderlyingDB().Model(&model.AdminAuthDept{}).Create(&param)
		if txDB.Error != nil {
			gf.Failed().SetMsg("添加失败").SetData(txDB.Error).Regin(ctx)
		} else {
			deptDB.WithContext(ctx).Where(deptDB.ID.Eq(gf.Int32(param["id"]))).Update(deptDB.Weigh, param["id"])
			gf.Success().SetMsg("添加成功！").SetData(param["id"]).Regin(ctx)
		}
	} else {
		delete(param, "children")
		delete(param, "spacer")
		res, err := deptDB.WithContext(ctx).Where(deptDB.ID.Eq(gf.Int32(f_id))).Updates(param)
		if err != nil {
			gf.Failed().SetMsg("更新失败").SetData(err).Regin(ctx)
		} else {
			gf.Success().SetMsg("更新成功！").SetData(res).Regin(ctx)
		}
	}
}

// 更新状态
func (api *Dept) UpStatus(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	deptDB := dao.Query().AdminAuthDept
	res, err := deptDB.WithContext(ctx).Where(deptDB.ID.Eq(gf.Int32(param["id"]))).Update(deptDB.Status, param["status"])
	if err != nil {
		gf.Failed().SetMsg("更新失败！").SetData(err).Regin(ctx)
	} else {
		gf.Success().SetMsg("更新成功！").SetData(res).Regin(ctx)
	}
}

// 更新父级-拖拽更新父id
func (api *Dept) Upgrouppid(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	deptDB := dao.Query().AdminAuthDept
	res, err := deptDB.WithContext(ctx).Where(deptDB.ID.In(gf.InterfaceToInt32(param["ids"])...)).Update(deptDB.Pid, param["pid"])
	if err != nil {
		gf.Failed().SetMsg("更新失败！").SetData(err).Regin(ctx)
	} else {
		gf.Success().SetMsg("更新成功！").SetData(res).Regin(ctx)
	}
}

// 删除
func (api *Dept) Del(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	deptDB := dao.Query().AdminAuthDept
	res, err := deptDB.WithContext(ctx).Where(deptDB.ID.In(gf.InterfaceToInt32(param["ids"])...)).Delete()
	if err != nil {
		gf.Failed().SetMsg("删除失败").SetData(err).Regin(ctx)
	} else {
		gf.Success().SetMsg("删除成功！").SetData(res).Regin(ctx)
	}
}
