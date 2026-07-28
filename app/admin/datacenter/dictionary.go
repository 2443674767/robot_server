package datacenter

import (
	"gofly/dao"
	"gofly/utils/gf"
	"gofly/utils/tools/gconv"
	"time"
)

// 字典数据
type Dictionary struct{ NoNeedAuths []string }

func init() {
	gf.Register(&Dictionary{NoNeedAuths: []string{"getTableDataForm"}})
}

// 获取列表
func (api *Dictionary) GetList(ctx *gf.GinCtx) {
	pageNo := gconv.Int(ctx.DefaultQuery("page", "1"))
	pageSize := gconv.Int(ctx.DefaultQuery("pageSize", "10"))
	//搜索添条件
	param, _ := gf.RequestParam(ctx)
	query := dao.DB().Table(dao.Use().TableName(gf.String(param["tablename"])))
	query = query.Where("group_id = ?", param["group_id"])
	if title, ok := param["title"]; ok && title != "" {
		query = query.Where("keyname like ?", "%"+gconv.String(title)+"%")
	}
	if status, ok := param["status"]; ok && status != "" {
		query = query.Where("status = ?", status)
	}
	if createtime, ok := param["createtime"]; ok && createtime != "" {
		datetime_arr := gf.SplitAndStr(gf.String(createtime), ",")
		query = query.Where("created_at between ? and ?", gf.Slice{datetime_arr[0], datetime_arr[1]})
	}
	var list gf.List
	var totalCount int64 // 总数
	query.Count(&totalCount)
	err := query.Offset(dao.Offset(pageNo, pageSize)).Limit(pageSize).Order("id asc").Find(&list).Error
	if err != nil {
		gf.Failed().SetMsg(err.Error()).Regin(ctx)
	} else {
		for _, val := range list {
			if _, ok := val["image"]; ok && !gf.IsEmpty(val["image"]) {
				val["image"] = gf.GetFullUrl(gf.String(val["image"]))
			}
		}
		gf.Success().SetMsg("获取全部列表").SetData(gf.Map{
			"page":     pageNo,
			"pageSize": pageSize,
			"total":    totalCount,
			"items":    list}).Regin(ctx)
	}
}

// 保存
func (api *Dictionary) Save(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	var f_id = gf.GetEditId(param["id"])
	tablename := gf.String(param["tablename"])
	delete(param, "tablename")
	if f_id == 0 {
		param["data_from"] = "admin"
		param["created_at"] = time.Now()
		tx := dao.DB().Table(dao.Use().TableName(tablename)).Create(&param)
		if tx.Error != nil {
			gf.Failed().SetMsg("添加失败").SetData(tx.Error).Regin(ctx)
		} else {
			dao.DB().Table(dao.Use().TableName(tablename)).Where("id", param["@id"]).Update("weigh", param["@id"])
			gf.Success().SetMsg("添加成功！").SetData(param).Regin(ctx)
		}
	} else {
		err := dao.DB().Table(dao.Use().TableName(tablename)).Where("id", f_id).Updates(param).Error
		if err != nil {
			gf.Failed().SetMsg("更新失败").SetData(err).Regin(ctx)
		} else {
			gf.Success().SetMsg("更新成功！").Regin(ctx)
		}
	}
}

// 更新状态
func (api *Dictionary) UpStatus(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	tx := dao.DB().Table(dao.Use().TableName(gf.String(param["tablename"]))).Where("id", param["id"]).Update("status", param["status"])
	if tx.Error != nil {
		gf.Failed().SetMsg("更新失败！").SetData(tx.Error).Regin(ctx)
	} else {
		msg := "更新成功！"
		if tx.RowsAffected == 0 {
			msg = "暂无数据更新"
		}
		gf.Success().SetMsg(msg).Regin(ctx)
	}
}

// 删除
func (api *Dictionary) Del(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	err := dao.DB().Table(dao.Use().TableName(gf.String(param["tablename"]))).Where("id = ?", param["id"]).Delete(nil).Error
	if err != nil {
		gf.Failed().SetMsg("删除失败").SetData(err).Regin(ctx)
	} else {
		gf.Success().SetMsg("删除成功！").Regin(ctx)
	}
}

// 使用指定数据-表单生成使用
func (api *Dictionary) GetTableDataForm(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	var custom interface{}
	if customstr, ok := param["custom"]; ok && customstr != "" {
		custom = gf.StringToJSON(gf.String(customstr))
	}
	if gf.DbHaseField(gf.String(param["tablename"]), "pid") {
		var list gf.List
		err := dao.DB().Table(dao.Use().TableName(gf.String(param["tablename"]))).Where(custom).Select("id,id as value,pid," + param["showfield"].(string) + " as label").Find(&list).Error
		if err != nil {
			gf.Failed().SetMsg("使用数据表数据失败！").SetData(err).Regin(ctx)
		} else {
			list = gf.GetTreeArray(list, 0, "")
			listarray := gf.GetTreeToList(list, "label")
			gf.Success().SetMsg("使用数据表数据列表").SetData(listarray).Regin(ctx)
		}
	} else {
		var list gf.List
		err := dao.DB().Table(dao.Use().TableName(gf.String(param["tablename"]))).Where(custom).Select("id as value," + param["showfield"].(string) + " as label").Find(&list).Error
		if err != nil {
			gf.Failed().SetMsg("使用数据表数据失败！").SetData(err).Regin(ctx)
		} else {
			gf.Success().SetMsg("使用数据表数据列表").SetData(list).Regin(ctx)
		}
	}
}
