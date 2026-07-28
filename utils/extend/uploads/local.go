package uploads

import (
	"fmt"
	"gofly/utils/gf"
	"mime/multipart"
	"os"
	"path"
	"path/filepath"
	"time"
)

// Local 实现 StaticCloud 接口
type Local struct {
	Config Config
}

func (m *Local) InitClient(config Config) {
	m.Config = config
}

// 上传文件 返回url文件路径，cover_url封面（有返回）
func (m *Local) UploadFile(c *gf.GinCtx, file *multipart.FileHeader) (url, cover_url string, err error) {
	err = VerifyAllowedExt(file)
	if err != nil {
		return
	}
	path, _ := os.Getwd()
	nowDate := time.Now().Format("20060102")
	filename := MakeFileName(file, "local") //重新命名文件名称
	url = fmt.Sprintf("%v%v/%v", m.Config.DirPath, nowDate, filename)
	save_path_name := filepath.Join(path, m.Config.DirPath, nowDate, filename)
	err = c.SaveUploadedFile(file, save_path_name)
	return
}

// 下载文件(本地不需要下载)
func (m *Local) DownloadFile(fileUrl string) error {
	return nil
}

// 删除本地文件
func (m *Local) RemoveFile(fileUrl string) error {
	return gf.DelOneFile(fileUrl)
}

// 移动文件
func (m *Local) MoveFile(fileUrl string, targerDir string) (string, error) {
	fileUrl, _ = InitFileUrl(fileUrl, m.Config)
	targerUrl := path.Join(targerDir, filepath.Base(fileUrl))
	targerUrl, _ = InitFileUrl(targerUrl, m.Config)
	err := MoveFile(fileUrl, targerUrl)
	if err != nil {
		return "", err
	}
	return targerUrl, nil
}
