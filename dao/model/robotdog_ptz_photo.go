package model

import (
	"time"

	"gorm.io/gorm"
)

const TableNameRobotdogPtzPhoto = "gf_robotdog_ptz_photo"

// RobotdogPtzPhoto 云台拍照记录
type RobotdogPtzPhoto struct {
	ID         int64          `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`
	TenantID   int32          `gorm:"column:tenant_id;not null;default:1;comment:租户唯一标识" json:"tenant_id"`
	WaypointID int64          `gorm:"column:waypoint_id;not null;comment:特殊航点ID" json:"waypoint_id"`
	PtzID      int64          `gorm:"column:ptz_id;not null;comment:云台ID" json:"ptz_id"`
	Filename   string         `gorm:"column:filename;not null;comment:文件名" json:"filename"`
	FilePath   string         `gorm:"column:file_path;not null;comment:照片路径" json:"file_path"`
	Mode       string         `gorm:"column:mode;not null;comment:拍照模式" json:"mode"`
	RawData    string         `gorm:"column:raw_data;comment:发送结果JSON" json:"raw_data"`
	CreatedAt  time.Time      `gorm:"column:created_at;comment:创建时间" json:"created_at"`
	UpdatedAt  time.Time      `gorm:"column:updated_at;comment:更新时间" json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"column:deleted_at;comment:删除时间" json:"deleted_at"`
}

func (*RobotdogPtzPhoto) TableName() string {
	return TableNameRobotdogPtzPhoto
}
