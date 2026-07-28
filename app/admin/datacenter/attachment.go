// ====================
// 文件管理
// 功能：后台文件上传、删除、编写、管理
// ====================
package datacenter

import (
	"fmt"
	"gofly/dao"
	"gofly/dao/model"
	"gofly/utils/extend/uploads"
	"gofly/utils/gf"
	"gofly/utils/tools/gconv"
	"gofly/utils/tools/gfile"
	"path/filepath"
)

type Attachment struct{}

func init() {
	gf.Register(&Attachment{})
}

// 获取我的附件
func (api *Attachment) GetMyFiles(ctx *gf.GinCtx) {
	attachmentDB := dao.Query().Attachment
	//组合搜索条件
	param, _ := gf.RequestParam(ctx)
	Pid := gf.Int64(param["pid"])
	var whereMap []dao.Condition
	whereMap = append(whereMap, attachmentDB.TenantID.Eq(ctx.GetInt32("tenant_id")))
	whereMap = append(whereMap, attachmentDB.Pid.Eq(Pid))
	whereMap = append(whereMap, attachmentDB.IsCommon.Eq(0))
	if searchword, ok := param["searchword"]; ok && searchword != "" {
		whereMap = append(whereMap, attachmentDB.Title.Like("%"+gconv.String(searchword)+"%"))
	}
	if typestr, ok := param["type"]; ok && typestr != "all" {
		whereMap = append(whereMap, attachmentDB.Type.Eq(gf.Int8(typestr)))
	}

	switch gf.String(param["filetype"]) {
	case "video":
		whereMap = append(whereMap, attachmentDB.Type.In(1, 2))
	case "audio":
		whereMap = append(whereMap, attachmentDB.Type.In(1, 3))
	case "file":
		whereMap = append(whereMap, attachmentDB.Type.In(1, 4))
	default:
		whereMap = append(whereMap, attachmentDB.Type.In(0, 1))
	}
	var list gf.List
	err := attachmentDB.WithContext(ctx).Where(whereMap...).Select(attachmentDB.ID, attachmentDB.Pid, attachmentDB.Name, attachmentDB.Title, attachmentDB.Type, attachmentDB.URL, attachmentDB.Filesize, attachmentDB.Mimetype, attachmentDB.Storage, attachmentDB.CoverURL, attachmentDB.IsCommon).Order(attachmentDB.Type.Desc(), attachmentDB.Weigh.Desc(), attachmentDB.ID.Desc()).Scan(&list)
	if err != nil {
		gf.Failed().SetMsg("加载数据失败").SetData(err).Regin(ctx)
		return
	}
	for _, val := range list {
		if _, ok := val["cover_url"]; ok && !gf.IsEmpty(val["cover_url"]) {
			val["cover_url"] = gf.GetFullUrl(gf.String(val["cover_url"]))
		}
	}
	//公共资源
	var common_list gf.List
	attachmentDB.WithContext(ctx).Where(attachmentDB.Pid.Eq(Pid), attachmentDB.IsCommon.Eq(1)).Select(attachmentDB.ID, attachmentDB.Pid, attachmentDB.Name, attachmentDB.Title, attachmentDB.Type, attachmentDB.URL, attachmentDB.Filesize, attachmentDB.Mimetype, attachmentDB.Storage, attachmentDB.CoverURL, attachmentDB.IsCommon).Order(attachmentDB.Type.Desc(), attachmentDB.Weigh.Desc(), attachmentDB.ID.Desc()).Scan(&common_list)
	if list != nil {
		list = append(common_list, list...)
	} else {
		list = common_list
	}
	var totalCount int64
	//获取目录
	allids := getAllParentIds(ctx, Pid)
	allids = append(allids, Pid)
	var dirmenu gf.List
	attachmentDB.WithContext(ctx).Where(attachmentDB.ID.In(allids...)).Select(attachmentDB.ID, attachmentDB.Pid, attachmentDB.Title).Scan(&dirmenu)
	if gf.IsEmpty(dirmenu) {
		dirmenu = make(gf.List, 0)
	}
	//判断是否存在素材库
	resdata := gf.Map{
		"total":       totalCount,
		"dirmenu":     dirmenu,
		"allids":      allids,
		"items":       list,
		"hasegallery": gfile.Exists(filepath.Join(gf.ROOTPATH, "/resource/codeinstall/tenant")), //是否存在图库
	}
	gf.Success().SetMsg("获取附件列表").SetData(resdata).Regin(ctx)
}

// 获取首页文件夹全部id
func getAllParentIds(ctx *gf.GinCtx, id int64) []int64 {
	var parent_ids []int64
	var parent_id int64
	dao.Query().Attachment.WithContext(ctx).Where(dao.Query().Attachment.ID.Eq(id)).Select(dao.Query().Attachment.Pid).Scan(&parent_id)
	if !gf.IsEmpty(parent_id) {
		parent_ids = append(parent_ids, parent_id)
		parent_ids = append(parent_ids, getAllParentIds(ctx, parent_id)...)
	}
	return parent_ids
}

// 创建新文件夹
func (api *Attachment) Save(ctx *gf.GinCtx) {
	attachmentDB := dao.Query().Attachment
	param, _ := gf.RequestParam(ctx)
	if gf.Int64(param["id"]) == gf.ZeroInt64 {
		Pid := gf.Int64(param["pid"])
		fileCount, _ := attachmentDB.WithContext(ctx).Where(attachmentDB.Pid.Eq(Pid), attachmentDB.Title.Like(fmt.Sprintf("%s%v%s", "%", param["title"], "%"))).Count()
		title := fmt.Sprintf("%s%v", param["title"], fileCount+1)
		attachment := model.Attachment{Pid: Pid, Title: title, Type: 1}
		err := attachmentDB.WithContext(ctx).Create(&attachment)
		if err != nil {
			gf.Failed().SetMsg("添加失败").SetData(err).Regin(ctx)
		} else {
			attachmentDB.WithContext(ctx).Where(attachmentDB.ID.Eq(attachment.ID)).Update(attachmentDB.Weigh, attachment.ID)
			var getdata gf.List
			attachmentDB.WithContext(ctx).Where(attachmentDB.ID.Eq(attachment.ID)).Select(attachmentDB.ID, attachmentDB.Pid, attachmentDB.Name, attachmentDB.Title, attachmentDB.Type, attachmentDB.URL, attachmentDB.Filesize, attachmentDB.Mimetype, attachmentDB.Storage).Scan(&getdata)
			gf.Success().SetMsg("添加成功").SetData(getdata).Regin(ctx)
		}
	} else { //更新文件名称
		result, err := attachmentDB.WithContext(ctx).Where(attachmentDB.ID.Eq(gf.Int64(param["id"]))).Update(attachmentDB.Title, param["title"])
		if err != nil {
			gf.Failed().SetMsg("更新失败").SetData(err).Regin(ctx)
		} else {
			gf.Success().SetMsg("更新成功！").SetData(result).Regin(ctx)
		}
	}
}

// 删除文件夹
func (api *Attachment) DelDir(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	attachmentDB := dao.Query().Attachment
	res, err := attachmentDB.WithContext(ctx).Where(attachmentDB.ID.In(gf.Int64(param["id"]))).Delete()
	if err != nil {
		gf.Failed().SetMsg("删除失败").SetData(err).Regin(ctx)
	} else {
		delFileCycle(ctx, gf.Int64(param["id"])) //删除文件夹内容文件
		gf.Success().SetMsg("文件夹成功！").SetData(res).Regin(ctx)
	}
}

// 循环删除文件夹及文件
func delFileCycle(ctx *gf.GinCtx, id int64) {
	attachmentDB := dao.Query().Attachment
	filedata, _ := attachmentDB.WithContext(ctx).Where(attachmentDB.Pid.Eq(id)).Select(attachmentDB.ID, attachmentDB.Type).Find()
	if len(filedata) > 0 {
		for _, val := range filedata {
			delFileCycle(ctx, val.ID)
			if val.Type == 0 {
				var file_path string
				attachmentDB.WithContext(ctx).Where(attachmentDB.ID.In(val.ID)).Select(attachmentDB.URL).Scan(&file_path)
				if !gf.IsEmpty(file_path) {
					gf.DelOneFile(file_path)
				}
			}
			attachmentDB.WithContext(ctx).Where(attachmentDB.ID.In(val.ID)).Delete()
		}
	}
}

// 删除文件
func (api *Attachment) Del(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	attachmentDB := dao.Query().Attachment
	fileList, _ := attachmentDB.WithContext(ctx).Where(attachmentDB.ID.In(gf.InterfaceToInt64(param["ids"])...)).Select(attachmentDB.ID, attachmentDB.Location, attachmentDB.URL).Find()
	_, err := attachmentDB.WithContext(ctx).Where(attachmentDB.ID.In(gf.InterfaceToInt64(param["ids"])...)).Delete()
	if err != nil {
		gf.Failed().SetMsg("删除失败").Regin(ctx)
	} else {
		for _, val := range fileList {
			uploads.DelFile(val.URL)
		}
		gf.Success().SetMsg("删除成功！").SetData(true).Regin(ctx)
	}
}

// 移动文件到文件夹
func (api *Attachment) UpImgPid(ctx *gf.GinCtx) {
	param, _ := gf.RequestParam(ctx)
	attachmentDB := dao.Query().Attachment
	result, err := attachmentDB.WithContext(ctx).Where(attachmentDB.ID.Eq(gf.Int64(param["imgid"]))).Update(attachmentDB.Pid, param["pid"])
	if err != nil {
		gf.Failed().SetMsg("更新失败").SetData(err).Regin(ctx)
	} else {
		gf.Success().SetMsg("更新成功！").SetData(result).Regin(ctx)
	}
}
