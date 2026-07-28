// 安全头：防 XSS、劫持、嗅探
package routeuse

import "github.com/gin-gonic/gin"

func SecureHeader() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// 禁止MIME类型嗅探，防止文件上传XSS
		ctx.Header("X-Content-Type-Options", "nosniff")
		// 开启XSS防护
		ctx.Header("X-XSS-Protection", "1; mode=block")
		// 禁止页面被iframe嵌套，防点击劫持
		ctx.Header("X-Frame-Options", "DENY")
		// 内容安全策略，限制资源加载（按需调整）
		ctx.Header("Content-Security-Policy", "default-src 'self'")
		// 强制HTTPS
		// ctx.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		// 隐藏服务版本，防止指纹探测
		ctx.Header("Server", "unknown")
		ctx.Next()
	}
}
