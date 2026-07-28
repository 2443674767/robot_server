package gf

import (
	"image/color"
	"time"

	"github.com/mojocn/base64Captcha"
)

type CaptchaResult struct {
	Id          string `json:"id"`
	Show        bool   `json:"show"`
	Base64Blog  string `json:"img"`
	VerifyValue string `json:"code"`
	ExpireTime  int64  `json:"expireTime"`
}

// 默认存储10240个验证码，每个验证码3分钟过期
// Expiration time of captchas used by default store.
var Expiration = 3 * time.Minute
var store = base64Captcha.NewMemoryStore(10240, Expiration)

// 生成图片验证码
// 生成base64图片
// 参数：pts 不传参数默认生成默认数字，如果是字母验证码则传char
func GenerateCaptcha(pts ...string) (interface{}, error) {
	var captcha *base64Captcha.Captcha
	if len(pts) > 0 && pts[0] == "char" {
		driver := base64Captcha.NewDriverString(
			80,  // 高
			240, // 宽
			2,   // 噪点
			2|3, // 干扰线：黏液+正弦
			4,   // 验证码中字符的数量
			"123456789abcdefghjkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ", // 用于定义验证码中使用的字符集。source变量需要在代码中定义，并包含所有可能用于生成验证码的字符。
			nil,                          // bgColor *color.RGBA，nil值表示不设置自定义背景颜色，将使用默认的背景颜色
			nil,                          // fontsStorage FontsStorage，nil值表示不使用自定义字体存储（FontsStorage），将使用库的默认字体
			[]string{"wqy-microhei.ttc"}, // 字体
		)
		captcha = base64Captcha.NewCaptcha(driver, store)
	} else {
		driver := base64Captcha.NewDriverMath(39, 110, 0, 0, &color.RGBA{0, 0, 0, 1}, nil, []string{"wqy-microhei.ttc"})
		captcha = base64Captcha.NewCaptcha(driver, store)
	}
	id, b64s, _, err := captcha.Generate()
	if err != nil {
		return "", err
	}
	captchaResult := CaptchaResult{Id: id, Show: Bool(appConf_arr["loginCaptcha"]), Base64Blog: b64s, ExpireTime: time.Now().Add(Expiration).UnixMilli()}
	return captchaResult, nil
}

// 校验图片验证码,并清除内存空间
func VerifyCaptcha(id string, value string) bool {
	// TODO 只要id存在，就会校验并清除，无论校验的值是否成功, 所以同一id只能校验一次
	// 注意：id,b64s是空 也会返回true 需要在加判断
	verifyResult := store.Verify(id, value, true)
	return verifyResult
}
