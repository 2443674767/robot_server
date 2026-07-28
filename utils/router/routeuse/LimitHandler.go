// ====================
// 限流可以防止滥用、暴力攻击和资源耗尽。你可以使用标准库的 golang.org/x/time/rate 包构建一个简单的按客户端限流中间件。
// ====================
package routeuse

import (
	"gofly/utils/router/appcfg"
	"gofly/utils/tools/gconv"
	"gofly/utils/tools/gvar"
	"net/http"
	"time"

	"github.com/didip/tollbooth"
	"github.com/didip/tollbooth/limiter"
	"github.com/gin-gonic/gin"
)

// 限流
func RateLimiter() gin.HandlerFunc {
	return func(c *gin.Context) {
		limiterMax := appcfg.AppConf_arr["limiterMax"]
		if limiterMax != nil {
			//读取限流配置失败,默认：30次/秒，峰值 10
			limiterMax = gvar.New(30)
		}
		// 创建过期配置
		expiraOption := &limiter.ExpirableOptions{
			// 默认过期时间设置为 3 分钟
			DefaultExpirationTTL: 3 * time.Minute,
		}
		lmt := tollbooth.NewLimiter(gconv.Float64(limiterMax), expiraOption)
		lmt.SetMessage("您访问过于频繁，系统安全检查认为恶意攻击。")
		httpError := tollbooth.LimitByRequest(lmt, c.Writer, c.Request)
		if httpError != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    1,
				"message": "您的操作太频繁，请稍后再试！",
				"data":    nil,
			})
			// c.Data(httpError.StatusCode, lmt.GetMessageContentType(), []byte(httpError.Message))
			c.Abort()
		} else {
			c.Next()
		}
	}
}
