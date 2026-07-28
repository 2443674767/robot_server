package gf

import (
	"encoding/json"
	"errors"
	"fmt"
	"gofly/utils/tools/gconv"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultMultipartMemory = 32 << 20 // 32 MB
// 组装Restful API 接口返回数据
// 返回信息主体
type (
	R struct {
		Code    int
		Message string
		Data    interface{}
		Exdata  interface{}
		Token   string
		Time    int64
	}
)

// 错误码
var (
	succCode = 0 // 成功
	errCode  = 1 // 失败
)

// 设置返回内容
func (r *R) SetData(data interface{}) *R {
	r.Data = data
	return r
}

// 设置返回扩展内容内容
func (r *R) SetExdata(exdata interface{}) *R {
	r.Exdata = exdata
	return r
}

// 设置编码
func (r *R) SetCode(code int) *R {
	r.Code = code
	return r
}

// 设置返回提示信息
func (r *R) SetMsg(msg string) *R {
	r.Message = msg
	return r
}

// 设置返回Token
func (r *R) SetToken(token string) *R {
	r.Token = token
	return r
}

// 返回成功内容
func Success() *R {
	r := &R{}
	r.Message = "Success"
	r.Code = succCode
	r.Time = time.Now().Unix()
	return r
}

// 返回接口内容(Failed和Success输出内容必须在最使用Regin)
func (r *R) Regin(ctx *GinCtx) {
	if r.Token == "" {
		//在cacheToken=false不使用缓存（仅使用原始jwt-token）在token失效前返回新的token
		subToken, exists := ctx.Get("sub_token") //当前用户
		if exists {
			r.Token = gconv.String(subToken)
		}
	}
	ctx.JSON(http.StatusOK, GinObj{
		"code":    r.Code,
		"message": r.Message,
		"data":    r.Data,
		"exdata":  r.Exdata,
		"token":   r.Token,
		"time":    r.Time,
	})
}

// 返回失败内容
func Failed() *R {
	r := &R{}
	r.Message = "Fail"
	r.Code = errCode
	r.Data = false
	r.Time = time.Now().Unix()
	return r
}

// 请求相关
// 批量获取请求参数-通用
func RequestParam(ctx *GinCtx) (map[string]interface{}, error) {
	ctx.Request.ParseForm()
	dataMap := make(map[string]interface{})
	// if ctx.Request.Method == "POST" || ctx.Request.Method == "PUT" || ctx.Request.Method == "DELETE" {
	if strings.Contains(ctx.Request.Header.Get("Content-Type"), "application/json") {
		body, _ := io.ReadAll(ctx.Request.Body)
		var parameter map[string]interface{}
		err := json.Unmarshal(body, &parameter)
		if err != nil {
			return nil, err
		}
		dataMap = parameter
	} else {
		//说明:须post方法,加: 'Content-Type': 'application/x-www-form-urlencoded'
		for key, valueArray := range ctx.Request.PostForm {
			if len(valueArray) > 1 {
				return nil, fmt.Errorf("#ERROR#[%s]参数设置了[%d]次,只能设置一次.", key, len(valueArray))
			}
			dataMap[key] = ctx.PostForm(key)
		}
	}
	//对form-data参数处理
	req := ctx.Request
	if err := req.ParseMultipartForm(defaultMultipartMemory); err != nil {
		if !errors.Is(err, http.ErrNotMultipart) {
			return nil, fmt.Errorf("error on parse multipart form array: %v", err)
		}
	}
	for key, val := range req.PostForm {
		if len(val) > 0 {
			dataMap[key] = val[0]
		}
	}
	// }
	for key, _ := range ctx.Request.URL.Query() {
		dataMap[key] = ctx.Query(key)
	}
	return dataMap, nil
}

// 获取post传过来的data
func PostParam(c *GinCtx) (map[string]interface{}, error) {
	body, _ := io.ReadAll(c.Request.Body)
	var parameter map[string]interface{}
	err := json.Unmarshal(body, &parameter)
	if err != nil {
		return nil, err
	}
	return parameter, nil
}

/***Token处理****/
