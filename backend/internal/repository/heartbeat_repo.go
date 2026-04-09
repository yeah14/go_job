package repository

import (
	"context"
	"time"

	"go-job/internal/domain/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type heartbeatRepository struct {
	db *gorm.DB
}

func NewHeartbeatRepository(db *gorm.DB) HeartbeatRepository {
	return &heartbeatRepository{db: db}
}

func (r *heartbeatRepository) Create(ctx context.Context, heartbeat *model.JobExecutorHeartbeat) error {
	return r.db.WithContext(ctx).Create(heartbeat).Error
}

func (r *heartbeatRepository) Upsert(ctx context.Context, heartbeat *model.JobExecutorHeartbeat) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "executor_app_name"}, {Name: "executor_address"}},
			DoUpdates: clause.AssignmentColumns([]string{"heartbeat_time", "updated_at"}),
		}).
		Create(heartbeat).Error
}

func (r *heartbeatRepository) GetByAppAndAddress(ctx context.Context, appName, address string) (*model.JobExecutorHeartbeat, error) {
	var hb model.JobExecutorHeartbeat
	if err := r.db.WithContext(ctx).
		Where("executor_app_name = ? AND executor_address = ?", appName, address).
		First(&hb).Error; err != nil {
		return nil, err
	}
	return &hb, nil
}

func (r *heartbeatRepository) List(ctx context.Context, page PageQuery, query HeartbeatQuery) ([]model.JobExecutorHeartbeat, int64, error) {
	db := r.db.WithContext(ctx).Model(&model.JobExecutorHeartbeat{})
	db = applyHeartbeatFilters(db, query)

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset, limit := page.Normalize()
	var rows []model.JobExecutorHeartbeat
	if err := db.Order("heartbeat_time DESC, id DESC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *heartbeatRepository) ListExpired(ctx context.Context, deadline time.Time) ([]model.JobExecutorHeartbeat, error) {
	var rows []model.JobExecutorHeartbeat
	if err := r.db.WithContext(ctx).
		Where("heartbeat_time < ?", deadline).
		Order("heartbeat_time ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *heartbeatRepository) DeleteByAppAndAddress(ctx context.Context, appName, address string) error {
	return r.db.WithContext(ctx).
		Where("executor_app_name = ? AND executor_address = ?", appName, address).
		Delete(&model.JobExecutorHeartbeat{}).Error
}

func (r *heartbeatRepository) BatchUpsert(ctx context.Context, heartbeats []model.JobExecutorHeartbeat) error {
	if len(heartbeats) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "executor_app_name"}, {Name: "executor_address"}},
			DoUpdates: clause.AssignmentColumns([]string{"heartbeat_time", "updated_at"}),
		}).
		CreateInBatches(heartbeats, 200).Error
}

func applyHeartbeatFilters(db *gorm.DB, query HeartbeatQuery) *gorm.DB {
	if query.AppName != "" {
		db = db.Where("executor_app_name = ?", query.AppName)
	}
	if query.Address != "" {
		db = db.Where("executor_address = ?", query.Address)
	}
	if query.BeforeTime != nil {
		db = db.Where("heartbeat_time <= ?", *query.BeforeTime)
	}
	if query.AfterTime != nil {
		db = db.Where("heartbeat_time >= ?", *query.AfterTime)
	}
	if query.OnlyExpired && query.ExpireBefore != nil {
		db = db.Where("heartbeat_time < ?", *query.ExpireBefore)
	}
	return db
}
