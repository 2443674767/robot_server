package excelexport

import (
	"bytes"
	"context"
	"fmt"

	"github.com/360EntSecGroup-Skylar/excelize"
	"github.com/gin-gonic/gin"

	"gofly/dao"
	"gofly/utils/tools/gcfg"
	"gofly/utils/tools/gconv"
	"log"
	"net/url"
	"strconv"
	"time"
)

var (
	appConf, _ = gcfg.Instance().Get(context.Background(), "database.default")
	dbConf_arr = gconv.Map(appConf)
)

type (
	List = []map[string]interface{}
)

// ExportToExcel 导出数据库记录为 Excel 文件流
// rows数据，columns导出表字段，tableName表名称
func ExportToExcel(rows List, columns []interface{}, tableName string, c *gin.Context) ([]byte, error) {
	// 创建 Excel 文件
	file := excelize.NewFile()
	sheet := "Sheet1"
	dbName := gconv.String(dbConf_arr["dbname"])
	// 获取列名和列注释
	var fields []string
	var columnComments []string
	if len(rows) > 0 {
		if len(columns) > 0 {
			// 按照查询结果的顺序存储列名和列注释
			for _, column := range columns {
				columndata := column.(map[string]interface{})
				colName := gconv.String(columndata["field"])
				colComment := gconv.String(columndata["title"])
				if colComment == "" {
					colComment = colName
				}
				fields = append(fields, colName)
				columnComments = append(columnComments, colComment)
			}
		} else {
			// 查询数据库获取字段注释
			query := "SELECT COLUMN_NAME, COLUMN_COMMENT FROM information_schema.columns WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? order by ordinal_position"
			var fieldData []map[string]any
			err := dao.DB().Raw(query, dbName, dao.Use().TableName(tableName)).Scan(&fieldData).Error
			if err != nil {
				return nil, fmt.Errorf("查询注释失败: %v", err)
			}

			// 按照查询结果的顺序存储列名和列注释
			for _, data := range fieldData {
				colName := gconv.String(data["COLUMN_NAME"])
				colComment := gconv.String(data["COLUMN_COMMENT"])
				if colComment == "" {
					colComment = colName
				}
				fields = append(fields, colName)
				columnComments = append(columnComments, colComment)
			}
		}

	} else {
		return nil, fmt.Errorf("没有数据")
	}

	// 写入表头（使用列注释作为表头）
	for i, col := range columnComments {
		file.SetCellValue(sheet, excelize.ToAlphaString(i)+"1", col)
	}

	// 写入数据
	rowIndex := 2
	for _, row := range rows {
		for i, col := range fields {
			val := row[col]
			file.SetCellValue(sheet, excelize.ToAlphaString(i)+strconv.Itoa(rowIndex), val)
		}
		rowIndex++
	}

	// 将 Excel 文件写入缓冲区
	var buf bytes.Buffer
	err := file.Write(&buf)

	if err != nil {
		log.Printf("将 Excel 文件写入缓冲区是出错： %v", err)
		return nil, err
	}

	fileName := url.QueryEscape(fmt.Sprintf("%s.xlsx", time.Now().Format("20060102150405"))) // 中文编码处理
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Access-Control-Expose-Headers", "Content-Disposition")
	c.Header("Content-Length", strconv.Itoa(len(buf.Bytes()))) // 添加文件大小标识
	file.Write(c.Writer)
	// 返回文件流
	return buf.Bytes(), nil
}
