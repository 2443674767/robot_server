package developer

import (
	"fmt"
	"gofly/dao"
	"gofly/utils/gf"
	"gofly/utils/tools/dbtool"
	"gofly/utils/tools/gcfg"
	"gofly/utils/tools/gcompress"
	"gofly/utils/tools/gconv"
	"gofly/utils/tools/gfile"
	"gofly/utils/tools/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// 用于自动注册路由
type Packinstall struct{ NoNeedAuths []string }

func init() {
	gf.Register(&Packinstall{NoNeedAuths: []string{"*"}})
}

// 打包插件
func (api *Packinstall) PackCode(ctx *gf.GinCtx) {
	runEnv, _ := gcfg.Instance("app").Get(ctx, "app.runEnv")
	if runEnv.String() == "release" {
		gf.Failed().SetMsg("生产环境禁止操作，请在开发环境下操作").Regin(ctx)
		return
	}
	params, err := gf.RequestParam(ctx)
	if err != nil {
		gf.Failed().SetMsg("解析参数失败" + err.Error()).Regin(ctx)
		return
	}
	if gf.IsEmpty(params["name"]) {
		gf.Failed().SetMsg("请填写插件包名").Regin(ctx)
		return
	}
	packName := gf.String(params["name"])
	path, err := os.Getwd()
	if err != nil {
		gf.Failed().SetMsg("项目路径获取失败").Regin(ctx)
		return
	}
	pack_path := filepath.Join(path, "/devsource/codemarket/release/", packName)
	//1制作打包文件
	if _, err := os.Stat(pack_path); err != nil && !os.IsExist(err) {
		os.MkdirAll(pack_path, os.ModePerm)
	} else {
		os.RemoveAll(pack_path)
		os.MkdirAll(pack_path, os.ModePerm)
	}
	//2复制包模板
	gfile.Copy(filepath.Join(path, "/devsource/developer/codetpl/packcode"), pack_path)
	//3.导出数据库表
	var tables = make([]string, 0)
	if !gf.IsEmpty(params["packtables"]) {
		pathname := filepath.Join(path, "/devsource/codemarket/release", packName, "install.sql")
		tables = strings.Split(gf.String(params["packtables"]), ",")
		dbtool.ExecSqlFile(tables, pathname)
	}
	//4.更新基础配置
	//处理数据表前缀
	var packtablesStr = ""
	if len(tables) > 0 {
		prefix, _ := gcfg.Instance().Get(ctx, "database.default.prefix")
		var tables_noprefix = make([]string, 0)
		for _, val := range tables {
			tables_noprefix = append(tables_noprefix, strings.Replace(val, prefix.String(), "", 1))
		}
		packtablesStr = strings.Join(tables_noprefix, ",")
	}
	upconf := map[string]interface{}{"version": params["version"], "title": params["title"], "installcover": params["installcover"], "isModTidy": params["isModTidy"], "commandLines": fmt.Sprintf("\"%v\"", params["commandLines"]), "des": params["des"], "name": params["name"],
		"packtables": packtablesStr, "goFiles": fmt.Sprintf("'%v'", gf.String(params["goFiles"])), "vueFiles": fmt.Sprintf("'%v'", gf.String(params["vueFiles"]))}
	gf.UpCodeinstall(path+"/devsource/codemarket/release/"+packName, upconf)
	//5.把后台菜单数据写入adminmenu.json文件
	if !gf.IsEmpty(params["menujson"]) {
		meni_json := filepath.Join(path, "/devsource/codemarket/release/", packName, "adminmenu.json")
		if _, err := os.Stat(meni_json); err != nil {
			if !os.IsExist(err) {
				os.MkdirAll(meni_json, os.ModePerm)
			}
		}
		menudata, _ := json.Marshal(params["menujson"])
		os.WriteFile(meni_json, menudata, 0777)
	}
	//6. 查看/resource/config/code是否存在动态配置文件-存在则复制到包目录下
	confFilePath := filepath.Join(path, "/resource/config/code", packName+".yaml")
	if _, err := os.Stat(confFilePath); !os.IsNotExist(err) { //存在
		gfile.CopyFile(confFilePath, filepath.Join(pack_path, packName+".yaml"))
	}
	//7. 查看/resource/static是否存在静态文件-存在则复制到包目录下
	staticFilePath := filepath.Join(path, "/resource/static", packName)
	if _, err := os.Stat(staticFilePath); !os.IsNotExist(err) { //存在
		gfile.Copy(staticFilePath, filepath.Join(pack_path, packName))
	}
	//8.把后端(Go)代码复制到插件包中
	if !gf.IsEmpty(params["goFiles"]) {
		goFiles, ok := params["goFiles"].([]interface{})
		if ok {
			for _, gofv := range goFiles {
				item, ok := gofv.(map[string]interface{})
				if ok {
					gfile.Copy(filepath.Join(path, gf.String(item["path"])), filepath.Join(path, "/devsource/codemarket/release/", packName, "/go", gf.String(item["path"])))
				}
			}
		}
	}
	//8.把前端(vue)代码复制到插件包中
	if !gf.IsEmpty(params["vueFiles"]) {
		vueobjroot, err := gcfg.Instance("app").Get(ctx, "app.vueobjroot")
		if err != nil {
			gf.Failed().SetMsg("获取前端路径配置失败").SetData(err).Regin(ctx)
			return
		}
		vueFiles, ok := params["vueFiles"].([]interface{})
		if ok {
			for _, gofv := range vueFiles {
				item, ok := gofv.(map[string]interface{})
				if ok {
					gfile.Copy(filepath.Join(vueobjroot.String(), gf.String(item["path"])), filepath.Join(path, "/devsource/codemarket/release/", packName, "/vue", gf.String(item["path"])))
				}
			}
		}
	}
	//打包文件路径
	if _, err := os.Stat(pack_path); err == nil {
		defer os.RemoveAll(pack_path) //最后删除文件夹
		dest := filepath.Join(path, "/devsource/codemarket/release", packName+".zip")
		err = gcompress.ZipCompress(pack_path, dest)
		if err != nil {
			gf.Failed().SetMsg("打包压缩成zip错误").SetData(err).Regin(ctx)
			return
		}
		gf.Success().SetMsg("打包成功").SetData(true).Regin(ctx)
	} else {
		gf.Failed().SetMsg("文件不存在").Regin(ctx)
	}
}

// 上传文件到代码仓
func (api *Packinstall) Upfile(ctx *gf.GinCtx) {
	params, err := gf.RequestParam(ctx)
	if err != nil {
		gf.Failed().SetMsg("解析参数失败" + err.Error()).Regin(ctx)
		return
	}
	fmt.Println("上传文件到代码仓:", params["code_token"])
	result, err := UpFileClient(ctx, map[string]string{"code_token": gf.String(params["code_token"])})
	if err != nil {
		gf.Failed().SetMsg(err.Error()).Regin(ctx)
		return
	}
	var parameter map[string]interface{}
	if err := json.Unmarshal([]byte(string(result)), &parameter); err == nil {
		if parameter["status"] == "done" {
			gf.Success().SetMsg("附件上传成功").SetData(parameter).Regin(ctx)
		} else {
			gf.Failed().SetMsg(gf.String(parameter["message"])).SetData(parameter).Regin(ctx)
		}
	}
}

// 下载插件代码到本地-安装使用
func (api *Packinstall) DownCode(ctx *gf.GinCtx) {
	runEnv, _ := gcfg.Instance("app").Get(ctx, "app.runEnv")
	if runEnv.String() == "release" {
		gf.Failed().SetMsg("生产环境禁止操作，请在开发环境下操作").Regin(ctx)
		return
	}
	params, perr := gf.PostParam(ctx)
	packName := gconv.String(params["name"])
	if packName == "" || perr != nil {
		gf.Failed().SetMsg("安装包名称不能为空").SetData(perr).Regin(ctx)
		return
	}
	path, _ := os.Getwd()
	install_apppath := filepath.Join(path, "/devsource/codemarket/install", packName)
	if _, err := os.Stat(install_apppath); err == nil {
		gf.Success().SetMsg("本地代码已存在直接安装").SetData(true).Regin(ctx)
		return
	}
	baseurl := ctx.DefaultQuery("baseurl", "")
	if baseurl == "" {
		gf.Failed().SetMsg("代码仓请求url不存在！").Regin(ctx)
		return
	}
	//1.请求社区下载插件下载地址
	params["frame"] = 3
	result, err := gf.HttpGet(baseurl+"/goflycode/content/getDownUrl", params)
	if err != nil {
		gf.Failed().SetMsg("请求GoFLy社区获取代码商店失败").SetData(err).Regin(ctx)
		return
	}
	if gconv.Int(result["code"]) != 0 || gf.IsEmpty(result["data"]) {
		gf.Failed().SetMsg(gf.String(result["message"])).SetData(result).Regin(ctx)
		return
	}
	//2.下载插件代码zip
	downdir := filepath.Join(path, "/devsource/codemarket/install", packName+".zip")
	downstatus, downdir_zippath := gf.DownFileToDir(gf.String(result["data"]), downdir)
	if downstatus {
		dezipdir := filepath.Join(path, "/devsource/codemarket/install", packName)
		err := gcompress.UnZipFile(downdir_zippath, dezipdir, packName)
		if err == nil {
			os.Remove(downdir_zippath)
			gf.Success().SetMsg("解压代码包成功").SetData(true).Regin(ctx)
		} else {
			gf.Failed().SetMsg("解压代码包代码失败").SetData(err).SetExdata(result["data"]).Regin(ctx)
		}
	} else {
		gf.Failed().SetMsg("下载代码失败").SetExdata(result["data"]).Regin(ctx)
	}
}

// 安装本地插件
func (api *Packinstall) InstallLocalCode(ctx *gf.GinCtx) {
	runEnv, _ := gcfg.Instance("app").Get(ctx, "app.runEnv")
	if runEnv.String() == "release" {
		gf.Failed().SetMsg("生产环境禁止操作，请在开发环境下操作").Regin(ctx)
		return
	}
	file, err := ctx.FormFile("file")
	if err != nil {
		gf.Failed().SetMsg("获取数据失败，").SetData(err).Regin(ctx)
		return
	}
	path, err := os.Getwd()
	if err != nil {
		gf.Failed().SetMsg("项目路径获取失败")
		return
	}
	//保存文件
	installPath := "/devsource/codemarket/install"
	downpathzip := filepath.Join(path, installPath, file.Filename)
	err = ctx.SaveUploadedFile(file, downpathzip)
	if err != nil {
		gf.Failed().SetMsg("上传失败").SetData(err).Regin(ctx)
		return
	}
	//解压
	dezipdir := filepath.Join(path, installPath)
	zipFilePath := filepath.Join(path, installPath, file.Filename)
	err = gcompress.UnZipFile(zipFilePath, dezipdir)
	if err == nil {
		os.Remove(zipFilePath)
		filename_arr := strings.Split(file.Filename, ".")
		gf.Success().SetMsg("解压上传本地插件包成功").SetData(filename_arr[0]).Regin(ctx)
	} else {
		gf.Failed().SetMsg("解压上传本地插件包失败").SetData(err).Regin(ctx)
	}
}

// 安装插件
func (api *Packinstall) InstallCode(ctx *gf.GinCtx) {
	//获取应用配置
	appConf, err := gcfg.Instance("app").Get(ctx, "app")
	if err != nil {
		gf.Failed().SetMsg("获取应用配置失败（app.yalm）").SetData(err).Regin(ctx)
		return
	}
	appConf_arr := gconv.Map(appConf)
	if gf.String(appConf_arr["runEnv"]) == "release" {
		gf.Failed().SetMsg("生产环境禁止操作，请在开发环境下操作").Regin(ctx)
		return
	}
	parameter, err := gf.PostParam(ctx)
	if err != nil {
		gf.Failed().SetMsg("安装参数错误！" + err.Error()).Regin(ctx)
		return
	}
	packName := gconv.String(parameter["name"])
	if packName == "" {
		gf.Failed().SetMsg("安装包名称不能为空").Regin(ctx)
		return
	}
	path, err := os.Getwd()
	if err != nil {
		gf.Failed().SetMsg("项目路径获取失败").Regin(ctx)
		return
	}
	//获取插件配置
	installConfig, err := gcfg.Instance("config", "devsource/codemarket/install/"+packName).Get(ctx, "app")
	if err != nil {
		gf.Failed().SetMsg("插件配置文件解析失败").SetData(err).Regin(ctx)
		return
	}
	installConf_arr := gconv.Map(installConfig)
	//1.导入后台菜单
	adminmenuPath := filepath.Join(path, "/devsource/codemarket/install", packName, "/adminmenu.json")
	amenufile, _ := os.Open(adminmenuPath)
	amenubytes, amenuerr := io.ReadAll(amenufile)
	anenuids := ""
	if amenuerr == nil && amenubytes != nil {
		var menudata interface{}
		json.Unmarshal([]byte(amenubytes), &menudata)
		m_nenuids := Insertmenu(ctx, menudata, 0)
		if len(m_nenuids) > 0 {
			ruleDB := dao.Query().AdminAuthRule
			var parent_ids = make([]int32, 0)
			for _, inMenuid := range m_nenuids {
				var parent_id *int32
				ruleDB.WithContext(ctx).Where(ruleDB.ID.Eq(inMenuid)).Select(ruleDB.Pid).Scan(&parent_id)
				if parent_id != nil {
					var cMenu_id *int32
					ruleDB.WithContext(ctx).Where(ruleDB.Pid.Eq(*parent_id), ruleDB.ID.NotIn(m_nenuids...)).Select(ruleDB.Pid).Scan(&cMenu_id)
					if cMenu_id == nil {
						parent_ids = append(parent_ids, *parent_id)
					}
				}
			}
			m_nenuids = append(m_nenuids, gf.RemoveDuplicates(parent_ids)...)
		}
		anenuids = gf.Int32Join(m_nenuids, ",")
	}
	//关闭menu.json读取
	amenufile.Close()
	//2.安装后端go
	var appfolders string = ""
	if !gf.IsEmpty(installConf_arr["goFiles"]) && gf.String(installConf_arr["goFiles"]) != "[]" {
		var goFileList []map[string]any
		if err := json.Unmarshal([]byte(gf.String(installConf_arr["goFiles"])), &goFileList); err != nil {
			gf.Failed().SetMsg("插件配置文件解析数据失败").SetData(err).Regin(ctx)
			return
		}

		//2.1导入数据表
		SqlPath := filepath.Join(path, "/devsource/codemarket/install", packName, "/install.sql")
		_, aqlerr := dbtool.ImportSql(SqlPath)
		if aqlerr != nil {
			gf.Failed().SetMsg("导入插件sql数据文件失败!").SetData(aqlerr).Regin(ctx)
			return
		}
		//如果存在前缀-添加导入表前缀
		if !gf.IsEmpty(installConf_arr["packtables"]) {
			prefix, err := gcfg.Instance().Get(ctx, "database.default.prefix")
			if err != nil {
				gf.Failed().SetMsg("获取数据表前缀配置失败").SetData(err).Regin(ctx)
				return
			}
			var tableNames_arr = strings.Split(gf.String(installConf_arr["packtables"]), ",")
			for _, tableName := range tableNames_arr {
				if tableName != "" {
					//删除已存在数据表
					dao.RenameTableWithPrefix(ctx, dao.DB(), tableName, prefix.String())
				}
			}
			//如果有新的数据表导入则需要到dao下运行gen命令生成model和query
			// dbrows, err := sqlRes.RowsAffected()
			// if err == nil && dbrows > 0 {
			cmd := exec.Command("go", "run", "gen.go")
			cmd.Dir = filepath.Join(gf.ROOTPATH, "/dao/cmd")
			err = cmd.Run()
			if err != nil {
				gf.Failed().SetMsg("执行生成数据表结构gen命令失败").SetData(err).Regin(ctx)
				return
			}
			// }
		}

		//2.2处理Go后端文件
		err = gfile.Copy(filepath.Join(path, "/devsource/codemarket/install", packName, "/go"), filepath.Join(path))
		if err != nil {
			gf.Failed().SetMsg("安装后端go失败").SetData(err).Regin(ctx)
			return
		}

		//2.3判断是否mod拉取依赖(使用框架为安装包)
		if gf.Bool(installConf_arr["isModTidy"]) {
			exec.Command("go", "mod", "tidy").Run()
		}

		//2.4 添加控制器
		go_app_str, go_app_arr, _ := GetAllFileApp(filepath.Join(path, "/devsource/codemarket/install", packName, "/go/app"))
		for _, go_app_dir := range go_app_arr {
			modelname := ""
			//添加根模块-在app下的控制器
			if strings.Contains(go_app_dir, "/") {
				go_path_arr := strings.Split(go_app_dir, "/")
				modelname = go_path_arr[0]
				//判断安装模块下是否存在控制器controller.go文件，haseMoleCtr=true是存在
				var haseMoleCtr bool = false
				if _, err := os.Stat(filepath.Join(path, "/devsource/codemarket/install", packName, "/go/app/", modelname, "controller.go")); !os.IsNotExist(err) {
					haseMoleCtr = true
				}
				CheckIsAddController("", modelname, haseMoleCtr)
			}
			//过滤lib-添加类型模块控制-例如business下面的控制
			if !strings.HasSuffix(go_app_dir, "lib") {
				CheckIsAddController(modelname, go_app_dir, false)
			}
		}
		appfolders = go_app_str
		//判断是否执行Service层命令
		if gfile.Exists(filepath.Join(gf.ROOTPATH, "/service")) && strings.Contains(gf.String(installConf_arr["goFiles"]), "/service") {
			cmd := exec.Command("go", "run", "gen.go")
			cmd.Dir = filepath.Join(gf.ROOTPATH, "/service/cmd")
			cmd.Run()
		}
	}
	//3.安装前端vue
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
	//3.2安装前端代码
	if !gf.IsEmpty(installConf_arr["vueFiles"]) && gf.String(installConf_arr["vueFiles"]) != "[]" {
		err = gfile.Copy(filepath.Join(path, "/devsource/codemarket/install", packName, "/vue"), filepath.Join(vueobjroot))
		if err != nil {
			gf.Failed().SetMsg("安装前端vue失败").SetData(err).Regin(ctx)
			return
		}
	}
	//4.更新配置文件
	upconf := gf.Map{"isinstall": true, "adminmenuids": anenuids, "appfolders": appfolders}
	cferr := gf.UpCodeinstall(path+"/devsource/codemarket/install/"+packName, upconf)
	if cferr != nil {
		gf.Failed().SetMsg("更新配置失败，请重新安装").SetData(packName).Regin(ctx)
		return
	}
	//5.复制配置文件到resource/codeinstall的的配置文件夹
	gfile.CopyFile(filepath.Join(path, "/devsource/codemarket/install/", packName, "config.yaml"), filepath.Join(path, "/resource/codeinstall", packName, "config.yaml"))
	//6.复制插件包配置到resource/config/code动态配置管理目录下
	confFilePath := filepath.Join(path, "/devsource/codemarket/install/", packName, packName+".yaml")
	if _, err := os.Stat(confFilePath); !os.IsNotExist(err) { //存在到统一管理插件使用的配置文件夹
		gfile.CopyFile(confFilePath, filepath.Join(path, "/resource/config/code", packName+".yaml"))
	}
	//6.如存在附件资源存在则复制到 /resource/static下
	staticFilePath := filepath.Join(path, "/devsource/codemarket/install/", packName, packName)
	if _, err := os.Stat(staticFilePath); !os.IsNotExist(err) { //存在则复制到 /resource/static下
		gfile.Copy(staticFilePath, filepath.Join(path, "/resource/static", packName))
	}
	//7.删除安装文件包
	os.RemoveAll(filepath.Join(path, "/devsource/codemarket/install/", packName))
	gf.Success().SetMsg("安装成功").SetData(packName).Regin(ctx)
}

// 卸载插件
func (api *Packinstall) UninstallCode(ctx *gf.GinCtx) {
	appConf, err := gcfg.Instance("app").Get(ctx, "app")
	if err != nil {
		gf.Failed().SetMsg("获取应用配置失败（app.yalm）").SetData(err).Regin(ctx)
		return
	}
	appConf_arr := gconv.Map(appConf)
	if gf.String(appConf_arr["runEnv"]) == "release" {
		gf.Failed().SetMsg("生产环境禁止操作，请在开发环境下操作").Regin(ctx)
		return
	}
	parameter, err := gf.PostParam(ctx)
	if err != nil {
		gf.Failed().SetMsg("安装参数错误！" + err.Error()).Regin(ctx)
		return
	}
	//插件包名
	packName := gconv.String(parameter["name"])
	if packName == "" {
		gf.Failed().SetMsg("卸载包名称不能为空").Regin(ctx)
		return
	}
	path, err := os.Getwd()
	if err != nil {
		gf.Failed().SetMsg("项目路径获取失败").Regin(ctx)
		return
	}
	//1.获取插件配置
	installConfig, err := gcfg.Instance("config", "resource/codeinstall/"+packName).Get(ctx, "app")
	if err != nil {
		gf.Failed().SetMsg("插件配置文件解析失败").SetData(err).Regin(ctx)
		return
	}
	installConf_arr := gconv.Map(installConfig)

	//2.删除安装的后端Go代码
	if !gf.IsEmpty(installConf_arr["goFiles"]) {
		var goFileList []map[string]any
		if err := json.Unmarshal([]byte(gf.String(installConf_arr["goFiles"])), &goFileList); err != nil {
			gf.Failed().SetMsg("解析Go后端目录文件数据失败").SetData(err).Regin(ctx)
			return
		}
		for _, goDir := range goFileList {
			if gf.Bool(goDir["isDir"]) {
				os.RemoveAll(filepath.Join(path, gf.String(goDir["path"])))
			} else {
				os.Remove(filepath.Join(path, gf.String(goDir["path"])))
			}
		}

	}
	//3.删除路由(控制器)
	if !gf.IsEmpty(installConf_arr["appfolders"]) {
		var app_folders_arr = strings.Split(gf.String(installConf_arr["appfolders"]), ",")
		for _, appFolderName := range app_folders_arr {
			if appFolderName != "" {
				modelname := appFolderName
				if strings.Contains(appFolderName, "/") {
					go_path_arr := strings.Split(appFolderName, "/")
					modelname = go_path_arr[0]
					if gf.IsDirEmpty(filepath.Join(path, "app", appFolderName)) {
						CheckApiRemoveController(modelname, appFolderName)
					}
				}
				//判断是否要删除model目录
				hasefolders, _ := gf.GetDirHasefolder(filepath.Join(path, "app", modelname))
				if !hasefolders {
					//删除APP下的控制模块
					CheckApiRemoveController("", modelname)
				}
			}
		}
	}
	//4.删除数据表结构dao文件和数据库对应表
	if !gf.IsEmpty(installConf_arr["packtables"]) {
		prefix, err := gcfg.Instance().Get(ctx, "database.default.prefix")
		if err != nil {
			gf.Failed().SetMsg("获取数据表前缀配置失败").SetData(err).Regin(ctx)
			return
		}
		tableNameArr := strings.Split(gf.String(installConf_arr["packtables"]), ",")
		for _, tableName := range tableNameArr {
			if tableName != "" {
				dao.DB().Exec("DROP TABLE IF EXISTS " + gf.String(prefix) + tableName)
			}
		}
		cmd := exec.Command("go", "run", "gen.go")
		cmd.Dir = filepath.Join(path, "/dao/cmd")
		err = cmd.Run()
		if err != nil {
			gf.Failed().SetMsg("执行重新生成数据表结构gen命令失败").SetData(err).Regin(ctx)
			return
		}
	}
	//5.卸载菜单
	if !gf.IsEmpty(installConf_arr["adminmenuids"]) {
		ruleDB := dao.Query().AdminAuthRule
		menuIds := strings.Split(gf.String(installConf_arr["adminmenuids"]), ",")
		menuIdsInt32 := gf.InterfaceToInt32(menuIds)
		var thirMenu []int32
		ruleDB.WithContext(ctx).Unscoped().Where(ruleDB.Pid.In(menuIdsInt32...)).Pluck(ruleDB.ID, &thirMenu)
		ruleDB.WithContext(ctx).Unscoped().Where(ruleDB.ID.In(menuIdsInt32...)).Delete()
		ruleDB.WithContext(ctx).Unscoped().Where(ruleDB.Pid.In(menuIdsInt32...)).Delete()
		if len(thirMenu) > 0 { //三级菜单
			ruleDB.WithContext(ctx).Unscoped().Where(ruleDB.Pid.In(thirMenu...)).Delete()
		}
	}
	//6判断是否mod拉取依赖(使用框架为安装包)
	if gf.Bool(installConf_arr["isModTidy"]) {
		exec.Command("go", "mod", "tidy").Run()
	}
	//判断是否执行Service层命令
	if gfile.Exists(filepath.Join(gf.ROOTPATH, "/service")) && strings.Contains(gf.String(installConf_arr["goFiles"]), "/service") {
		cmd := exec.Command("go", "run", "gen.go")
		cmd.Dir = filepath.Join(gf.ROOTPATH, "/service/cmd")
		cmd.Run()
	}
	//7.删除配置文件
	os.RemoveAll(filepath.Join(path, "/resource/codeinstall", packName)) //删除配置文件
	//8.删除config配置文件
	ResourceConfig := filepath.Join(path, "/resource/config/code", packName+".yaml")
	if _, err := os.Stat(ResourceConfig); err == nil {
		os.Remove(ResourceConfig)
	}
	//9.如果/resource/static下存在资源文件则删除
	staticFilePath := filepath.Join(path, "/resource/static", packName)
	if _, err := os.Stat(staticFilePath); !os.IsNotExist(err) { //存在
		os.RemoveAll(staticFilePath)
	}
	//10.删除vue前端
	if !gf.IsEmpty(installConf_arr["vueFiles"]) {
		var vueFileList []map[string]any
		if err := json.Unmarshal([]byte(gf.String(installConf_arr["vueFiles"])), &vueFileList); err != nil {
			gf.Failed().SetMsg("解析Vue前端目录文件数据失败").SetData(err).Regin(ctx)
			return
		}
		if len(vueFileList) > 0 {
			vueobjroot := gf.String(appConf_arr["vueobjroot"])
			for _, vueDir := range vueFileList {
				if gf.Bool(vueDir["isDir"]) {
					os.RemoveAll(filepath.Join(vueobjroot, gf.String(vueDir["path"])))
				} else {
					os.Remove(filepath.Join(vueobjroot, gf.String(vueDir["path"])))
				}
			}
			//卸载前端依赖包
			commandArr := strings.Split(gf.String(installConf_arr["commandLines"]), ",")
			isNpm := gfile.Exists(filepath.Join(vueobjroot, "package-lock.json"))
			isYarn := gfile.Exists(filepath.Join(vueobjroot, "yarn.lock"))
			for _, commstr := range commandArr {
				if isYarn && !isNpm {
					cmd := exec.Command("yarn", "remove", commstr)
					cmd.Dir = vueobjroot
					cmd.Run()
				} else {
					cmd := exec.Command("npm", "uninstall", commstr)
					cmd.Dir = vueobjroot
					cmd.Run()
				}
			}
		}
	}
	gf.Success().SetMsg("卸载成功").Regin(ctx)
}
