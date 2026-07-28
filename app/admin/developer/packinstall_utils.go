package developer

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"gofly/dao"
	"gofly/dao/model"
	"gofly/utils/gf"
	"gofly/utils/tools/gfile"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// UpFileClient .上传附件到社区
func UpFileClient(ctx *gf.GinCtx, params map[string]string) ([]byte, error) {
	domainurl := ctx.DefaultPostForm("domainurl", "")
	file, err := ctx.FormFile("file")
	if err != nil {
		return nil, errors.New("获取数据失败，")
	}
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	// 其他参数列表写入 body
	for k, v := range params {
		if err := writer.WriteField(k, v); err != nil {
			return nil, err
		}
	}
	// 一个是输入表单的 name，一个上传的文件名称
	uploadWriter, _ := writer.CreateFormFile("file", file.Filename)
	uploadFile, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer uploadFile.Close()
	_, err = io.Copy(uploadWriter, uploadFile)
	if err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	resp, err := http.Post(domainurl+"/goflycode/upfile/codeFile",
		writer.FormDataContentType(),
		body,
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	content, err := io.ReadAll(resp.Body)
	return content, err
}

// 导入菜单数据、并返回插入数据id
func Insertmenu(ctx *gf.GinCtx, data interface{}, pid interface{}) []int32 {
	userID := ctx.GetInt64("uid")
	ruleDB := dao.Query().AdminAuthRule
	var menuids = make([]int32, 0)
	for _, menuitem := range data.([]interface{}) {
		menuitem_obj := menuitem.(map[string]interface{})
		menuitem_obj["pid"] = pid
		menuitem_obj["uid"] = userID
		menuitem_obj["created_at"] = time.Now()
		delete(menuitem_obj, "id")
		var parent_id *int32
		ruleDB.WithContext(ctx).Where(ruleDB.Pid.Eq(0), ruleDB.Routename.Eq(gf.String(menuitem_obj["routename"]))).Select(ruleDB.ID).Scan(&parent_id)
		if parent_id == nil {
			if subdata, ok := menuitem_obj["children"]; ok {
				delete(menuitem_obj, "children")
				ruleDB.WithContext(ctx).UnderlyingDB().Model(&model.AdminAuthRule{}).Create(&menuitem_obj)
				ruleDB.WithContext(ctx).Where(ruleDB.ID.Eq(gf.Int32(menuitem_obj["id"]))).Update(ruleDB.Weigh, menuitem_obj["id"])
				menuids = append(menuids, gf.Int32(menuitem_obj["id"]))
				if !gf.IsEmpty(subdata) {
					m_menuids := Insertmenu(ctx, subdata, menuitem_obj["id"])
					menuids = append(menuids, m_menuids...)
				}
			} else {
				var hase_nemuid *int32
				ruleDB.WithContext(ctx).Where(ruleDB.Routepath.Neq(""), ruleDB.Routepath.Eq(gf.String(menuitem_obj["routepath"])), ruleDB.Routename.Neq(""), ruleDB.Routename.Eq(gf.String(menuitem_obj["routename"]))).Or(ruleDB.Type.Eq(2), ruleDB.Path.Neq(""), ruleDB.Path.Eq(gf.String(menuitem_obj["path"]))).Select(ruleDB.ID).Scan(&hase_nemuid)
				if hase_nemuid == nil {
					ruleDB.WithContext(ctx).UnderlyingDB().Model(&model.AdminAuthRule{}).Create(&menuitem_obj)
					ruleDB.WithContext(ctx).Where(ruleDB.ID.Eq(gf.Int32(menuitem_obj["id"]))).Update(ruleDB.Weigh, menuitem_obj["id"])
					menuids = append(menuids, gf.Int32(menuitem_obj["id"]))
				} else {
					menuids = append(menuids, *hase_nemuid)
				}
			}
		} else {
			if subdata, ok := menuitem_obj["children"]; ok {
				m_menuids := Insertmenu(ctx, subdata, *parent_id)
				menuids = append(menuids, m_menuids...)
			}
		}
	}
	return menuids
}

// =====================================控制器添加和删除============================================================
// 检查该类是否添加到控制器，参数：modelname控制器模块名、path添加模块、haseMoleCtr模块是否存在控制器
func CheckIsAddController(modelname, path string, haseMoleCtr bool) error {
	filePath := filepath.Join("app/", modelname, "/controller.go")
	//1判断文件没有则添加
	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			if modelname == "" {
				return errors.New("app下的controller.go文件不存在")
			}
			//模块控制器没可以自动创建
			os.Create(filePath)
			//复制文件
			err := gfile.CopyFile("devsource/developer/codetpl/go/controller.gos", filePath)
			if err != nil {
				return err
			}
		}
	}
	con_path := "gofly/app/" + path
	//打开controller.go控制文件
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()
	buf := bufio.NewReader(f)
	var result = ""
	ishase := false
	for {
		a, _, c := buf.ReadLine()
		if c == io.EOF {
			break
		}
		//对非根目录控制器文件包名进行替换成改模块名字
		if strings.Contains(string(a), "package controller") && modelname != "" {
			datestr := strings.ReplaceAll(string(a), "package controller", "package "+modelname)
			result += datestr + "\n"
		} else {
			result += string(a) + "\n"
		}
		//判断控制器内容是否存在要引入的模块
		if strings.Contains(string(a), con_path) {
			ishase = true
		}
	}
	if !ishase {
		if modelname == "" && haseMoleCtr { //app根控制器器模块下存在控制器-根控制器
			//1.引入模块控制器
			addstr := "	\"" + con_path + "\"\n"
			addstr += ")"
			result = strings.Replace(result, ")", addstr, 1)
			//2添加路由钩子
			addrouterstr := fmt.Sprintf("	%v.RouterHandler(ctx, \"%v\")\n", path, path)
			addrouterstr += "}"
			result = strings.Replace(result, "}", addrouterstr, 1)
		} else { //处理模块控制器
			addstr := "	_ \"" + con_path + "\"\n"
			addstr += ")"
			result = strings.Replace(result, ")", addstr, 1)
		}
	}

	fw, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666) //os.O_TRUNC清空文件重新写入，否则原文件内容可能残留
	w := bufio.NewWriter(fw)
	w.WriteString(result)
	if err != nil {
		return err
	}
	w.Flush()
	fw.Close()
	return nil
}

// 存在控制器则移除
func CheckApiRemoveController(modelname, path string) {
	filePath := filepath.Join("app/", modelname, "/controller.go")
	if _, err := os.Stat(filePath); os.IsNotExist(err) { //不存在
		return
	}
	con_path := "gofly/app/" + path
	f, err := os.Open(filePath)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	buf := bufio.NewReader(f)
	var result = ""
	for {
		a, _, c := buf.ReadLine()
		if c == io.EOF {
			break
		}
		if strings.Contains(string(a), con_path) || strings.Contains(string(a), fmt.Sprintf("%v.RouterHandler(ctx, \"%v\")", path, path)) { //存在路由则移除
			continue
		} else {
			result += string(a) + "\n"
		}
	}
	fw, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666) //os.O_TRUNC清空文件重新写入，否则原文件内容可能残留
	w := bufio.NewWriter(fw)
	w.WriteString(result)
	if err != nil {
		panic(err)
	}
	w.Flush()
	fw.Close()
}

// 获取文文件夹下的文件及文件goApp
func GetAllFileApp(pathname string) (string, []string, error) {
	rd, err := os.ReadDir(pathname)
	var folders = make([]string, 0)
	if err != nil {
		return "", folders, err
	}
	for _, fi := range rd {
		if fi.IsDir() {
			fullDir := pathname + "/" + fi.Name()
			sec_rd, _ := os.ReadDir(fullDir)
			if len(sec_rd) > 0 {
				hase_dir := false
				for _, sec_fi := range sec_rd {
					if sec_fi.IsDir() {
						folders = append(folders, fi.Name()+"/"+sec_fi.Name())
						hase_dir = true
					}
				}
				if hase_dir == false {
					folders = append(folders, fi.Name())
				}
			} else {
				folders = append(folders, fi.Name())
			}
		}
	}
	return strings.Join(folders, ","), folders, nil
}
