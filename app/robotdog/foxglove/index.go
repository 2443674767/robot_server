package foxglove

import (
	"encoding/base64"
	"encoding/hex"
	"strings"
	"time"

	internalfoxglove "gofly/app/robotdog/internal/foxglove"
	"gofly/utils/gf"
)

type Index struct{ NoNeedAuths []string }

func init() {
	gf.Register(&Index{})
}

func (api *Index) GetLatestPose(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	timeoutMs := gf.Int(param["timeout_ms"])
	data, err := internalfoxglove.FetchLatestOdometry(ctx.Request.Context(), internalfoxglove.FetchOptions{
		WSURL:   gf.String(param["ws_url"]),
		Topic:   gf.String(param["topic"]),
		Timeout: time.Duration(timeoutMs) * time.Millisecond,
	})
	if err != nil {
		gf.Failed().SetMsg("获取Foxglove位置数据失败").SetData(err.Error()).Regin(ctx)
		return
	}
	gf.Success().SetMsg("获取Foxglove位置数据").SetData(data).Regin(ctx)
}

func (api *Index) DecodeOdometry(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	payload, err := payloadBytes(param)
	if err != nil {
		gf.Failed().SetMsg("payload参数错误").SetData(err.Error()).Regin(ctx)
		return
	}
	decoded, err := internalfoxglove.DecodeOdometry(payload)
	if err != nil {
		gf.Failed().SetMsg("解码Odometry失败").SetData(err.Error()).Regin(ctx)
		return
	}
	gf.Success().SetMsg("解码Odometry成功").SetData(decoded).Regin(ctx)
}

func payloadBytes(param map[string]interface{}) ([]byte, error) {
	if v := strings.TrimSpace(gf.String(param["payload_base64"])); v != "" {
		return base64.StdEncoding.DecodeString(v)
	}
	v := strings.TrimSpace(gf.String(param["payload_hex"]))
	if v == "" {
		v = strings.TrimSpace(gf.String(param["payload"]))
	}
	v = strings.ReplaceAll(v, " ", "")
	v = strings.TrimPrefix(v, "0x")
	return hex.DecodeString(v)
}
