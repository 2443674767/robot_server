package user

import (
	"gofly/dao"
	"gofly/utils/gf"
)

// 用于自动注册路由
type Account struct {
	NoNeedAuths []string
}

func init() {
	gf.Register(&Account{NoNeedAuths: []string{"GetMenu"}})
}

// 系统设置-获取菜单
func (api *Account) GetMenu(ctx *gf.GinCtx) {
	_, exists := ctx.Get("user") //当前用户
	ruleDB := dao.Query().AdminAuthRule
	roleDB := dao.Query().AdminAuthRole
	roleAccessDB := dao.Query().AdminAuthRoleAccess
	if !exists {
		rouid := ctx.DefaultQuery("rouid", "")
		if rouid != "" {
			nemu_list, _ := ruleDB.WithContext(ctx).Where(ruleDB.ID.Eq(gf.Int32(rouid))).Order(ruleDB.Weigh.Asc()).Find()
			rulemenu := GetMenuArray(ctx, nemu_list, 0, make([]int32, 0))
			gf.Success().SetMsg("获取菜单").SetData(rulemenu).Regin(ctx)
			return
		}
		gf.Failed().SetMsg("登录失效").SetCode(401).Regin(ctx)
		return
	}
	//获取用户权限菜单
	var role_id []int32
	acerr := roleAccessDB.WithContext(ctx).Where(roleAccessDB.UID.Eq(ctx.GetInt64("uid"))).Pluck(roleAccessDB.RoleID, &role_id)
	if acerr != nil {
		gf.Failed().SetMsg("获取用户授权信息失败").SetData(acerr).Regin(ctx)
		return
	}
	if role_id == nil {
		gf.Failed().SetMsg("您没有管理后台权限，请联系管理员授权").Regin(ctx)
		return
	}
	var menu_ids []string
	rerr := roleDB.WithContext(ctx).Where(roleDB.ID.In(role_id...)).Pluck(roleDB.Rules, &menu_ids)
	if rerr != nil {
		gf.Failed().SetMsg("查找用户role信息失败").SetData(rerr).Regin(ctx)
		return
	}
	//获取超级角色
	super_role, _ := roleDB.WithContext(ctx).Where(roleDB.ID.In(role_id...), roleDB.Rules.Eq("*")).Count()
	RMDB := ruleDB.WithContext(ctx)
	var roles []int32
	if super_role == 0 { //不是超级权限-过滤菜单权限
		getmenus := ArrayMergeInt32(menu_ids)
		RMDB = RMDB.Where(ruleDB.ID.In(getmenus...))
		roles = getmenus
	} else {
		roles = make([]int32, 0)
	}
	nemu_list, ruleerr := RMDB.Where(ruleDB.Status.Eq(0), ruleDB.Type.In(0, 1)).Order(ruleDB.Weigh.Asc()).Find()
	if ruleerr != nil {
		gf.Failed().SetMsg("获取菜单错误").SetData(ruleerr).Regin(ctx)
		return
	}
	rulemenu := GetMenuArray(ctx, nemu_list, 0, roles)
	gf.Success().SetMsg("获取菜单").SetData(rulemenu).Regin(ctx)
}

// 保存数据
func (api *Account) Upuserinfo(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	adminDB := dao.Query().Admin
	res, err := adminDB.WithContext(ctx).Where(adminDB.ID.Eq(ctx.GetInt64("uid"))).Updates(param)
	if err != nil {
		gf.Failed().SetMsg("更新头像失败！").SetData(err).Regin(ctx)
	} else {
		gf.Success().SetMsg("更新用户数据成功").SetData(res).Regin(ctx)
	}
}

// 更新头像
func (api *Account) Upavatar(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	userID := ctx.GetInt64("uid")
	adminDB := dao.Query().Admin
	res, err := adminDB.WithContext(ctx).Where(adminDB.ID.Eq(userID)).Update(adminDB.Avatar, param["url"])
	if err != nil {
		gf.Failed().SetMsg("更新头像失败！").SetData(err).Regin(ctx)
	} else {
		gf.Success().SetMsg("更新头像成功").SetData(res).Regin(ctx)
	}
}

// 修改密码
func (api *Account) Changepwd(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	userID := ctx.GetInt64("uid")
	adminDB := dao.Query().Admin
	userdata, err := adminDB.WithContext(ctx).Where(adminDB.ID.Eq(userID)).Select(adminDB.Password, adminDB.Salt).First()
	if err != nil {
		gf.Failed().SetMsg("查找账号失败！").SetData(err).Regin(ctx)
	} else {
		pass := gf.Md5(param["passwordOld"].(string) + userdata.Salt)
		if userdata.Password != pass {
			gf.Failed().SetMsg("原来密码输入错误！").SetData(err).Regin(ctx)
		} else {
			newpass := gf.Md5(param["passwordNew"].(string) + userdata.Salt)
			res, err := adminDB.WithContext(ctx).Where(adminDB.ID.Eq(userID)).Update(adminDB.Password, newpass)
			if err != nil {
				gf.Failed().SetMsg("修改密码失败").SetData(err).Regin(ctx)
			} else {
				gf.Success().SetMsg("修改密码成功！").SetData(res).Regin(ctx)
			}
		}
	}
}
