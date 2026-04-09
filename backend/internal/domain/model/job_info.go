package model

import "time"

// JobInfo maps to table job_info.
type JobInfo struct {
	ID              int       `gorm:"column:id;primaryKey;autoIncrement;comment:任务ID" json:"id"`
	JobName         string    `gorm:"column:job_name;type:varchar(100);not null;comment:任务名称" json:"job_name"`
	ExecutorID      int       `gorm:"column:executor_id;not null;index:idx_executor_id;comment:关联执行器ID" json:"executor_id"`
	ExecutorHandler string    `gorm:"column:executor_handler;type:varchar(200);not null;comment:执行器处理函数名" json:"executor_handler"`
	ExecutorParam   *string   `gorm:"column:executor_param;type:varchar(512);comment:执行器参数" json:"executor_param,omitempty"`
	Cron            string    `gorm:"column:cron;type:varchar(50);not null;index:idx_cron;comment:Cron表达式" json:"cron"`
	ShardTotal      int8      `gorm:"column:shard_total;type:tinyint;not null;default:1;comment:分片总数：1-非分片任务，>1-分片任务" json:"shard_total"`
	ShardParam      *string   `gorm:"column:shard_param;type:varchar(256);comment:分片参数，逗号分隔" json:"shard_param,omitempty"`
	Timeout         int       `gorm:"column:timeout;not null;default:30;comment:任务超时时间（秒）" json:"timeout"`
	RetryCount      int8      `gorm:"column:retry_count;type:tinyint;not null;default:0;comment:失败重试次数" json:"retry_count"`
	Priority        int8      `gorm:"column:priority;type:tinyint;not null;default:1;comment:任务优先级：1-低，2-中，3-高" json:"priority"`
	Status          int8      `gorm:"column:status;type:tinyint;not null;default:0;index:idx_status;comment:任务状态：0-暂停，1-运行" json:"status"`
	Creator         *string   `gorm:"column:creator;type:varchar(50);comment:创建人" json:"creator,omitempty"`
	CreatedAt       time.Time `gorm:"column:created_at;autoCreateTime;comment:创建时间" json:"created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at;autoUpdateTime;comment:更新时间" json:"updated_at"`
}

func (JobInfo) TableName() string {
	return "job_info"
}
