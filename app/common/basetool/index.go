package basetool

import (
	"gofly/dao"
	"gofly/utils/gf"
	"gofly/utils/tools/grand"
	"time"
)

// 公共工具
type Index struct {
	NoNeedLogin []string
	NoNeedAuths []string
}

func init() {
	gf.Register(&Index{NoNeedLogin: []string{"getCaptcha"}, NoNeedAuths: []string{"checkHaseRule"}})
}

/**
*  1获取验证码、图片验证码、手机验证码、邮箱验证码
 */
func (api *Index) GetCaptcha(ctx *gf.GinCtx) {
	typeStr := ctx.DefaultQuery("type", "")
	switch typeStr {
	case "image": //获取图片验证码
		data, err := gf.GenerateCaptcha()
		if err != nil {
			gf.Failed().SetMsg("获取图片验证码失败").Regin(ctx)
		} else {
			gf.Success().SetMsg("获取图片验证码成功").SetData(data).Regin(ctx)
		}
	case "mobile": //获取手机验证码
		mobile := ctx.DefaultQuery("mobile", "")
		if mobile == "" {
			gf.Failed().SetMsg("请填写手机").Regin(ctx)
		} else {
			code := grand.Digits(6)
			gf.SetVerifyCode(mobile, code) //验证码存在本地缓存中
			gf.Success().SetMsg("获取手机验证码").SetData(code).Regin(ctx)
		}
	case "email": //获取邮箱验证码
		email := ctx.DefaultQuery("email", "")
		if email == "" {
			gf.Failed().SetMsg("请填写邮箱").Regin(ctx)
		} else {
			res, erro := gf.SendEmail(ctx, []string{email}, "", "")
			if erro == nil {
				gf.Success().SetMsg("获取验证码").SetData(res).Regin(ctx)
			} else {
				gf.Failed().SetMsg(erro.Error()).Regin(ctx)
			}
		}
	case "qrcode": //扫码登录
		keyval := ctx.DefaultQuery("keyval", "")
		if keyval == "" {
			gf.Failed().SetMsg("选择扫码类型").Regin(ctx)
		} else {
			addtime, _ := time.ParseDuration("60s")
			expiretime := time.Now().Add(addtime).UnixMilli()
			gf.Success().SetMsg("获取扫码内容").SetData(gf.Map{"url": keyval, "expiretime": expiretime}).Regin(ctx)
		}
	}
}

// 1 获取使用字典数据接口
func (api *Index) GetDicData(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	dicDB := dao.Query().DictionaryGroup
	var tablename string
	dicDB.WithContext(ctx).Where(dicDB.ID.Eq(gf.Int32(param["group_id"]))).Select(dicDB.Tablename).Scan(&tablename)
	getfield := "id,keyname as label,keyvalue as value"
	if gf.DbHaseField(tablename, "tagcolor") {
		getfield = "id,keyname as label,keyvalue as value,tagcolor as color"
	}
	rawSQL := "SELECT " + getfield + " FROM " + dao.Use().TableName(tablename) + " WHERE group_id = ? AND status = 0"
	// 执行原生查询
	var list []map[string]any
	err := dao.DB().Raw(rawSQL, param["group_id"]).Scan(&list).Error
	if err != nil {
		gf.Failed().SetMsg("获取字典数据失败！").SetData(err).Regin(ctx)
	} else {
		gf.Success().SetMsg("获取字典数据列表").SetData(list).Regin(ctx)
	}
}

// 检查路由是否已存在-因为路由不能重复
func (api *Index) CheckHaseRule(c *gf.GinCtx) {
	param, _ := gf.RequestParam(c)
	rawSQL := `
SELECT EXISTS(
    SELECT 1 
    FROM  ` + dao.Use().TableName(gf.String(param["codelocation"])) + ` 
    WHERE id != ? AND routename IN (?)
)
`
	// 执行原生查询
	var hase bool
	err := dao.DB().Raw(rawSQL, param["id"], param["routename"]).Scan(&hase).Error
	if err != nil {
		gf.Failed().SetMsg("检查路由失败").SetData(err).Regin(c)
	} else {
		gf.Success().SetMsg("检查路由成功！").SetData(!hase).Regin(c)
	}
}
