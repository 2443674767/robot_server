package gf

import (
	"encoding/json"
	"fmt"
	"gofly/dao"
	"gofly/utils/tools/gstr"
	"gofly/utils/tools/gvar"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// 判断元素是否存在数组中
func IsContain(items []interface{}, item string) bool {
	for _, eachItem := range items {
		if eachItem == item {
			return true
		}
	}
	return false
}

// 判断元素是否存在数组中
func IsContainVal(items []interface{}, item *interface{}) bool {
	for _, eachItem := range items {
		if eachItem == item {
			return true
		}
	}
	return false
}

// IsInSlice 判断目标字符串是否是在切片中-字符串类型
func IsInSlice(items []string, item string) bool {
	if len(items) == 0 {
		return false
	}
	for _, eachItem := range items {
		if eachItem == item {
			return true
		}
	}
	return false
}

// 获取ip函数
func GetIp(c *GinCtx) string {
	reqIP := c.Request.Header.Get("X-Forwarded-For")
	if reqIP == "::1" {
		reqIP = "127.0.0.1"
	}
	return reqIP
}

// 获取本地ip
func LocalIP() string {
	ip := ""
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && !ipnet.IP.IsMulticast() && !ipnet.IP.IsLinkLocalUnicast() && !ipnet.IP.IsLinkLocalMulticast() && ipnet.IP.To4() != nil {
				ip = ipnet.IP.String()
			}
		}
	}
	return ip
}

/*
*
  - 1.批量获取子节点id
  - @tablename 数据表名称
    @ids 要获取的id
*/
func GetAllChilIds(tablename string, ids []interface{}) []interface{} {
	var allsubids []interface{}
	for _, id := range ids {
		sub_ids := GetAllChilId(tablename, id)
		allsubids = append(allsubids, sub_ids...)
	}
	return allsubids
}

// 1.2获取所有子级ID，tablename是表名(不含前缀)
func GetAllChilId(tablename string, id interface{}) []interface{} {
	var subids []interface{}
	sub_ids, _ := dao.Use().Array(tablename, "id", "pid", id)
	if len(sub_ids) > 0 {
		for _, sid := range sub_ids {
			subids = append(subids, sid)
			subids = append(subids, GetAllChilId(tablename, sid)...)
		}
	}
	return subids
}

// 合并数组-两个数组合并为一个数组
func MergeArr(a []interface{}, b []interface{}) []interface{} {
	var arr []interface{}
	for _, i := range a {
		arr = append(arr, i)
	}
	for _, j := range b {
		arr = append(arr, j)
	}
	return arr
}

// 合并数组-两个数组合并为一个int32数组
func MergeArrInt32(a []interface{}, b []interface{}) []int32 {
	var arr []int32
	for _, i := range a {
		arr = append(arr, Int32(i))
	}
	for _, j := range b {
		arr = append(arr, Int32(j))
	}
	return arr
}

// 多维数组合并-权限
func ArrayMerge(data []string) []interface{} {
	var rule_ids_arr []interface{}
	for _, mainv := range data {
		ids_arr := strings.Split(mainv, `,`)
		for _, intv := range ids_arr {
			rule_ids_arr = append(rule_ids_arr, intv)
		}
	}
	return rule_ids_arr
}

// 把字符串打散为数组
func Axplode(data string) []interface{} {
	var rule_ids_arr []interface{}
	ids_arr := strings.Split(data, `,`)
	for _, intv := range ids_arr {
		rule_ids_arr = append(rule_ids_arr, intv)
	}
	return rule_ids_arr
}

// 获取账号的数据权限
func GetDataAuthor(ctx *GinCtx) ([]interface{}, bool) {
	user_id := ctx.GetInt64("uid") //当前用户ID
	table_str := "admin"           //默认admin
	urlPath := strings.Split(ctx.Request.URL.Path, "/")
	if len(urlPath) > 1 {
		table_str = urlPath[1]
	}
	var acount_id []interface{} = Slice{user_id}
	role_ids, _ := dao.Use().Array(table_str+"_auth_role_access", "role_id", "uid", user_id)
	data_access, _ := dao.Use().ArrayIn(table_str+"_auth_role", "data_access", "id", role_ids)
	if IntInVarArray(1, data_access) { //数据权限0=自己1=自己及子权限，2=全部
		chri_role_ids := GetAllChilIds(table_str+"_auth_role", role_ids) //批量获取子节点id
		uid_ids, _ := dao.Use().ArrayIn(table_str+"_auth_role_access", "uid", "role_id", chri_role_ids)
		for _, val := range uid_ids {
			acount_id = append(acount_id, val)
		}
		return acount_id, true //自己及子权限
	} else if IntInVarArray(0, data_access) {
		return acount_id, true //自己
	}
	return acount_id, false //全部
}

// Int类型是否存在Var数组中
func IntInVarArray(target int, arr []interface{}) bool {
	for _, element := range arr {
		if target == Int(element) {
			return true
		}
	}
	return false
}

// Int类型是否存在interface数组中
func IntInInterfaceArray(target int, arr []interface{}) bool {
	for _, element := range arr {
		if target == Int(element) {
			return true
		}
	}
	return false
}

// 转JSON编码为字符串
func JSONToString(data interface{}) string {
	if str, err := json.Marshal(data); err != nil {
		return ""
	} else {
		return string(str)
	}
}

// 字符串转JSON编码
func StringToJSON(val interface{}) interface{} {
	str := val.(string)
	if strings.HasPrefix(str, "{") && strings.HasSuffix(str, "}") {
		var parameter interface{}
		_ = json.Unmarshal([]byte(str), &parameter)
		return parameter
	} else {
		var parameter []interface{}
		_ = json.Unmarshal([]byte(str), &parameter)
		return parameter
	}
}

// tool-获取树状数组
func GetTreeArray(list List, pid int64, itemprefix string) List {
	childs := ToolFar(list, pid) //获取pid下的所有数据
	var chridnum List
	if childs != nil {
		var number int = 1
		var total int = len(childs)
		for _, v := range childs {
			j := ""
			k := ""
			if number == total {
				j += "└"
				k = ""
				if itemprefix != "" {
					k = "&nbsp;"
				}

			} else {
				j += "├"
				k = ""
				if itemprefix != "" {
					k = "│"
				}
			}
			spacer := ""
			if itemprefix != "" {
				spacer = itemprefix + j
			}
			v["spacer"] = spacer
			v["children"] = GetTreeArray(list, Int64(v["id"]), itemprefix+k+"&nbsp;")
			chridnum = append(chridnum, v)
			number++
		}
	}
	return chridnum
}

// 将getTreeArray的结果返回为二维数组
func GetTreeToList(list []Map, field string) []Map {
	var midleArr []Map
	for _, v := range list {
		var children []Map
		if childrendata, ok := v["children"]; ok && childrendata != nil {
			switch data := childrendata.(type) {
			case []interface{}:
				for _, cv := range data {
					children = append(children, cv.(Map))
				}
			case []Map:
				children = childrendata.([]Map)
			}
		} else {
			children = nil
		}
		delete(v, "children")
		v[field+"_txt"] = fmt.Sprintf("%v %v", v["spacer"], v[field+""])
		if _, ok := v["id"]; ok {
			midleArr = append(midleArr, v)
		}
		if len(children) > 0 {
			newarr := GetTreeToList(children, field)
			midleArr = ArrayMerge_x(midleArr, newarr)
		}
	}
	return midleArr
}

// 数组拼接
func ArrayMerge_x(ss ...[]Map) []Map {
	n := 0
	for _, v := range ss {
		n += len(v)
	}
	s := make([]Map, 0, n)
	for _, v := range ss {
		s = append(s, v...)
	}
	return s
}

// 获取菜单树形-打包代码菜单
func GetRuleTreeArrayByPack(list List, pid int64) List {
	childs := ToolFar(list, pid) //获取pid下的所有数据
	var chridnum List
	for _, v := range childs {
		newdata := GetRuleTreeArrayByPack(list, Int64(v["id"]))
		if newdata != nil {
			v["children"] = GetRuleTreeArrayByPack(list, Int64(v["id"]))
		}
		chridnum = append(chridnum, v)
	}
	return chridnum
}

// base_tool-获取pid下所有数组
func ToolFar(data List, pid int64) List {
	var mapString List
	for _, v := range data {
		if Int64(v["pid"]) == pid {
			mapString = append(mapString, v)
		}
	}
	return mapString
}

// 获取子菜单包含的父级ID-返回全部ID
func GetRulesID(tablename string, field string, menus interface{}) interface{} {
	menus_rang := menus.([]interface{})
	var fnemuid []interface{}
	for _, v := range menus_rang {
		fid := getParentID(tablename, field, v)
		if fid != nil {
			fnemuid = MergeArrInterface(fnemuid, fid)
		}
	}
	r_nemu := MergeArrInterface(menus_rang, fnemuid)
	uni_fnemuid := UniqueArr(r_nemu) //去重
	return uni_fnemuid
}

// 获取所有父级ID
func getParentID(tablename string, field string, id interface{}) []interface{} {
	var pids []interface{}
	var pid int32
	err := dao.DB().Raw("SELECT pid FROM "+dao.Use().TableName(tablename)+" WHERE id = ?", id).Scan(&pid).Error
	if err == nil {
		a_pid := Int32(pid)
		var zr_pid int32 = 0
		if a_pid != zr_pid {
			pids = append(pids, a_pid)
			getParentID(tablename, field, pid)
		}
	}
	return pids
}

// 去重
func UniqueArr(datas []interface{}) []interface{} {
	d := make([]interface{}, 0)
	tempMap := make(map[int]bool, len(datas))
	for _, v := range datas { // 以值作为键名
		keyv := Int(v)
		if tempMap[keyv] == false {
			tempMap[keyv] = true
			d = append(d, v)
		}
	}
	return d
}

// 合并数组-interface
func MergeArrInterface(a, b []interface{}) []interface{} {
	var arr []interface{}
	for _, i := range a {
		arr = append(arr, i)
	}
	for _, j := range b {
		arr = append(arr, j)
	}
	return arr
}

// 将带有逗号的数组中字符串差分合并为数组
func ArraymoreMerge(data []interface{}) []interface{} {
	var rule_ids_arr []interface{}
	for _, mainv := range data {
		ids_arr := strings.Split(String(mainv), `,`)
		for _, intv := range ids_arr {
			rule_ids_arr = append(rule_ids_arr, intv)
		}
	}
	return rule_ids_arr
}

// 获取树结构数据
func GetTreeData(pdata List, parent_id int64, pid_file string) List {
	var returnList List
	for _, v := range pdata {
		if Int64(v[pid_file]) == parent_id {
			children := GetTreeData(pdata, Int64(v["id"]), pid_file)
			if children != nil {
				v["children"] = children
			}
			returnList = append(returnList, v)
		}
	}
	if returnList == nil {
		returnList = make(List, 0)
	}
	return returnList
}

// 获取后台菜单子树结构
func GetMenuChildrenArray(pdata List, parent_id int64, pid_file string) List {
	var returnList List
	for _, v := range pdata {
		if Int64(v[pid_file]) == parent_id {
			children := GetMenuChildrenArray(pdata, Int64(v["id"]), pid_file)
			if children != nil {
				v["children"] = children
			}
			returnList = append(returnList, v)
		}
	}
	return returnList
}

// 删除本地附件
func DelFile(file_list []*gvar.Var) {
	path, _ := os.Getwd()
	for _, val := range file_list {
		deldir := filepath.Join(path, val.String())
		os.Remove(deldir)
	}
}

// 删除单文件本地附件
func DelOneFile(file_path string) error {
	path, _ := os.Getwd()
	deldir := filepath.Join(path, file_path)
	if _, err := os.Stat(deldir); err != nil && os.IsNotExist(err) { //文件不存在直接返回
		return nil
	}
	return os.Remove(deldir)
}

// 判断某个数据表是否存在指定字段
// tablename=表名 field=字段
func DbHaseField(tablename, fields string) bool {
	// 原生 SQL 查询
	rawSQL := "select COLUMN_NAME from information_schema.columns where TABLE_SCHEMA= ? AND TABLE_NAME= ?"
	var dielddata []map[string]any
	err := dao.DB().Raw(rawSQL, String(dbConf_arr["dbname"]), dao.Use().TableName(tablename)).Scan(&dielddata).Error
	if err != nil {
		return false
	}
	var tablefields []interface{}
	for _, val := range dielddata {
		var valjson map[string]interface{}
		mdata, _ := json.Marshal(val)
		json.Unmarshal(mdata, &valjson)
		tablefields = append(tablefields, valjson["COLUMN_NAME"])
	}
	return IsContain(tablefields, fields)
}

// 获取数据表下的字段值，tablename是表名(不含前缀)
func GetTalbeFieldVal(tablename, field string, id interface{}) string {
	var val string
	err := dao.DB().Raw("SELECT "+field+" FROM "+dao.Use().TableName(tablename)+" WHERE id = ?", id).Scan(&val).Error
	if err != nil {
		return ""
	}
	return val
}

// 获取字典数据下的字段值
func GetDicFieldVal(ctx *GinCtx, group_id, val interface{}) interface{} {
	if String(val) == "" {
		return nil
	}
	dicGroupDB := dao.Query().DictionaryGroup
	var tablename string
	dicGroupDB.WithContext(ctx).Where(dicGroupDB.ID.Eq(Int32(group_id))).Select(dicGroupDB.Tablename).Scan(&tablename)
	data := Map{}
	dao.DB().Raw("SELECT keyname,tagcolor FROM "+dao.Use().TableName(tablename)+" WHERE group_id = ? AND keyvalue = ?", group_id, val).Scan(data)
	return data
}

// 获取请求参数id-用于数据保存或更新
func GetEditId(idstr interface{}) (f_id float64) {
	if idstr != nil {
		f_id = Float64(idstr)
	} else {
		f_id = 0
	}
	return
}

// 判断字符串是否包含
func StrContains(str, filed string) bool {
	return strings.Contains(str, filed)
}

// 把字符串打散为数组
func SplitAndStr(str, step string) []string {
	return strings.Split(str, step)
}

// 把数组转字符串,号分隔
func ArrayToStr(data interface{}, step string) string {
	if data != nil && data != "" {
		data_arr := data.([]interface{})
		var str_arr = make([]string, len(data_arr))
		for k, v := range data_arr {
			str_arr[k] = fmt.Sprintf("%v", v)
		}
		return strings.Join(str_arr, step)
	} else {
		return ""
	}
}

// 判断字符串是否在一个数组中
func StrInArray(target string, str_array []string) bool {
	for _, element := range str_array {
		if target == element {
			return true
		}
	}
	return false
}

// 获取分类下全部子id包含自己
func CateAllChilId(tablename string, cid interface{}) []interface{} {
	cids := GetAllChilId(tablename, cid)
	return append(cids, cid)
}

// 判断请求路由是否是该模块
func IsModelPath(path, model string) bool {
	if strings.HasPrefix(path, "/"+model+"/") {
		return true
	} else {
		return false
	}
}

// 隐藏手机号等敏感信息用*替换展示
func HideStrInfo(strtype, val string) string {
	if val == "" {
		return ""
	}
	switch strtype {
	case "email":
		var arr = strings.Split(val, "@")
		var star = ""
		if len(arr[0]) <= 3 {
			star = "*"
			arr[0] = gstr.SubStr(arr[0], 0, len(arr[0])) + star
		} else {
			star = "***"
			arr[0] = gstr.SubStr(arr[0], 0, 1) + star + gstr.SubStr(arr[0], len(arr[0])-1, 1)
		}
		return arr[0] + "@" + arr[1]
	case "mobile":
		if len(val) <= 10 {
			return val
		}
		return val[:3] + "****" + val[len(val)-4:]
	}
	return ""
}

// int32数组去重
func RemoveDuplicates(arr []int32) []int32 {
	uniqueMap := make(map[int32]bool)
	var result []int32
	for _, str := range arr {
		if _, exists := uniqueMap[str]; !exists {
			uniqueMap[str] = true
			result = append(result, str)
		}
	}
	return result
}

// Int32Join []int32转逗号分隔字符串
func Int32Join(ids []int32, sep string) string {
	var strIDs []string
	for _, id := range ids {
		strIDs = append(strIDs, strconv.FormatInt(int64(id), 10))
	}
	return strings.Join(strIDs, sep)
}

// TimeNow 获取当前服务器本地时间
func TimeNow() time.Time {
	return time.Now()
}
