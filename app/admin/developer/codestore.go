package developer

import (
	"context"
	"encoding/json"
	"gofly/dao"
	"gofly/utils/gf"
	"gofly/utils/tools/gcfg"
	"gofly/utils/tools/gcompress"
	"gofly/utils/tools/gconv"
	"gofly/utils/tools/gfile"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// 用于自动注册路由
type Codestore struct{ NoNeedAuths []string }

func init() {
	gf.Register(&Codestore{NoNeedAuths: []string{"*"}})
}

// 获取代码商城分类
func (api *Codestore) GetCodeCate(ctx *gf.GinCtx) {
	appConf, _ := gcfg.Instance("app").Get(ctx, "app")
	appConf_arr := gconv.Map(appConf)
	baseurl := ctx.DefaultQuery("baseurl", "")
	if baseurl == "" {
		gf.Success().SetMsg("代码仓地址不存在！").SetData(make([]interface{}, 0)).Regin(ctx)
	} else {
		ref := gf.Get(baseurl + "/goflycode/cate/getCate")
		var parameter gf.Map
		if err := json.Unmarshal([]byte(ref), &parameter); err == nil {
			if gconv.Int(parameter["code"]) == 0 {
				path, _ := os.Getwd()
				downdir := filepath.Join(path, "/devsource/codemarket/release")
				gf.Success().SetMsg("获取代码商城分类").SetData(gf.Map{
					"catedata":     parameter["data"],
					"privateHouse": appConf_arr["privateHouse"],
					"codepack":     downdir,
					"version":      appConf_arr["version"],
				}).Regin(ctx)
			} else {
				gf.Failed().SetMsg("请求GoFLy社区获取代码商店分类失败").SetData(parameter).Regin(ctx)
			}
		}
	}
}

// 获取代码商城
func (api *Codestore) CodeList(ctx *gf.GinCtx) {
	baseurl := ctx.DefaultQuery("baseurl", "")
	param, _ := gf.PostParam(ctx)
	if baseurl == "" {
		gf.Success().SetMsg("代码仓地址不存在！").SetData(make(gf.Slice, 0)).Regin(ctx)
	} else {
		param["frame"] = 3
		ref, resErro := gf.HttpGet(baseurl+"/goflycode/content/getCode", param)
		if resErro != nil {
			gf.Failed().SetMsg("请求GoFLy社区获取代码商店失败").SetData(resErro).Regin(ctx)
		} else {
			if gconv.Int64(ref["code"]) == 0 {
				data := ref["data"].(map[string]interface{})
				list := data["items"].([]interface{})
				path, _ := os.Getwd()
				for _, val := range list {
					item := val.(map[string]interface{})
					installconfig := filepath.Join(path, "/resource/codeinstall", gconv.String(item["name"]))
					if _, err := os.Stat(installconfig); os.IsNotExist(err) { //不存在
						item["is_install"] = false
					} else {
						codeConf, errs := gcfg.Instance("config", "resource/codeinstall/"+gconv.String(item["name"])).Get(ctx, "app.isinstall")
						if errs == nil {
							item["is_install"] = codeConf.Bool()
						}
					}
				}
				gf.Success().SetMsg("获取代码商城成功！").SetData(data).Regin(ctx)
			} else {
				gf.Failed().SetMsg("请求代码商店数据失败").SetData(ref).Regin(ctx)
			}
		}

	}
}

// 查找本地已经安装的包
func (api *Codestore) GetInstallPack(c *gf.GinCtx) {
	path, _ := os.Getwd()
	pathname := filepath.Join(path, "/resource/codeinstall")
	rd, err := os.ReadDir(pathname)
	if err != nil {
		gf.Success().SetMsg("本地没有安装的包").SetData("").Regin(c)
		return
	}
	var folders = make([]string, 0)
	for _, fi := range rd {
		if fi.IsDir() {
			codeConf, errs := gcfg.Instance("config", "resource/codeinstall/"+fi.Name()).Get(context.Background(), "app.isinstall")
			if errs == nil && codeConf.Bool() {
				folders = append(folders, fi.Name())
			}
		}
	}
	gf.Success().SetMsg("本地已经安装的包").SetData(strings.Join(folders, ",")).Regin(c)
}

// 更新私有代码仓地址
func (api *Codestore) UpPrivateHouse(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	path, err := os.Getwd() //获取当前路径
	if err != nil {
		gf.Failed().SetMsg("项目路径获取失败").Regin(ctx)
		return
	}
	privateHouse := ""
	if param["privateHouse"] != nil {
		privateHouse = gconv.String(param["privateHouse"])
	}
	//修改应用配置
	upAppconf := gf.Map{
		"privateHouse": privateHouse,
	}
	cferr := gf.UpAppConfData(path, upAppconf, "  ")
	if cferr != nil {
		gf.Failed().SetMsg("修改数据库配置失败").Regin(ctx)
	} else {
		gf.RefreshAppConf()
		gf.Success().SetMsg("更新代码仓成功").Regin(ctx)
	}
}

/***账号登录***/
// 1登录
func (api *Codestore) Login(ctx *gf.GinCtx) {
	baseurl := ctx.DefaultQuery("baseurl", "")
	if baseurl == "" {
		gf.Failed().SetMsg("代码仓地址不存在！").Regin(ctx)
	} else {
		param, _ := gf.RequestParam(ctx)
		ref := gf.Post(baseurl+"/goflycode/user/login", param, "application/json")
		var parameter map[string]interface{}
		if err := json.Unmarshal([]byte(ref), &parameter); err == nil {
			if gconv.Int(parameter["code"]) == 0 {
				gf.Success().SetMsg("登录").SetData(parameter["data"]).Regin(ctx)
			} else {
				gf.Failed().SetMsg(gconv.String(parameter["message"])).Regin(ctx)
			}
		}
	}
}

// 2.免密登录
func (api *Codestore) FreeLogin(ctx *gf.GinCtx) {
	baseurl := ctx.DefaultQuery("baseurl", "")
	if baseurl == "" {
		gf.Failed().SetMsg("代码仓地址不存在！").Regin(ctx)
	} else {
		param, _ := gf.RequestParam(ctx)
		ref := gf.Post(baseurl+"/goflycode/user/freeLogin", param, "application/json")
		var parameter map[string]interface{}
		if err := json.Unmarshal([]byte(ref), &parameter); err == nil {
			if gconv.Int(parameter["code"]) == 0 {
				gf.Success().SetMsg("登录").SetData(parameter["data"]).Regin(ctx)
			} else {
				gf.Failed().SetMsg("登录失败").Regin(ctx)
			}
		}
	}
}

// 3.注册账号
func (api *Codestore) RegisterUser(ctx *gf.GinCtx) {
	baseurl := ctx.DefaultQuery("baseurl", "")
	if baseurl == "" {
		gf.Failed().SetMsg("代码仓地址不存在！").Regin(ctx)
	} else {
		ref := gf.Get(baseurl + "/goflycode/user/registerUser")
		var parameter map[string]interface{}
		if err := json.Unmarshal([]byte(ref), &parameter); err == nil {
			if gconv.Int(parameter["code"]) == 0 {
				gf.Success().SetMsg("注册").SetData(parameter["data"]).Regin(ctx)
			} else {
				gf.Failed().SetMsg("注册失败").Regin(ctx)
			}
		}
	}
}

// 4.获取验证码
func (api *Codestore) LoginCode(ctx *gf.GinCtx) {
	baseurl := ctx.DefaultQuery("baseurl", "")
	if baseurl == "" {
		gf.Failed().SetMsg("代码仓地址不存在！").Regin(ctx)
	} else {
		param, _ := gf.RequestParam(ctx)
		ref := gf.Post(baseurl+"/goflycode/user/loginCode", param, "application/json")
		var parameter map[string]interface{}
		if err := json.Unmarshal([]byte(ref), &parameter); err == nil {
			if gconv.Int(parameter["code"]) == 0 {
				gf.Success().SetMsg("获取验证码").SetData(parameter["data"]).Regin(ctx)
			} else {
				gf.Failed().SetMsg("获取验证码失败").Regin(ctx)
			}
		}
	}
}

// 5.把代码推到服务器
func (api *Codestore) UpPackToService(ctx *gf.GinCtx) {
	baseurl := ctx.DefaultQuery("baseurl", "")
	if baseurl == "" {
		gf.Failed().SetMsg("代码仓地址不存在！").Regin(ctx)
	} else {
		param, _ := gf.RequestParam(ctx)
		pushdata := gf.Map{
			"frame":         "[3]",
			"cid":           param["cid"],
			"code_token":    param["code_token"],
			"title":         param["title"],
			"name":          param["name"],
			"des":           param["des"],
			"price":         param["price"],
			"version":       param["version"],
			"goflygen_file": param["goflygen_file"],
		}
		ref := gf.Post(baseurl+"/goflycode/content/save", pushdata, "application/json")
		var parameter map[string]interface{}
		if err := json.Unmarshal([]byte(ref), &parameter); err == nil {
			if gconv.Int(parameter["code"]) == 0 {
				gf.Success().SetMsg("代码推到服务器成功").SetData(parameter["data"]).Regin(ctx)
			} else {
				gf.Failed().SetMsg("代码推到服务器失败").Regin(ctx)
			}
		}
	}
}

// 6.发布需求
func (api *Codestore) Requirement(ctx *gf.GinCtx) {
	baseurl := ctx.DefaultQuery("baseurl", "")
	if baseurl == "" {
		gf.Failed().SetMsg("代码仓地址不存在！").Regin(ctx)
	} else {
		param, _ := gf.RequestParam(ctx)
		pushdata := gf.Map{
			"frame":      "[3]",
			"code_token": param["code_token"],
			"title":      param["title"],
			"name":       param["name"],
			"des":        param["des"],
			"price":      param["price"],
			"customer":   param["customer"],
			"mobile":     param["mobile"],
			"wx":         param["wx"],
			"type":       1,
			"cid":        11,
			"status":     2,
			"content":    param["content"],
		}
		ref := gf.Post(baseurl+"/goflycode/content/save", pushdata, "application/json")
		var parameter map[string]interface{}
		if err := json.Unmarshal([]byte(ref), &parameter); err == nil {
			if gconv.Int(parameter["code"]) == 0 {
				gf.Success().SetMsg("发布需求到服务器成功").SetData(parameter["data"]).Regin(ctx)
			} else {
				gf.Failed().SetMsg("发布需求到服务器失败").Regin(ctx)
			}
		}
	}
}

// 7.检查更新代码版本
func (api *Codestore) AsyncVersion(ctx *gf.GinCtx) {
	baseurl := ctx.DefaultQuery("baseurl", "")
	if baseurl == "" {
		gf.Failed().SetMsg("代码仓地址不存在！").Regin(ctx)
	} else {
		appConf, _ := gcfg.Instance("app").Get(ctx, "app")
		appConf_arr := gconv.Map(appConf)
		param, _ := gf.RequestParam(ctx)
		ref := gf.Post(baseurl+"/goflycode/version/asyncVersion", gf.Map{"code_token": param["code_token"], "version": appConf_arr["version"], "from": "check", "frame": 3}, "application/json")
		var parameter map[string]interface{}
		if err := json.Unmarshal([]byte(ref), &parameter); err == nil {
			if gconv.Int(parameter["code"]) == 0 {
				gf.Success().SetMsg("检查更新代码版本成功").SetData(parameter["data"]).Regin(ctx)
			} else {
				gf.Failed().SetMsg(gconv.String(parameter["message"])).SetData(parameter).Regin(ctx)
			}
		} else {
			gf.Failed().SetMsg("请求gofly社区服务失败！").SetData(err).Regin(ctx)
		}
	}
}

// 8.更新基座代码
func (api *Codestore) UpBaseCode(ctx *gf.GinCtx) {
	appConf, _ := gcfg.Instance("app").Get(ctx, "app")
	appConf_arr := gconv.Map(appConf)
	if appConf_arr["runEnv"] == "release" {
		gf.Failed().SetMsg("生产环境禁止操作，请在开发环境下操作").Regin(ctx)
		return
	}
	baseurl := ctx.DefaultQuery("baseurl", "")
	if baseurl == "" {
		gf.Failed().SetMsg("代码仓地址不存在！").Regin(ctx)
	} else {
		param, _ := gf.RequestParam(ctx)
		ref := gf.Post(baseurl+"/goflycode/version/asyncVersion", gf.Map{"code_token": param["code_token"], "version": appConf_arr["version"], "from": "up", "frame": 3}, "application/json")
		var parameter map[string]interface{}
		if err := json.Unmarshal([]byte(ref), &parameter); err == nil {
			if gconv.Int(parameter["code"]) == 0 {
				codedata := parameter["data"].(map[string]interface{})
				filename := gconv.String(codedata["filename"])
				updirpath := "devsource/developer/upnewversion/"
				path, _ := os.Getwd()
				upnewversion := filepath.Join(path, updirpath)
				defer os.RemoveAll(upnewversion) //删除更新源代码包文件
				if _, err := os.Stat(upnewversion); os.IsNotExist(err) {
					os.MkdirAll(upnewversion, os.ModePerm) //创建更新文件容器
				}
				downdir := filepath.Join(path, updirpath, filename+".zip")
				downstatus, downdir_zippath := gf.DownFileToDir(gconv.String(codedata["dowurl"]), downdir)
				if downstatus {
					//解压
					dezipdir := filepath.Join(path, updirpath, filename)
					err := gcompress.UnZipFile(downdir_zippath, dezipdir, filename)
					if err == nil {
						os.Remove(downdir_zippath)
						//1 获取插件配置
						installConfig, err := gcfg.Instance("config", updirpath+filename).Get(ctx, "app")
						if err != nil {
							gf.Failed().SetMsg("插件配置文件解析失败").SetData(err).Regin(ctx)
							return
						}
						installConf_arr := gconv.Map(installConfig)
						//2处理Go后端文件
						if !gf.IsEmpty(installConf_arr["goFiles"]) && gf.String(installConf_arr["goFiles"]) != "[]" {
							err = gfile.Copy(filepath.Join(path, updirpath+filename+"/go"), filepath.Join(path))
							if err != nil {
								gf.Failed().SetMsg("更新后端Go代码失败").SetData(err).Regin(ctx)
								return
							}
							//2.3判断是否mod拉取依赖(使用框架为安装包)
							if gf.Bool(installConf_arr["isModTidy"]) {
								exec.Command("go", "mod", "tidy").Run()
							}
						}

						//3.更新前端vue
						//3.1安装前端依赖包
						commandArr := strings.Split(gf.String(installConf_arr["commandLines"]), ",")
						vueobjroot := gf.String(appConf_arr["vueobjroot"])
						isNpm := gfile.Exists(filepath.Join(vueobjroot, "package-lock.json"))
						isYarn := gfile.Exists(filepath.Join(vueobjroot, "yarn.lock"))
						for _, commstr := range commandArr {
							if isYarn && !isNpm {
								cmd := exec.Command("yarn", "add", commstr)
								cmd.Dir = vueobjroot
								cmd.Run()
							} else {
								cmd := exec.Command("npm", "install", commstr)
								cmd.Dir = vueobjroot
								cmd.Run()
							}
						}
						//3.2更新前端代码
						if !gf.IsEmpty(installConf_arr["vueFiles"]) && gf.String(installConf_arr["vueFiles"]) != "[]" {
							err = gfile.Copy(filepath.Join(path, updirpath+filename+"/vue"), filepath.Join(vueobjroot))
							if err != nil {
								gf.Failed().SetMsg("更新前端vue失败").SetData(err).Regin(ctx)
								return
							}
						}

						//更新版本号
						upAppconf := gf.Map{
							"version": codedata["version"],
						}
						gf.UpAppConfData(path, upAppconf, "  ")
						gf.Success().SetMsg("更新成功").SetData(codedata["version"]).SetExdata(err).Regin(ctx)
					} else {
						gf.Failed().SetMsg("解压代码包失败").SetData(err).Regin(ctx)
					}
				} else {
					gf.Failed().SetMsg("下载代码失败").SetData(parameter).Regin(ctx)
				}
			} else {
				gf.Failed().SetMsg("检查更新代码版本失败").SetData(parameter).Regin(ctx)
			}
		}
	}
}

// 检测包名是否可用
func (api *Codestore) CheckPackName(ctx *gf.GinCtx) {
	baseurl := ctx.DefaultQuery("baseurl", "")
	if baseurl == "" {
		gf.Success().SetMsg("代码仓地址不存在！").SetData(make([]interface{}, 0)).Regin(ctx)
	} else {
		param, _ := gf.PostParam(ctx)
		ref := gf.Post(baseurl+"/goflycode/ident/checkPackName", gf.Map{"type": param["type"], "name": param["name"]}, "application/json")
		var parameter gf.Map
		if err := json.Unmarshal([]byte(ref), &parameter); err == nil {
			if gconv.Int(parameter["code"]) == 0 {
				gf.Success().SetMsg("检测包名结果").SetData(parameter["data"]).Regin(ctx)
			} else {
				gf.Failed().SetMsg("请求GoFLy社区获取检查数据失败").Regin(ctx)
			}
		}
	}
}

// 提交标识占用
func (api *Codestore) SavePackName(ctx *gf.GinCtx) {
	baseurl := ctx.DefaultQuery("baseurl", "")
	if baseurl == "" {
		gf.Success().SetMsg("代码仓地址不存在！").SetData(make([]interface{}, 0)).Regin(ctx)
	} else {
		param, _ := gf.PostParam(ctx)
		ref := gf.Post(baseurl+"/goflycode/ident/savePackName", gf.Map{"type": param["type"], "name": param["name"]}, "application/json")
		var parameter gf.Map
		if err := json.Unmarshal([]byte(ref), &parameter); err == nil {
			if gconv.Int(parameter["code"]) == 0 {
				gf.Success().SetMsg("提交标识占用").SetData(parameter["data"]).Regin(ctx)
			} else {
				gf.Failed().SetMsg("请求GoFLy社区提交标识占用失败").SetData(parameter).Regin(ctx)
			}
		}
	}
}

// 检测插件是否已经支付
func (api *Codestore) CheckIsPay(ctx *gf.GinCtx) {
	baseurl := ctx.DefaultQuery("baseurl", "")
	if baseurl == "" {
		gf.Success().SetMsg("代码仓地址不存在！").SetData(make([]interface{}, 0)).Regin(ctx)
	} else {
		param, _ := gf.PostParam(ctx)
		ref := gf.Post(baseurl+"/goflycode/order/checkIsPay", param, "application/json")
		var parameter gf.Map
		if err := json.Unmarshal([]byte(ref), &parameter); err == nil {
			if gconv.Int(parameter["code"]) == 0 {
				gf.Success().SetMsg("检测插件是否已经支付成功").SetData(parameter["data"]).SetExdata(parameter).Regin(ctx)
			} else {
				gf.Failed().SetMsg("检测插件是否已经支付失败").Regin(ctx)
			}
		}
	}
}

// 提交插件订单
func (api *Codestore) AddOrder(ctx *gf.GinCtx) {
	baseurl := ctx.DefaultQuery("baseurl", "")
	if baseurl == "" {
		gf.Success().SetMsg("gofly请求地址不存在！").SetData(make([]interface{}, 0)).Regin(ctx)
	} else {
		param, _ := gf.PostParam(ctx)
		ref := gf.Post(baseurl+"/goflycode/order/addOrder", param, "application/json")
		var parameter gf.Map
		if err := json.Unmarshal([]byte(ref), &parameter); err == nil {
			if gconv.Int(parameter["code"]) == 0 {
				gf.Success().SetMsg("提交插件订单成功").SetData(parameter["data"]).Regin(ctx)
			} else {
				gf.Failed().SetMsg("提交插件订单失败").SetData(parameter).Regin(ctx)
			}
		}
	}
}

// 获取菜单列表
func (api *Codestore) GetMenutree(ctx *gf.GinCtx) {
	ruleDB := dao.Query().AdminAuthRule
	var admin_menuList gf.List
	err := ruleDB.WithContext(ctx).Where(ruleDB.Status.Eq(0)).Select(ruleDB.ID, ruleDB.Pid, ruleDB.Title, ruleDB.Locale).
		Order(ruleDB.Weigh.Asc()).Scan(&admin_menuList)
	if err != nil {
		gf.Failed().SetMsg("获取菜单数据失败").SetData(err).Regin(ctx)
		return
	}
	if admin_menuList == nil {
		admin_menuList = make(gf.List, 0)
	} else {
		for _, val := range admin_menuList {
			if gf.IsEmpty(val["title"]) {
				val["title"] = val["locale"]
			}
		}
		admin_menuList = gf.GetTreeArray(admin_menuList, 0, "")
	}
	gf.Success().SetMsg("获取数据数据列表").SetData(admin_menuList).Regin(ctx)
}

// 获取菜单选择ID转json
func (api *Codestore) MenuTreeToJson(ctx *gf.GinCtx) {
	params, _ := gf.RequestParam(ctx)
	rules := gf.GetRulesID("admin_auth_rule", "pid", params["menu"]) //获取子菜单包含的父级ID
	ruleDB := dao.Query().AdminAuthRule
	var menuList gf.List
	err := ruleDB.WithContext(ctx).Where(ruleDB.ID.In(gf.InterfaceToInt32(rules)...)).Select(
		ruleDB.ID, ruleDB.Pid, ruleDB.Title, ruleDB.Locale, ruleDB.Type, ruleDB.Icon, ruleDB.Routepath,
		ruleDB.Routename, ruleDB.Component, ruleDB.Permission, ruleDB.Path, ruleDB.Redirect, ruleDB.Isext,
		ruleDB.Keepalive, ruleDB.Hideinmenu, ruleDB.Activemenu, ruleDB.Noaffix, ruleDB.Onlypage, ruleDB.Requiresauth,
	).Order(ruleDB.Weigh.Asc()).Scan(&menuList)
	if err != nil {
		gf.Failed().SetMsg("获取菜单数据失败").SetData(err).Regin(ctx)
		return
	}
	if menuList == nil {
		menuList = make(gf.List, 0)
	} else {
		for _, val := range menuList {
			if gf.IsEmpty(val["title"]) {
				val["title"] = val["locale"]
			}
		}
		menuList = gf.GetRuleTreeArrayByPack(menuList, 0)
	}
	gf.Success().SetMsg("获取菜单选择ID转json").SetData(menuList).Regin(ctx)
}

// 获取文件路径
func (api *Codestore) GetPackdirs(ctx *gf.GinCtx) {
	runEnv, _ := gcfg.Instance("app").Get(ctx, "app.runEnv")
	if runEnv.String() == "release" {
		gf.Failed().SetMsg("生产环境禁止查看目录文件，请在开发环境下操作").Regin(ctx)
		return
	}
	params, _ := gf.RequestParam(ctx)
	if params["type"] == "go" {
		var option gf.DirOption
		option.RootPath = []string{"/app", "/service", "/resource", "/utils", "/skilldoc"} // 目标根目录
		option.SubFlag = true                                                              // 遍历子目录标志 true: 遍历 false: 不遍历
		option.IgnorePath = []string{"router", "system", "codeinstall", "webadmin"}        // 忽略目录
		option.IgnoreFile = []string{`.gitkeep`, `.gitignore`, "codestore.go", "packinstall.go", "packinstall_utils.go", "attachment.go", "configuration.go",
			"dicgroup.go", "dictionary.go", "upfile.go", "uploadconfig.go", "service.go", "controller.go"} // 忽略文件
		appDir, err := gf.TraverDir(option)
		if err != nil {
			gf.Failed().SetMsg("获取后端目录失败").SetData(err).Regin(ctx)
			return
		}
		gf.Success().SetMsg("获取后端文件路径").SetData(appDir).Regin(ctx)
	} else {
		//前端
		vueobjroot, err := gcfg.Instance("app").Get(ctx, "app.vueobjroot")
		if err != nil {
			gf.Failed().SetMsg("获取前端路径配置失败").SetData(err).Regin(ctx)
			return
		}
		var option gf.DirOption
		option.RootPath = []string{"/src", "/public", "/types"} // 目标根目录
		option.SubFlag = true                                   // 遍历子目录标志 true: 遍历 false: 不遍历
		option.IgnorePath = []string{"api", "router", "directive", "systool", "configuration", "dictionary",
			"account", "log", "dept", "role", "rule", "usersetting", "codestore", "system"} // 忽略目录
		option.IgnoreFile = []string{"App.vue", "main.ts", "env.d.ts", "common.go", "settings.json"} // 忽略文件
		appDir, err := gf.TraverPathDir(option, vueobjroot.String())
		if err != nil {
			gf.Failed().SetMsg("获取前端目录失败").SetData(err).Regin(ctx)
			return
		}
		appDir.Path = vueobjroot.String()
		gf.Success().SetMsg("获取前端文件路径").SetData(appDir).Regin(ctx)
	}
}
