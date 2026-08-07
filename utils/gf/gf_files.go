package gf

import (
	"bufio"
	"errors"
	"fmt"
	"gofly/utils/tools/gcfg"
	"gofly/utils/tools/gfile"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/viper"
)

var (
	ROOTPATH = gfile.RootPath()
)

// 清除文件存储配置缓存-强制刷新配置
func RefreshUploadConf() {
	gcfg.Instance("upload").ClearCache()
}

// 获取附件访问的完整地址，传入路径返回完整
func GetFullUrl(url string) string {
	if url[0:4] == "http" {
		return url
	}
	if !strings.HasPrefix(url, "/") {
		url = "/" + url
	}
	//处理地址
	filseName := gfile.Name(url)
	if strings.HasPrefix(filseName, "local") { //本地存储
		BaseUrl, _ := gcfg.Instance("upload").Get(ctx, "local.LBaseUrl")
		return BaseUrl.String() + url
	} else if strings.HasPrefix(filseName, "alioss") { //阿里云
		BaseUrl, _ := gcfg.Instance("upload").Get(ctx, "alioss.ABaseUrl")
		return BaseUrl.String() + url
	} else if strings.HasPrefix(filseName, "tencentcos") { //腾讯云
		BaseUrl, _ := gcfg.Instance("upload").Get(ctx, "tencentcos.TBaseUrl")
		return BaseUrl.String() + url
	} else if strings.HasPrefix(filseName, "qiniuoss") { //七牛云
		BaseUrl, _ := gcfg.Instance("upload").Get(ctx, "qiniuoss.QBaseUrl")
		return BaseUrl.String() + url
	} else if strings.HasPrefix(filseName, "minio") { //MinIO
		Endpoint, _ := gcfg.Instance("upload").Get(ctx, "minio.endpoint")
		Bucket, _ := gcfg.Instance("upload").Get(ctx, "minio.bucket")
		UseSSL, _ := gcfg.Instance("upload").Get(ctx, "minio.useSSL")
		endpoint := strings.TrimRight(Endpoint.String(), "/")
		if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
			scheme := "http"
			if UseSSL.Bool() {
				scheme = "https"
			}
			endpoint = scheme + "://" + endpoint
		}
		return endpoint + "/" + Bucket.String() + url
	} else { //默认返回设置上传方式的地址
		return GetRootUrl() + url
	}
}

// 获取本地附件访问的完整地址，传入路径返回完整
func GetLocalFullUrl(url string) string {
	if url[0:4] == "http" {
		return url
	}
	if !strings.HasPrefix(url, "/") {
		url = "/" + url
	}
	BaseUrl, _ := gcfg.Instance("upload").Get(ctx, "local.LBaseUrl")
	return BaseUrl.String() + url
}

// 获取当前设置上传方式的附件访问域名，如当前是本地(local)则返回本地访问地址
func GetRootUrl() string {
	UpType, _ := gcfg.Instance("upload").Get(ctx, "Type")
	switch UpType.String() {
	case "alioss":
		BaseUrl, _ := gcfg.Instance("upload").Get(ctx, "alioss.ABaseUrl")
		return BaseUrl.String()
	case "tencentcos":
		BaseUrl, _ := gcfg.Instance("upload").Get(ctx, "tencentcos.TBaseUrl")
		return BaseUrl.String()
	case "qiniuoss":
		BaseUrl, _ := gcfg.Instance("upload").Get(ctx, "qiniuoss.QBaseUrl")
		return BaseUrl.String()
	case "minio":
		Endpoint, _ := gcfg.Instance("upload").Get(ctx, "minio.endpoint")
		Bucket, _ := gcfg.Instance("upload").Get(ctx, "minio.bucket")
		UseSSL, _ := gcfg.Instance("upload").Get(ctx, "minio.useSSL")
		endpoint := strings.TrimRight(Endpoint.String(), "/")
		if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
			scheme := "http"
			if UseSSL.Bool() {
				scheme = "https"
			}
			endpoint = scheme + "://" + endpoint
		}
		return endpoint + "/" + Bucket.String()
	default:
		BaseUrl, _ := gcfg.Instance("upload").Get(ctx, "local.LBaseUrl")
		return BaseUrl.String()
	}
}

// 获取所有上传方式附件访问的地址域名
func GetAllRootUrl() Map {
	LBaseUrl, _ := gcfg.Instance("upload").Get(ctx, "local.LBaseUrl")
	ABaseUrl, _ := gcfg.Instance("upload").Get(ctx, "alioss.ABaseUrl")
	TBaseUrl, _ := gcfg.Instance("upload").Get(ctx, "tencentcos.TBaseUrl")
	QBaseUrl, _ := gcfg.Instance("upload").Get(ctx, "qiniuoss.QBaseUrl")
	return Map{
		"local":      LBaseUrl.String(),
		"alioss":     ABaseUrl.String(),
		"tencentcos": TBaseUrl.String(),
		"qiniuoss":   QBaseUrl.String(),
	}
}

// 获取本地文件访问的地址域名
func GetLocalUrl() string {
	LBaseUrl, _ := gcfg.Instance("upload").Get(ctx, "local.LBaseUrl")
	return LBaseUrl.String()
}

// 覆盖写入文件
// filePath文件路径
func WriteToFile(filePath string, content string) error {
	if _, err := os.Stat(filePath); err != nil {
		if !os.IsExist(err) {
			pathstr_arr := strings.Split(filePath, `/`)
			path_dirs := strings.Split(filePath, (pathstr_arr[len(pathstr_arr)-1]))
			os.MkdirAll(path_dirs[0], os.ModePerm)
			os.Create(filePath)
		}
	}
	f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	if err != nil {
		fmt.Println("file create failed. err: " + err.Error())
	} else {
		n, _ := f.Seek(0, io.SeekEnd)
		_, err = f.WriteAt([]byte(content), n)
		defer f.Close()
	}
	return err
}

// 逐行读取文件
// filePath文件路径
func ReaderFileByline(filePath string) ([]interface{}, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := bufio.NewReader(f)
	var list []interface{}
	for {
		line, _, err := buf.ReadLine()
		if err == io.EOF {
			break
		}
		list = append(list, string(line))
	}
	return list, nil
}

// 一次性读取全部文件
// filePath文件路径
func ReaderFileBystring(filePath string) string {
	file, err := os.Open(filePath)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	bytes, err := io.ReadAll(file)
	if err != nil {
		panic(err.Error())
	}
	return string(bytes)
}

// DownPic 远程下载图片
func DownPic(src, dest string) (string, error) {
	re, err := http.Get(src)
	if err != nil {
		return "", err
	}
	defer re.Body.Close()
	fix := "png"
	if idx := strings.LastIndex(src, "."); idx != -1 {
		fix = strings.ToLower(src[idx+1:])
		if strings.Contains(fix, "?") {
			fix_arr := strings.Split(fix, "?")
			fix = fix_arr[0]
		}
	}
	if fix == "" {
		return "", fmt.Errorf("unknow pic type, pic path: %s", src)
	}
	thumbF, err := os.OpenFile(dest+"."+fix, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return "", err
	}
	defer thumbF.Close()
	if fix == "jpeg" || fix == "jpg" {
		img, err := jpeg.Decode(re.Body)
		if err != nil {
			return "", err
		}
		if err = jpeg.Encode(thumbF, img, &jpeg.Options{Quality: 40}); err != nil {
			return "", err
		}
	} else if fix == "png" {
		img, err := png.Decode(re.Body)
		if err != nil {
			return "", err
		}
		if err = png.Encode(thumbF, img); err != nil {
			return "", err
		}
	} else if fix == "gif" {
		img, err := gif.Decode(re.Body)
		if err != nil {
			return "", err
		}
		if err = gif.Encode(thumbF, img, nil); err != nil {
			return "", err
		}
	} else {
		return "", errors.New("不支持的格式")
	}
	return "." + fix, nil
}

// 获取文文件夹下的文件及文件返回数组
func GetAllFileArray(pathname string) (map[string]interface{}, error) {
	var folders = make([]string, 0)
	var files = make([]string, 0)
	rd, err := os.ReadDir(pathname)
	if err != nil {
		return map[string]interface{}{"folders": folders, "files": files}, err
	}
	for _, fi := range rd {
		if fi.IsDir() {
			folders = append(folders, fi.Name())
		} else {
			files = append(files, fi.Name())
		}
	}
	return map[string]interface{}{"folders": folders, "files": files}, nil
}

// 读取Yaml配置文件， struct结构
func GetYamlConfigData(path, name string) (interface{}, error) {
	var config interface{}
	vip := viper.New()
	vip.AddConfigPath(path)   //设置读取的文件路径
	vip.SetConfigName(name)   //设置读取的文件名
	vip.SetConfigType("yaml") //设置文件的类型
	//尝试进行配置读取
	if err := vip.ReadInConfig(); err != nil {
		return nil, err
	}
	verr := vip.Unmarshal(&config)
	if verr != nil {
		return nil, verr
	}
	return config, nil
}

// 下载文件到指定目录 url下载地址 downdir 下载到位置及文件名称
func DownFileToDir(url, downdir string) (bool, string) {
	// 获取网络文件的数据流
	resp, err := http.Get(url)
	if err != nil {
		// 处理错误
		return false, ""
	}
	defer resp.Body.Close()
	// 读取数据流到内存中
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, ""
	}
	// 将文件写入到指定目录
	err = os.WriteFile(downdir, data, 0644)
	if err != nil {
		return false, ""
	}
	return true, downdir
}

// 判断文件夹下是否存在目录，存在返回：true
func GetDirHasefolder(pathname string) (bool, error) {
	var folders bool = false
	rd, err := os.ReadDir(pathname)
	if err != nil {
		return folders, err
	}
	for _, fi := range rd {
		if fi.IsDir() {
			folders = true
		}
	}
	return folders, nil
}

// 判断文件夹是否为空，为空时返回：true
func IsDirEmpty(dir string) bool {
	// 打开目录
	f, err := os.Open(dir)
	if err != nil {
		return true // 打不开、不存在、无权限 → 都算空
	}
	defer f.Close()
	_, err = f.Readdir(1)
	if err == nil {
		return false // 能读到内容 = 非空
	}
	// 读到 EOF = 真正的空目录
	return err == io.EOF
}
