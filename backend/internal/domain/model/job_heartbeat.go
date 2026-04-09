package model

import "time"

// JobExecutorHeartbeat maps to table job_executor_heartbeat.
type JobExecutorHeartbeat struct {
	ID              int       `gorm:"column:id;primaryKey;autoIncrement;comment:主键ID" json:"id"`
	ExecutorAppName string    `gorm:"column:executor_app_name;type:varchar(100);not null;uniqueIndex:uk_app_address;comment:执行器应用名" json:"executor_app_name"`
	ExecutorAddress string    `gorm:"column:executor_address;type:varchar(100);not null;uniqueIndex:uk_app_address;comment:执行器节点地址" json:"executor_address"`
	HeartbeatTime   time.Time `gorm:"column:heartbeat_time;not null;index:idx_heartbeat_time;comment:最新心跳时间" json:"heartbeat_time"`
	CreatedAt       time.Time `gorm:"column:created_at;autoCreateTime;comment:创建时间" json:"created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at;autoUpdateTime;comment:更新时间" json:"updated_at"`
}

func (JobExecutorHeartbeat) TableName() string {
	return "job_executor_heartbeat"
}
