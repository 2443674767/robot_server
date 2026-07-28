package datacenter

import (
	"gofly/dao"
	"gofly/dao/model"
	"gofly/utils/gf"
)

type Dicgroup struct{}

func init() {
	gf.Register(&Dicgroup{})
}

// 获取列表
func (api *Dicgroup) GetList(ctx *gf.GinCtx) {
	groupDB := dao.Query().DictionaryGroup
	list, err := groupDB.WithContext(ctx).Select(groupDB.ID, groupDB.TenantID, groupDB.Title, groupDB.Remark, groupDB.Tablename, groupDB.Status, groupDB.Weigh, groupDB.DataFrom, groupDB.DbWay).Order(groupDB.Weigh.Asc()).Find()
	if err != nil {
		gf.Failed().SetMsg("获取列表失败!" + err.Error()).Regin(ctx)
		return
	}
	gf.Success().SetMsg("获取列表").SetData(list).Regin(ctx)
}

// 保存
func (api *Dicgroup) Save(ctx *gf.GinCtx) {
	groupDB := dao.Query().DictionaryGroup
	param, _ := gf.RequestParam(ctx)
	if param["db_way"] == "sys" {
		param["tablename"] = "dictionary_data"
	}
	var f_id = gf.GetEditId(param["id"])
	group := model.DictionaryGroup{
		TenantID:  ctx.GetInt32("tenant_id"),
		Title:     gf.String(param["title"]),
		Remark:    gf.String(param["remark"]),
		Tablename: gf.String(param["tablename"]),
		DataFrom:  "admin",
		DbWay:     gf.String(param["db_way"]),
		Status:    gf.Int8(param["status"]),
		Weigh:     gf.Int32(param["weigh"]),
	}

	if f_id == 0 {
		err := groupDB.WithContext(ctx).Create(&group)
		if err != nil {
			gf.Failed().SetMsg("添加失败").SetData(group).Regin(ctx)
		} else {
			groupDB.WithContext(ctx).Where(groupDB.ID.Eq(group.ID)).Update(groupDB.Weigh, group.ID)
			gf.Success().SetMsg("添加成功！").SetData(group.ID).Regin(ctx)
		}
	} else {
		res, err := groupDB.WithContext(ctx).Where(groupDB.ID.Eq(gf.Int32(f_id))).Updates(group)
		if err != nil {
			gf.Failed().SetMsg("更新失败").SetData(err).Regin(ctx)
		} else {
			gf.Success().SetMsg("更新成功！").SetData(res).Regin(ctx)
		}
	}
}

// 删除
func (api *Dicgroup) Del(ctx *gf.GinCtx) {
	groupDB := dao.Query().DictionaryGroup
	param, _ := gf.RequestParam(ctx)
	var tablename string
	groupDB.WithContext(ctx).Where(groupDB.ID.Eq(gf.Int32(param["id"]))).Select(groupDB.Tablename).Scan(&tablename)
	res, err := groupDB.WithContext(ctx).Where(groupDB.ID.Eq(gf.Int32(param["id"]))).Delete()
	if err != nil {
		gf.Failed().SetMsg("删除失败").SetData(err).Regin(ctx)
	} else {
		dao.DB().Exec("DELETE FROM "+dao.Use().TableName(tablename)+" WHERE group_id IN (?)", param["ids"])
		gf.Success().SetMsg("删除成功！").SetData(res).Regin(ctx)
	}
}
