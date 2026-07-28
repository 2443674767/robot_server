package system

import (
	"gofly/dao"
	"gofly/dao/model"
	"gofly/utils/gf"
	"net"
	"time"
)

// 登录和操作日志
type Log struct {
	NoNeedAuths []string
	NoNeedLogin []string
}

func init() {
	gf.Register(&Log{NoNeedLogin: []string{"getList"}})
}

// 登录日志-查询数据在原来数据结构扩展新字段
type LoginLogVO struct {
	*model.LoginLog             // 嵌套原结构体（继承所有字段）
	User            interface{} `json:"user"` // 动态扩展用户信息字段
}

// 操作日志-查询数据列表在原来数据结构扩展新字段
type OperationLogVO struct {
	*model.OperationLog             // 嵌套原结构体（继承所有字段）
	User                interface{} `json:"user"` // 动态扩展用户信息字段
}

// 操作日志-查询数据在原来数据结构扩展新字段
type OperationLogVOne struct {
	*model.OperationLog        // 嵌套原结构体（继承所有字段）
	Username            string `json:"username"` // 动态扩展用户名称
}

// 获取登录日志列表
func (api *Log) GetLogin(ctx *gf.GinCtx) {
	loginDB := dao.Query().LoginLog
	adminDB := dao.Query().Admin
	//搜索添条件
	pageNo := gf.Int(ctx.DefaultQuery("page", "1"))
	pageSize := gf.Int(ctx.DefaultQuery("pageSize", "10"))
	param, _ := gf.RequestParam(ctx)
	var whereMap []dao.Condition
	whereMap = append(whereMap, loginDB.TenantID.Eq(ctx.GetInt32("tenant_id")))
	account_id, filter := gf.GetDataAuthor(ctx)
	if filter { //需要数据权限过滤
		whereMap = append(whereMap, loginDB.AccountID.In(gf.InterfaceToInt64(account_id)...))
	}
	if user, ok := param["user"]; ok && user != "" {
		var userids []int64
		adminDB.WithContext(ctx).Where(adminDB.Name.Like("%"+gf.String(user)+"%")).Pluck(adminDB.ID, &userids)
		whereMap = append(whereMap, loginDB.UID.In(userids...))
	}
	if ip, ok := param["ip"]; ok && ip != "" {
		address := net.ParseIP(gf.String(ip))
		if address == nil {
			whereMap = append(whereMap, loginDB.Address.Like("%"+gf.String(ip)+"%"))
		} else {
			whereMap = append(whereMap, loginDB.IP.Eq(gf.String(ip)))
		}
	}
	if status, ok := param["status"]; ok && status != "" {
		whereMap = append(whereMap, loginDB.Status.Eq(gf.Int8(status)))
	}
	if createtime, ok := param["createtime"]; ok && createtime != "" {
		datetime_arr := gf.SplitAndStr(gf.String(createtime), ",")
		whereMap = append(whereMap, loginDB.CreatedAt.Between(gf.StrToTime(datetime_arr[0]), gf.StrToTime(datetime_arr[1])))
	}
	whereMap = append(whereMap, loginDB.Type.Eq("admin"))
	list, totalCount, err := loginDB.WithContext(ctx).Where(whereMap...).Or(loginDB.UID.Eq(ctx.GetInt64("uid"))).Order(loginDB.ID.Desc()).FindByPage(dao.Offset(pageNo, pageSize), pageSize)
	if err != nil {
		gf.Failed().SetMsg(err.Error()).Regin(ctx)
	} else {
		var voList []*LoginLogVO
		for _, val := range list {
			userdata, _ := adminDB.WithContext(ctx).Where(adminDB.ID.Eq(gf.Int64(val.UID))).Select(adminDB.Username, adminDB.Name, adminDB.Nickname, adminDB.Avatar).First()
			voList = append(voList, &LoginLogVO{
				LoginLog: val,      // 原日志数据（指针直接赋值，无报错）
				User:     userdata, // 查询的用户数据
			})
		}
		gf.Success().SetMsg("获取登录日志").SetData(gf.Map{
			"page":     pageNo,
			"pageSize": pageSize,
			"total":    totalCount,
			"items":    voList}).Regin(ctx)
	}
}

// 删除上个月登录日志
func (api *Log) DelLastLogin(ctx *gf.GinCtx) {
	loginDB := dao.Query().LoginLog
	res, err := loginDB.WithContext(ctx).Where(loginDB.Type.Neq("admin"), loginDB.CreatedAt.Lt(time.Now().AddDate(0, -1, 0))).Delete()
	if err != nil {
		gf.Failed().SetMsg("删除失败").SetData(err).Regin(ctx)
	} else {
		gf.Success().SetMsg("删除成功！").SetData(res).Regin(ctx)
	}
}

var os, browser string

// 获取操作日志列表
func (api *Log) GetOperation(ctx *gf.GinCtx) {
	operationDB := dao.Query().OperationLog
	adminDB := dao.Query().Admin
	//搜索添条件
	pageNo := gf.Int(ctx.DefaultQuery("page", "1"))
	pageSize := gf.Int(ctx.DefaultQuery("pageSize", "10"))
	param, _ := gf.RequestParam(ctx)
	var whereMap []dao.Condition
	whereMap = append(whereMap, operationDB.TenantID.Eq(ctx.GetInt32("tenant_id")))
	account_id, filter := gf.GetDataAuthor(ctx)
	if filter { //需要数据权限过滤
		whereMap = append(whereMap, operationDB.AccountID.In(gf.InterfaceToInt64(account_id)...))
	}
	if user, ok := param["user"]; ok && user != "" {
		var userids []int64
		adminDB.WithContext(ctx).Where(adminDB.Name.Like("%"+gf.String(user)+"%")).Pluck(adminDB.ID, &userids)
		whereMap = append(whereMap, operationDB.UID.In(userids...))
	}
	if ip, ok := param["ip"]; ok && ip != "" {
		address := net.ParseIP(gf.String(ip))
		if address == nil {
			whereMap = append(whereMap, operationDB.Address.Like("%"+gf.String(ip)+"%"))
		} else {
			whereMap = append(whereMap, operationDB.IP.Eq(gf.String(ip)))
		}
	}
	if status, ok := param["status"]; ok && status != "" {
		if gf.Int64(status) == 200 {
			whereMap = append(whereMap, operationDB.Status.Eq(gf.Int32(status)))
		} else {
			whereMap = append(whereMap, operationDB.Status.Neq(200))
		}
	}
	if createtime, ok := param["createtime"]; ok && createtime != "" {
		datetime_arr := gf.SplitAndStr(gf.String(createtime), ",")
		whereMap = append(whereMap, operationDB.CreatedAt.Between(gf.StrToTime(datetime_arr[0]), gf.StrToTime(datetime_arr[1])))
	}

	whereMap = append(whereMap, operationDB.Type.Eq("admin"))
	list, totalCount, err := operationDB.WithContext(ctx).Where(whereMap...).Or(operationDB.UID.Eq(ctx.GetInt64("uid"))).Order(operationDB.ID.Desc()).FindByPage(dao.Offset(pageNo, pageSize), pageSize)
	if err != nil {
		gf.Failed().SetMsg("查询失败，请稍后再试！" + err.Error()).Regin(ctx)
	} else {
		var voList []*OperationLogVO
		for _, val := range list {
			userdata, _ := adminDB.WithContext(ctx).Where(adminDB.ID.Eq(gf.Int64(val.UID))).Select(adminDB.Username, adminDB.Name, adminDB.Nickname, adminDB.Avatar).First()
			voList = append(voList, &OperationLogVO{
				OperationLog: val,      // 原日志数据（指针直接赋值，无报错）
				User:         userdata, // 查询的用户数据
			})
		}
		gf.Success().SetMsg("获取操作日志").SetData(gf.Map{
			"page":     pageNo,
			"pageSize": pageSize,
			"total":    totalCount,
			"items":    voList}).Regin(ctx)
	}
}

// 删除上个月操作日志
func (api *Log) DelLastOperation(ctx *gf.GinCtx) {
	operationDB := dao.Query().OperationLog
	res, err := operationDB.WithContext(ctx).Where(operationDB.Type.Neq("admin"), operationDB.CreatedAt.Lt(time.Now().AddDate(0, -1, 0))).Delete()
	if err != nil {
		gf.Failed().SetMsg("删除失败").SetData(err).Regin(ctx)
	} else {
		gf.Success().SetMsg("删除成功！").SetData(res).Regin(ctx)
	}
}

// 获取操作日志内容
func (api *Log) GetOperationDetail(ctx *gf.GinCtx) {
	id := ctx.DefaultQuery("id", "")
	if id == "" {
		gf.Failed().SetMsg("请传参数id").Regin(ctx)
	} else {
		operationDB := dao.Query().OperationLog
		data, err := operationDB.WithContext(ctx).Where(operationDB.ID.Eq(gf.Int32(id))).First()
		if err != nil {
			gf.Failed().SetMsg("获取内容失败").SetData(err).Regin(ctx)
		} else {
			adminDB := dao.Query().Admin
			var username string
			adminDB.WithContext(ctx).Where(adminDB.ID.Eq(data.UID)).Select(adminDB.Name).Scan(&username)
			voData := &OperationLogVOne{
				OperationLog: data,     // 原日志数据（指针直接赋值，无报错）
				Username:     username, // 查询的用户数据
			}
			gf.Success().SetMsg("获取内容成功！").SetData(voData).Regin(ctx)
		}
	}
}
