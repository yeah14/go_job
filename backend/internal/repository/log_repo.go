package repository

import (
	"context"
	"time"

	"go-job/internal/domain/model"

	"gorm.io/gorm"
)

type logRepository struct {
	db *gorm.DB
}

func NewLogRepository(db *gorm.DB) LogRepository {
	return &logRepository{db: db}
}

func (r *logRepository) Create(ctx context.Context, log *model.JobLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *logRepository) GetByID(ctx context.Context, id int64) (*model.JobLog, error) {
	var log model.JobLog
	if err := r.db.WithContext(ctx).First(&log, id).Error; err != nil {
		return nil, err
	}
	return &log, nil
}

func (r *logRepository) List(ctx context.Context, page PageQuery, query LogQuery) ([]model.JobLog, int64, error) {
	db := r.db.WithContext(ctx).Model(&model.JobLog{})
	db = applyLogFilters(db, query)

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset, limit := page.Normalize()
	var logs []model.JobLog
	if err := db.Order("trigger_time DESC, id DESC").Offset(offset).Limit(limit).Find(&logs).Error; err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

func (r *logRepository) Update(ctx context.Context, log *model.JobLog) error {
	return r.db.WithContext(ctx).Model(&model.JobLog{}).Where("id = ?", log.ID).Updates(log).Error
}

func (r *logRepository) UpdateStatus(ctx context.Context, id int64, status int8, costTime *int, errorMsg *string, endTime *time.Time) error {
	updates := map[string]interface{}{
		"status":   status,
		"cost_time": costTime,
		"error_msg": errorMsg,
	}
	if endTime != nil {
		updates["end_time"] = *endTime
	}
	return r.db.WithContext(ctx).Model(&model.JobLog{}).Where("id = ?", id).Updates(updates).Error
}

func (r *logRepository) DeleteByJobID(ctx context.Context, jobID int) error {
	return r.db.WithContext(ctx).Where("job_id = ?", jobID).Delete(&model.JobLog{}).Error
}

func (r *logRepository) BatchCreate(ctx context.Context, logs []model.JobLog) error {
	if len(logs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).CreateInBatches(logs, 200).Error
}

func applyLogFilters(db *gorm.DB, query LogQuery) *gorm.DB {
	if query.JobID != nil {
		db = db.Where("job_id = ?", *query.JobID)
	}
	if query.ExecutorID != nil {
		db = db.Where("executor_id = ?", *query.ExecutorID)
	}
	if query.Status != nil {
		db = db.Where("status = ?", *query.Status)
	}
	if query.StartTime != nil {
		db = db.Where("trigger_time >= ?", *query.StartTime)
	}
	if query.EndTime != nil {
		db = db.Where("trigger_time <= ?", *query.EndTime)
	}
	return db
}
