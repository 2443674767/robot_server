package user

import (
	"gofly/dao"
	"gofly/utils/auth"
	"gofly/utils/gf"
	"gofly/utils/tools/gcfg"
	"strings"
	"time"
)

/**
*使用 Index 是省略路径中的index
*本路径为： /business/user/login -省去了index
 */

type Index struct {
	NoNeedLogin []string //忽略登录接口配置-忽略全部传[*]
	NoNeedAuths []string //忽略RBAC权限认证接口配置-忽略全部传[*]
}

// 初始化路由
func init() {
	gf.Register(&Index{NoNeedLogin: []string{"login", "logout"}, NoNeedAuths: []string{"*"}})
}

/**
*1.《登录》
 */
func (api *Index) Login(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	adminDB := dao.Query().Admin
	if _, ok := param["username"]; ok {
		if param["username"] == nil || param["password"] == nil {
			gf.Failed().SetMsg("请提交用户账号或密码！").Regin(ctx)
			return
		}
		username := param["username"].(string)
		password := param["password"].(string)
		user, err := adminDB.WithContext(ctx).Where(adminDB.Username.Eq(username)).Or(adminDB.Email.Eq(username)).First()
		if err != nil {
			if strings.Contains(err.Error(), " dial tcp") {
				gf.Failed().SetMsg("数据库连接失败，请检查一下数据库是否正常！").Regin(ctx)
			} else if strings.Contains(err.Error(), "record not found") {
				gf.Failed().SetMsg("账号不存在！").Regin(ctx)
			} else {
				gf.Failed().SetMsg(err.Error()).Regin(ctx)
			}
			return
		}
		if user.Status == 1 {
			gf.Failed().SetMsg("账号被禁用了").Regin(ctx)
			return
		}
		if time.Now().Before(user.LockTime) {
			gf.Failed().SetMsg("账户已被锁定，请稍后再试").Regin(ctx)
			return
		}
		pass := gf.Md5(password + user.Salt)
		if pass != user.Password {
			gf.AddloginLog(ctx, gf.Map{"uid": user.ID, "account_id": user.AccountID, "tenant_id": user.TenantID, "type": "admin", "status": 1, "des": "账号登录", "error_msg": "输入的密码不正确！"})
			if user.LoginAttempts >= 3 {
				adminDB.WithContext(ctx).Where(adminDB.ID.Eq(user.ID)).Updates(gf.Map{"login_attempts": 0, "lock_time": time.Now().Add(30 * time.Minute)}) //记录
				gf.Failed().SetMsg("密码错误次数过多，账户已被锁定30分钟").Regin(ctx)
				return
			}
			adminDB.WithContext(ctx).Where(adminDB.ID.Eq(user.ID)).UpdateSimple(adminDB.LoginAttempts.Add(1))
			gf.Failed().SetMsg("您输入的密码不正确！").Regin(ctx)
			return
		}
		loginCaptcha, _ := gcfg.Instance("app").Get(ctx, "app.loginCaptcha")
		if loginCaptcha.Bool() && !gf.VerifyCaptcha(gf.String(param["codeid"]), gf.String(param["captcha"])) {
			gf.AddloginLog(ctx, gf.Map{"uid": user.ID, "account_id": user.AccountID, "tenant_id": user.TenantID, "status": 1, "des": "账号登录", "error_msg": "输入的验证码不正确！"})
			gf.Failed().SetMsg("您输入的验证码不正确！").Regin(ctx)
			return
		}
		//创建token（先清除该用户旧缓存，避免多点登录复用已过期 token）
		_ = auth.RemoveToken(gf.String(user.ID))
		token, err := auth.GenerateToken(gf.String(user.ID), gf.Map{"uid": user.ID, "account_id": user.AccountID, "tenant_id": user.TenantID})
		if err != nil {
			gf.Failed().SetMsg(err.Error()).Regin(ctx)
		} else {
			adminDB.WithContext(ctx).Where(adminDB.ID.Eq(user.ID)).Updates(map[string]interface{}{"loginstatus": 1, "last_login_time": time.Now().Unix(), "last_login_ip": gf.GetIp(ctx)})
			gf.AddloginLog(ctx, gf.Map{"uid": user.ID, "account_id": user.AccountID, "tenant_id": user.TenantID, "status": 0, "des": "账号登录"})
			gf.Success().SetMsg("登录成功！").SetData(token).Regin(ctx)
		}
	} else if email, ok := param["email"]; ok {
		user, err := adminDB.WithContext(ctx).Where(adminDB.Email.Eq(gf.String(email))).First()
		if user == nil || err != nil {
			gf.Failed().SetMsg("邮箱账号不存在！").Regin(ctx)
			return
		}
		if user.Status == 1 {
			gf.AddloginLog(ctx, gf.Map{"uid": user.ID, "account_id": user.AccountID, "tenant_id": user.TenantID, "status": 1, "des": "邮箱登录", "error_msg": "账号被禁用"})
			gf.Failed().SetMsg("账号被禁用了").Regin(ctx)
			return
		}
		if time.Now().Before(user.LockTime) {
			gf.Failed().SetMsg("账户已被锁定，请稍后再试").Regin(ctx)
			return
		}
		code, emerr := gf.GetVerifyCode(gf.String(param["email"]))
		if emerr != nil || code != gf.Int(param["captcha"]) {
			gf.AddloginLog(ctx, gf.Map{"uid": user.ID, "account_id": user.AccountID, "tenant_id": user.TenantID, "status": 1, "des": "邮箱登录", "error_msg": "验证码无效"})
			if user.LoginAttempts >= 3 {
				adminDB.WithContext(ctx).Where(adminDB.ID.Eq(user.ID)).Updates(gf.Map{"login_attempts": 0, "lock_time": time.Now().Add(30 * time.Minute)}) //记录
				gf.Failed().SetMsg("验证码错误次数过多，账户已被锁定30分钟").Regin(ctx)
				return
			}
			adminDB.WithContext(ctx).Where(adminDB.ID.Eq(user.ID)).UpdateSimple(adminDB.LoginAttempts.Add(1))
			gf.Failed().SetMsg("验证码无效").SetData(emerr).Regin(ctx)
			return
		}
		//创建token（先清除该用户旧缓存，避免复用已过期 token）
		_ = auth.RemoveToken(gf.String(user.ID))
		token, err := auth.GenerateToken(gf.String(user.ID), gf.Map{"uid": user.ID, "account_id": user.AccountID, "tenant_id": user.TenantID})
		if err != nil {
			gf.Failed().SetMsg(err.Error()).Regin(ctx)
		} else {
			adminDB.WithContext(ctx).Where(adminDB.ID.Eq(user.ID)).Updates(map[string]interface{}{"loginstatus": 1, "last_login_time": time.Now().Unix(), "last_login_ip": gf.GetIp(ctx)})
			gf.AddloginLog(ctx, gf.Map{"uid": user.ID, "account_id": user.AccountID, "tenant_id": user.TenantID, "status": 0, "des": "邮箱登录"})
			gf.Success().SetMsg("登录成功返回token！").SetData(token).Regin(ctx)
		}
	} else if mobile, ok := param["mobile"]; ok {
		user, err := adminDB.WithContext(ctx).Where(adminDB.Mobile.Eq(gf.String(mobile))).First()
		if user == nil || err != nil {
			gf.Failed().SetMsg("手机账号不存在！").Regin(ctx)
			return
		}
		if user.Status == 1 {
			gf.AddloginLog(ctx, gf.Map{"uid": user.ID, "account_id": user.AccountID, "tenant_id": user.TenantID, "status": 1, "des": "手机号登录", "error_msg": "账号被禁用"})
			gf.Failed().SetMsg("账号被禁用了").Regin(ctx)
			return
		}
		if time.Now().Before(user.LockTime) {
			gf.Failed().SetMsg("账户已被锁定，请稍后再试").Regin(ctx)
			return
		}
		code, emerr := gf.GetVerifyCode(gf.String(param["mobile"]))
		if emerr != nil || code != gf.Int(param["captcha"]) {
			gf.AddloginLog(ctx, gf.Map{"uid": user.ID, "account_id": user.AccountID, "tenant_id": user.TenantID, "status": 1, "des": "手机号登录", "error_msg": "验证码无效"})
			if user.LoginAttempts >= 3 {
				adminDB.WithContext(ctx).Where(adminDB.ID.Eq(user.ID)).Updates(gf.Map{"login_attempts": 0, "lock_time": time.Now().Add(30 * time.Minute)}) //记录
				gf.Failed().SetMsg("验证码错误次数过多，账户已被锁定30分钟").Regin(ctx)
				return
			}
			adminDB.WithContext(ctx).Where(adminDB.ID.Eq(user.ID)).UpdateSimple(adminDB.LoginAttempts.Add(1))
			gf.Failed().SetMsg("验证码无效").SetData(emerr).Regin(ctx)
			return
		}
		//创建token（先清除该用户旧缓存，避免复用已过期 token）
		_ = auth.RemoveToken(gf.String(user.ID))
		token, err := auth.GenerateToken(gf.String(user.ID), gf.Map{"uid": user.ID, "account_id": user.AccountID, "tenant_id": user.TenantID})
		if err != nil {
			gf.Failed().SetMsg(err.Error()).Regin(ctx)
		} else {
			adminDB.WithContext(ctx).Where(adminDB.ID.Eq(user.ID)).Updates(map[string]interface{}{"loginstatus": 1, "last_login_time": time.Now().Unix(), "last_login_ip": gf.GetIp(ctx)})
			gf.AddloginLog(ctx, gf.Map{"uid": user.ID, "account_id": user.AccountID, "tenant_id": user.TenantID, "status": 0, "des": "手机号登录"})
			gf.Success().SetMsg("登录成功！").SetData(token).Regin(ctx)
		}
	} else {
		gf.Failed().SetMsg("该登录方式为开发请使用其他方式登录！").Regin(ctx)
	}
}

/**
* 2.《获取用户》
 */
func (api *Index) GetUserinfo(ctx *gf.GinCtx) {
	adminDB := dao.Query().Admin
	userdata, err := adminDB.WithContext(ctx).Where(adminDB.ID.Eq(ctx.GetInt64("uid"))).Select(adminDB.ID, adminDB.Name, adminDB.Nickname, adminDB.Mobile, adminDB.Email, adminDB.Avatar, adminDB.Status, adminDB.CreatedAt, adminDB.PwdResetTime).First()
	if err != nil {
		gf.Failed().SetMsg("查找用户数据错误：" + err.Error()).Regin(ctx)
	} else {
		if gf.IsEmpty(userdata.Avatar) {
			userdata.Avatar = gf.GetLocalUrl() + "resource/uploads/static/unknown.png"
		} else {
			userdata.Avatar = gf.GetFullUrl(userdata.Avatar)
		}
		//处理敏感信息
		userdata.Mobile = gf.HideStrInfo("mobile", userdata.Mobile)
		userdata.Email = gf.HideStrInfo("email", userdata.Email)
		res := gf.Map{
			"id":             userdata.ID,
			"name":           userdata.Name,
			"nickname":       userdata.Nickname,
			"mobile":         userdata.Mobile,
			"email":          userdata.Email,
			"avatar":         userdata.Avatar,
			"status":         userdata.Status,
			"created_at":     userdata.CreatedAt,
			"pwd_reset_time": userdata.PwdResetTime,
			"rooturls":       gf.GetAllRootUrl(),
			"defrooturl":     gf.GetRootUrl(),
		}
		gf.Success().SetMsg("获取用户信息").SetData(res).Regin(ctx)
	}
}

/**
*  3退出登录
 */
func (api *Index) Logout(ctx *gf.GinCtx) {
	auth.RemoveToken(ctx.GetString("uid")) //清除token，让当前token失效
	gf.Success().SetMsg("退出登录").SetData(true).Regin(ctx)
}
