package dashboard

import (
	"gofly/dao"
	"gofly/dao/model"
	"gofly/utils/gf"
)

/**
* 使用说明：
* 首页统计是根据业务需求数据来统计的，框架无法预知你的项目实际需求，我们只能内置一些方法仅供参考，
* 实际项目开发完成后，根据项目需求自己编写统计数据接口
* gf_youtablebane 是你的项目实际数据表(泛指)，不是实际测存在表，切记！自己根据需求开发出对应接口
 */
type Workplace struct{ NoNeedAuths []string }

func init() {
	gf.Register(&Workplace{NoNeedAuths: []string{"*"}})
}

// 1获取快捷操作
func (api *Workplace) GetQuick(ctx *gf.GinCtx) {
	tenantID := ctx.GetInt32("tenant_id") //当前商户ID
	homeDB := dao.Query().HomeQuickop
	var list []map[string]any
	err := homeDB.WithContext(ctx).Where(homeDB.TenantID.Eq(tenantID)).Or(homeDB.IsCommon.Eq(1)).Select(homeDB.ID, homeDB.UID, homeDB.PathURL, homeDB.Name, homeDB.Icon, homeDB.Type, homeDB.IsCommon, homeDB.Weigh).Order(homeDB.Weigh.Asc(), homeDB.ID.Asc()).Scan(&list)
	if err != nil {
		gf.Failed().SetMsg("获取快捷操作失败").SetData(err).Regin(ctx)
	} else {
		gf.Success().SetMsg("获取快捷操作数据").SetData(list).Regin(ctx)
	}
}

// 3保存快捷操作
func (api *Workplace) SaveQuick(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	homeDB := dao.Query().HomeQuickop
	var f_id = gf.GetEditId(param["id"])
	if f_id == 0 {
		param["uid"] = ctx.GetInt64("uid")             //当前用户
		param["tenant_id"] = ctx.GetInt32("tenant_id") //当前商户
		txDB := homeDB.WithContext(ctx).UnderlyingDB().Model(&model.HomeQuickop{}).Create(param)
		if txDB.Error != nil {
			gf.Failed().SetMsg("添加失败").SetData(txDB.Error).Regin(ctx)
		} else {
			homeDB.WithContext(ctx).Where(homeDB.ID.Eq(gf.Int32(param["id"]))).Update(homeDB.Weigh, param["id"])
			gf.Success().SetMsg("添加成功！").SetData(param["id"]).Regin(ctx)
		}
	} else {
		res, err := homeDB.WithContext(ctx).Where(homeDB.ID.Eq(gf.Int32(f_id))).Updates(param)
		if err != nil {
			gf.Failed().SetMsg("更新失败").SetData(err).Regin(ctx)
		} else {
			gf.Success().SetMsg("更新成功！").SetData(res).Regin(ctx)
		}
	}
}

// 3删除快捷操作
func (api *Workplace) DelQuick(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	homeDB := dao.Query().HomeQuickop
	res2, err := homeDB.WithContext(ctx).Where(homeDB.ID.Eq(gf.Int32(param["id"]))).Delete()
	if err != nil {
		gf.Failed().SetMsg("删除失败").SetData(err).Regin(ctx)
	} else {
		gf.Success().SetMsg("删除成功！").SetData(res2).Regin(ctx)
	}
}
