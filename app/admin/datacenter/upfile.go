package datacenter

import (
	"crypto/md5"
	"encoding/hex"
	"gofly/dao"
	"gofly/dao/model"
	"gofly/utils/extend/uploads"
	"gofly/utils/gf"
	"gofly/utils/tools/gcfg"
	"path"
	"slices"
	"strings"
)

type Upfile struct{}

func init() {
	gf.Register(&Upfile{})
}

// 管理后台上传附件接口
func (api *Upfile) Upload(ctx *gf.GinCtx) {
	Pid := gf.Int64(ctx.DefaultPostForm("pid", "0"))
	fileType := ctx.DefaultPostForm("filetype", "image") //文件类型
	file, err := ctx.FormFile("file")
	if err != nil {
		gf.Failed().SetMsg("获取数据失败，").SetData(err).Regin(ctx)
		return
	}
	attachmentDB := dao.Query().Attachment
	AllowedExt, _ := gcfg.Instance("upload").Get(ctx, "AllowedExt")
	AllowedExt_arr := strings.Split(AllowedExt.String(), ",")
	ext := strings.ToLower(path.Ext(file.Filename))
	if !slices.Contains(AllowedExt_arr, ext) {
		gf.Failed().SetMsg("上传不支持" + ext + "的文件类型").SetExdata(AllowedExt_arr).Regin(ctx)
		return
	}
	//判断文件是否已经传过
	fileContent, _ := file.Open()
	defer fileContent.Close()
	var byteContainer []byte = make([]byte, 1000000)
	fileContent.Read(byteContainer)
	m_d5 := md5.New()
	m_d5.Write(byteContainer)
	sha1_str := hex.EncodeToString(m_d5.Sum(nil))
	//查找该用户是否传过
	attachment, _ := attachmentDB.WithContext(ctx).Where(attachmentDB.Sha1.Eq(sha1_str)).Select(attachmentDB.ID, attachmentDB.Pid, attachmentDB.Name, attachmentDB.Title, attachmentDB.Type, attachmentDB.URL, attachmentDB.Filesize, attachmentDB.Mimetype, attachmentDB.CoverURL.As("cover")).First()
	if attachment != nil { //文件经存在，则直接返回
		//更新排序到最前面
		var maxId int64
		attachmentDB.WithContext(ctx).Order(attachmentDB.Weigh.Desc()).Select(attachmentDB.ID).Scan(&maxId)
		if maxId > 0 {
			attachmentDB.WithContext(ctx).Where(attachmentDB.ID.Eq(attachment.ID)).UpdateSimple(attachmentDB.Weigh.Value(maxId+1), attachmentDB.Pid.Value(Pid))
		}
		gf.Success().SetMsg("文件已上传").SetData(attachment).Regin(ctx)
		return
	}
	//文件类型
	var ftype int8 = 0
	switch fileType {
	case "video": //视频
		ftype = 2
	case "audio": //音频
		ftype = 3
	case "file": //附件类
		ftype = 4
	case "image": //图片
		ftype = 0
	default:
		ftype = 5
	}
	filename_arr := strings.Split(file.Filename, ".")
	location, _ := gcfg.Instance("upload").Get(ctx, "Type")
	//处理文件上传，bin返回地址
	url, cover_url, err := uploads.New().UploadFile(ctx, file)
	if err != nil {
		gf.Failed().SetMsg("上传文件失败").SetData(err).Regin(ctx)
		return
	}
	fileData := model.Attachment{
		TenantID: ctx.GetInt32("tenant_id"),
		Type:     ftype,
		Location: location.String(), //存储位置
		Pid:      Pid,
		Sha1:     sha1_str, //文件唯一指纹
		Name:     file.Filename,
		Title:    filename_arr[0],
		URL:      url,       //附件路径
		CoverURL: cover_url, //封面
		Filesize: file.Size,
		Mimetype: file.Header["Content-Type"][0],
	}
	//保存到数据库
	err = attachmentDB.WithContext(ctx).Create(&fileData)
	if err != nil {
		gf.Failed().SetMsg("保存上传数据失败").SetData(err).Regin(ctx)
		return
	}
	//处理预览url地址
	fileData.URL = gf.GetFullUrl(url)
	gf.Success().SetMsg("文件上传成功").SetData(fileData).Regin(ctx)
}
