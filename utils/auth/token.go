// ======================================================
// MaxRefresh：处理携带token的请求时当前时间大于超时时间并小于缓存刷新时间时token将自动刷新即重置token存活时间
// ======================================================
package auth

import (
	"encoding/base64"
	"errors"
	"fmt"
	"gofly/utils/tools/cryptojs"
	"gofly/utils/tools/gmd5"
	"gofly/utils/tools/grand"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// TokenData Token 数据
type TokenData struct {
	JwtToken string `json:"jwtToken"`
	UuId     string `json:"uuId"`
}

// token存活时间 (存活时间 = 超时时间 + 缓存刷新时间)
func makeExpiresTime() time.Time {
	return time.Now().Add(time.Second * time.Duration(TokenTimeout+MaxRefresh))
}

// GenerateToken 应用层传入加密内容返回token
func GenerateToken(key string, data interface{}) (keys string, err error) {
	if len(key) <= 0 {
		err = fmt.Errorf("The key value cannot be empty")
		return
	}
	var (
		uuid   string
		tData  *TokenData
		tokens string
	)
	// 支持多端重复登录，返回新token（在使用缓存情况下生效）
	if CacheToken && MultiLogin {
		tData, _ = getCache(TokenCacheKey + key)
		if tData != nil {
			keys, _, err = EncryptToken(key, tData.UuId)
			doRefresh(key, tData) //刷新token
			return
		}
	}
	//获取 JWT创建原token
	tokens, err = createToken(CustomClaims{
		data,
		jwt.RegisteredClaims{
			NotBefore: jwt.NewNumericDate(time.Unix(time.Now().Unix()-10, 0)), // 生效开始时间
			ExpiresAt: jwt.NewNumericDate(makeExpiresTime()),                  // 失效截止时间
			// ID:        key,                                                    // 把key作为Id，在非缓存登录使用(jti)
		},
	})
	if err != nil {
		return
	}
	//使用缓存存储Token
	if CacheToken {
		keys, uuid, err = EncryptToken(key)
		if err != nil {
			return
		}
		err = setCache(TokenCacheKey+key, &TokenData{
			JwtToken: tokens,
			UuId:     uuid,
		})
	} else { //返回原生jwt-token
		keys = tokens
	}
	return
}

// 创建原始Jwt-token
func GenerateJwtToken(key string, data interface{}) (tokens string, err error) {
	tokens, err = createToken(CustomClaims{
		data,
		jwt.RegisteredClaims{
			NotBefore: jwt.NewNumericDate(time.Unix(time.Now().Unix()-10, 0)), // 生效开始时间
			ExpiresAt: jwt.NewNumericDate(makeExpiresTime()),                  // 失效截止时间
			ID:        key,                                                    // 把key作为Id，在单点登录使用(jti)
		},
	})
	return
}

// 仅生成token,不对token进行加密
func MarkeToken(data interface{}) (token string, err error) {
	token, err = createToken(CustomClaims{
		data,
		jwt.RegisteredClaims{
			NotBefore: jwt.NewNumericDate(time.Unix(time.Now().Unix()-10, 0)), // 生效开始时间
			ExpiresAt: jwt.NewNumericDate(makeExpiresTime()),                  // 失效截止时间
		},
	})
	return
}

// RemoveToken 删除token，仅在使用缓存存储token时生效
func RemoveToken(key string) (err error) {
	err = removeCache(TokenCacheKey + key)
	return
}

// 撤销/作废token，兼容缓存token和非缓存原生jwt-token
func RevokeToken(ctx *gin.Context) (err error) {
	uid := ctx.GetString("uid")
	if CacheToken {
		//使用缓存存储token，移出缓存即可作废token
		err = removeCache(TokenCacheKey + uid)
	} else {
		//记录强制撤销的token
		token := GetRequestToken(ctx)
		err = setStrCache(token, "off")
	}
	return
}

// 解析token (验证格式并验证过期)
func ParseToken(ctx *gin.Context) (*CustomClaims, error) {
	if CacheToken {
		//使用缓存存储token,支持单点登录
		token, key, err := GetJwtToken(ctx)
		if err != nil {
			return nil, err
		}
		if customClaims, err := JwtParseToken(token.JwtToken); err == nil {
			IsEffective(key, token) //检查token是否需要刷新
			return customClaims, nil
		} else {
			return &CustomClaims{}, err
		}
	} else {
		//不使用缓存token，直接解析jwt-token
		token := GetRequestToken(ctx)
		//判断token是否撤销
		isOff, _ := getStrCache(token)
		if isOff == "off" {
			return &CustomClaims{}, errors.New("token has been revoked")
		}
		if customClaims, err := JwtParseToken(token); err == nil {
			customClaims.Subject = IsEffectiveOne(token) //检查token是否需要刷新
			return customClaims, nil
		} else {
			return &CustomClaims{}, err
		}
	}
}

// 使用缓存存储token-检查缓存的token是否有效且自动刷新缓存token(请求时返回最新token)
func IsEffective(key string, cacheToken *TokenData) bool {
	_, code := IsNotExpired(cacheToken.JwtToken)
	if JwtTokenOK == code {
		// 刷新缓存
		if IsRefresh(cacheToken.JwtToken) {
			return doRefresh(key, cacheToken)
		}
		return true
	}
	return false
}

// 不用缓存-检查token是否有效且自动刷新缓存token(请求时返回最新token)
func IsEffectiveOne(token string) string {
	_, code := IsNotExpired(token)
	if JwtTokenOK == code && IsRefresh(token) { // 刷新token
		if newToken, err := TokenRefreshToken(token); err == nil {
			return newToken
		}
	}
	return ""
}

// token是否处于刷新期
func IsRefresh(token string) bool {
	if MaxRefresh == 0 {
		return false
	}
	if customClaims, err := JwtParseToken(token); err == nil {
		now := time.Now().Unix()
		if now < customClaims.ExpiresAt.Unix() && now > (customClaims.ExpiresAt.Unix()-MaxRefresh) {
			return true
		}
	}
	return false
}

// 刷新token并存到缓存中
func doRefresh(key string, cacheToken *TokenData) bool {
	if newToken, err := TokenRefreshToken(cacheToken.JwtToken); err == nil {
		cacheToken.JwtToken = newToken
		err = setCache(TokenCacheKey+key, cacheToken)
		if err != nil {
			return false
		}
	}
	return true
}

// 刷新token的缓存有效期
func TokenRefreshToken(oldToken string) (newToken string, err error) {
	if newToken, err = JWTRefreshToken(oldToken, makeExpiresTime().Unix()); err != nil {
		return
	}
	return
}

// 检查token是否过期 (过期时间 = 超时时间 + 缓存刷新时间)
func IsNotExpired(token string) (*CustomClaims, int) {
	if customClaims, err := JwtParseToken(token); err == nil {
		if time.Now().Unix()-customClaims.ExpiresAt.Unix() < 0 {
			// token有效
			return customClaims, JwtTokenOK
		} else {
			// 过期的token
			return customClaims, JwtTokenExpired
		}
	} else {
		// 无效的token
		return customClaims, JwtTokenInvalid
	}
}

// EncryptToken token加密方法
func EncryptToken(key string, randStr ...string) (encryptStr, uuid string, err error) {
	if key == "" {
		err = errors.New("encrypt key empty")
		return
	}
	// 生成随机串
	if len(randStr) > 0 {
		uuid = randStr[0]
	} else {
		uuid = gmd5.MustEncrypt(grand.String(12))
	}
	strdata := fmt.Sprintf("%v%v", key, uuid)
	token, err := cryptojs.AesEncrypt(strdata)
	if err != nil {
		err = errors.New("encrypt error")
		return
	}
	encryptStr = base64.StdEncoding.EncodeToString([]byte(token))
	return
}

// DecryptToken token解密方法
func DecryptToken(token string) (DecryptStr, uuid string, err error) {
	if token == "" {
		err = errors.New("decrypt Token empty")
		return
	}
	tokenBase64, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		err = errors.New("base64 decode error")
		return
	}
	decryptToken, err := cryptojs.AesDecrypt(string(tokenBase64))
	if err != nil {
		err = errors.New("decrypt error")
		return
	}
	length := len(decryptToken)
	uuid = string(decryptToken[length-32:])
	DecryptStr = string(decryptToken[:length-32])
	return
}
