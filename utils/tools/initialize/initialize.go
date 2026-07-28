// ==============================
// 初始化中文翻译器 + 注册label标签
// ==============================
package initialize

import (
	"reflect"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	zhTrans "github.com/go-playground/validator/v10/translations/zh"
)

// 全局翻译器（初始化一次即可）
var (
	uni      *ut.UniversalTranslator
	validate *validator.Validate
	trans    ut.Translator
)

func init() {
	InitTranslator()
}

// 初始化中文翻译器
func InitTranslator() {
	// 1. 初始化中文语言包
	zhLocale := zh.New()
	uni = ut.New(zhLocale, zhLocale)
	trans, _ = uni.GetTranslator("zh")

	// 2. 获取gin的校验器
	validate := binding.Validator.Engine().(*validator.Validate)

	//  注册label标签，优先使用label作为字段名（彻底解决.和label无效问题）
	validate.RegisterTagNameFunc(func(field reflect.StructField) string {
		label := field.Tag.Get("label")
		if label == "" {
			return field.Name
		}
		return label
	})

	// 3. 注册中文翻译
	_ = zhTrans.RegisterDefaultTranslations(validate, trans)
}

// TranslateErr 翻译校验错误为中文
func TranslateErr(err error) string {
	// 类型断言为校验错误
	errs, ok := err.(validator.ValidationErrors)
	if !ok {
		return "参数校验失败"
	}

	// 拼接所有错误信息
	var errMsg string
	for _, e := range errs {
		errMsg += e.Translate(trans) + "；"
	}
	return errMsg
}
