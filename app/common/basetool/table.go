package basetool

import (
	"fmt"
	"gofly/dao"
	"gofly/utils/gf"
	"gofly/utils/tools/gcfg"
	"strings"
)

// 数据表操作
type Table struct{}

func init() {
	gf.Register(&Table{})
}

// 更新排序
func (api *Table) Weigh(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	var count = 0
	// 类型断言，避免 panic
	list, ok := param["weighList"].([]interface{})
	if !ok {
		gf.Failed().SetMsg("weighList 数据格式错误").Regin(ctx)
		return
	}
	for _, v := range list {
		item, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		id, hasId := item["id"]
		weigh, hasWeigh := item["weigh"]
		if !hasId || !hasWeigh {
			continue
		}
		tx := dao.DB().Table(dao.Use().TableName(gf.String(param["tablename"]))).Where("id = ? AND pid = ?", id, param["pid"]).Updates(map[string]any{
			"weigh": weigh,
		})
		if tx.RowsAffected > 0 {
			count++
		}
	}
	gf.Success().SetMsg(fmt.Sprintf("更新排序完成，影响%v条数据。", count)).Regin(ctx)
}

// 获取数据表
func (api *Table) GetTables(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	rawSQL := `
		SELECT TABLE_NAME,TABLE_COMMENT,ENGINE,TABLE_COLLATION 
		FROM information_schema.TABLES 
		WHERE TABLE_SCHEMA = DATABASE() 
		AND TABLE_TYPE = 'BASE TABLE'
	`
	var tablelist []map[string]any
	err := dao.DB().Raw(rawSQL).Scan(&tablelist).Error
	if err != nil {
		gf.Failed().SetMsg("获取数据表失败！" + err.Error()).Regin(ctx)
		return
	}
	var talbe_list []map[string]any
	for _, item := range tablelist {
		tableFull, ok := item["TABLE_NAME"].(string)
		if !ok {
			continue
		}
		prefix, _ := gcfg.Instance().Get(ctx, "database.default.prefix")
		// 去除gf_前缀
		shortName := strings.TrimPrefix(tableFull, prefix.String())
		if filter, ok := param["filter"]; ok && gf.Bool(filter) && gf.IsInSlice([]string{"admin", "admin_auth_dept", "admin_auth_role", "admin_auth_role_access", "admin_auth_rule", "attachment", "login_log", "operation_log", "dictionary_group", "dictionary_data"}, shortName) {
			continue
		}
		talbe_list = append(talbe_list, map[string]interface{}{"name": item["TABLE_NAME"], "title": item["TABLE_COMMENT"], "engine": item["ENGINE"], "collation": item["TABLE_COLLATION"]})
	}
	gf.Success().SetMsg("获取数据表").SetData(talbe_list).Regin(ctx)
}
