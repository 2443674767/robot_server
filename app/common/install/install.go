package install

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"gofly/utils/gf"
	"gofly/utils/tools/gcfg"
	"gofly/utils/tools/gcompress"
	"gofly/utils/tools/gconv"
	"gofly/utils/tools/grand"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

/**
* 项目安装
 */
type Index struct{ NoNeedLogin []string }

func init() {
	gf.Register(&Index{NoNeedLogin: []string{"*"}})
}

// 安装页面
func (api *Index) Index(c *gf.GinCtx) {
	path, err := os.Getwd()
	if err != nil {
		gf.Failed().SetMsg("项目路径获取失败").SetData(err.Error()).Regin(c)
		return
	}
	ctx := context.Background()
	appConf, _ := gcfg.Instance("app").Get(ctx, "app")
	appConf_arr := gconv.Map(appConf)
	dbConf, _ := gcfg.Instance().Get(ctx, "database.default")
	dbConf_arr := gconv.Map(dbConf)
	if appConf_arr["runEnv"] == "release" {
		gf.Failed().SetMsg("线上环境无法进行安装操作,如下安装请在在runEnv=debug环境下安装！").Regin(c)
		return
	}
	filePath := filepath.Join(path, "/devsource/developer/install/install.lock")
	_, errLock := os.Stat(filePath)
	gf.Success().SetMsg("获取应用配置").SetData(gf.Map{
		"database": gf.Map{
			"hostname": dbConf_arr["hostname"],
			"hostport": dbConf_arr["hostport"],
			"username": dbConf_arr["username"],
			"password": dbConf_arr["password"],
			"dbname":   dbConf_arr["dbname"],
			"prefix":   dbConf_arr["prefix"],
		},
		"app": gf.Map{
			"vueobjroot": appConf_arr["vueobjroot"],
		},
		"isLock": errLock == nil,
	}).Regin(c)
}

// 链接数据库配置
func dsn(username, password, hostname, hostport, dbName interface{}) string {
	return fmt.Sprintf("%s:%s@tcp(%s:%v)/%s?charset=utf8&parseTime=True&loc=Local&timeout=1000ms&sql_mode=%s&multiStatements=true", username, password, hostname, hostport, dbName, "NO_ENGINE_SUBSTITUTION")
}

// 安装
func (api *Index) Save(c *gf.GinCtx) {
	body, _ := io.ReadAll(c.Request.Body)
	var parame map[string]interface{}
	_ = json.Unmarshal(body, &parame)
	path, err := os.Getwd() //获取当前路径
	if err != nil {
		gf.Failed().SetMsg("项目路径获取失败").SetData(err.Error()).Regin(c)
		return
	}
	formDB := parame["formDB"].(map[string]interface{})
	formApp := parame["formApp"].(map[string]interface{})
	//1链接数据
	conlink := dsn(formDB["username"], formDB["password"], formDB["hostname"], formDB["hostport"], "")
	db, err := sql.Open("mysql", conlink)
	if err != nil {
		gf.Failed().SetMsg(fmt.Sprintf("链接数据库Error %s when opening DB\n", err)).Regin(c)
	}
	defer db.Close()
	ctx, cancelfunc := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelfunc()
	//2创建数据库
	sqlstr := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %v DEFAULT CHARACTER SET utf8mb4 DEFAULT COLLATE utf8mb4_general_ci", formDB["dbname"])
	dbres, adderr := db.ExecContext(ctx, sqlstr)
	if adderr != nil {
		gf.Failed().SetMsg(fmt.Sprintf("创建数据库Error %s when creating DB\n", adderr)).SetData(conlink).Regin(c)
		return
	}
	_, rferr := dbres.RowsAffected()
	if rferr != nil {
		gf.Failed().SetMsg(fmt.Sprintf("创建数据库链接Error %s when fetching rows", rferr)).Regin(c)
		return
	}
	db.Close()
	db, err = sql.Open("mysql", dsn(formDB["username"], formDB["password"], formDB["hostname"], formDB["hostport"], formDB["dbname"]))
	if err != nil {
		gf.Failed().SetMsg(fmt.Sprintf("链接数据库%v，失败Error %s when opening DB", formDB["dbname"], err)).Regin(c)
		return
	}
	defer db.Close()
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(20)
	db.SetConnMaxLifetime(time.Minute * 5)
	ctx, cancelfunc = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelfunc()
	err = db.PingContext(ctx)
	if err != nil {
		gf.Failed().SetMsg(fmt.Sprintf("检查重链接数据库%v，失败Errors %s pinging DB", formDB["dbname"], err)).Regin(c)
		return
	}
	//4导入数据库表
	SqlPath := filepath.Join(path, "/devsource/developer/install/install.sql")
	sqls, sqlerr := os.ReadFile(SqlPath)
	if sqlerr != nil {
		gf.Failed().SetMsg("数据库文件不存在：" + SqlPath).SetData(sqlerr).Regin(c)
		return
	}
	db.Exec(string(sqls))
	//5.修改表前缀
	rows, _ := db.Query("SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_TYPE = 'BASE TABLE' AND TABLE_SCHEMA='" + formDB["dbname"].(string) + "'")
	defer rows.Close()
	var tablename_str string
	var sql_str string
	var table_name string
	table_prefix := gconv.String(formDB["prefix"])
	for rows.Next() {
		rows.Scan(&tablename_str)
		if strings.HasPrefix(tablename_str, "gf_") {
			table_name = strings.Replace(tablename_str, "gf_", table_prefix, 1)
			// db.Exec("DROP TABLE " + table_name + ";") //删除已存在数据表，再更新新表前缀
			sql_str = "ALTER TABLE " + tablename_str + " RENAME TO " + table_name + ";"
		} else if strings.HasPrefix(tablename_str, table_prefix) {
			continue
		} else { //安装sql数据没有前缀
			sql_str = "ALTER TABLE " + tablename_str + " RENAME TO " + table_prefix + tablename_str + ";"
		}
		db.Exec(sql_str)
	}
	//6.修改后台账号
	salt := grand.Digits(6)
	businesspass := fmt.Sprintf("%v%v", gf.Md5(formApp["password"].(string)), salt)
	_, upberr := db.Exec("update "+table_prefix+"admin set username = ?, password = ?, salt = ? where id = ?", formApp["username"], gf.Md5(businesspass), salt, 1)
	if upberr != nil {
		gf.Failed().SetMsg("更新超级管理员账号密码失败：").SetData(upberr).Regin(c)
		return
	}
	//7 安装前端页面
	//7.1 如果没有filepath文件目录就创建一个
	file_path := formApp["vueobjroot"].(string)
	if len(file_path) == 0 {
		file_path = filepath.Join(path, "/web")
	}
	if _, err := os.Stat(file_path); err != nil {
		if !os.IsExist(err) {
			os.MkdirAll(file_path, os.ModePerm)
		}
	}
	//7.2 复制解压前端文件到指定位置
	srcFilePath := filepath.Join(path, "/devsource/developer/install/webcode.zip")
	gcompress.UnZipFile(srcFilePath, file_path, "webcode")

	//8修改配置文件-数据库
	upDbconf := gf.Map{
		"hostname": formDB["hostname"],
		"hostport": formDB["hostport"],
		"username": formDB["username"],
		"password": formDB["password"],
		"dbname":   formDB["dbname"],
		"prefix":   table_prefix,
	}
	cferr := gf.UpConfFieldData(path, upDbconf, "    ")
	if cferr != nil {
		gf.Failed().SetMsg("修改数据库配置失败").SetData(err.Error()).Regin(c)
		return
	}

	//9.修改应用配置
	upAppconf := gf.Map{
		"vueobjroot": strings.ReplaceAll(file_path, "\\", "/"),
	}
	cferr = gf.UpAppConfData(path, upAppconf, "  ")
	if cferr != nil {
		gf.Failed().SetMsg("修改前端路径失败").SetData(err.Error()).Regin(c)
		return
	}
	//11.创建安装锁文件
	filePath := filepath.Join(path, "/devsource/developer/install/install.lock")
	os.Create(filePath)
	//如果前缀不等gf_重新生dao
	if table_prefix != "gf_" {
		cmd := exec.Command("go", "run", "gen.go")
		cmd.Dir = filepath.Join(gf.ROOTPATH, "/dao/cmd")
		err = cmd.Run()
		if err != nil {
			gf.Failed().SetMsg("执行gen命令生成数据表结构失败:" + err.Error() + "，请手动到/dao/cmd目录运行：go run gen.go").Regin(c)
			return
		}
	}
	gf.Success().SetMsg("安装成功，去安装前端并运行试试！如果数据库报错可重启服务再试").Regin(c)
}
