package handler

import (
	"go-job/internal/repository"
	"go-job/internal/service"
	"go-job/pkg/response"

	"github.com/gin-gonic/gin"
)

type LogHandler struct {
	logService service.LogService
}

func NewLogHandler(logService service.LogService) *LogHandler {
	return &LogHandler{logService: logService}
}

func (h *LogHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/job/log", h.List)
	rg.GET("/job/log/:id", h.GetByID)
}

// GetByID godoc
// @Summary 查询任务日志详情
// @Description 根据日志ID查询任务执行日志
// @Tags Log
// @Produce json
// @Param id path int true "日志ID"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/job/log/{id} [get]
func (h *LogHandler) GetByID(c *gin.Context) {
	id, ok := parsePathInt64(c, "id")
	if !ok {
		return
	}
	row, err := h.logService.GetByID(c.Request.Context(), id)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.Success(c, row)
}

// List godoc
// @Summary 查询任务日志列表
// @Description 分页查询日志列表，可按任务ID/执行器ID/状态过滤
// @Tags Log
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(10)
// @Param job_id query int false "任务ID"
// @Param executor_id query int false "执行器ID"
// @Param status query int false "日志状态"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/job/log [get]
func (h *LogHandler) List(c *gin.Context) {
	page := parseQueryIntDefault(c, "page", 1)
	pageSize := parseQueryIntDefault(c, "page_size", 10)
	query := repository.LogQuery{}
	if v, ok := parseOptionalQueryInt(c, "job_id"); ok {
		query.JobID = &v
	}
	if v, ok := parseOptionalQueryInt(c, "executor_id"); ok {
		query.ExecutorID = &v
	}
	if v, ok := parseOptionalQueryInt8(c, "status"); ok {
		query.Status = &v
	}

	rows, total, err := h.logService.List(c.Request.Context(), repository.PageQuery{
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
