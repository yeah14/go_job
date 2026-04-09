package model

import "time"

// JobLog maps to table job_log.
type JobLog struct {
	ID              int64      `gorm:"column:id;primaryKey;autoIncrement;comment:日志ID" json:"id"`
	JobID           int        `gorm:"column:job_id;not null;index:idx_job_id;comment:关联任务ID" json:"job_id"`
	ExecutorID      int        `gorm:"column:executor_id;not null;comment:执行器ID" json:"executor_id"`
	ExecutorAddress string     `gorm:"column:executor_address;type:varchar(100);not null;comment:执行器节点地址" json:"executor_address"`
	ShardIndex      int8       `gorm:"column:shard_index;type:tinyint;not null;default:0;comment:分片索引：0-非分片任务，>0-分片索引" json:"shard_index"`
	ExecutorParam   *string    `gorm:"column:executor_param;type:varchar(512);comment:实际执行参数" json:"executor_param,omitempty"`
	TriggerTime     time.Time  `gorm:"column:trigger_time;not null;index:idx_trigger_time;comment:任务触发时间" json:"trigger_time"`
	StartTime       *time.Time `gorm:"column:start_time;comment:任务开始执行时间" json:"start_time,omitempty"`
	EndTime         *time.Time `gorm:"column:end_time;comment:任务结束执行时间" json:"end_time,omitempty"`
	CostTime        *int       `gorm:"column:cost_time;comment:执行耗时（毫秒）" json:"cost_time,omitempty"`
	Status          int8       `gorm:"column:status;type:tinyint;not null;index:idx_status;comment:执行状态：0-待执行，1-执行中，2-执行成功，3-执行失败" json:"status"`
	ErrorMsg        *string    `gorm:"column:error_msg;type:text;comment:失败错误信息" json:"error_msg,omitempty"`
	CreatedAt       time.Time  `gorm:"column:created_at;autoCreateTime;comment:创建时间" json:"created_at"`
}

func (JobLog) TableName() string {
	return "job_log"
}
