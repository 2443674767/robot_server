package system

//系统账号管理
import (
	"fmt"
	"gofly/dao"
	"gofly/dao/model"
	"gofly/utils/gf"
	"gofly/utils/tools/grand"
	"time"
)

// 用户账号管理
type Account struct{ NoNeedAuths []string }

func init() {
	gf.Register(&Account{NoNeedAuths: []string{"getList", "GetRole", "isaccountexist"}})
}

// 查询数据列表-在原来数据结构扩展新字段
type AdminListVO struct {
	*model.Admin             // 嵌套原结构体（继承所有字段）
	Rolename     interface{} `json:"rolename"` // 动态扩展-角色名称
	Roleid       interface{} `json:"roleid"`   // 动态扩展-角色id
	Depname      string      `json:"depname"`  // 动态扩展-部门名称
}

// 获取成员列表
func (api *Account) GetList(ctx *gf.GinCtx) {
	adminDB := dao.Query().Admin
	param, _ := gf.RequestParam(ctx)
	pageNo := gf.Int(ctx.DefaultQuery("page", "1"))
	pageSize := gf.Int(ctx.DefaultQuery("pageSize", "10"))
	//搜索添条件
	var whereMap []dao.Condition
	//过滤租户数据
	whereMap = append(whereMap, adminDB.TenantID.Eq(ctx.GetInt32("tenant_id")))
	account_id, filter := gf.GetDataAuthor(ctx)
	if filter { //需要数据权限过滤
		whereMap = append(whereMap, adminDB.AccountID.In(gf.InterfaceToInt64(account_id)...))
	}
	if name, ok := param["name"]; ok && name != "" {
		whereMap = append(whereMap, adminDB.Name.Like("%"+gf.String(name)+"%"))
	}
	if mobile, ok := param["mobile"]; ok && mobile != "" {
		whereMap = append(whereMap, adminDB.Mobile.Eq(gf.String(mobile)))
	}
	if status, ok := param["status"]; ok && status != "" {
		whereMap = append(whereMap, adminDB.Status.Eq(gf.Int8(status)))
	}
	if createtime, ok := param["createtime"]; ok && createtime != "" {
		datetime_arr := gf.SplitAndStr(gf.String(createtime), ",")
		whereMap = append(whereMap, adminDB.CreatedAt.Between(gf.StrToTime(datetime_arr[0]+" 00:00"), gf.StrToTime(datetime_arr[1]+" 23:59")))
	}
	list, totalCount, err := adminDB.WithContext(ctx).Where(whereMap...).Or(adminDB.ID.Eq(ctx.GetInt64("uid"))).
		Select(adminDB.ID, adminDB.Status, adminDB.Name, adminDB.Username, adminDB.Avatar, adminDB.Tel, adminDB.Mobile, adminDB.Email, adminDB.DeptID, adminDB.Remark, adminDB.CreatedAt).
		Order(adminDB.ID.Desc()).FindByPage(dao.Offset(pageNo, pageSize), pageSize)
	if err != nil {
		gf.Failed().SetMsg("Failed to obtain data;" + err.Error()).Regin(ctx)
	} else {
		var voList []*AdminListVO
		for _, val := range list {
			//处理头像
			if gf.IsEmpty(val.Avatar) {
				val.Avatar = gf.GetLocalUrl() + "/resource/uploads/static/unknown.png"
			} else {
				val.Avatar = gf.GetFullUrl(val.Avatar)
			}
			//处理关联数据
			roleDB := dao.Query().AdminAuthRole
			roleAccessDB := dao.Query().AdminAuthRoleAccess
			var roleid []int32
			var rolename []string
			roleAccessDB.WithContext(ctx).Where(roleAccessDB.UID.Eq(val.ID)).Pluck(roleAccessDB.RoleID, &roleid)
			roleDB.WithContext(ctx).Where(roleDB.ID.In(roleid...)).Pluck(roleDB.Name, &rolename)
			//部门
			var depname string
			deptDB := dao.Query().AdminAuthDept
			deptDB.WithContext(ctx).Where(deptDB.ID.Eq(val.DeptID)).Scan(&depname)
			voList = append(voList, &AdminListVO{
				Admin:    val,      // 原日志数据（指针直接赋值，无报错）
				Rolename: rolename, // 扩展字段
				Roleid:   roleid,   // 扩展字段
				Depname:  depname,  // 扩展字段
			})
		}
		gf.Success().SetMsg("获取全部列表").SetData(gf.Map{
			"page":     pageNo,
			"pageSize": pageSize,
			"total":    totalCount,
			"items":    voList}).Regin(ctx)
	}
}

// 保存、编辑
func (api *Account) Save(ctx *gf.GinCtx) {
	adminDB := dao.Query().Admin
	param, _ := gf.RequestParam(ctx)
	var f_id = gf.GetEditId(param["id"])
	var roleid []interface{}
	if param["roleid"] != nil {
		roleid = param["roleid"].([]interface{})
		delete(param, "roleid")
	}
	if !gf.IsEmpty(param["password"]) {
		salt := grand.Digits(6)
		mdpass := fmt.Sprintf("%v%v", param["password"], salt)
		param["password"] = gf.Md5(mdpass)
		param["salt"] = salt
	}
	if param["avatar"] == "" {
		param["avatar"] = "resource/uploads/static/unknown.png"
	}
	if f_id == 0 {
		param["account_id"] = ctx.GetInt64("uid")      //当前用户ID
		param["tenant_id"] = ctx.GetInt32("tenant_id") //当前租户
		param["created_at"] = time.Now()
		txDB := adminDB.WithContext(ctx).UnderlyingDB().Model(&model.HomeQuickop{}).Create(param)
		if txDB.Error != nil {
			gf.Failed().SetMsg("添加失败").SetData(txDB.Error).Regin(ctx)
		} else {
			//添加角色-多个
			appRoleAccess(ctx, roleid, param["id"])
			gf.Success().SetMsg("添加成功！").SetData(param["id"]).Regin(ctx)
		}
	} else {
		if gf.IsEmpty(param["password"]) {
			delete(param, "password")
			delete(param, "salt")
		}
		res, err := adminDB.WithContext(ctx).Where(adminDB.ID.Eq(gf.Int64(f_id))).Select(
			// 使用Select设置需要更新字段
			adminDB.Name,
			adminDB.Nickname,
			adminDB.DeptID,
			adminDB.Username,
			adminDB.Password,
			adminDB.Salt,
			adminDB.Mobile,
			adminDB.Tel,
			adminDB.Email,
			adminDB.Remark,
			adminDB.Avatar,
		).Updates(param)
		if err != nil {
			gf.Failed().SetMsg("更新失败").SetData(err).Regin(ctx)
		} else {
			//添加角色-多个
			if roleid != nil {
				appRoleAccess(ctx, roleid, f_id)
			}
			gf.Success().SetMsg("更新成功！").SetData(res).Regin(ctx)
		}
	}
}

// 更新状态
func (api *Account) UpStatus(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	adminDB := dao.Query().Admin
	res, err := adminDB.WithContext(ctx).Where(adminDB.ID.Eq(gf.Int64(param["id"]))).Update(adminDB.Status, param["status"])
	if err != nil {
		gf.Failed().SetMsg("更新失败！").SetData(err).Regin(ctx)
	} else {
		gf.Success().SetMsg("更新成功！").SetData(res).Regin(ctx)
	}
}

// 删除
func (api *Account) Del(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	adminDB := dao.Query().Admin
	res, err := adminDB.WithContext(ctx).Where(adminDB.ID.In(gf.InterfaceToInt64(param["ids"])...)).Delete()
	if err != nil {
		gf.Failed().SetMsg("删除失败").SetData(err).Regin(ctx)
	} else {
		gf.Success().SetMsg("删除成功！").SetData(res).Regin(ctx)
	}
}

// 添加授权
func appRoleAccess(ctx *gf.GinCtx, roleids []interface{}, uid interface{}) {
	roleAccessDB := dao.Query().AdminAuthRoleAccess
	//批量提交
	roleAccessDB.WithContext(ctx).Where(roleAccessDB.UID.Eq(gf.Int64(uid))).Delete()
	save_arr := []map[string]interface{}{}
	for _, val := range roleids {
		marr := map[string]interface{}{"uid": uid, "role_id": val}
		save_arr = append(save_arr, marr)
	}
	roleAccessDB.WithContext(ctx).UnderlyingDB().Model(&model.AdminAuthRoleAccess{}).Create(save_arr)
}

// 表单-选择角色
func (api *Account) GetRole(ctx *gf.GinCtx) {
	roleAccessDB := dao.Query().AdminAuthRoleAccess
	roleDB := dao.Query().AdminAuthRole
	userID := ctx.GetInt64("uid") //当前用户ID
	var user_role_ids []interface{}
	err := roleAccessDB.WithContext(ctx).Where(roleAccessDB.UID.Eq(userID)).Pluck(roleAccessDB.RoleID, &user_role_ids)
	if err != nil {
		gf.Failed().SetMsg("获取用户授权信息失败！").SetData(err.Error()).Regin(ctx)
		return
	}
	role_chil_ids := gf.GetAllChilIds("admin_auth_role", user_role_ids) //批量获取子节点id
	all_role_id := gf.MergeArrInt32(user_role_ids, role_chil_ids)
	//查找条件
	var whereMap []dao.Condition
	whereMap = append(whereMap, roleDB.Status.Eq(0))
	whereMap = append(whereMap, roleDB.ID.In(all_role_id...))

	account_id, _ := gf.GetDataAuthor(ctx)
	account_id = append(account_id, 0)
	//获取自己权限组-显示自己所在的权限组
	var my_role_account_id int32
	err = roleDB.WithContext(ctx).Where(roleDB.ID.In(gf.InterfaceToInt32(user_role_ids)...)).Select(roleDB.AccountID).Scan(&my_role_account_id)
	if err != nil {
		gf.Failed().SetMsg("获取角色信息失败！").SetData(err.Error()).Regin(ctx)
		return
	}
	account_id = append(account_id, my_role_account_id)
	whereMap = append(whereMap, roleDB.AccountID.In(gf.InterfaceToInt64(account_id)...))

	var roleList gf.List
	err = roleDB.WithContext(ctx).Where(whereMap...).Order(roleDB.Weigh.Asc()).Scan(&roleList)
	if err != nil {
		gf.Failed().SetMsg("获取角色列表数据失败！").SetData(err.Error()).Regin(ctx)
		return
	}
	//获取最最早一条的pid
	roleData, _ := roleDB.WithContext(ctx).Where(whereMap...).Order(roleDB.ID.Asc()).Select(roleDB.Pid).First()
	var min_role_id int32 = 0
	if roleData != nil {
		min_role_id = roleData.Pid
	}
	roleList = gf.GetTreeArray(roleList, gf.Int64(min_role_id), "")
	if roleList == nil {
		roleList = make(gf.List, 0)
	}
	gf.Success().SetMsg("表单选择角色多选用数据").SetData(roleList).Regin(ctx)
}

// 判断账号是否存在
func (api *Account) Isaccountexist(ctx *gf.GinCtx) {
	adminDB := dao.Query().Admin
	param, _ := gf.RequestParam(ctx)
	if param["id"] != nil {
		res, err := adminDB.WithContext(ctx).Where(adminDB.ID.Neq(gf.Int64(param["id"])), adminDB.Username.Eq(gf.String(param["username"]))).Select(adminDB.ID).Count()
		if err != nil {
			gf.Failed().SetMsg("验证失败").SetData(err).Regin(ctx)
		} else if res > 0 {
			gf.Failed().SetMsg("账号已存在").Regin(ctx)
		} else {
			gf.Success().SetMsg("验证通过").SetData(res).Regin(ctx)
		}
	} else {
		res, err := adminDB.WithContext(ctx).Where(adminDB.Username.Eq(gf.String(param["username"]))).Select(adminDB.ID).Count()
		if err != nil {
			gf.Failed().SetMsg("验证失败").SetData(err).Regin(ctx)
		} else if res > 0 {
			gf.Failed().SetMsg("账号已存在").Regin(ctx)
		} else {
			gf.Success().SetMsg("验证通过").Regin(ctx)
		}
	}
}
