package datacenter

import (
	"gofly/utils/gf"
	"gofly/utils/tools/gcfg"
	"gofly/utils/tools/gconv"
	"os"
	"path/filepath"
)

// 文件上传配置
type Uploadconfig struct{}

func init() {
	gf.Register(&Uploadconfig{})
}

// 获取文件上传配置数据
func (api *Uploadconfig) GetConfig(c *gf.GinCtx) {
	Type, _ := gcfg.Instance("upload").Get(c, "Type")
	MaxBodySize, _ := gcfg.Instance("upload").Get(c, "MaxBodySize")
	AllowedExt, _ := gcfg.Instance("upload").Get(c, "AllowedExt")
	local, _ := gcfg.Instance("upload").Get(c, "local")
	alioss, _ := gcfg.Instance("upload").Get(c, "alioss")
	tencentcos, _ := gcfg.Instance("upload").Get(c, "tencentcos")
	qiniuoss, _ := gcfg.Instance("upload").Get(c, "qiniuoss")
	gf.Success().SetMsg("获取文件上传配置").SetData(gf.Map{
		"Type":        Type.Val(),
		"MaxBodySize": MaxBodySize.Val(),
		"AllowedExt":  AllowedExt.Val(),
		"local":       gconv.Map(local),
		"alioss":      gconv.Map(alioss),
		"tencentcos":  gconv.Map(tencentcos),
		"qiniuoss":    gconv.Map(qiniuoss),
	}).Regin(c)
}

// 保存文件上传配置数据
func (api *Uploadconfig) SaveConfig(c *gf.GinCtx) {
	param, _ := gf.RequestParam(c)
	path, _ := os.Getwd()
	configPath := filepath.Join(path, "/resource/config/upload.yaml")
	//更新基础配置
	err := gf.UpConfigFild(configPath, gf.Map{
		"Type":        param["Type"],
		"MaxBodySize": param["MaxBodySize"],
		"AllowedExt":  gf.String(param["AllowedExt"]),
	}, "")
	if err != nil {
		gf.Failed().SetMsg("更新上传基础配置失败！").SetData(err.Error()).Regin(c)
		return
	}
	switch param["Type"] {
	case "local":
		err = gf.UpConfigFild(configPath, gf.Map{
			"LBaseUrl": gf.String(param["BaseUrl"]),
			"LDirPath": gf.String(param["DirPath"]),
		}, "  ")
	case "alioss":
		err = gf.UpConfigFild(configPath, gf.Map{
			"ABaseUrl":    gf.String(param["BaseUrl"]),
			"AEndpoint":   gf.String(param["Endpoint"]),
			"AKeyId":      gf.String(param["KeyId"]),
			"ASecret":     gf.String(param["Secret"]),
			"ABucketName": gf.String(param["BucketName"]),
			"ADirPath":    gf.String(param["DirPath"]),
		}, "  ")
	case "tencentcos":
		err = gf.UpConfigFild(configPath, gf.Map{
			"TBaseUrl":    gf.String(param["BaseUrl"]),
			"TEndpoint":   gf.String(param["Endpoint"]),
			"TKeyId":      gf.String(param["KeyId"]),
			"TSecret":     gf.String(param["Secret"]),
			"TBucketName": gf.String(param["BucketName"]),
			"TRegion":     gf.String(param["Region"]),
			"TDirPath":    gf.String(param["DirPath"]),
		}, "  ")
	case "qiniuoss":
		err = gf.UpConfigFild(configPath, gf.Map{
			"QBaseUrl":        gf.String(param["BaseUrl"]),
			"QEndpoint":       gf.String(param["Endpoint"]),
			"QKeyId":          gf.String(param["KeyId"]),
			"QSecret":         gf.String(param["Secret"]),
			"QBucketName":     gf.String(param["BucketName"]),
			"QDestBucketName": gf.String(param["DestBucketName"]),
			"QDirPath":        gf.String(param["DirPath"]),
			"QUseHTTPS":       gf.String(param["UseHTTPS"]),
			"QZone":           gf.String(param["Zone"]),
		}, "  ")
	}
	if err != nil {
		gf.Failed().SetMsg("保存文件上传配置失败！").SetData(err.Error()).Regin(c)
		return
	}
	gf.RefreshUploadConf()
	gf.Success().SetMsg("保存文件上传配置成功").SetData(true).Regin(c)
}
