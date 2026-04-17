package handler

import (
	"errors"
	"net/http"
	"strconv"

	"go-job/internal/domain/enum"
	"go-job/internal/domain/model"
	"go-job/internal/repository"
	"go-job/internal/service"
	"go-job/pkg/response"

	"github.com/gin-gonic/gin"
)

type JobHandler struct {
	jobService service.JobService
}

func NewJobHandler(jobService service.JobService) *JobHandler {
	return &JobHandler{jobService: jobService}
}

type createJobRequest struct {
	JobName         string `json:"job_name" binding:"required"`
	ExecutorID      int    `json:"executor_id" binding:"required"`
	ExecutorHandler string `json:"executor_handler" binding:"required"`
	ExecutorParam   string `json:"executor_param"`
	Cron            string `json:"cron" binding:"required"`
	ShardTotal      int8   `json:"shard_total"`
	ShardParam      string `json:"shard_param"`
	Timeout         int    `json:"timeout"`
	RetryCount      int8   `json:"retry_count"`
	Priority        int8   `json:"priority"`
	Status          int8   `json:"status"`
	Creator         string `json:"creator"`
}

type updateStatusRequest struct {
	Status int8 `json:"status" binding:"required"`
}

func (h *JobHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/job", h.Create)
	rg.GET("/job", h.List)
	rg.GET("/job/:id", h.GetByID)
	rg.PUT("/job/:id", h.Update)
	rg.PUT("/job/:id/status", h.UpdateStatus)
	rg.DELETE("/job/:id", h.Delete)
}

// Create godoc
// @Summary 创建任务
// @Description 创建一个新的调度任务
// @Tags Job
// @Accept json
// @Produce json
// @Param payload body createJobRequest true "任务创建参数"
// @Success 201 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/job [post]
func (h *JobHandler) Create(c *gin.Context) {
	var req createJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	job := &model.JobInfo{
		JobName:         req.JobName,
		ExecutorID:      req.ExecutorID,
		ExecutorHandler: req.ExecutorHandler,
		Cron:            req.Cron,
		ShardTotal:      req.ShardTotal,
		Timeout:         req.Timeout,
		RetryCount:      req.RetryCount,
		Priority:        req.Priority,
		Status:          req.Status,
	}
	if req.ExecutorParam != "" {
		job.ExecutorParam = &req.ExecutorParam
	}
	if req.ShardParam != "" {
		job.ShardParam = &req.ShardParam
	}
	if req.Creator != "" {
		job.Creator = &req.Creator
	}

	if err := h.jobService.Create(c.Request.Context(), job); err != nil {
		writeServiceError(c, err)
		return
	}
	response.Created(c, job)
}

// GetByID godoc
// @Summary 查询任务详情
// @Description 根据任务ID查询任务
// @Tags Job
// @Produce json
// @Param id path int true "任务ID"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/job/{id} [get]
func (h *JobHandler) GetByID(c *gin.Context) {
	id, ok := parsePathInt(c, "id")
	if !ok {
		return
	}
	job, err := h.jobService.GetByID(c.Request.Context(), id)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.Success(c, job)
}

// List godoc
// @Summary 查询任务列表
// @Description 分页查询任务列表，可按执行器、状态、关键字过滤
// @Tags Job
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(10)
// @Param executor_id query int false "执行器ID"
// @Param status query int false "任务状态"
// @Param keyword query string false "关键字（任务名/处理器）"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/job [get]
func (h *JobHandler) List(c *gin.Context) {
	page := parseQueryIntDefault(c, "page", 1)
	pageSize := parseQueryIntDefault(c, "page_size", 10)
	query := repository.JobQuery{
		Keyword: c.Query("keyword"),
	}
	if v, ok := parseOptionalQueryInt(c, "executor_id"); ok {
		query.ExecutorID = &v
	}
	if v, ok := parseOptionalQueryInt8(c, "status"); ok {
		query.Status = &v
	}

	rows, total, err := h.jobService.List(c.Request.Context(), repository.PageQuery{
		Page:     page,
		PageSize: pageSize,
	}, query)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.Success(c, gin.H{
		"list":  rows,
		"total": total,
		"page":  page,
		"size":  pageSize,
	})
}

// Update godoc
// @Summary 更新任务
// @Description 根据任务ID更新任务配置
// @Tags Job
// @Accept json
// @Produce json
// @Param id path int true "任务ID"
// @Param payload body createJobRequest true "任务更新参数"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/job/{id} [put]
func (h *JobHandler) Update(c *gin.Context) {
	id, ok := parsePathInt(c, "id")
	if !ok {
		return
	}
	var req createJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	job := &model.JobInfo{
		ID:              id,
		JobName:         req.JobName,
		ExecutorID:      req.ExecutorID,
		ExecutorHandler: req.ExecutorHandler,
		Cron:            req.Cron,
		ShardTotal:      req.ShardTotal,
		Timeout:         req.Timeout,
		RetryCount:      req.RetryCount,
		Priority:        req.Priority,
		Status:          req.Status,
	}
	if req.ExecutorParam != "" {
		job.ExecutorParam = &req.ExecutorParam
	}
	if req.ShardParam != "" {
		job.ShardParam = &req.ShardParam
	}
	if req.Creator != "" {
		job.Creator = &req.Creator
	}

	if err := h.jobService.Update(c.Request.Context(), job); err != nil {
		writeServiceError(c, err)
		return
	}
	response.Success(c, job)
}

// UpdateStatus godoc
// @Summary 更新任务状态
// @Description 启动或暂停任务
// @Tags Job
// @Accept json
// @Produce json
// @Param id path int true "任务ID"
// @Param payload body updateStatusRequest true "状态参数"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/job/{id}/status [put]
func (h *JobHandler) UpdateStatus(c *gin.Context) {
	id, ok := parsePathInt(c, "id")
	if !ok {
		return
	}
	var req updateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.jobService.UpdateStatus(c.Request.Context(), id, enum.JobStatus(req.Status)); err != nil {
		writeServiceError(c, err)
		return
	}
	response.Success(c, gin.H{"id": id, "status": req.Status})
}

// Delete godoc
// @Summary 删除任务
// @Description 根据任务ID删除任务
// @Tags Job
// @Produce json
// @Param id path int true "任务ID"
// @Success 204 {string} string "No Content"
// @Failure 400 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/job/{id} [delete]
func (h *JobHandler) Delete(c *gin.Context) {
	id, ok := parsePathInt(c, "id")
	if !ok {
		return
	}
	if err := h.jobService.Delete(c.Request.Context(), id); err != nil {
		writeServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func parsePathInt(c *gin.Context, key string) (int, bool) {
	v, err := strconv.Atoi(c.Param(key))
	if err != nil {
		response.BadRequest(c, "invalid path param: "+key)
		return 0, false
	}
	return v, true
}

func parsePathInt64(c *gin.Context, key string) (int64, bool) {
	v, err := strconv.ParseInt(c.Param(key), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid path param: "+key)
		return 0, false
	}
	return v, true
}

func parseQueryIntDefault(c *gin.Context, key string, def int) int {
	v := c.Query(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func parseOptionalQueryInt(c *gin.Context, key string) (int, bool) {
	v := c.Query(key)
	if v == "" {
		return 0, false
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, false
	}
	return n, true
}

func parseOptionalQueryInt8(c *gin.Context, key string) (int8, bool) {
	v := c.Query(key)
	if v == "" {
		return 0, false
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, false
	}
	return int8(n), true
}

func writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidID),
		errors.Is(err, service.ErrInvalidJob),
		errors.Is(err, service.ErrInvalidJobStatus),
		errors.Is(err, service.ErrInvalidExecutorID),
		errors.Is(err, service.ErrInvalidJobName),
		errors.Is(err, service.ErrInvalidCron),
		errors.Is(err, service.ErrInvalidJobHandler),
		errors.Is(err, service.ErrInvalidExecutor),
		errors.Is(err, service.ErrInvalidExecutorState),
		errors.Is(err, service.ErrInvalidLog),
		errors.Is(err, service.ErrInvalidPageQuery):
		response.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrJobNotFound),
		errors.Is(err, service.ErrExecutorNotFound),
		errors.Is(err, service.ErrLogNotFound):
		response.NotFound(c, err.Error())
	default:
		response.InternalError(c, err.Error())
	}
}
