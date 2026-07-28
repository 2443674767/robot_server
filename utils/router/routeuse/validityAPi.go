package routeuse

import (
	"encoding/base64"
	"gofly/utils/router/appcfg"
	"gofly/utils/tools/gconv"
	"gofly/utils/tools/gmd5"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// 验证接口合法性
func ValidityAPi() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 验证-根目录
		var noVerifyApiRoot []string
		apiRoot_str := gconv.String(appcfg.AppConf_arr["noVerifyApiRoot"])
		if apiRoot_str != "" {
			noVerifyApiRoot = strings.Split(apiRoot_str, `,`)
		} else {
			noVerifyApiRoot = make([]string, 0)
		}
		// 验证-具体路径
		var NoVerifyAPI_arr []string
		NoVerifyAPI_str := gconv.String(appcfg.AppConf_arr["noVerifyApi"])
		if NoVerifyAPI_str != "" {
			NoVerifyAPI_arr = strings.Split(NoVerifyAPI_str, `,`)
		} else {
			NoVerifyAPI_arr = make([]string, 0)
		}
		rootPath := strings.Split(c.Request.URL.Path, "/")
		if c.Request.URL.Path == "/" || (len(rootPath) > 2 && IsContain(noVerifyApiRoot, rootPath[1])) || IsContain(NoVerifyAPI_arr, c.Request.URL.Path) || strings.Contains(c.Request.URL.Path, "/common/uploadfile/get_image") { //过滤附件访问接口
			c.Next() //不需验证
		} else { //需要验证
			//判断时间差
			apisecret := gconv.String(appcfg.AppConf_arr["apisecret"])
			Apiverify := c.Request.Header.Get("Apiverify")
			decodedBytes, err := base64.StdEncoding.DecodeString(Apiverify)
			if err != nil || Apiverify == "" { //先判断数据是否传值
				c.JSON(http.StatusOK, gin.H{
					"code":    1,
					"message": "您的请求不合法，请按规范请求数据",
					"data":    nil,
				})
				c.Abort()
				return
			}
			decodedBytes_arr := strings.Split(string(decodedBytes), "#")
			encrypt := decodedBytes_arr[0]
			verifytime := decodedBytes_arr[1]
			mdsecret, _ := gmd5.Encrypt(apisecret + verifytime)
			verifytimeint, _ := strconv.ParseInt(verifytime, 10, 64)
			if mdsecret == encrypt && (time.Now().Unix()-verifytimeint < 60*5) { //验证码5分钟内有效，所以两端时间戳相差不能大于5分钟
				c.Next()
			} else {
				c.JSON(http.StatusOK, gin.H{
					"code":    1,
					"message": "您的请求不合法，请按规范请求数据!",
					"data":    nil,
				})
				c.Abort()
			}
		}
	}
}

// 数组包含
func IsContain(items []string, item string) bool {
	for _, eachItem := range items {
		if eachItem == item {
			return true
		}
	}
	return false
}
