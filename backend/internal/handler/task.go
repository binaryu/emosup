package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"emosup/backend/internal/service"
)

type TaskHandler struct {
	service *service.TaskService
}

func NewTaskHandler(service *service.TaskService) *TaskHandler {
	return &TaskHandler{service: service}
}

func (h *TaskHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.POST("/tasks/batch-create", h.batchCreateTasks)
	router.POST("/tasks/batch-delete", h.batchDeleteTasks)
	router.GET("/tasks", h.listTasks)
	router.GET("/tasks/ids", h.listTaskIDs)
	router.GET("/tasks/stats", h.getTaskStats)
	router.GET("/tasks/:id", h.getTask)
	router.DELETE("/tasks/:id", h.deleteTask)
	router.GET("/tasks/:id/logs", h.getTaskLogs)
	router.POST("/tasks/:id/retry", h.retryTask)
	router.POST("/tasks/:id/cancel", h.cancelTask)
	router.POST("/tasks/:id/pause", h.pauseTask)
	router.POST("/tasks/:id/resume", h.resumeTask)
}

func (h *TaskHandler) batchCreateTasks(c *gin.Context) {
	var req service.BatchCreateTasksRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.service.BatchCreateTasks(c.Request.Context(), req)
	if err != nil {
		respondTaskError(c, err)
		return
	}

	respondOK(c, result)
}

func (h *TaskHandler) listTasks(c *gin.Context) {
	page, err := parseQueryInt(c.Query("page"), 1)
	if err != nil {
		respondError(c, http.StatusBadRequest, "page must be a positive integer")
		return
	}
	pageSize, err := parseQueryInt(c.Query("page_size"), 20)
	if err != nil {
		respondError(c, http.StatusBadRequest, "page_size must be a positive integer")
		return
	}

	result, err := h.service.ListTasks(c.Request.Context(), service.ListTasksRequest{
		Status:   c.Query("status"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		respondTaskError(c, err)
		return
	}

	respondOK(c, result)
}

func (h *TaskHandler) getTaskStats(c *gin.Context) {
	stats, err := h.service.GetTaskStats(c.Request.Context())
	if err != nil {
		respondTaskError(c, err)
		return
	}

	respondOK(c, stats)
}

func (h *TaskHandler) getTask(c *gin.Context) {
	task, err := h.service.GetTask(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondTaskError(c, err)
		return
	}

	respondOK(c, task)
}

func (h *TaskHandler) getTaskLogs(c *gin.Context) {
	taskLog, err := h.service.GetTaskLog(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondTaskError(c, err)
		return
	}

	respondOK(c, taskLog)
}

func (h *TaskHandler) retryTask(c *gin.Context) {
	task, err := h.service.RetryTask(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondTaskError(c, err)
		return
	}

	respondOK(c, task)
}

func (h *TaskHandler) cancelTask(c *gin.Context) {
	task, err := h.service.CancelTask(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondTaskError(c, err)
		return
	}

	respondOK(c, task)
}

func (h *TaskHandler) pauseTask(c *gin.Context) {
	task, err := h.service.PauseTask(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondTaskError(c, err)
		return
	}
	respondOK(c, task)
}

func (h *TaskHandler) resumeTask(c *gin.Context) {
	task, err := h.service.ResumeTask(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondTaskError(c, err)
		return
	}
	respondOK(c, task)
}

func (h *TaskHandler) deleteTask(c *gin.Context) {
	err := h.service.DeleteTask(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondTaskError(c, err)
		return
	}

	respondOK(c, gin.H{"deleted": true})
}

func (h *TaskHandler) listTaskIDs(c *gin.Context) {
	status := c.Query("status")
	ids, err := h.service.ListAllTaskIDs(c.Request.Context(), status)
	if err != nil {
		respondTaskError(c, err)
		return
	}
	respondOK(c, ids)
}

func (h *TaskHandler) batchDeleteTasks(c *gin.Context) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	deleted, failed := h.service.BatchDeleteTasks(c.Request.Context(), req.IDs)
	respondOK(c, gin.H{"deleted": deleted, "failed": failed})
}

func respondTaskError(c *gin.Context, err error) {
	var serviceErr *service.TaskServiceError
	if errors.As(err, &serviceErr) {
		respondError(c, serviceErr.Code, serviceErr.Message)
		return
	}

	respondError(c, http.StatusInternalServerError, err.Error())
}

func parseQueryInt(raw string, fallback int) (int, error) {
	if raw == "" {
		return fallback, nil
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, errors.New("invalid integer")
	}
	return value, nil
}
