package datacenter

import (
	"gofly/dao"
	"gofly/dao/model"
	"gofly/utils/gf"
	"gofly/utils/tools/gconv"
)

type Filemanage struct{}

func init() {
	gf.Register(&Filemanage{})
}

// 获取文件数据列表
func (api *Filemanage) GetList(ctx *gf.GinCtx) {
	attachmentDB := dao.Query().Attachment
	pageNo := gconv.Int(ctx.DefaultQuery("page", "1"))
	pageSize := gconv.Int(ctx.DefaultQuery("pageSize", "10"))
	//组合搜索条件
	param, _ := gf.RequestParam(ctx)
	var whereMap []dao.Condition
	whereMap = append(whereMap, attachmentDB.TenantID.Eq(ctx.GetInt32("tenant_id")))
	whereMap = append(whereMap, attachmentDB.Type.Neq(1))
	if title, ok := param["title"]; ok && title != "" {
		whereMap = append(whereMap, attachmentDB.Title.Like("%"+gconv.String(title)+"%"))
	}
	if typestr, ok := param["type"]; ok && typestr != "all" {
		whereMap = append(whereMap, attachmentDB.Type.Eq(gf.Int8(typestr)))
	}
	if createtime, ok := param["createtime"]; ok && createtime != "" {
		datetime_arr := gf.SplitAndStr(gf.String(createtime), ",")
		whereMap = append(whereMap, attachmentDB.CreatedAt.Between(gf.StrToTime(datetime_arr[0]), gf.StrToTime(datetime_arr[1])))
	}
	mDB := attachmentDB.WithContext(ctx).Where(whereMap...).
		Select(attachmentDB.ID, attachmentDB.Pid, attachmentDB.URL, attachmentDB.Type, attachmentDB.Name, attachmentDB.Title, attachmentDB.Mimetype, attachmentDB.CoverURL, attachmentDB.Filesize, attachmentDB.CreatedAt)
	if gf.String(param["sort"]) == "desc" {
		mDB = mDB.Order(attachmentDB.ID.Desc())
	} else {
		mDB = mDB.Order(attachmentDB.ID.Asc())
	}
	list, totalCount, err :=
		mDB.FindByPage(dao.Offset(pageNo, pageSize), pageSize)
	if err != nil {
		gf.Failed().SetMsg("获取文件失败").SetData(err).Regin(ctx)
		return
	}
	if list == nil {
		list = make([]*model.Attachment, 0)
	}
	gf.GetFullUrl("/resource/uploads/20251230/localdfbm180mksygzeamo5.jpg")
	gf.Success().SetMsg("获取文件数据列表").SetData(gf.Map{
		"page":     pageNo,
		"pageSize": pageSize,
		"total":    totalCount,
		"items":    list}).Regin(ctx)
}

// 文件管理信息
func (api *Filemanage) GetFileInfo(ctx *gf.GinCtx) {
	attachmentDB := dao.Query().Attachment
	allnumber, _ := attachmentDB.WithContext(ctx).Where(attachmentDB.Type.Neq(1)).Count()
	var useSize float64 = 0
	attachmentDB.WithContext(ctx).Where(attachmentDB.Type.Neq(1)).Select(attachmentDB.Filesize.Sum()).Scan(&useSize)
	//各类型文件占比
	var imageSize float64 = 0
	var fielSize float64 = 0
	var videoSize float64 = 0
	var audioSize float64 = 0
	var otherSize float64 = 0
	attachmentDB.WithContext(ctx).Where(attachmentDB.Type.Eq(0)).Select(attachmentDB.Filesize.Sum()).Scan(&imageSize)
	attachmentDB.WithContext(ctx).Where(attachmentDB.Type.Eq(4)).Select(attachmentDB.Filesize.Sum()).Scan(&fielSize)
	attachmentDB.WithContext(ctx).Where(attachmentDB.Type.Eq(2)).Select(attachmentDB.Filesize.Sum()).Scan(&videoSize)
	attachmentDB.WithContext(ctx).Where(attachmentDB.Type.Eq(3)).Select(attachmentDB.Filesize.Sum()).Scan(&audioSize)
	attachmentDB.WithContext(ctx).Where(attachmentDB.Type.Eq(5)).Select(attachmentDB.Filesize.Sum()).Scan(&otherSize)
	datainfo := map[string]interface{}{"allnumber": allnumber, "useSize": useSize,
		"imageSize": imageSize, "fielSize": fielSize, "videoSize": videoSize, "audioSize": audioSize, "otherSize": otherSize}
	gf.Success().SetMsg("获取附件存储信息").SetData(datainfo).Regin(ctx)
}
