package common

/**
* 系统消息
 */
import (
	"gofly/dao"
	"gofly/utils/gf"
)

// 用于自动注册路由
type Message struct{ NoNeedAuths []string }

func init() {
	gf.Register(&Message{NoNeedAuths: []string{"*"}})
}

// 获取消息列表
func (api *Message) GetList(ctx *gf.GinCtx) {
	userID := ctx.GetInt32("userID")
	messageDB := dao.Query().CommonMessage
	var usertype int8 = 1 //用户类型
	list, err := messageDB.WithContext(ctx).Where(messageDB.Usertype.In(0, usertype), messageDB.Touid.Eq(userID)).
		Select(messageDB.ID, messageDB.Type, messageDB.Title, messageDB.Path, messageDB.Content, messageDB.Isread, messageDB.CreatedAt).Order(messageDB.ID.Desc()).Find()
	if err != nil {
		gf.Failed().SetMsg("加载数据失败").Regin(ctx)
	} else {
		totalCount, _ := messageDB.WithContext(ctx).Where(messageDB.Usertype.In(0, usertype), messageDB.Touid.Eq(userID)).Count()
		gf.Success().SetMsg("获取全部消息列表").SetData(gf.Map{
			"total": totalCount,
			"items": list,
		}).Regin(ctx)
	}

}

// 设置为已读
func (api *Message) Read(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	messageDB := dao.Query().CommonMessage
	res2, err := messageDB.WithContext(ctx).Where(messageDB.ID.In(gf.InterfaceToInt32(param["ids"])...)).Update(messageDB.Isread, 1)
	if err != nil {
		gf.Failed().SetMsg("更新失败！").Regin(ctx)
	} else {
		gf.Success().SetMsg("更新成功！").SetData(res2).Regin(ctx)
	}
}
