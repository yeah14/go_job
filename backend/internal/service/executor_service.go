package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go-job/internal/domain/enum"
	"go-job/internal/domain/model"
	"go-job/internal/repository"

	"gorm.io/gorm"
)

type executorService struct {
	executorRepo repository.ExecutorRepository
}

func NewExecutorService(executorRepo repository.ExecutorRepository) ExecutorService {
	return &executorService{executorRepo: executorRepo}
}

func (s *executorService) Create(ctx context.Context, executor *model.JobExecutor) error {
	if err := validateExecutorPayload(executor); err != nil {
		return err
	}
	return wrapExecutorRepoErr(s.executorRepo.Create(ctx, executor))
}

func (s *executorService) GetByID(ctx context.Context, id int) (*model.JobExecutor, error) {
	if id <= 0 {
		return nil, ErrInvalidID
	}
	row, err := s.executorRepo.GetByID(ctx, id)
	if err != nil {
		return nil, wrapExecutorRepoErr(err)
	}
	return row, nil
}

func (s *executorService) List(ctx context.Context, page repository.PageQuery, query repository.ExecutorQuery) ([]model.JobExecutor, int64, error) {
	if page.Page < 0 || page.PageSize < 0 {
		return nil, 0, ErrInvalidPageQuery
	}
	if query.Status != nil && !enum.ExecutorStatus(*query.Status).IsValid() {
		return nil, 0, ErrInvalidExecutorState
	}
	rows, total, err := s.executorRepo.List(ctx, page, query)
	if err != nil {
		return nil, 0, wrapExecutorRepoErr(err)
	}
	return rows, total, nil
}

func (s *executorService) Update(ctx context.Context, executor *model.JobExecutor) error {
	if executor == nil || executor.ID <= 0 {
		return ErrInvalidExecutor
	}
	if err := validateExecutorPayload(executor); err != nil {
		return err
	}
	return wrapExecutorRepoErr(s.executorRepo.Update(ctx, executor))
}

func (s *executorService) UpdateStatus(ctx context.Context, id int, status enum.ExecutorStatus) error {
	if id <= 0 {
		return ErrInvalidID
	}
	if !status.IsValid() {
		return ErrInvalidExecutorState
	}
	return wrapExecutorRepoErr(s.executorRepo.UpdateStatus(ctx, id, int8(status)))
}

func (s *executorService) Delete(ctx context.Context, id int) error {
	if id <= 0 {
		return ErrInvalidID
	}
	return wrapExecutorRepoErr(s.executorRepo.Delete(ctx, id))
}

func validateExecutorPayload(executor *model.JobExecutor) error {
	if executor == nil {
		return ErrInvalidExecutor
	}
	if strings.TrimSpace(executor.AppName) == "" || strings.TrimSpace(executor.Name) == "" {
		return ErrInvalidExecutor
	}
	if !enum.ExecutorStatus(executor.Status).IsValid() {
		return ErrInvalidExecutorState
	}
	return nil
}

func wrapExecutorRepoErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrExecutorNotFound
	}
	return fmt.Errorf("repository error: %w", err)
}
