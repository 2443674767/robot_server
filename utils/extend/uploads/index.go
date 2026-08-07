// =====================
// 附件上传入口
// =====================
package uploads

import (
	"context"
	"errors"
	"mime/multipart"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"gofly/utils/gf"
	"gofly/utils/tools/gcfg"
	"gofly/utils/tools/gconv"
	"gofly/utils/tools/gfile"
	"gofly/utils/tools/grand"
)

var (
	ctx = context.Background()
)

// 配置
type Config struct {
	Endpoint       string //自定义服务请求的访问域名
	KeyId          string
	Secret         string
	BucketName     string //空间名称
	DestBucketName string //移动目标空间名称
	Region         string //所属地域
	LocalBaseUrl   string
	BaseUrl        string
	DirPath        string //上传文件路径
	UseHTTPS       bool   // 是否使用https域名进行资源管理
	Zone           int    //指定仓库内的存储区域，比如华东区域
}

type StaticCloud interface {
	InitClient(config Config)
	UploadFile(c *gf.GinCtx, file *multipart.FileHeader) (url, cover_url string, err error)
	DownloadFile(fileUrl string) error
	RemoveFile(fileUrl string) error
	MoveFile(fileUrl string, targerDir string) (string, error)
}

// 规划文件路径，localUrl=本地路径，cloudUrl=云路径
func InitFileUrl(fileUrl string, config Config) (localUrl string, cloudUrl string) {
	if fileUrl[0:2] == "./" {
		localUrl = fileUrl
		cloudUrl = fileUrl[2:]
	} else if fileUrl[0:1] == "/" {
		localUrl = "." + fileUrl
		cloudUrl = fileUrl[1:]
	} else if fileUrl[0:4] == "http" {
		cloudUrl = strings.ReplaceAll(fileUrl, config.BaseUrl, "")
		localUrl = "./" + cloudUrl
	} else {
		localUrl = fileUrl
		cloudUrl = fileUrl
	}
	return localUrl, cloudUrl
}

// 初始配置，如果传uptype(上传方式)，则直接示例传入上传方式，不传则从配置文件获取设置的上传方式
func New(uptype ...string) StaticCloud {
	var staticCloud StaticCloud
	var upTypeStr string = ""
	var config = Config{}
	if len(uptype) > 0 && uptype[0] != "" {
		upTypeStr = uptype[0]
	} else {
		confType, _ := gcfg.Instance("upload").Get(ctx, "Type")
		upTypeStr = confType.String()
	}
	switch upTypeStr {
	case "minio":
		minioConf, _ := gcfg.Instance("upload").Get(ctx, "minio")
		mapConf := gconv.Map(minioConf)
		endpoint := gf.String(mapConf["endpoint"])
		scheme := "http"
		if gf.Bool(mapConf["useSSL"]) {
			scheme = "https"
		}
		baseURL := endpoint
		if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
			baseURL = scheme + "://" + strings.TrimRight(endpoint, "/") + "/" + gf.String(mapConf["bucket"])
		} else {
			baseURL = strings.TrimRight(baseURL, "/") + "/" + gf.String(mapConf["bucket"])
		}
		config = Config{
			BaseUrl:    baseURL,
			Endpoint:   endpoint,
			KeyId:      gf.String(mapConf["accessKey"]),
			Secret:     gf.String(mapConf["secretKey"]),
			BucketName: gf.String(mapConf["bucket"]),
			DirPath:    gf.String(mapConf["dirPath"]),
			UseHTTPS:   gf.Bool(mapConf["useSSL"]),
		}
		staticCloud = &MinIO{}
	case "alioss":
		alioss, _ := gcfg.Instance("upload").Get(ctx, "alioss")
		mapConf := gconv.Map(alioss)
		config = Config{
			BaseUrl:    gf.String(mapConf["ABaseUrl"]),
			Endpoint:   gf.String(mapConf["AEndpoint"]),
			KeyId:      gf.String(mapConf["AKeyId"]),
			Secret:     gf.String(mapConf["ASecret"]),
			BucketName: gf.String(mapConf["ABucketName"]),
			DirPath:    gf.String(mapConf["ADirPath"]),
		}
		staticCloud = &AliOSS{}
	case "tencentcos":
		tencentcos, _ := gcfg.Instance("upload").Get(ctx, "tencentcos")
		mapConf := gconv.Map(tencentcos)
		config = Config{
			BaseUrl:    gf.String(mapConf["TBaseUrl"]),
			Endpoint:   gf.String(mapConf["TEndpoint"]),
			KeyId:      gf.String(mapConf["TKeyId"]),
			Secret:     gf.String(mapConf["TSecret"]),
			BucketName: gf.String(mapConf["TBucketName"]),
			Region:     gf.String(mapConf["TRegion"]),
			DirPath:    gf.String(mapConf["TDirPath"]),
		}
		staticCloud = &TencentCOS{}
	case "qiniuoss":
		qiniuoss, _ := gcfg.Instance("upload").Get(ctx, "qiniuoss")
		mapConf := gconv.Map(qiniuoss)
		config = Config{
			BaseUrl:    gf.String(mapConf["QBaseUrl"]),
			Endpoint:   gf.String(mapConf["QEndpoint"]),
			KeyId:      gf.String(mapConf["QKeyId"]),
			Secret:     gf.String(mapConf["QSecret"]),
			BucketName: gf.String(mapConf["QBucketName"]),
			DirPath:    gf.String(mapConf["QDirPath"]),
			UseHTTPS:   gf.Bool(mapConf["QUseHTTPS"]),
			Zone:       gf.Int(mapConf["QZone"]),
		}
		staticCloud = &QiniuOSS{}
	default:
		local, _ := gcfg.Instance("upload").Get(ctx, "local")
		mapConf := gconv.Map(local)
		config = Config{
			BaseUrl: gf.String(mapConf["LBaseUrl"]),
			DirPath: gf.String(mapConf["LDirPath"]),
		}
		staticCloud = &Local{}
	}
	staticCloud.InitClient(config)
	return staticCloud
}

func MoveFile(fileUrl string, targerUrl string) error {
	// 检查目标目录是否存在，不存在则创建
	targerDir := filepath.Dir(targerUrl)
	if _, err := os.Stat(targerDir); os.IsNotExist(err) {
		err := os.MkdirAll(targerDir, 0777)
		if err != nil {
			return err
		}
	}
	// 移动文件至目标文件夹
	return os.Rename(fileUrl, targerUrl)
}

// 上传云本地零时文件存储
// func MomentLocal(file *multipart.FileHeader) string {
// 	return ""
// }

// 删除附件统一入口，结果返回：bool
func DelFile(fileUrl string) error {
	//处理地址
	filseName := gfile.Name(fileUrl)
	uptype := ""
	if strings.HasPrefix(filseName, "local") { //本地存储
		uptype = "local"
	} else if strings.HasPrefix(filseName, "alioss") { //阿里云
		uptype = "alioss"
	} else if strings.HasPrefix(filseName, "tencentcos") { //腾讯云
		uptype = "tencentcos"
	} else if strings.HasPrefix(filseName, "qiniuoss") { //七牛云
		uptype = "qiniuoss"
	} else { //默认返回设置上传方式的地址
		UpType, _ := gcfg.Instance("upload").Get(ctx, "Type")
		uptype = UpType.String()
	}
	return New(uptype).RemoveFile(fileUrl)
}

// 生成文件名,ext文件名前缀
func MakeFileName(file *multipart.FileHeader, ext ...string) string {
	name := strings.ToLower(strconv.FormatInt(time.Now().UnixNano(), 36) + grand.String(6))
	extStr := ""
	if len(ext) > 0 && ext[0] != "" {
		extStr = ext[0]
	}
	return extStr + name + gfile.Ext(file.Filename)
}

// 验证允许文件格式
func VerifyAllowedExt(file *multipart.FileHeader) error {
	AllowedExt, _ := gcfg.Instance("upload").Get(ctx, "AllowedExt")
	AllowedExt_arr := strings.Split(AllowedExt.String(), ",")
	ext := strings.ToLower(path.Ext(file.Filename))
	if !slices.Contains(AllowedExt_arr, ext) {
		return errors.New("上传不支持" + ext + "的文件类型")
	}
	return nil
}
