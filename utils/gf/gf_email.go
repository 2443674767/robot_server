package gf

import (
	"context"
	"errors"
	"gofly/utils/tools/gcfg"
	"gofly/utils/tools/gconv"
	"gofly/utils/tools/grand"
	"strings"

	"gopkg.in/gomail.v2"
)

// 清除邮箱配置缓存-强制刷新配置
func RefreshEmailConf() {
	gcfg.Instance("email").ClearCache()
}

// 发送邮件
// 请求参数：email邮箱地址，title邮件标题(如果为空则默认则从配置获取)，text邮件内容(如果为空则默认则从配置获取)
// 返回参数：bool 结果, error 错误提示
func SendEmail(c *GinCtx, email []string, title, text string) (bool, error) {
	if len(email) == 0 {
		return false, errors.New("请填写邮箱")
	} else {
		emailConf, err := gcfg.Instance("email").Data(context.Background())
		if err != nil {
			return false, errors.New("获取邮箱配置视失败")
		}
		emailConf_arr := gconv.Map(emailConf)
		if emailConf_arr["senderEmail"] == "" {
			return false, errors.New("请到业务端后台“配置管理”配置邮箱")
		} else {
			sender := gconv.String(emailConf_arr["senderEmail"])  //发送者邮箱
			authCode := gconv.String(emailConf_arr["authCode"])   //邮箱授权码
			mailTitle := gconv.String(emailConf_arr["mailTitle"]) //邮件标题
			mailBody := gconv.String(emailConf_arr["mailBody"])   //邮件内容,可以是html
			if title != "" {
				mailTitle = title //邮件标题
			}
			if text == "" {
				code := grand.Digits(6)
				mailBody = strings.Replace(mailBody, "{code}", code, 1)
				for _, val := range email {
					SetVerifyCode(val, code) //验证码存在本地缓存
				}
			} else {
				mailBody = text
			}
			m := gomail.NewMessage()
			m.SetHeader("From", sender)       //发送者邮箱账号
			m.SetHeader("To", email...)       //接收者邮箱列表
			m.SetHeader("Subject", mailTitle) //邮件标题
			m.SetBody("text/html", mailBody)  //邮件内容,可以是html
			//服务器地址和端口是默认腾讯的
			service_host := "smtp.qq.com"
			if emailConf_arr["serviceHost"] != "" {
				service_host = gconv.String(emailConf_arr["serviceHost"])
			}
			service_port := 587
			if emailConf_arr["servicePort"] != "" {
				service_port = gconv.Int(emailConf_arr["servicePort"])
			}
			d := gomail.NewDialer(service_host, service_port, sender, authCode)
			err := d.DialAndSend(m)
			if err != nil {
				return false, err
			} else {
				return true, nil
			}
		}
	}
}
