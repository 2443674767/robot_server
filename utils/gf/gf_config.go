package gf

import (
	"bufio"
	"fmt"
	"gofly/utils/tools/gcfg"
	"gofly/utils/tools/gconv"
	"gofly/utils/tools/gctx"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// 获取配置
var (
	ctx         = gctx.New()
	appConf, _  = gcfg.Instance("app").Get(ctx, "app")
	appConf_arr = gconv.Map(appConf)
	dbConf, _   = gcfg.Instance().Get(ctx, "database.default")
	dbConf_arr  = gconv.Map(dbConf)
	//配置项
	VERSION = fmt.Sprintf("v%s", appConf_arr["version"])
)

// 清除App配置缓存-强制刷新配置
func RefreshAppConf() {
	gcfg.Instance("app").ClearCache()
	appConf, _ = gcfg.Instance("app").Get(ctx, "app")
	appConf_arr = gconv.Map(appConf)
}

// 更新配置文件，config.yaml
func UpConfFieldData(path string, parameter map[string]interface{}, emptystr string) error {
	file_path := filepath.Join(path, "/resource/config/config.yaml")
	f, err := os.Open(file_path)
	if err != nil {
		return err
	}
	defer f.Close()
	buf := bufio.NewReader(f)
	var result = ""
	var is_hose = false
	for {
		is_hose = false
		a, _, c := buf.ReadLine()
		if c == io.EOF {
			break
		}
		for keys, Val := range parameter {
			if strings.Contains(string(a), keys) {
				is_hose = true
				datestr := strings.ReplaceAll(string(a), string(a), fmt.Sprintf("%s%v: %v\n", emptystr, keys, Val))
				result += datestr
			}
		}
		if !is_hose {
			result += string(a) + "\n"
		}
	}
	fw, err := os.OpenFile(file_path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666) //os.O_TRUNC清空文件重新写入，否则原文件内容可能残留
	w := bufio.NewWriter(fw)
	w.WriteString(result)
	if err != nil {
		panic(err)
	}
	w.Flush()
	return nil
}

// 更新应用配置文件，app.yaml
func UpAppConfData(path string, parameter map[string]interface{}, emptystr string) error {
	file_path := filepath.Join(path, "/resource/config/app.yaml")
	return UpConfigFild(file_path, parameter, emptystr)
}

// 更新/resource/config指定文件
func UpConfigFild(file_path string, parameter map[string]interface{}, emptystr string) error {
	f, err := os.Open(file_path)
	if err != nil {
		return err
	}
	defer f.Close()
	buf := bufio.NewReader(f)
	var result = ""
	var is_hose = false
	for {
		is_hose = false
		a, _, c := buf.ReadLine()
		if c == io.EOF {
			break
		}
		for keys, Val := range parameter {
			if strings.Contains(string(a), keys) {
				is_hose = true
				datestr := strings.ReplaceAll(string(a), string(a), fmt.Sprintf("%s%v: %v\n", emptystr, keys, Val))
				result += datestr
			}
		}
		if !is_hose {
			result += string(a) + "\n"
		}
	}
	fw, err := os.OpenFile(file_path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666) //os.O_TRUNC清空文件重新写入，否则原文件内容可能残留
	w := bufio.NewWriter(fw)
	w.WriteString(result)
	if err != nil {
		panic(err)
	}
	w.Flush()
	return nil
}

// 更新插件配置文件
func UpCodeinstall(path string, parameter map[string]interface{}) error {
	file_path := filepath.Join(path, "config.yaml")
	f, err := os.Open(file_path)
	if err != nil {
		return err
	}
	defer f.Close()
	buf := bufio.NewReader(f)
	var result = ""
	var is_hose = false
	for {
		is_hose = false
		a, _, c := buf.ReadLine()
		if c == io.EOF {
			break
		}
		for keys, Val := range parameter {
			if strings.Contains(string(a), keys) {
				is_hose = true
				datestr := strings.ReplaceAll(string(a), string(a), fmt.Sprintf("     %v: %v\n", keys, Val))
				result += datestr
			}
		}
		if !is_hose {
			result += string(a) + "\n"
		}
	}
	fw, err := os.OpenFile(file_path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666) //os.O_TRUNC清空文件重新写入，否则原文件内容可能残留
	w := bufio.NewWriter(fw)
	w.WriteString(result)
	if err != nil {
		panic(err)
	}
	w.Flush()
	fw.Close()
	return nil
}

// 读取resource/config下指定文件配置内容
func GetConfByFile(packName string) (interface{}, error) {
	path, _ := os.Getwd()
	data, err := GetYamlConfigData(filepath.Join(path, "/resource/config"), packName)
	return data, err
}
