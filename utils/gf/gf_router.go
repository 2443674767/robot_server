package gf

/**
* 自动路由工具
 */
import (
	"gofly/utils/tools/gstr"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
)

// 路由结构体
type Route struct {
	Path        string         //url路径
	HttpMethod  string         //http方法 get post
	Method      reflect.Value  //方法路由
	Args        []reflect.Type //参数类型
	Action      string         //函数名
	NoNeedLogin []string       //无需登录的方法，全部忽略则为["*"]
	NoNeedAuths []string       //无需接口权限方法，全部忽略则为["*"]
}

// 路由集合
var Routes = []Route{}

// 注册控制器中的路由，controller必须是结构体指针(&结构体)
func Register[T any](controller *T) bool {
	ctrlVal := reflect.ValueOf(controller)
	ctrlType := ctrlVal.Type()     // 指针类型 *T
	structVal := ctrlVal.Elem()    // 结构体 Value
	structType := structVal.Type() // 结构体 Type
	// 包路径、结构体名称
	PkgPathstr := structType.PkgPath()
	ctrlName := ctrlType.String()

	var noNeedLoginSlice []string = make([]string, 0) //1.无需要验证权限
	if structVal.NumField() > 0 && structVal.FieldByName("NoNeedLogin").IsValid() {
		noNeedLoginSlice = Strings(structVal.FieldByName("NoNeedLogin"))
	}
	var noNeedAuthsSlice []string = make([]string, 0) //2.无需要验证权限
	if structVal.NumField() > 0 && structVal.FieldByName("NoNeedAuths").IsValid() {
		noNeedAuthsSlice = Strings(structVal.FieldByName("NoNeedAuths"))
	}
	var CustomRoutes Map //3.自定义路由
	if structVal.NumField() > 0 && structVal.FieldByName("CustomRoutes").IsValid() {
		custom_route := structVal.FieldByName("CustomRoutes").Interface()
		switch custom_route := custom_route.(type) {
		case Map:
			CustomRoutes = custom_route
		}
	}
	//非控制器或无方法则直接返回
	if ctrlVal.NumMethod() == 0 {
		return false
	}
	rootPkg := ""
	idx := strings.Index(PkgPathstr, "/app/")
	if idx != -1 {
		rootPkg = PkgPathstr[idx+len("/app/"):]
	}
	module := ctrlName
	if strings.Contains(ctrlName, ".") {
		module = ctrlName[strings.Index(ctrlName, ".")+1:]
	}
	if module == "Index" { //去index
		module = "/"
	} else {
		module = "/" + strings.ToLower(module) + "/"
	}
	for i := 0; i < ctrlVal.NumMethod(); i++ {
		method := ctrlVal.Method(i)
		action := ctrlType.Method(i).Name
		//拼接路由地址
		httpMethod := "POST"
		path := rootPkg + module + gstr.LcFirst(action)
		//遍历参数
		params := make([]reflect.Type, 0, ctrlVal.NumMethod())
		if (strings.HasPrefix(action, "Get") && !strings.HasPrefix(action, "GetPost")) || action == "Index" {
			httpMethod = "GET"
		} else if strings.HasPrefix(action, "Del") || action == "Del" {
			httpMethod = "DELETE"
		} else if strings.HasPrefix(action, "Put") || action == "Put" {
			httpMethod = "PUT"
		}
		for j := 0; j < method.Type().NumIn(); j++ {
			params = append(params, method.Type().In(j))
		}
		if strings.HasSuffix(action, "ParId") {
			path = path + "/:id"
		}
		if curl, ok := CustomRoutes[action]; ok { //自定义路由-优先级最高-只有没自定义路由才使用默认生成路由
			curl_arr := strings.Split(curl.(string), ":")
			if len(curl_arr) == 2 {
				httpMethod = curl_arr[0]
				path = curl_arr[1]
			}
		}
		route := Route{Path: path, Method: method, Args: params, Action: action, NoNeedLogin: noNeedLoginSlice, NoNeedAuths: noNeedAuthsSlice, HttpMethod: httpMethod}
		Routes = append(Routes, route)
		if strings.HasPrefix(action, "GetPost") { //再增加一个get请求
			route := Route{Path: path, Method: method, Args: params, Action: action, NoNeedLogin: noNeedLoginSlice, NoNeedAuths: noNeedAuthsSlice, HttpMethod: "GET"}
			Routes = append(Routes, route)
		}
	}
	return true
}

// 根据path匹配对应的方法
func PathMatch(path string, route Route) gin.HandlerFunc {
	return func(c *gin.Context) {
		fields := strings.Split(path, "/")
		if len(fields) < 2 { //路径最少两层，少于两层将无法注册
			return
		}
		if len(Routes) > 0 {
			arguments := make([]reflect.Value, 1)
			arguments[0] = reflect.ValueOf(c) // *gin.Context
			route.Method.Call(arguments)
		}
	}
}

// 检测当前控制器和方法是否需要权限认证-需要=true,不需要=false
// Action是请求方法，NoNeedAuths需要验证权限的数组
func NeedAuthMatch(c *GinCtx) bool {
	Actionstr := gstr.LcFirst(c.GetString("Action"))
	for _, eachItem := range c.GetStringSlice("NoNeedAuths") {
		if gstr.LcFirst(eachItem) == Actionstr || eachItem == "*" {
			return false
		}
	}
	return true
}

// 判断是否需要登录验证-需要=true,不需要=false
func NeedLoginMatch(actiond string, noNeedLoginMap []string) bool {
	Actionstr := gstr.LcFirst(actiond)
	for _, eachItem := range noNeedLoginMap {
		if gstr.LcFirst(eachItem) == Actionstr || eachItem == "*" {
			return false
		}
	}
	return true
}
