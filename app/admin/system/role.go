package system

import (
	"fmt"
	"gofly/dao"
	"gofly/dao/model"
	"gofly/utils/gf"
	"strings"
	"time"
)

// 角色管理
type Role struct{ NoNeedAuths []string }

func init() {
	gf.Register(&Role{NoNeedAuths: []string{"getParent", "getMenuList"}})
}

// 获取数据列表-子树结构
func (api *Role) GetList(ctx *gf.GinCtx) {
	roleAccessDB := dao.Query().AdminAuthRoleAccess
	roleDB := dao.Query().AdminAuthRole
	var user_role_ids []interface{}
	err := roleAccessDB.WithContext(ctx).Where(roleAccessDB.UID.Eq(ctx.GetInt64("uid"))).Pluck(roleAccessDB.RoleID, &user_role_ids)
	if err != nil {
		gf.Failed().SetMsg("获取角色数据失败," + err.Error()).Regin(ctx)
		return
	}
	role_chil_ids := gf.GetAllChilIds("admin_auth_role", user_role_ids) //批量获取子节点id
	all_role_id := gf.MergeArrInt32(user_role_ids, role_chil_ids)
	//查找条件
	param, _ := gf.RequestParam(ctx)
	var whereMap []dao.Condition
	whereMap = append(whereMap, roleDB.ID.In(all_role_id...))
	// whereMap = append(whereMap, roleDB.TenantID.Eq(ctx.GetInt32("tenant_id")))
	account_id, _ := gf.GetDataAuthor(ctx)
	account_id = append(account_id, 0)
	//获取自己权限组-显示自己所在的权限组
	var my_role_account_id []interface{}
	err = roleDB.WithContext(ctx).Where(roleDB.ID.In(gf.InterfaceToInt32Slice(user_role_ids)...)).Pluck(roleDB.AccountID, &my_role_account_id)
	account_id = append(account_id, my_role_account_id...)
	whereMap = append(whereMap, roleDB.AccountID.In(gf.InterfaceToInt64Slice(account_id)...))
	if name, ok := param["name"]; ok && name != "" {
		whereMap = append(whereMap, roleDB.Name.Like("%"+gf.String(name)+"%"))
	}
	if status, ok := param["status"]; ok && status != "" {
		whereMap = append(whereMap, roleDB.Status.Eq(gf.Int8(status)))
	}
	if createtime, ok := param["createtime"]; ok && createtime != "" {
		datetime_arr := gf.SplitAndStr(gf.String(createtime), ",")
		whereMap = append(whereMap, roleDB.CreatedAt.Between(gf.StrToTime(datetime_arr[0]), gf.StrToTime(datetime_arr[1])))
	}
	var roleList gf.List
	err = roleDB.WithContext(ctx).Where(whereMap...).Order(roleDB.Weigh.Asc()).Scan(&roleList)
	if err != nil {
		gf.Failed().SetMsg("获取角色数据失败," + err.Error()).Regin(ctx)
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
	gf.Success().SetMsg("获取拥有角色列表").SetData(gf.Map{"list": roleList, "max_pid": min_role_id}).Regin(ctx)
}

// 表单获取选择父级
func (api *Role) GetParent(ctx *gf.GinCtx) {
	id := ctx.DefaultQuery("id", "0")
	userID := ctx.GetInt64("uid") //当前用户ID
	roleDB := dao.Query().AdminAuthRole
	roleAccessDB := dao.Query().AdminAuthRoleAccess
	var user_role_ids []interface{}
	err := roleAccessDB.WithContext(ctx).Where(roleAccessDB.UID.Eq(userID)).Pluck(roleAccessDB.RoleID, &user_role_ids)
	if err != nil {
		gf.Failed().SetMsg("获取角色授权数据失败," + err.Error()).Regin(ctx)
		return
	}
	role_chil_ids := gf.GetAllChilIds("admin_auth_role", user_role_ids) //批量获取子节点id
	all_role_id := gf.MergeArrInt32(user_role_ids, role_chil_ids)
	//查找条件
	var whereMap []dao.Condition
	whereMap = append(whereMap, roleDB.ID.In(all_role_id...))
	// whereMap = append(whereMap, roleDB.TenantID.Eq(ctx.GetInt32("tenant_id")))
	account_id, _ := gf.GetDataAuthor(ctx)
	account_id = append(account_id, 0)
	//获取自己权限组-显示自己所在的权限组
	var my_role_account_id []interface{}
	err = roleDB.WithContext(ctx).Where(roleDB.ID.In(gf.InterfaceToInt32Slice(user_role_ids)...)).Pluck(roleDB.AccountID, &my_role_account_id)
	if err != nil {
		gf.Failed().SetMsg("获取自己权限组失败," + err.Error()).Regin(ctx)
		return
	}
	account_id = append(account_id, my_role_account_id...)
	whereMap = append(whereMap, roleDB.AccountID.In(gf.InterfaceToInt64Slice(account_id)...))
	if gf.Int64(id) != 0 {
		whereMap = append(whereMap, roleDB.ID.Neq(gf.Int32(id)))
	}
	var roleList gf.List
	err = roleDB.WithContext(ctx).Where(whereMap...).Order(roleDB.Weigh.Asc()).Scan(&roleList)
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
	gf.Success().SetMsg("角色父级数据！").SetData(roleList).Regin(ctx)
}

// 表单获取菜单-角色
func (api *Role) GetMenuList(ctx *gf.GinCtx) {
	pid := ctx.DefaultQuery("pid", "0")
	ruleDB := dao.Query().AdminAuthRule
	roleDB := dao.Query().AdminAuthRole
	roleAccessDB := dao.Query().AdminAuthRoleAccess
	var rule_ids []interface{}
	//查找条件
	var whereMenuMap []dao.Condition
	whereMenuMap = append(whereMenuMap, ruleDB.Status.Eq(0))
	whereMenuMap = append(whereMenuMap, ruleDB.Type.In(0, 1))
	if pid == "0" { //获取本账号所拥有的权限
		var role_id []int32
		rerr := roleAccessDB.WithContext(ctx).Where(roleAccessDB.UID.Eq(ctx.GetInt64("uid"))).Pluck(roleAccessDB.RoleID, &role_id)
		if rerr != nil {
			gf.Failed().SetMsg("查找角色授权失败").SetData(rerr).Regin(ctx)
			return
		}
		var menu_id []string
		rerr = roleDB.WithContext(ctx).Where(roleDB.ID.In(role_id...)).Pluck(roleDB.Rules, &menu_id)
		if rerr != nil {
			gf.Failed().SetMsg("查找角色授权失败").SetData(rerr).Regin(ctx)
			return
		}
		//获取超级角色
		var super_role int32
		roleDB.WithContext(ctx).Where(roleDB.ID.In(role_id...), roleDB.Rules.Eq("*")).Select(roleDB.ID).Scan(&super_role)
		if super_role == 0 { //不是超级权限-过滤菜单权限
			getmenus := gf.ArrayMerge(menu_id)
			whereMenuMap = append(whereMenuMap, ruleDB.ID.In(gf.InterfaceToInt32Slice(getmenus)...))
			rule_ids = getmenus
		}
	} else {
		//获取用户权限
		var menu_id_str string
		errs := roleDB.WithContext(ctx).Where(roleDB.ID.Eq(gf.Int32(pid))).Select(roleDB.Rules).Scan(&menu_id_str)
		if errs == nil && !strings.Contains(menu_id_str, "*") { //不是超级权限-过滤菜单权限
			getmenus := gf.Axplode(menu_id_str)
			whereMenuMap = append(whereMenuMap, ruleDB.ID.In(gf.InterfaceToInt32Slice(getmenus)...))
			rule_ids = getmenus
		}
	}
	var menuList gf.List
	err := ruleDB.WithContext(ctx).Where(whereMenuMap...).Select(ruleDB.ID, ruleDB.Pid, ruleDB.Title, ruleDB.Locale).Order(ruleDB.Weigh.Asc()).Scan(&menuList)
	if err != nil {
		gf.Failed().SetMsg("查找获取菜单信息失败").SetData(err.Error()).Regin(ctx)
		return
	}
	for _, val := range menuList {
		if gf.IsEmpty(val["title"]) {
			val["title"] = val["locale"]
		}
		delete(val, "locale")
		//获取按钮
		var whereMap2 []dao.Condition
		if len(rule_ids) > 0 {
			whereMap2 = append(whereMap2, ruleDB.ID.In(gf.InterfaceToInt32Slice(rule_ids)...))
		}
		whereMap2 = append(whereMap2, ruleDB.Status.Eq(0))
		whereMap2 = append(whereMap2, ruleDB.Type.Eq(2))
		whereMap2 = append(whereMap2, ruleDB.Pid.Eq(gf.Int32(val["id"])))
		var btn_rules gf.List
		ruleDB.WithContext(ctx).Where(whereMap2...).Select(ruleDB.ID, ruleDB.Pid, ruleDB.Title, ruleDB.Des, ruleDB.Locale).Order(ruleDB.Weigh.Asc()).Scan(&btn_rules)
		if len(btn_rules) > 0 {
			item := gf.Map{
				"title":     "按钮权限",
				"id":        btn_rules[0]["id"],
				"pid":       val["id"],
				"checkable": false,
				"btn_rules": btn_rules,
			}
			var valitem []gf.Map
			valitem = append(valitem, item)
			val["children"] = valitem
			var btnids []interface{}
			for _, btnid := range btn_rules {
				btnids = append(btnids, btnid["id"])
			}
			val["btnids"] = btnids
		} else if gf.Float32(val["pid"]) == 0 {
			//一级菜单获取子级菜单按钮
			var sub_rule_ids []int32
			ruleDB.WithContext(ctx).Where(ruleDB.Pid.Eq(gf.Int32(val["id"])), ruleDB.Status.Eq(0), ruleDB.Type.Neq(2)).Pluck(ruleDB.ID, &sub_rule_ids)
			var btn_rule_ids []int32
			ruleDB.WithContext(ctx).Where(ruleDB.Pid.In(sub_rule_ids...), ruleDB.Status.Eq(0), ruleDB.Type.Eq(2)).Pluck(ruleDB.ID, &btn_rule_ids)
			val["btnids"] = btn_rule_ids
		}
		val["checkable"] = true
	}
	menuList = gf.GetMenuChildrenArray(menuList, 0, "pid")
	if rule_ids == nil {
		var btn_idsdata []int32
		ruleDB.WithContext(ctx).Where(ruleDB.Status.Eq(0), ruleDB.Type.Eq(2)).Pluck(ruleDB.ID, &btn_idsdata)
		gf.Success().SetMsg("获取菜单数据1").SetData(gf.Map{"list": menuList, "btn_rule_ids": btn_idsdata}).Regin(ctx)
	} else {
		var btn_idsdata []int32
		ruleDB.WithContext(ctx).Where(ruleDB.Status.Eq(0), ruleDB.Type.Eq(2), ruleDB.ID.In(gf.InterfaceToInt32Slice(rule_ids)...)).Pluck(ruleDB.ID, &btn_idsdata)
		gf.Success().SetMsg("获取菜单数据2").SetData(gf.Map{"list": menuList, "btn_rule_ids": btn_idsdata}).Regin(ctx)
	}
}

// 保存编辑
func (api *Role) Save(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	var f_id = gf.GetEditId(param["id"])
	roleDB := dao.Query().AdminAuthRole
	if param["menu"] != nil && param["menu"] != "*" {
		rules := gf.GetRulesID("admin_auth_rule", "pid", param["menu"]) //获取子菜单包含的父级ID
		rudata := rules.([]interface{})
		var rulesStr []string
		for _, v := range rudata {
			str := fmt.Sprintf("%v", v) //interface{}强转string
			rulesStr = append(rulesStr, str)
		}
		for _, bv := range param["btns"].([]interface{}) {
			str := fmt.Sprintf("%v", bv) //interface{}强转string
			rulesStr = append(rulesStr, str)
		}
		param["rules"] = strings.Join(rulesStr, ",")
	}
	if gf.IsSlice(param["menu"]) {
		param["menu"] = gf.JSONToString(param["menu"])
	}
	if gf.IsSlice(param["btns"]) {
		param["btns"] = gf.JSONToString(param["btns"])
	}
	if f_id == 0 {
		param["tenant_id"] = ctx.GetInt32("tenant_id") //当前租户ID（预留）
		param["account_id"] = ctx.GetInt64("uid")      //当前用户ID
		param["created_at"] = time.Now()
		txDB := roleDB.WithContext(ctx).UnderlyingDB().Model(&model.AdminAuthRole{}).Create(param)
		if txDB.Error != nil {
			gf.Failed().SetMsg("添加失败").SetData(txDB.Error).Regin(ctx)
		} else {
			roleDB.WithContext(ctx).Where(roleDB.ID.Eq(gf.Int32(param["id"]))).Update(roleDB.Weigh, param["id"])
			gf.Success().SetMsg("添加成功！").SetData(param["id"]).Regin(ctx)
		}
	} else {
		delete(param, "children")
		delete(param, "spacer")
		res, err := roleDB.WithContext(ctx).Where(roleDB.ID.Eq(gf.Int32(f_id))).Updates(param)
		if err != nil {
			gf.Failed().SetMsg("更新失败").SetData(err).Regin(ctx)
		} else {
			gf.Success().SetMsg("更新成功！").SetData(res).Regin(ctx)
		}
	}
}

// 更新状态
func (api *Role) UpStatus(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	roleDB := dao.Query().AdminAuthRole
	res, err := roleDB.WithContext(ctx).Where(roleDB.ID.Eq(gf.Int32(param["id"]))).Update(roleDB.Status, param["status"])
	if err != nil {
		gf.Failed().SetMsg("更新失败！").SetData(err).Regin(ctx)
	} else {
		gf.Success().SetMsg("更新成功！").SetData(res).Regin(ctx)
	}
}

// 删除
func (api *Role) Del(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	roleDB := dao.Query().AdminAuthRole
	res2, err := roleDB.WithContext(ctx).Where(roleDB.ID.In(gf.InterfaceToInt32(param["ids"])...)).Delete()
	if err != nil {
		gf.Failed().SetMsg("删除失败").SetData(err).Regin(ctx)
	} else {
		gf.Success().SetMsg("删除成功！").SetData(res2).Regin(ctx)
	}
}
