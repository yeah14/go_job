package model

import "time"

// JobExecutor maps to table job_executor.
type JobExecutor struct {
	ID          int       `gorm:"column:id;primaryKey;autoIncrement;comment:执行器ID" json:"id"`
	AppName     string    `gorm:"column:app_name;type:varchar(100);not null;uniqueIndex:uk_app_name;comment:执行器应用名（集群唯一）" json:"app_name"`
	Name        string    `gorm:"column:name;type:varchar(100);not null;comment:执行器显示名" json:"name"`
	AddressType int8      `gorm:"column:address_type;type:tinyint;not null;default:0;comment:地址类型：0-自动注册，1-手动配置" json:"address_type"`
	AddressList *string   `gorm:"column:address_list;type:varchar(512);comment:手动配置地址列表，逗号分隔" json:"address_list,omitempty"`
	Status      int8      `gorm:"column:status;type:tinyint;not null;default:1;index:idx_status;comment:状态：0-禁用，1-正常" json:"status"`
	Creator     *string   `gorm:"column:creator;type:varchar(50);comment:创建人" json:"creator,omitempty"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime;comment:创建时间" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at;autoUpdateTime;comment:更新时间" json:"updated_at"`
}

func (JobExecutor) TableName() string {
	return "job_executor"
}
