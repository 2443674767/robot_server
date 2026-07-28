package datacenter

import (
	"fmt"
	"gofly/utils/gf"
	"gofly/utils/tools/gcfg"
	"gofly/utils/tools/gconv"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Configuration struct{}

func init() {
	gf.Register(&Configuration{})
}

// 获取邮箱
func (api *Configuration) GetEmail(ctx *gf.GinCtx) {
	email, _ := gcfg.Instance("email").Data(ctx)
	gf.Success().SetMsg("获取邮箱").SetData(email).Regin(ctx)
}

// 保存邮箱
func (api *Configuration) SaveEmail(c *gf.GinCtx) {
	param, _ := gf.RequestParam(c)
	path, _ := os.Getwd()
	emailPath := filepath.Join(path, "/resource/config/email.yaml")
	//更新系统邮箱配置
	err := gf.UpConfigFild(emailPath, gf.Map{
		"senderEmail": gf.String(param["senderEmail"]),
		"authCode":    gf.String(param["authCode"]),
		"serviceHost": gf.String(param["serviceHost"]),
		"servicePort": gf.Int(param["servicePort"]),
		"mailTitle":   gf.String(param["mailTitle"]),
		"mailBody":    gf.String(param["mailBody"]),
	}, "")
	if err != nil {
		gf.Failed().SetMsg("更新系统邮箱配置失败！").SetData(err.Error()).Regin(c)
		return
	}
	gf.RefreshEmailConf()
	gf.Success().SetMsg("保存邮箱成功！").SetData(param).Regin(c)
}

// 获取安装的代码仓配置
func (api *Configuration) GetCodestoreConfig(c *gf.GinCtx) {
	path, _ := os.Getwd()
	configPath := filepath.Join(path, "/resource/config/code")
	go_app_dir, _ := gf.GetAllFileArray(configPath)
	var list []interface{} = make([]interface{}, 0)
	for _, val := range gf.Strings(go_app_dir["files"]) {
		configstr, err := gf.ReaderFileByline(filepath.Join(path, "/resource/config/code", val))
		if err != nil {
			gf.Failed().SetMsg("读取插件配置失败：" + err.Error()).Regin(c)
			return
		}
		fileName := strings.Split(val, ".")
		install_cofig, _ := gf.GetYamlConfigData(filepath.Join(path, "/resource/config/code"), fileName[0])
		data_item := install_cofig.(map[string]interface{})
		if data_item["conftype"] == "configuration" {
			var new_data []map[string]interface{}
			for k, v := range data_item["data"].(map[string]interface{}) {
				keyname := k
				for _, cfstr := range configstr {
					cf_str := gf.String(cfstr)
					cf_str_arr := strings.Split(cf_str, "#")
					if strings.Contains(cf_str, k) && len(cf_str_arr) == 2 {
						keyname = cf_str_arr[1]
					}
				}
				new_data = append(new_data, gf.Map{"keyname": keyname, "keyfield": k, "keyvalue": v})
			}
			data_item["name"] = fileName[0]
			data_item["data"] = new_data
			list = append(list, install_cofig)
		}
	}
	gf.Success().SetMsg("获取安装的代码仓配置列表").SetData(list).Regin(c)
}

// 修改安装的代码仓配置
func (api *Configuration) SaveCodeStoreConfig(c *gf.GinCtx) {
	param, _ := gf.RequestParam(c)
	path, _ := os.Getwd()
	configPath := filepath.Join(path, "/resource/config/code", gf.String(param["name"])+".yaml")
	var upAppconf gf.Map = make(map[string]interface{}, 0)
	for _, val := range param["data"].([]interface{}) {
		item := val.(map[string]interface{})
		var val_data interface{}
		switch item["keyvalue"].(type) {
		case string: // 处理 string 类型
			val_str := gf.String(item["keyvalue"])
			if val_str == "" {
				val_str = "\"\""
			} else {
				val_str = strconv.Quote(val_str)
			}
			val_data = val_str
		default:
			val_data = gf.String(item["keyvalue"])
		}
		upAppconf[gf.String(item["keyfield"])] = fmt.Sprintf("%s  #%v", val_data, item["keyname"])
	}
	gf.UpConfigFild(configPath, upAppconf, "  ")
	gf.Success().SetMsg("修改安装的代码仓配置").SetData(param).SetExdata(configPath).Regin(c)
}

// 更新配置使用状态
func (api *Configuration) UpConfigStatus(c *gf.GinCtx) {
	param, _ := gf.RequestParam(c)
	path, _ := os.Getwd()
	//如果改为true，则把其他通用标识改为false
	if gf.Bool(param["status"]) {
		path, _ := os.Getwd()
		configPath := filepath.Join(path, "/resource/config")
		go_app_dir, _ := gf.GetAllFileArray(configPath)
		for _, val := range gf.Strings(go_app_dir["files"]) {
			fileName := strings.Split(val, ".")
			install_cofig, _ := gf.GetYamlConfigData(filepath.Join(path, "/resource/config"), fileName[0])
			data_item := install_cofig.(map[string]interface{})
			if fileName[0] != gf.String(param["name"]) && gf.String(data_item["pluginident"]) == gf.String(param["pluginident"]) && gf.Bool(data_item["status"]) {
				configPath := filepath.Join(path, "/resource/config", val)
				gf.UpConfigFild(configPath, gf.Map{"status": false}, "")
			}
		}
	}
	configPath := filepath.Join(path, "/resource/config", gf.String(param["name"])+".yaml")
	gf.UpConfigFild(configPath, gf.Map{"status": param["status"]}, "")
	gf.Success().SetMsg("更新配置使用状态成功").SetData(param).SetExdata(configPath).Regin(c)
}

//=============应用配置=========================

// 获取config.yaml应用配置数据
func (api *Configuration) GetAppConfig(ctx *gf.GinCtx) {
	appConf, _ := gcfg.Instance("app").Get(ctx, "app")
	mapdata := gconv.Map(appConf)
	gf.Success().SetMsg("获取应用配置").SetData(gf.Map{
		"vueobjroot":   mapdata["vueobjroot"],
		"loginCaptcha": mapdata["loginCaptcha"],
		"cacheToken":   mapdata["cacheToken"],
		"multiLogin":   mapdata["multiLogin"],
		"validityApi":  mapdata["validityApi"],
	}).Regin(ctx)
}

// 保存系统配置
func (api *Configuration) SaveAppConfig(c *gf.GinCtx) {
	param, _ := gf.RequestParam(c)
	path, _ := os.Getwd()
	upAppconf := gf.Map{
		"vueobjroot":   strings.Replace(param["vueobjroot"].(string), "\\", "/", -1),
		"loginCaptcha": param["loginCaptcha"],
		"cacheToken":   param["cacheToken"],
		"multiLogin":   param["multiLogin"],
		"validityApi":  param["validityApi"],
	}
	cferr := gf.UpAppConfData(path, upAppconf, "  ")
	if cferr != nil {
		gf.Failed().SetMsg("修改前端路径失败").SetData(cferr.Error()).Regin(c)
		return
	}
	gf.RefreshAppConf() //刷新app配置
	gf.Success().SetMsg("保存系统配置成功").Regin(c)
}
