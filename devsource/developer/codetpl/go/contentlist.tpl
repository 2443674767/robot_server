package packageName

import (
	"gofly/dao"
	"gofly/dao/model"
	"gofly/utils/extend/excelexport"
	"gofly/utils/gf"
)

// {fileName}数据
type Replace struct{}

func init() {
	gf.Register(&Replace{})
}

//{ListVO}

// 获取列表
func (api *Replace) GetList(ctx *gf.GinCtx) {
	contentDB := dao.Query().ModelName
	pageNo := gf.Int(ctx.DefaultQuery("page", "1"))
	pageSize := gf.Int(ctx.DefaultQuery("pageSize", "10"))
	//搜索添条件
	param, _ := gf.RequestParam(ctx)
	var whereMap []dao.Condition
	//{ReplayWhereTenantID}
//{ReplaySearch}
	list, totalCount, err := contentDB.WithContext(ctx).Where(whereMap...).
		{replayFields}
		Order(contentDB.ID.Desc()).FindByPage(dao.Offset(pageNo, pageSize), pageSize)
	if err != nil {
		gf.Failed().SetMsg(err.Error()).Regin(ctx)
	} else {
//{ReplayFieldVal}
		gf.Success().SetMsg("获取全部列表").SetData(gf.Map{
			"page":     pageNo,
			"pageSize": pageSize,
			"total":    totalCount,
			"items":    {listName}}).Regin(ctx)
	}
}

// 保存
func (api *Replace) Save(ctx *gf.GinCtx) {
	contentDB := dao.Query().ModelName
	param, _ := gf.RequestParam(ctx)
	var f_id = gf.GetEditId(param["id"])
//{ReplayArrayToStr}
	if f_id == 0 {
		//{ReplaySaveTenantID}
		//{replayCreatedAt}
		txDB := contentDB.WithContext(ctx).UnderlyingDB().Model(&model.ModelName{}).Create(&param)
		if txDB.Error != nil {
			gf.Failed().SetMsg("添加失败").SetData(txDB.Error).Regin(ctx)
		} else {
			//{replayUpdateWeigh}
			gf.Success().SetMsg("添加成功！").SetData(param["id"]).Regin(ctx)
		}
	} else {
		//{EditDeleteField}
		res, err := contentDB.WithContext(ctx).Where(contentDB.ID.Eq(gf.Int32(f_id))).Updates(param)
		if err != nil {
			gf.Failed().SetMsg("更新失败").SetData(err).Regin(ctx)
		} else {
			gf.Success().SetMsg("更新成功！").SetData(res).Regin(ctx)
		}
	}
}

// 更新状态
func (api *Replace) UpStatus(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	contentDB := dao.Query().ModelName
	res, err := contentDB.WithContext(ctx).Where(contentDB.ID.Eq(gf.Int32(param["id"]))).Update(contentDB.Status, param["status"])
	if err != nil {
		gf.Failed().SetMsg("更新失败！").SetData(err).Regin(ctx)
	} else {
		gf.Success().SetMsg("更新成功！").SetData(res).Regin(ctx)
	}
}

// 删除
func (api *Replace) Del(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	contentDB := dao.Query().ModelName
	res, err := contentDB.WithContext(ctx).Where(contentDB.ID.In(gf.InterfaceToInt32(param["ids"])...)).Delete()
	if err != nil {
		gf.Failed().SetMsg("删除失败").SetData(err).Regin(ctx)
	} else {
		gf.Success().SetMsg("删除成功！").SetData(res).Regin(ctx)
	}
}

// 获取内容
func (api *Replace) GetContent(ctx *gf.GinCtx) {
	id := ctx.DefaultQuery("id", "")
	if id == "" {
		gf.Failed().SetMsg("请传参数id").Regin(ctx)
	} else {
		contentDB := dao.Query().ModelName
		var data gf.Map
		err := contentDB.WithContext(ctx).Where(contentDB.ID.Eq(gf.Int32(id))).Scan(&data)
		if err != nil {
			gf.Failed().SetMsg("获取内容失败").SetData(err).Regin(ctx)
		} else {
//{ReplayToJSON}
			gf.Success().SetMsg("获取内容成功！").SetData(data).Regin(ctx)
		}
	}
}

// 导出项目数据到excel
func (api *Replace) ExportExcel(ctx *gf.GinCtx) {
	contentDB := dao.Query().ModelName
	//搜索添条件
	param, _ := gf.RequestParam(ctx)
	var whereMap []dao.Condition
	//{ReplayWhereTenantID}
//{ReplaySearch}
	var list excelexport.List
	err := contentDB.WithContext(ctx).Where(whereMap...).Scan(&list)
	var columns = make([]interface{}, 0)
	if _, ok := param["columns"]; ok {
		columns = gf.Interfaces(param["columns"])
	}
	_, err = excelexport.ExportToExcel(list, columns, "{tablename}", ctx)
	if err != nil {
		gf.Failed().SetMsg("导出失败").SetData(err).Regin(ctx)
		return
	}
}
