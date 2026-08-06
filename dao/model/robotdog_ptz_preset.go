package model

import (
	"time"

	"gorm.io/gorm"
)

const TableNameRobotdogPtzPreset = "gf_robotdog_ptz_preset"

// RobotdogPtzPreset 云台预置位
type RobotdogPtzPreset struct {
	ID         int64          `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`
	TenantID   int32          `gorm:"column:tenant_id;not null;default:1;comment:租户唯一标识" json:"tenant_id"`
	WaypointID int64          `gorm:"column:waypoint_id;not null;comment:特殊航点ID" json:"waypoint_id"`
	PtzID      int64          `gorm:"column:ptz_id;not null;comment:云台ID" json:"ptz_id"`
	Name       string         `gorm:"column:name;not null;comment:预置位名称" json:"name"`
	SortNo     int32          `gorm:"column:sort_no;not null;default:1;comment:排序号" json:"sort_no"`
	ServoPhoto int8           `gorm:"column:servo_photo;not null;default:0;comment:伺服拍照是否" json:"servo_photo"`
	AutoHome   int8           `gorm:"column:auto_home;not null;default:0;comment:是否回正" json:"auto_home"`
	Pitch      float64        `gorm:"column:pitch;not null;default:0.00;comment:俯仰角" json:"pitch"`
	Yaw        float64        `gorm:"column:yaw;not null;default:0.00;comment:偏航角" json:"yaw"`
	Roll       float64        `gorm:"column:roll;not null;default:0.00;comment:横滚角" json:"roll"`
	Zoom       float64        `gorm:"column:zoom;not null;default:0.00;comment:变倍倍率" json:"zoom"`
	Focus      string         `gorm:"column:focus_status;not null;comment:对焦状态" json:"focus_status"`
	RawData    string         `gorm:"column:raw_data;comment:实时原始数据JSON" json:"raw_data"`
	Remark     string         `gorm:"column:remark;not null;comment:备注" json:"remark"`
	CreatedAt  time.Time      `gorm:"column:created_at;comment:创建时间" json:"created_at"`
	UpdatedAt  time.Time      `gorm:"column:updated_at;comment:更新时间" json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"column:deleted_at;comment:删除时间" json:"deleted_at"`
}

func (*RobotdogPtzPreset) TableName() string {
	return TableNameRobotdogPtzPreset
}
