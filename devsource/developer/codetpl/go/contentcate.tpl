package packageName

import (
	"gofly/dao"
	"gofly/dao/model"
	"gofly/utils/gf"
)

// {fileName}分类
type Replace struct{ NoNeedAuths []string }

func init() {
	gf.Register(&Replace{NoNeedAuths: []string{"getTree"}})
}

// 获取分类列表-tree
func (api *Replace) GetTree(ctx *gf.GinCtx) {
	cateDB := dao.Query().ModelCateName
	//搜索添条件
	param, _ := gf.RequestParam(ctx)
	var whereMap []dao.Condition
	//{ReplayWhereTenantID}
	if name, ok := param["name"]; ok && name != "" {
		whereMap = append(whereMap, cateDB.Name.Like("%"+gf.String(name)+"%"))
	}
	if status, ok := param["status"]; ok && status != "" {
		whereMap = append(whereMap, cateDB.Status.Eq(gf.Int8(status)))
	}
	if createtime, ok := param["createtime"]; ok && createtime != "" {
		datetime_arr := gf.SplitAndStr(gf.String(createtime), ",")
		whereMap = append(whereMap, cateDB.CreatedAt.Between(gf.StrToTime(datetime_arr[0]), gf.StrToTime(datetime_arr[1])))
	}
	var list gf.List
	err := cateDB.WithContext(ctx).Where(whereMap...).Order(cateDB.ID.Asc()).Scan(&list)
	if err != nil {
		gf.Failed().SetMsg(err.Error()).Regin(ctx)
	} else {
		for _, val := range list {
			val["key"] = val["id"]
		}
		list = gf.GetMenuChildrenArray(list, 0, "pid")
		gf.Success().SetMsg("获取分类树形列表").SetData(list).Regin(ctx)
	}
}

// 保存
func (api *Replace) Save(ctx *gf.GinCtx) {
	cateDB := dao.Query().ModelCateName
	param, _ := gf.RequestParam(ctx)
	var f_id = gf.GetEditId(param["id"])
	if f_id == 0 {
		//{ReplaySaveTenantID}
		txDB := cateDB.WithContext(ctx).UnderlyingDB().Model(&model.ModelCateName{}).Create(&param)
		if txDB.Error != nil {
			gf.Failed().SetMsg("添加失败").SetData(txDB.Error).Regin(ctx)
		} else {
			cateDB.WithContext(ctx).Where(cateDB.ID.Eq(gf.Int32(param["id"]))).Update(cateDB.Weigh, param["id"])
			gf.Success().SetMsg("添加成功！").SetData(param["id"]).Regin(ctx)
		}
	} else {
		delete(param, "showbtn")
		delete(param, "key")
		res, err := cateDB.WithContext(ctx).Where(cateDB.ID.Eq(gf.Int32(f_id))).Updates(param)
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
	cateDB := dao.Query().ModelCateName
	res, err := cateDB.WithContext(ctx).Where(cateDB.ID.Eq(gf.Int32(param["id"]))).Update(cateDB.Status, param["status"])
	if err != nil {
		gf.Failed().SetMsg("更新失败！").SetData(err).Regin(ctx)
	} else {
		gf.Success().SetMsg("更新成功！").SetData(res).Regin(ctx)
	}
}

// 删除
func (api *Replace) Del(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	cateDB := dao.Query().ModelCateName
	res, err := cateDB.WithContext(ctx).Where(cateDB.ID.In(gf.InterfaceToInt32(param["ids"])...)).Delete()
	if err != nil {
		gf.Failed().SetMsg("删除失败").SetData(err).Regin(ctx)
	} else {
		//{deleteSubData}
		gf.Success().SetMsg("删除成功！").SetData(res).Regin(ctx)
	}
}
