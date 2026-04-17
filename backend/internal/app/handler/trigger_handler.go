package handler

import (
	"context"
	"errors"
	"io"

	"go-job/internal/service"
	"go-job/pkg/response"

	"github.com/gin-gonic/gin"
)

type TriggerHandler struct {
	jobService     service.JobService
	triggerService service.TriggerService
}

func NewTriggerHandler(jobService service.JobService, triggerService service.TriggerService) *TriggerHandler {
	return &TriggerHandler{
		jobService:     jobService,
		triggerService: triggerService,
	}
}

type triggerRequest struct {
	ExecutorAddress string `json:"executor_address"`
	ShardIndex      int8   `json:"shard_index"`
}

func (h *TriggerHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/job/:id/trigger", h.TriggerJob)
}

// TriggerJob godoc
// @Summary 手动触发任务
// @Description 立即触发一次任务执行，并返回生成的日志ID
// @Tags Trigger
// @Accept json
// @Produce json
// @Param id path int true "任务ID"
// @Param payload body triggerRequest false "触发参数"
// @Success 200 {object} response.Body
// @Failure 400 {object} response.Body
// @Failure 404 {object} response.Body
// @Failure 500 {object} response.Body
// @Router /api/v1/job/{id}/trigger [post]
func (h *TriggerHandler) TriggerJob(c *gin.Context) {
	id, ok := parsePathInt(c, "id")
	if !ok {
		return
	}
	job, err := h.jobService.GetByID(c.Request.Context(), id)
	if err != nil {
		writeServiceError(c, err)
		return
	}

	var req triggerRequest
	if c.Request.Body != nil {
		if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
			response.BadRequest(c, err.Error())
			return
		}
	}
	if req.ExecutorAddress == "" {
		req.ExecutorAddress = "manual-trigger"
	}

	logID, err := h.triggerService.TriggerJob(
		c.Request.Context(),
		*job,
		job.ExecutorID,
		req.ExecutorAddress,
		req.ShardIndex,
		func(context.Context) error { return nil },
	)
	if err != nil {
		writeServiceError(c, err)
		return
	}

	response.Success(c, gin.H{
		"job_id": job.ID,
		"log_id": logID,
	})
}
