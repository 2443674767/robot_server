package dao

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Dbins struct {
	confName string
}

// confName是数据库配置名 默认default
func Use(confName ...string) *Dbins {
	name := ""
	if len(confName) > 0 && confName[0] != "" {
		name = confName[0]
	}
	return &Dbins{
		confName: name,
	}
}

// TableName: Retrieves the table name with a prefix, table name, name database configuration name, default: "default"
// 获取带前缀的表名，table表名，name数据库配置名称，默认default
func (d *Dbins) TableName(table string) string {
	return DB(d.confName).Config.NamingStrategy.TableName(table)
}

// 原生 SQL 返回指定数据表、指定字段数组，类似Pluck
// tablename数据表、feild字段、查询字段key，查询值value
func (d *Dbins) Array(tablename, feild, key string, value interface{}) ([]interface{}, error) {
	var arr []interface{}
	err := DB(d.confName).Session(&gorm.Session{Logger: logger.Discard}).Raw("SELECT "+feild+" FROM "+d.TableName(tablename)+" WHERE "+key+" = ?", value).Scan(&arr).Error
	if err != nil {
		return nil, err
	}
	return arr, nil
}

// 原生 SQL 返回指定数据表、指定字段数组，类似Pluck
// tablename数据表、feild字段、查询字段key，查询值value是数组
func (d *Dbins) ArrayIn(tablename, feild, key string, value []interface{}) ([]interface{}, error) {
	var arr []interface{}
	err := DB(d.confName).Session(&gorm.Session{Logger: logger.Discard}).Raw("SELECT "+feild+" FROM "+d.TableName(tablename)+" WHERE "+key+" IN(?)", value).Scan(&arr).Error
	if err != nil {
		return nil, err
	}
	return arr, nil
}

// 获取指定字段最大值
// tablename数据表、feild字段
func (d *Dbins) MaxVal(tablename, feild string) (int64, error) {
	var val int64
	err := DB(d.confName).Session(&gorm.Session{Logger: logger.Discard}).Raw("SELECT MAX(" + feild + ") FROM " + d.TableName(tablename)).Scan(&val).Error
	if err != nil {
		return 0, err
	}
	return val, nil
}

// 获取指定字段（默认id）最早一条某个字段值
// tablename数据表、maxFeild最小值字段、getFeild获取值字段
func (d *Dbins) MaxValByAsc(tablename, getFeild string, maxFeild ...string) (interface{}, error) {
	idfeild := "id"
	if len(maxFeild) > 0 && maxFeild[0] != "" {
		idfeild = maxFeild[0]
	}
	var val map[string]interface{}
	err := DB(d.confName).Session(&gorm.Session{Logger: logger.Discard}).Raw("SELECT " + getFeild + " FROM " + d.TableName(tablename) + " ORDER BY " + idfeild + " ASC LIMIT 1;").Scan(&val).Error
	if err != nil {
		return nil, err
	}
	return val[getFeild], nil
}

// TableIsExist 判断数据表是否存在
// tableName: 表名，不带库名
func (d *Dbins) TableIsExist(ctx context.Context, tableName string) (bool, error) {
	var count int64
	// dao.DB() 是GORM Gen全局DB实例
	err := DB(d.confName).WithContext(ctx).Raw(`
		SELECT COUNT(*) FROM information_schema.tables 
		WHERE table_schema = DATABASE() AND table_name = ?
	`, tableName).Scan(&count).Error

	if err != nil {
		return false, err
	}
	return count > 0, nil
}
