package handler

import (
	"net/http"

	"go-job/internal/domain/enum"
	"go-job/internal/domain/model"
	"go-job/internal/repository"
	"go-job/internal/service"
	"go-job/pkg/response"

	"github.com/gin-gonic/gin"
)

type ExecutorHandler struct {
	executorService service.ExecutorService
}

func NewExecutorHandler(executorService service.ExecutorService) *ExecutorHandler {
	return &ExecutorHandler{executorService: executorService}
}

type createExecutorRequest struct {
	AppName     string `json:"app_name" binding:"required"`
	Name        string `json:"name" binding:"required"`
	AddressType int8   `json:"address_type"`
	AddressList string `json:"address_list"`
	Status      int8   `json:"status"`
	Creator     string `json:"creator"`
}

func (h *ExecutorHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/executor", h.Create)
	rg.GET("/executor", h.List)
	rg.GET("/executor/:id", h.GetByID)
	rg.PUT("/executor/:id", h.Update)
	rg.PUT("/executor/:id/status", h.UpdateStatus)
	rg.DELETE("/executor/:id", h.Delete)
}

// Create godoc
// @Summary 创建执行器
// @Description 创建一个执行器配置
// @Tags Executor
// @Accept json
// @Produce json
// @Param payload body createExecutorRequest true "执行器创建参数"
// @Success 201 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/executor [post]
func (h *ExecutorHandler) Create(c *gin.Context) {
	var req createExecutorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	executor := &model.JobExecutor{
		AppName:     req.AppName,
		Name:        req.Name,
		AddressType: req.AddressType,
		Status:      req.Status,
	}
	if req.AddressList != "" {
		executor.AddressList = &req.AddressList
	}
	if req.Creator != "" {
		executor.Creator = &req.Creator
	}

	if err := h.executorService.Create(c.Request.Context(), executor); err != nil {
		writeServiceError(c, err)
		return
	}
	response.Created(c, executor)
}

// GetByID godoc
// @Summary 查询执行器详情
// @Description 根据执行器ID查询执行器
// @Tags Executor
// @Produce json
// @Param id path int true "执行器ID"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/executor/{id} [get]
func (h *ExecutorHandler) GetByID(c *gin.Context) {
	id, ok := parsePathInt(c, "id")
	if !ok {
		return
	}
	row, err := h.executorService.GetByID(c.Request.Context(), id)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.Success(c, row)
}

// List godoc
// @Summary 查询执行器列表
// @Description 分页查询执行器列表，可按状态/关键字过滤
// @Tags Executor
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(10)
// @Param status query int false "执行器状态"
// @Param keyword query string false "关键字（应用名/名称）"
// @Success 200 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/executor [get]
func (h *ExecutorHandler) List(c *gin.Context) {
	page := parseQueryIntDefault(c, "page", 1)
	pageSize := parseQueryIntDefault(c, "page_size", 10)
	query := repository.ExecutorQuery{
		Keyword: c.Query("keyword"),
	}
	if v, ok := parseOptionalQueryInt8(c, "status"); ok {
		query.Status = &v
	}
	rows, total, err := h.executorService.List(c.Request.Context(), repository.PageQuery{
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
// @Summary 更新执行器
// @Description 根据执行器ID更新执行器
// @Tags Executor
// @Accept json
// @Produce json
// @Param id path int true "执行器ID"
// @Param payload body createExecutorRequest true "执行器更新参数"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/executor/{id} [put]
func (h *ExecutorHandler) Update(c *gin.Context) {
	id, ok := parsePathInt(c, "id")
	if !ok {
		return
	}
	var req createExecutorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	executor := &model.JobExecutor{
		ID:          id,
		AppName:     req.AppName,
		Name:        req.Name,
		AddressType: req.AddressType,
		Status:      req.Status,
	}
	if req.AddressList != "" {
		executor.AddressList = &req.AddressList
	}
	if req.Creator != "" {
		executor.Creator = &req.Creator
	}
	if err := h.executorService.Update(c.Request.Context(), executor); err != nil {
		writeServiceError(c, err)
		return
	}
	response.Success(c, executor)
}

// UpdateStatus godoc
// @Summary 更新执行器状态
// @Description 启用或禁用执行器
// @Tags Executor
// @Accept json
// @Produce json
// @Param id path int true "执行器ID"
// @Param payload body object{status=int} true "状态参数"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/executor/{id}/status [put]
func (h *ExecutorHandler) UpdateStatus(c *gin.Context) {
	id, ok := parsePathInt(c, "id")
	if !ok {
		return
	}
	var req struct {
		Status int8 `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.executorService.UpdateStatus(c.Request.Context(), id, enum.ExecutorStatus(req.Status)); err != nil {
		writeServiceError(c, err)
		return
	}
	response.Success(c, gin.H{"id": id, "status": req.Status})
}

// Delete godoc
// @Summary 删除执行器
// @Description 根据执行器ID删除执行器
// @Tags Executor
// @Produce json
// @Param id path int true "执行器ID"
// @Success 204 {string} string "No Content"
// @Failure 400 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/executor/{id} [delete]
func (h *ExecutorHandler) Delete(c *gin.Context) {
	id, ok := parsePathInt(c, "id")
	if !ok {
		return
	}
	if err := h.executorService.Delete(c.Request.Context(), id); err != nil {
		writeServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
