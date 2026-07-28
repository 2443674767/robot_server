package auth

import (
	"encoding/base64"
	"errors"
	"gofly/utils/tools/empty"
	"gofly/utils/tools/gconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	BearerPrefix = "Bearer "
)

// 获取请求中的token
func GetRequestToken(ctx *gin.Context) string {
	// 请求头获取
	tokenstr := ctx.GetHeader("Authorization")
	if empty.IsEmpty(tokenstr) {
		tokenstr = ctx.GetHeader("authorization")
	}
	if !empty.IsEmpty(tokenstr) {
		if CacheToken && ctx.GetHeader("Ott") == "true" {
			pathArr := strings.Split(ctx.Request.URL.Path, "/")
			//处理动态token-30秒时效
			if len(pathArr) > 1 && strings.Contains(DynamicToken, pathArr[1]) {
				//可以换成AES加密更安全
				// tokenstrAes, err := cryptojs.AesDecrypt(tokenstr)
				// if tokenstrAesArr := strings.Split(tokenstrAes, "#"); err == nil && len(tokenstrAesArr) == 2 && (time.Now().Unix()-gconv.Int64(tokenstrAesArr[1]) < 30) { //30秒时效
				//Base64 编解码(效率高)
				tokenstrAes, err := base64.StdEncoding.DecodeString(tokenstr)
				if tokenstrAesArr := strings.Split(string(tokenstrAes), "#"); err == nil && len(tokenstrAesArr) == 2 && (time.Now().Unix()-gconv.Int64(tokenstrAesArr[1]) < 30) { //30秒时效
					tokenstr = tokenstrAesArr[0]
				} else {
					return ""
				}
			}
		}
		tokenArr := strings.Split(tokenstr, BearerPrefix)
		if len(tokenstr) > 0 && len(tokenArr) >= 2 && tokenArr[0] == BearerPrefix {
			return tokenArr[1]
		} else if len(tokenstr) > 0 && len(tokenArr) == 1 {
			return tokenArr[0]
		}
	}

	// 参数传递token
	if qtoken := ctx.GetString("token"); qtoken != "" {
		return qtoken
	}
	// Cookies传递token
	if cToken, err := ctx.Cookie("token"); err == nil {
		return cToken
	}
	return ""
}

// 获取jwt-token中的数据，token是加密过的字符串
func GetTokenData(token string) (tData *TokenData, key string, err error) {
	var uuid string
	key, uuid, err = DecryptToken(token)
	if err != nil {
		return
	}
	tData, err = getCache(TokenCacheKey + key)
	if tData == nil || tData.UuId != uuid {
		err = errors.New("token is invalid")
	}
	return
}

// 获取jwttoken
func GetJwtToken(ctx *gin.Context) (tData *TokenData, key string, err error) {
	token := GetRequestToken(ctx)
	tData, key, err = GetTokenData(token)
	return
}
