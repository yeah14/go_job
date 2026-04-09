package repository

import (
	"context"
	"strings"

	"go-job/internal/domain/model"

	"gorm.io/gorm"
)

type executorRepository struct {
	db *gorm.DB
}

func NewExecutorRepository(db *gorm.DB) ExecutorRepository {
	return &executorRepository{db: db}
}

func (r *executorRepository) Create(ctx context.Context, executor *model.JobExecutor) error {
	return r.db.WithContext(ctx).Create(executor).Error
}

func (r *executorRepository) GetByID(ctx context.Context, id int) (*model.JobExecutor, error) {
	var executor model.JobExecutor
	if err := r.db.WithContext(ctx).First(&executor, id).Error; err != nil {
		return nil, err
	}
	return &executor, nil
}

func (r *executorRepository) GetByAppName(ctx context.Context, appName string) (*model.JobExecutor, error) {
	var executor model.JobExecutor
	if err := r.db.WithContext(ctx).Where("app_name = ?", appName).First(&executor).Error; err != nil {
		return nil, err
	}
	return &executor, nil
}

func (r *executorRepository) List(ctx context.Context, page PageQuery, query ExecutorQuery) ([]model.JobExecutor, int64, error) {
	db := r.db.WithContext(ctx).Model(&model.JobExecutor{})
	db = applyExecutorFilters(db, query)

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset, limit := page.Normalize()
	var executors []model.JobExecutor
	if err := db.Order("id DESC").Offset(offset).Limit(limit).Find(&executors).Error; err != nil {
		return nil, 0, err
	}
	return executors, total, nil
}

func (r *executorRepository) Update(ctx context.Context, executor *model.JobExecutor) error {
	return r.db.WithContext(ctx).Model(&model.JobExecutor{}).Where("id = ?", executor.ID).Updates(executor).Error
}

func (r *executorRepository) UpdateStatus(ctx context.Context, id int, status int8) error {
	return r.db.WithContext(ctx).Model(&model.JobExecutor{}).Where("id = ?", id).Update("status", status).Error
}

func (r *executorRepository) Delete(ctx context.Context, id int) error {
	return r.db.WithContext(ctx).Delete(&model.JobExecutor{}, id).Error
}

func (r *executorRepository) BatchCreate(ctx context.Context, executors []model.JobExecutor) error {
	if len(executors) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).CreateInBatches(executors, 100).Error
}

func applyExecutorFilters(db *gorm.DB, query ExecutorQuery) *gorm.DB {
	if query.Status != nil {
		db = db.Where("status = ?", *query.Status)
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("app_name LIKE ? OR name LIKE ?", like, like)
	}
	return db
}
