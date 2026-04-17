package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-job/internal/domain/enum"
	"go-job/internal/domain/model"
	"go-job/internal/repository"
	"go-job/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type stubJobService struct {
	createFn       func(context.Context, *model.JobInfo) error
	getByIDFn      func(context.Context, int) (*model.JobInfo, error)
	listFn         func(context.Context, repository.PageQuery, repository.JobQuery) ([]model.JobInfo, int64, error)
	updateFn       func(context.Context, *model.JobInfo) error
	updateStatusFn func(context.Context, int, enum.JobStatus) error
	deleteFn       func(context.Context, int) error
}

func (s *stubJobService) Create(ctx context.Context, job *model.JobInfo) error {
	if s.createFn != nil {
		return s.createFn(ctx, job)
	}
	return nil
}
func (s *stubJobService) GetByID(ctx context.Context, id int) (*model.JobInfo, error) {
	if s.getByIDFn != nil {
		return s.getByIDFn(ctx, id)
	}
	return &model.JobInfo{ID: id, JobName: "demo", ExecutorID: 1, ExecutorHandler: "h", Cron: "* * * * *", Status: 1}, nil
}
func (s *stubJobService) List(ctx context.Context, p repository.PageQuery, q repository.JobQuery) ([]model.JobInfo, int64, error) {
	if s.listFn != nil {
		return s.listFn(ctx, p, q)
	}
	return []model.JobInfo{}, 0, nil
}
func (s *stubJobService) Update(ctx context.Context, job *model.JobInfo) error {
	if s.updateFn != nil {
		return s.updateFn(ctx, job)
	}
	return nil
}
func (s *stubJobService) UpdateStatus(ctx context.Context, id int, st enum.JobStatus) error {
	if s.updateStatusFn != nil {
		return s.updateStatusFn(ctx, id, st)
	}
	return nil
}
func (s *stubJobService) Delete(ctx context.Context, id int) error {
	if s.deleteFn != nil {
		return s.deleteFn(ctx, id)
	}
	return nil
}

type stubExecutorService struct {
	createFn       func(context.Context, *model.JobExecutor) error
	getByIDFn      func(context.Context, int) (*model.JobExecutor, error)
	listFn         func(context.Context, repository.PageQuery, repository.ExecutorQuery) ([]model.JobExecutor, int64, error)
	updateFn       func(context.Context, *model.JobExecutor) error
	updateStatusFn func(context.Context, int, enum.ExecutorStatus) error
	deleteFn       func(context.Context, int) error
}

func (s *stubExecutorService) Create(ctx context.Context, e *model.JobExecutor) error {
	if s.createFn != nil {
		return s.createFn(ctx, e)
	}
	return nil
}
func (s *stubExecutorService) GetByID(ctx context.Context, id int) (*model.JobExecutor, error) {
	if s.getByIDFn != nil {
		return s.getByIDFn(ctx, id)
	}
	return &model.JobExecutor{ID: id, AppName: "app", Name: "name", Status: 1}, nil
}
func (s *stubExecutorService) List(ctx context.Context, p repository.PageQuery, q repository.ExecutorQuery) ([]model.JobExecutor, int64, error) {
	if s.listFn != nil {
		return s.listFn(ctx, p, q)
	}
	return []model.JobExecutor{}, 0, nil
}
func (s *stubExecutorService) Update(ctx context.Context, e *model.JobExecutor) error {
	if s.updateFn != nil {
		return s.updateFn(ctx, e)
	}
	return nil
}
func (s *stubExecutorService) UpdateStatus(ctx context.Context, id int, st enum.ExecutorStatus) error {
	if s.updateStatusFn != nil {
		return s.updateStatusFn(ctx, id, st)
	}
	return nil
}
func (s *stubExecutorService) Delete(ctx context.Context, id int) error {
	if s.deleteFn != nil {
		return s.deleteFn(ctx, id)
	}
	return nil
}

type stubLogService struct {
	getByIDFn func(context.Context, int64) (*model.JobLog, error)
	listFn    func(context.Context, repository.PageQuery, repository.LogQuery) ([]model.JobLog, int64, error)
}

func (s *stubLogService) Create(context.Context, *model.JobLog) error { return nil }
func (s *stubLogService) Update(context.Context, *model.JobLog) error { return nil }
func (s *stubLogService) GetByID(ctx context.Context, id int64) (*model.JobLog, error) {
	if s.getByIDFn != nil {
		return s.getByIDFn(ctx, id)
	}
	return &model.JobLog{ID: id, JobID: 1, ExecutorID: 1, ExecutorAddress: "x", Status: 2, TriggerTime: time.Now()}, nil
}
func (s *stubLogService) List(ctx context.Context, p repository.PageQuery, q repository.LogQuery) ([]model.JobLog, int64, error) {
	if s.listFn != nil {
		return s.listFn(ctx, p, q)
	}
	return []model.JobLog{}, 0, nil
}
func (s *stubLogService) MarkRunning(context.Context, int64, time.Time) error             { return nil }
func (s *stubLogService) MarkSuccess(context.Context, int64, int, time.Time) error        { return nil }
func (s *stubLogService) MarkFailed(context.Context, int64, int, string, time.Time) error { return nil }
func (s *stubLogService) DeleteByJobID(context.Context, int) error                         { return nil }

type stubTriggerService struct {
	triggerFn func(context.Context, model.JobInfo, int, string, int8, func(context.Context) error) (int64, error)
}

func (s *stubTriggerService) TriggerJob(ctx context.Context, job model.JobInfo, executorID int, executorAddress string, shardIndex int8, run func(context.Context) error) (int64, error) {
	if s.triggerFn != nil {
		return s.triggerFn(ctx, job, executorID, executorAddress, shardIndex, run)
	}
	return 1001, nil
}

type noopScheduleService struct{}

func (n *noopScheduleService) Start(context.Context) error             { return nil }
func (n *noopScheduleService) Stop() error                             { return nil }
func (n *noopScheduleService) LoadRunningJobs(context.Context) error   { return nil }
func (n *noopScheduleService) ScheduleJob(context.Context, model.JobInfo) error { return nil }
func (n *noopScheduleService) RemoveJob(int)                           {}

func setupRouter(register func(*gin.RouterGroup)) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api/v1")
	register(api)
	return r
}

func doJSON(t *testing.T, r http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body failed: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

type apiResp struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func decodeResponse(t *testing.T, w *httptest.ResponseRecorder) apiResp {
	t.Helper()
	var resp apiResp
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	return resp
}

func newHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:handler_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	schema := []string{
		`CREATE TABLE job_executor (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			app_name TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			address_type INTEGER NOT NULL DEFAULT 0,
			address_list TEXT NULL,
			status INTEGER NOT NULL DEFAULT 1,
			creator TEXT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE job_info (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			job_name TEXT NOT NULL,
			executor_id INTEGER NOT NULL,
			executor_handler TEXT NOT NULL,
			executor_param TEXT NULL,
			cron TEXT NOT NULL,
			shard_total INTEGER NOT NULL DEFAULT 1,
			shard_param TEXT NULL,
			timeout INTEGER NOT NULL DEFAULT 30,
			retry_count INTEGER NOT NULL DEFAULT 0,
			priority INTEGER NOT NULL DEFAULT 1,
			status INTEGER NOT NULL DEFAULT 0,
			creator TEXT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE job_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			job_id INTEGER NOT NULL,
			executor_id INTEGER NOT NULL,
			executor_address TEXT NOT NULL,
			shard_index INTEGER NOT NULL DEFAULT 0,
			executor_param TEXT NULL,
			trigger_time DATETIME NOT NULL,
			start_time DATETIME NULL,
			end_time DATETIME NULL,
			cost_time INTEGER NULL,
			status INTEGER NOT NULL,
			error_msg TEXT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
	}
	for _, sql := range schema {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatalf("create schema failed: %v", err)
		}
	}
	return db
}

func TestJobHandler_BindingAndErrors(t *testing.T) {
	js := &stubJobService{
		listFn: func(ctx context.Context, p repository.PageQuery, q repository.JobQuery) ([]model.JobInfo, int64, error) {
			return nil, 0, service.ErrInvalidPageQuery
		},
		getByIDFn: func(ctx context.Context, id int) (*model.JobInfo, error) {
			return nil, service.ErrJobNotFound
		},
	}
	h := NewJobHandler(js)
	r := setupRouter(h.RegisterRoutes)

	// 参数绑定失败
	w := doJSON(t, r, http.MethodPost, "/api/v1/job", map[string]any{"job_name": "x"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	// path 参数非法
	w = doJSON(t, r, http.MethodGet, "/api/v1/job/abc", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	// not found 映射
	w = doJSON(t, r, http.MethodGet, "/api/v1/job/1", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}

	// list 错误映射
	req := httptest.NewRequest(http.MethodGet, "/api/v1/job?page=-1", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestExecutorHandler_CreateAndStatusValidation(t *testing.T) {
	es := &stubExecutorService{
		createFn: func(ctx context.Context, e *model.JobExecutor) error {
			return service.ErrInvalidExecutor
		},
	}
	h := NewExecutorHandler(es)
	r := setupRouter(h.RegisterRoutes)

	// 创建参数绑定通过，但 service 校验失败
	w := doJSON(t, r, http.MethodPost, "/api/v1/executor", map[string]any{
		"app_name": "a", "name": "n", "status": 9,
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	// 状态接口 path 非法
	w = doJSON(t, r, http.MethodPut, "/api/v1/executor/x/status", map[string]any{"status": 1})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestLogHandler_GetAndList(t *testing.T) {
	ls := &stubLogService{
		getByIDFn: func(ctx context.Context, id int64) (*model.JobLog, error) {
			return nil, service.ErrLogNotFound
		},
	}
	h := NewLogHandler(ls)
	r := setupRouter(h.RegisterRoutes)

	w := doJSON(t, r, http.MethodGet, "/api/v1/job/log/abc", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	w = doJSON(t, r, http.MethodGet, "/api/v1/job/log/1", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestTriggerHandler_BindingAndError(t *testing.T) {
	js := &stubJobService{
		getByIDFn: func(ctx context.Context, id int) (*model.JobInfo, error) {
			return &model.JobInfo{
				ID:              id,
				JobName:         "demo",
				ExecutorID:      1,
				ExecutorHandler: "h",
				Cron:            "* * * * *",
				Status:          1,
			}, nil
		},
	}
	ts := &stubTriggerService{
		triggerFn: func(ctx context.Context, job model.JobInfo, executorID int, addr string, shard int8, run func(context.Context) error) (int64, error) {
			return 0, service.ErrInvalidLog
		},
	}
	h := NewTriggerHandler(js, ts)
	r := setupRouter(h.RegisterRoutes)

	// path 非法
	w := doJSON(t, r, http.MethodPost, "/api/v1/job/x/trigger", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	// trigger 失败映射
	w = doJSON(t, r, http.MethodPost, "/api/v1/job/1/trigger", map[string]any{"executor_address": "a"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	// 非法 JSON 映射
	req := httptest.NewRequest(http.MethodPost, "/api/v1/job/1/trigger", bytes.NewBufferString("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid json, got %d", w.Code)
	}
}

func TestAPI_EndToEnd_TriggerAndLogLifecycle(t *testing.T) {
	db := newHandlerTestDB(t)
	exeRepo := repository.NewExecutorRepository(db)
	jobRepo := repository.NewJobRepository(db)
	logRepo := repository.NewLogRepository(db)

	mockSchedule := &noopScheduleService{}
	executorSvc := service.NewExecutorService(exeRepo)
	jobSvc := service.NewJobService(jobRepo, mockSchedule)
	logSvc := service.NewLogService(logRepo)
	triggerSvc := service.NewTriggerService(logSvc)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api/v1")
	NewExecutorHandler(executorSvc).RegisterRoutes(api)
	NewJobHandler(jobSvc).RegisterRoutes(api)
	NewTriggerHandler(jobSvc, triggerSvc).RegisterRoutes(api)
	NewLogHandler(logSvc).RegisterRoutes(api)

	w := doJSON(t, r, http.MethodPost, "/api/v1/executor", map[string]any{
		"app_name": "e2e-app",
		"name":     "e2e-executor",
		"status":   1,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create executor expected 201, got %d", w.Code)
	}
	executorResp := decodeResponse(t, w)
	var executor model.JobExecutor
	if err := json.Unmarshal(executorResp.Data, &executor); err != nil {
		t.Fatalf("decode executor data failed: %v", err)
	}

	w = doJSON(t, r, http.MethodPost, "/api/v1/job", map[string]any{
		"job_name":         "e2e-job",
		"executor_id":      executor.ID,
		"executor_handler": "demo.handler",
		"cron":             "*/5 * * * * *",
		"status":           1,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create job expected 201, got %d", w.Code)
	}
	jobResp := decodeResponse(t, w)
	var job model.JobInfo
	if err := json.Unmarshal(jobResp.Data, &job); err != nil {
		t.Fatalf("decode job data failed: %v", err)
	}

	w = doJSON(t, r, http.MethodPost, fmt.Sprintf("/api/v1/job/%d/trigger", job.ID), map[string]any{
		"executor_address": "127.0.0.1:9090",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("trigger job expected 200, got %d", w.Code)
	}
	triggerResp := decodeResponse(t, w)
	var triggerData struct {
		JobID int   `json:"job_id"`
		LogID int64 `json:"log_id"`
	}
	if err := json.Unmarshal(triggerResp.Data, &triggerData); err != nil {
		t.Fatalf("decode trigger data failed: %v", err)
	}
	if triggerData.JobID != job.ID || triggerData.LogID <= 0 {
		t.Fatalf("unexpected trigger result: %+v", triggerData)
	}

	w = doJSON(t, r, http.MethodGet, fmt.Sprintf("/api/v1/job/log/%d", triggerData.LogID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get log expected 200, got %d", w.Code)
	}
	logResp := decodeResponse(t, w)
	var logRow model.JobLog
	if err := json.Unmarshal(logResp.Data, &logRow); err != nil {
		t.Fatalf("decode log data failed: %v", err)
	}
	if logRow.Status != int8(enum.LogStatusSuccess) {
		t.Fatalf("expected success log status, got %d", logRow.Status)
	}

	w = doJSON(t, r, http.MethodGet, fmt.Sprintf("/api/v1/job/log?job_id=%d", job.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list logs expected 200, got %d", w.Code)
	}
}
