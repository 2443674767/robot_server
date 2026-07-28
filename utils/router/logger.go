package router

//保存操作日志记录到数据库
import (
	"bytes"
	"fmt"
	"gofly/dao"
	"gofly/dao/model"
	"gofly/utils/gf"
	"gofly/utils/plugin"
	"io"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 保存日志记录到
// CustomResponseWriter 封装 gin ResponseWriter 用于获取回包内容。
type CustomResponseWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w CustomResponseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// 忽略操作日志请求接口
var (
	noSaveUrl = []string{"/user/login", "/datacenter/upfile/upload", "/datacenter/upfile/saveFile", "/user/logout", "/system/log/", "/system/dept/getParen", "/system/account/getRole", "/system/rule/getRoutes", "/system/rule/getParent", "/user/getUserinfo", "/user/account/getMenu", "/dashboard/workplace/getQuick"}
)

// 日志中间件。
func Logger() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		url := ctx.FullPath()
		//目前日志只监听admin、business、common模块，如果新增的模块需要记录操作日志，请求自行添加。
		if GetNoInUrls(url) && (strings.HasPrefix(url, "/admin/") || strings.HasPrefix(url, "/common/")) {
			// 记录请求时间
			start := time.Now()
			// 使用自定义 ResponseWriter-解决多次读取响应 Body 的问题
			crw := &CustomResponseWriter{
				body:           bytes.NewBufferString(""),
				ResponseWriter: ctx.Writer,
			}
			ctx.Writer = crw

			// 打印请求信息
			var reqBodyStr interface{}
			if ctx.Request.Method == "GET" {
				dataMap := make(map[string]interface{})
				for key, _ := range ctx.Request.URL.Query() {
					dataMap[key] = ctx.Query(key)
				}
				reqBodyStr = gf.JSONToString(dataMap)
			} else {
				reqBody, _ := ctx.GetRawData()
				// 请求包体写回-解决多次读取请求 Body 的问题
				if len(reqBody) > 0 {
					ctx.Request.Body = io.NopCloser(bytes.NewBuffer(reqBody))
				}
				reqBodyStr = string(reqBody)
			}

			// 执行请求处理程序和其他中间件函数
			ctx.Next()

			// 记录回包内容和处理时间
			end := time.Now()
			latency := end.Sub(start)
			//操作日志入库
			getuser, exists := ctx.Get("user")
			if exists { //对登录用户进行操作日志保存
				user := getuser.(map[string]interface{})
				ip := ctx.ClientIP()
				address, _ := plugin.NewIpRegion(ip)
				savedata := gf.Map{
					"uid":          user["uid"],
					"account_id":   user["account_id"],
					"tenant_id":    user["tenant_id"],
					"method":       ctx.Request.Method,
					"url":          url,
					"ip":           ip,
					"address":      address,
					"status":       crw.Status(),
					"req_headers":  gf.JSONToString(ctx.Request.Header),
					"req_body":     reqBodyStr,
					"resp_body":    crw.body.Bytes(),
					"resp_headers": gf.JSONToString(crw.Header()),
					"latency":      latency.String(),
					"created_at":   time.Now().Format("2006-01-02 15:04:05"),
				}
				url_arr := strings.Split(url, "/")
				if len(url_arr) > 1 {
					savedata["type"] = url_arr[1]
					var authRule map[string]interface{}
					modelName := url_arr[1]
					if modelName == "common" {
						modelName = "admin"
					}
					err := dao.DB().Session(&gorm.Session{Logger: logger.Discard}).Raw("SELECT id,pid,title,des FROM "+dao.Use().TableName(modelName)+"_auth_rule WHERE path = ? ", ctx.FullPath()).Scan(&authRule).Error
					if err == nil {
						var ftitle string
						if len(authRule) > 0 {
							dao.DB().Session(&gorm.Session{Logger: logger.Discard}).Raw("SELECT title FROM "+dao.Use().TableName(modelName)+"_auth_rule WHERE id = ? ", authRule["pid"]).Scan(&ftitle)
							substr := authRule["des"]
							if gf.IsEmpty(authRule["des"]) {
								substr = authRule["title"]
							}
							savedata["des"] = fmt.Sprintf("%s【%s】", ftitle, substr)
						} else if len(url_arr) > 3 {
							dao.DB().Session(&gorm.Session{Logger: logger.Discard}).Raw("SELECT title FROM "+dao.Use().TableName(modelName)+"_auth_rule WHERE component = ? ", fmt.Sprintf("/%s/%s/index", url_arr[2], url_arr[3])).Scan(&ftitle)
							des := ""
							switch url_arr[len(url_arr)-1] {
							case "getList":
								des = "查看"
							case "save":
								des = "添加/编辑"
							case "del":
								des = "删除"
							case "upStatus":
								des = "更新状态"
							case "getContent":
								des = "获取详情"
							case "exportExcel":
								des = "导出数据"
							default:
								des = "其他"
							}
							savedata["des"] = fmt.Sprintf("%s【%s】", ftitle, des)
						}
					}
				}
				dao.Query().OperationLog.WithContext(ctx).Session(&gorm.Session{Logger: logger.Discard}).UnderlyingDB().Model(&model.OperationLog{}).Create(savedata)
			}

		} else {
			ctx.Next()
		}
	}
}

// 过滤忽略请求
func GetNoInUrls(url string) bool {
	for _, val := range noSaveUrl {
		if strings.Contains(url, val) {
			return false
		}
	}
	return true
}
