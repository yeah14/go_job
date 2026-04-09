package repository

import (
	"context"
	"strings"

	"go-job/internal/domain/model"

	"gorm.io/gorm"
)

type jobRepository struct {
	db *gorm.DB
}

func NewJobRepository(db *gorm.DB) JobRepository {
	return &jobRepository{db: db}
}

func (r *jobRepository) Create(ctx context.Context, job *model.JobInfo) error {
	return r.db.WithContext(ctx).Create(job).Error
}

func (r *jobRepository) GetByID(ctx context.Context, id int) (*model.JobInfo, error) {
	var job model.JobInfo
	if err := r.db.WithContext(ctx).First(&job, id).Error; err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *jobRepository) List(ctx context.Context, page PageQuery, query JobQuery) ([]model.JobInfo, int64, error) {
	db := r.db.WithContext(ctx).Model(&model.JobInfo{})
	db = applyJobFilters(db, query)

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset, limit := page.Normalize()
	var jobs []model.JobInfo
	if err := db.Order("id DESC").Offset(offset).Limit(limit).Find(&jobs).Error; err != nil {
		return nil, 0, err
	}
	return jobs, total, nil
}

func (r *jobRepository) Update(ctx context.Context, job *model.JobInfo) error {
	return r.db.WithContext(ctx).Model(&model.JobInfo{}).Where("id = ?", job.ID).Updates(job).Error
}

func (r *jobRepository) UpdateStatus(ctx context.Context, id int, status int8) error {
	return r.db.WithContext(ctx).Model(&model.JobInfo{}).Where("id = ?", id).Update("status", status).Error
}

func (r *jobRepository) Delete(ctx context.Context, id int) error {
	return r.db.WithContext(ctx).Delete(&model.JobInfo{}, id).Error
}

func (r *jobRepository) BatchCreate(ctx context.Context, jobs []model.JobInfo) error {
	if len(jobs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).CreateInBatches(jobs, 100).Error
}

func (r *jobRepository) ListRunning(ctx context.Context) ([]model.JobInfo, error) {
	var jobs []model.JobInfo
	if err := r.db.WithContext(ctx).
		Where("status = ?", int8(1)).
		Order("id ASC").
		Find(&jobs).Error; err != nil {
		return nil, err
	}
	return jobs, nil
}

func applyJobFilters(db *gorm.DB, query JobQuery) *gorm.DB {
	if query.ExecutorID != nil {
		db = db.Where("executor_id = ?", *query.ExecutorID)
	}
	if query.Status != nil {
		db = db.Where("status = ?", *query.Status)
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("job_name LIKE ? OR executor_handler LIKE ?", like, like)
	}
	return db
}
