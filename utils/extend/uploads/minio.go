package uploads

import (
	"context"
	"errors"
	"fmt"
	"gofly/utils/gf"
	"mime/multipart"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinIO 对象存储
type MinIO struct {
	Client *minio.Client
	Ready  bool
	Config Config
}

func (m *MinIO) InitClient(config Config) {
	m.Ready = false
	m.Config = config
	endpoint := strings.TrimSpace(config.Endpoint)
	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "https://")
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.KeyId, config.Secret, ""),
		Secure: config.UseHTTPS,
	})
	if err != nil {
		return
	}
	m.Client = client
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	exists, err := m.Client.BucketExists(ctx, config.BucketName)
	if err != nil {
		return
	}
	if !exists {
		if err := m.Client.MakeBucket(ctx, config.BucketName, minio.MakeBucketOptions{}); err != nil {
			return
		}
	}
	m.Ready = true
}

func (m *MinIO) UploadFile(c *gf.GinCtx, file *multipart.FileHeader) (url, cover_url string, err error) {
	if !m.Ready {
		err = errors.New("not ready")
		return
	}
	if err = VerifyAllowedExt(file); err != nil {
		return
	}
	fd, err := file.Open()
	if err != nil {
		return
	}
	defer fd.Close()
	objectName := strings.Trim(fmt.Sprintf("%v/%v/%v", m.Config.DirPath, time.Now().Format("20060102"), MakeFileName(file, "minio")), "/")
	url = objectName
	_, err = m.Client.PutObject(c.Request.Context(), m.Config.BucketName, objectName, fd, file.Size, minio.PutObjectOptions{ContentType: file.Header.Get("Content-Type")})
	return
}

func (m *MinIO) DownloadFile(fileUrl string) error {
	if !m.Ready {
		return errors.New("not ready")
	}
	localFileUrl, cloudFileUrl := InitFileUrl(fileUrl, m.Config)
	return m.Client.FGetObject(context.Background(), m.Config.BucketName, cloudFileUrl, localFileUrl, minio.GetObjectOptions{})
}

func (m *MinIO) RemoveFile(fileUrl string) error {
	if !m.Ready {
		return errors.New("not ready")
	}
	_, cloudFileUrl := InitFileUrl(fileUrl, m.Config)
	return m.Client.RemoveObject(context.Background(), m.Config.BucketName, cloudFileUrl, minio.RemoveObjectOptions{})
}

func (m *MinIO) MoveFile(fileUrl string, targerDir string) (string, error) {
	if !m.Ready {
		return "", errors.New("not ready")
	}
	_, cloudFileUrl := InitFileUrl(fileUrl, m.Config)
	targerUrl := path.Join(targerDir, filepath.Base(cloudFileUrl))
	_, cloudTargetUrl := InitFileUrl(targerUrl, m.Config)
	src := minio.CopySrcOptions{Bucket: m.Config.BucketName, Object: cloudFileUrl}
	dst := minio.CopyDestOptions{Bucket: m.Config.BucketName, Object: cloudTargetUrl}
	if _, err := m.Client.CopyObject(context.Background(), dst, src); err != nil {
		return "", err
	}
	if err := m.Client.RemoveObject(context.Background(), m.Config.BucketName, cloudFileUrl, minio.RemoveObjectOptions{}); err != nil {
		return "", err
	}
	return cloudTargetUrl, nil
}
