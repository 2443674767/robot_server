package user

import (
	"gofly/dao"
	"gofly/utils/gf"
	"strings"
	"time"
)

// 修改账号信息
type Setting struct{}

func init() {
	gf.Register(&Setting{})
}
func (api *Setting) GetUserinfo(ctx *gf.GinCtx) {
	adminDB := dao.Query().Admin
	deptDB := dao.Query().AdminAuthDept
	a := dao.Query().AdminAuthRoleAccess
	r := dao.Query().AdminAuthRole
	var userdata gf.Map
	err := adminDB.WithContext(ctx).Where(adminDB.ID.Eq(ctx.GetInt64("uid"))).Select(adminDB.ID, adminDB.Username, adminDB.DeptID, adminDB.Nickname, adminDB.Status).Scan(&userdata)
	if err != nil {
		gf.Failed().SetMsg("查找用户数据！").Regin(ctx)
	} else {
		var deptname string
		deptDB.WithContext(ctx).Where(deptDB.ID.Eq(gf.Int32(userdata["dept_id"]))).Select(deptDB.Name).Scan(&deptname)
		userdata["deptname"] = deptname
		var ruleName []string
		a.WithContext(ctx).LeftJoin(r.WithContext(ctx), a.RoleID.EqCol(r.ID)).Where(a.UID.Eq(ctx.GetInt64("uid"))).Pluck(r.Name, &ruleName)
		userdata["roles"] = strings.Join(ruleName, "，")
		gf.Success().SetMsg("获取用户信息").SetData(userdata).Regin(ctx)
	}
}

// 更新账号信息
func (api *Setting) SaveInfo(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	userID := ctx.GetInt64("uid") //当前用户ID
	adminDB := dao.Query().Admin
	if oldpassword, ok := param["oldpassword"]; ok {
		if gf.String(param["type"]) == "mobile" {
			code, emerr := gf.GetVerifyCode(gf.String(param["mobile"]))
			if emerr != nil || code != gf.Int(param["captcha"]) {
				gf.Failed().SetMsg("验证码无效").SetData(emerr).Regin(ctx)
				return
			}
			delete(param, "captcha")
		} else if gf.String(param["type"]) == "email" {
			code, emerr := gf.GetVerifyCode(gf.String(param["email"]))
			if emerr != nil || code != gf.Int(param["captcha"]) {
				gf.Failed().SetMsg("验证码无效").SetData(emerr).Regin(ctx)
				return
			}
			delete(param, "captcha")
		}
		// "password,salt"
		account, err := adminDB.WithContext(ctx).Where(adminDB.ID.Eq(userID)).Select(adminDB.Password, adminDB.Salt).First()
		if err != nil {
			gf.Failed().SetMsg("查找账号信息失败").SetData(err).Regin(ctx)
			return
		}
		salt := account.Salt
		oldpass := gf.Md5(gf.String(oldpassword) + salt)
		if oldpass != account.Password {
			gf.Failed().SetMsg("输入的当前密码不正确！").Regin(ctx)
			return
		}
		delete(param, "oldpassword")
		if param["type"] == "password" {
			param["password"] = gf.Md5(gf.String(param["newpassword"]) + salt)
			param["pwd_reset_time"] = time.Now().Format("2006-01-02 15:04:05")
			delete(param, "newpassword")
		}
	}
	delete(param, "type")
	res, err := adminDB.WithContext(ctx).Where(adminDB.ID.Eq(userID)).Updates(param)
	if err != nil {
		gf.Failed().SetMsg("更新失败").SetData(err).Regin(ctx)
	} else {
		gf.Success().SetMsg("更新成功！").SetData(res).Regin(ctx)
	}
}
