package auth

import (
	"gofly/utils/tools/gcfg"
	"gofly/utils/tools/gconv"
	"gofly/utils/tools/gctx"
)

// 常量
const (
	JwtTokenOK      int = 200 //token有效
	JwtTokenInvalid int = 401 //无效的token
	JwtTokenExpired int = 403 //过期的token
)

// 从配置中获取配置数据
var (
	appConf, _     = gcfg.Instance("app").Get(gctx.New(), "app")
	appConf_arr    = gconv.Map(appConf)
	TokenTimeout   = gconv.Int64(appConf_arr["tokenTimeout"])    // 超时时间 默认30分钟（秒）：60 * 30
	MaxRefresh     = gconv.Int64(appConf_arr["maxRefresh"])      // 刷新token时间 默认10分钟（秒）：60 * 10(maxRefresh值为0时,token将不会自动刷新)
	TokenSecretKey = gconv.String(appConf_arr["tokenSecretKey"]) //JWT-Token加密key 32位
	CacheToken     = gconv.Bool(appConf_arr["cacheToken"])       // 是否缓存的Token信息
	MultiLogin     = gconv.Bool(appConf_arr["multiLogin"])       // 是否允许多点登录
	TokenCacheKey  = "gftoken_"                                  // 缓存key前缀（防止与其他业务冲突）
	DynamicToken   = gconv.String(appConf_arr["dynamicToken"])   // 开启动态token模块
)
