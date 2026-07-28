package router

import (
	//一定要导入这个Controller包，用来注册需要访问的方法
	//这里路由-由构架是添加-开发者仅在指定工程目录下controller.go文件添加宝即可
	"context"
	"fmt"
	AppCtr "gofly/app"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"strings"
	"time"

	"gofly/utils/auth"
	"gofly/utils/gf"
	"gofly/utils/router/appcfg"
	"gofly/utils/router/routeuse"
	"gofly/utils/tools/gconv"
	"gofly/utils/tools/gfile"
	"gofly/utils/tools/glogger"
	"gofly/utils/tools/gstr"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// 优雅重启/停止服务器
func RunServer() {
	//启动定时清除日志文件功能
	if appcfg.OpenLogger.Bool() {
		glogger.StartCleanCron()
	}

	//设置cpu个数
	cpu_num := gconv.Int(appcfg.AppConf_arr["cpunum"])
	if cpu_num > 0 {
		mycpu := runtime.NumCPU()
		if cpu_num > mycpu { //如果配置cpu核数大于当前计算机核数，则等当前计算机核数
			cpu_num = mycpu
		}
		runtime.GOMAXPROCS(cpu_num)
	}
	//加载gin路由
	path, _ := os.Getwd()
	R := InitRouter(path)
	//把路由推保存文件中
	routerfilePath := "runtime/app/routers.txt"
	routerfileFullPath := filepath.Join(path, routerfilePath)
	routes := ""
	for _, route := range R.Routes() {
		if !strings.Contains(route.Path, "filename") && route.Path != "/" && !strings.Contains(route.Path, "/*filepath") {
			routes = routes + fmt.Sprintf("%v:%v\n", route.Method, route.Path)
		}
	}
	gfile.PutBytes(routerfileFullPath, []byte(routes))
	//默认禁止ip直接访问
	addrStr := "127.0.0.1:" + gconv.String(appcfg.AppConf_arr["port"])
	if gconv.Bool(appcfg.AppConf_arr["ipAccess"]) {
		addrStr = ":" + gconv.String(appcfg.AppConf_arr["port"])
	}
	srv := &http.Server{
		Addr:           addrStr,
		Handler:        R,
		MaxHeaderBytes: 1024 * 20,        // 最大请求头20KB
		ReadTimeout:    10 * time.Second, // 读超时，防慢速攻击
	}
	//启动服务
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen serve err: %s\n", err)
		}
	}()

	// 启动pprof服务
	if gf.Bool(appcfg.AppConf_arr["runpprof"]) {
		R.GET("/debug/pprof/*any", gin.WrapH(http.DefaultServeMux))
		go func() {
			if err := http.ListenAndServe("127.0.0.1:8081", nil); err != nil {
				log.Fatalf("pprof server failed: %s\n", err)
			}
		}()
		fmt.Printf("%c[1;40;33m%s%c[0m\n", 0x1B, "已开启pprof性能分析工具-浏览器访问：​​http://127.0.0.1:8081/debug/pprof/ ​进行查看​", 0x1B)
	}
	//开发环境
	if gconv.String(appcfg.AppConf_arr["runEnv"]) == "debug" {
		fmt.Printf("%c[1;40;32m%s%c[0m\n", 0x1B, "如果还没有安装-请在浏览器访问​进行​安装​：​​http://127.0.0.1:"+gconv.String(appcfg.AppConf_arr["port"])+"/install", 0x1B)
		fmt.Println("Listening and serving HTTP on :" + gconv.String(appcfg.AppConf_arr["port"]))
	}

	// 等待中断信号以优雅地关闭服务器（设置 5 秒的超时时间）
	// Wait for interrupt signal to gracefully shutdown the server with a timeout of 5 seconds.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	glogger.Error("Shutdown Server ...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		glogger.Error(fmt.Sprintf("Server Shutdown err:%v", err))
	}
	// catching ctx.Done(). timeout of 5 seconds.
	select {
	case <-ctx.Done():
		glogger.Error("timeout of 5 seconds.")
	}
	glogger.Error("Server exiting/服务已经优雅退出")
}

// 路由初始化
func InitRouter(path string) *gin.Engine {
	runEnv := gconv.String(appcfg.AppConf_arr["runEnv"])
	//初始化路由
	R := gin.New()
	R.Use(gin.Recovery())
	if runEnv == "release" {
		R.Use(glogger.GinLogger())
	} else {
		R.Use(gin.Logger())
	}
	//控制台日志级别
	gin.SetMode(runEnv)                        //ReleaseMode 为方便调试，Gin 框架在运行的时候默认是debug模式，在控制台默认会打印出很多调试日志，上线的时候我们需要关闭debug模式，改为release模式。
	gin.DisableConsoleColor()                  //取消日志彩色转义字符,运维优化
	R.SetTrustedProxies([]string{"127.0.0.1"}) // 设置受信任代理,如果不设置默认信任所有代理，不安全
	//根域名下获取static静态文件,主要用于微信公众号验证
	R.GET("/:filename", func(c *gin.Context) {
		filename := c.Param("filename")
		if filename == "" || strings.Contains(filename, "../") || strings.Contains(filename, "./") {
			c.JSON(404, gin.H{"code": 404, "message": "文件不存在或者禁止访问"})
			return
		}
		filePath := filepath.Join(path, "/resource/static/", filename)
		if _, err := os.Stat(filePath); err == nil {
			c.File(filePath)
			c.Abort()
			return
		} else {
			c.JSON(404, gin.H{"code": 404, "message": "文件不存在"})
		}
	})
	//静态资源处理-在static目录部署vue项目用的
	staticAssets := filepath.Join(path, "resource/static/assets")
	if _, err := os.Stat(staticAssets); err == nil {
		R.Static("/assets", "./resource/static/assets")
	}
	staticStatic := filepath.Join(path, "resource/static/static")
	if _, err := os.Stat(staticStatic); err == nil {
		R.Static("/static", "./resource/static/static")
	}
	//注册网页资源访问目录，如admin后台、business后台、手机h5页面
	webStatic := appcfg.AppConf_arr["webStatic"]
	if webStatic != "" {
		webStatic_arr := strings.Split(gconv.String(webStatic), ",")
		for _, val := range webStatic_arr {
			file_path := filepath.Join(path, "/resource/", val)
			if _, err := os.Stat(file_path); err != nil {
				if !os.IsExist(err) {
					os.MkdirAll(file_path, os.ModePerm)
				}
			}
			R.Static("/"+val, "./resource/"+val)
		}
	}
	//文件访问拦截-根据业务需求添加
	// R.Use(resourceAuth())
	//附件资源访问
	R.Static("/resource/uploads", "./resource/uploads")
	//在debug环境下注册安装页面
	if gconv.String(appcfg.AppConf_arr["runEnv"]) == "debug" {
		R.StaticFile("/install", "./devsource/developer/install/install.html")
		R.StaticFile("/vue.js", "./devsource/developer/install/vue.js")
		R.StaticFile("/axios.min.js", "./devsource/developer/install/axios.min.js")
	}
	// 为 multipart forms 设置较低的内存限制 (默认是 32 MiB)
	R.MaxMultipartMemory = 8 << 20 // 8 MiB
	//0.跨域访问-注意跨域要放在gin.Default下
	var allowurl_arr []string = []string{"*"}
	allowurl_str := gconv.String(appcfg.AppConf_arr["allowurl"])
	if allowurl_str != "" {
		allowurl_arr = strings.Split(allowurl_str, `,`)
	}
	R.Use(cors.New(cors.Config{
		AllowOrigins:     allowurl_arr,
		AllowOriginFunc:  func(origin string) bool { return true },
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowHeaders:     []string{"*"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
	// 请求头加固
	R.Use(routeuse.SecureHeader())
	// 对错误处理
	R.Use(routeuse.Recover)
	// 限流rate-limit 中间件
	R.Use(routeuse.RateLimiter())
	// 判断接口是否合法
	if gconv.Bool(appcfg.AppConf_arr["validityApi"]) {
		R.Use(routeuse.ValidityAPi())
	}
	// 验证token
	R.Use(useToken)
	//接口请求日志
	R.Use(Logger())
	//5.没有注册的路由
	R.NoRoute(func(c *gin.Context) {
		pathURL := c.Request.URL.Path
		method := c.Request.Method
		if method == "GET" && pathURL == "/" { //部署同服务器域名下的网站
			indexfilePath := filepath.Join(path, "resource/static/index.html")
			if _, err := os.Stat(indexfilePath); err == nil {
				data, err := os.ReadFile(indexfilePath)
				if err != nil {
					c.AbortWithError(http.StatusInternalServerError, err)
					return
				}
				c.Data(http.StatusOK, "text/html; charset=utf-8", data)
			} else {
				c.Redirect(http.StatusMovedPermanently, fmt.Sprintf("%v/", appcfg.AppConf_arr["rootview"]))
			}
			c.Abort()
			return
		}
		//找不到路由
		c.JSON(http.StatusOK, gin.H{"code": 404, "message": "您" + method + "请求地址：" + pathURL + "不存在！"})
	})
	Bind(R)
	return R
}

// 绑定路由 m是方法GET POST等
// 绑定基本路由
func Bind(c *gin.Engine) {
	for _, route := range gf.Routes {
		if route.HttpMethod == "GET" {
			c.GET(route.Path, minHandler(route), gf.PathMatch(route.Path, route))
		}
		if route.HttpMethod == "POST" {
			c.POST(route.Path, minHandler(route), gf.PathMatch(route.Path, route))
		}
		if route.HttpMethod == "DELETE" {
			c.DELETE(route.Path, minHandler(route), gf.PathMatch(route.Path, route))
		}
		if route.HttpMethod == "PUT" {
			c.PUT(route.Path, minHandler(route), gf.PathMatch(route.Path, route))
		}
	}
}

// 解析token信息，当jwtempty=true时会被路由拦截
func useToken(ctx *gin.Context) {
	user, err := auth.ParseToken(ctx)
	if err != nil {
		ctx.Set("jwtempty", true)
		ctx.Set("jwtmsg", err.Error())
	} else {
		if userMap, ok := user.Data.(map[string]interface{}); ok {
			ctx.Set("jwtempty", false)
			ctx.Set("user", userMap)
			if uid, uok := userMap["uid"]; uok {
				ctx.Set("uid", gf.Int64(uid))
			}
			if accountId, uok := userMap["account_id"]; uok {
				ctx.Set("account_id", gf.Int32(accountId))
			}
			if tenantId, uok := userMap["tenant_id"]; uok {
				ctx.Set("tenant_id", gf.Int32(tenantId))
			}
			ctx.Set("sub_token", user.Subject)
		} else {
			ctx.Set("jwtempty", true)
			ctx.Set("jwtmsg", "Failed to parse the content of the token")
		}
	}
}

// minHandler统一处理登录操作和独立模块处理路由拦截
func minHandler(rule gf.Route) gin.HandlerFunc {
	return func(c *gin.Context) {
		//先处理登录
		noVerifyToken := gconv.String(appcfg.AppConf_arr["noVerifyToken"])
		var noVerifyToken_arr []string
		if noVerifyToken != "" {
			noVerifyToken_arr = strings.Split(noVerifyToken, `,`)
		} else {
			noVerifyToken_arr = make([]string, 0)
		}
		rootPath := strings.Split(rule.Path, "/")
		if !gstr.InArray(noVerifyToken_arr, rootPath[0]) && gf.NeedLoginMatch(rule.Action, rule.NoNeedLogin) && c.GetBool("jwtempty") { //需要登录-且token无效
			gf.Failed().SetMsg(gf.LocaleMsg().SetLanguage(c.Request.Header.Get("locale")).Message("sys_login_invalid")).SetExdata(c.GetString("jwtmsg")).SetCode(401).Regin(c)
			c.Abort()
			return
		} else {
			//放行token登录验证
			c.Set("Action", rule.Action)
			c.Set("NoNeedAuths", rule.NoNeedAuths)
			AppCtr.RouterHandler(c)
		}
	}
}

// 在此添加文件访问拦截
func resourceAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		url := c.FullPath()
		if strings.HasPrefix(url, "/resource/uploads") {
			c.JSON(http.StatusOK, gin.H{
				"code":    401,
				"message": "没有文件访问权限",
				"data":    "",
				"exdata":  "",
			})
			c.Abort()
		} else {
			c.Next()
		}
	}
}
