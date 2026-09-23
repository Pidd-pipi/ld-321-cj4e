package handler

import (
	"github.com/agridispatch/agridispatch/internal/dto"
	"github.com/agridispatch/agridispatch/internal/service"
	"github.com/agridispatch/agridispatch/internal/util"
	"github.com/gin-gonic/gin"
)

// MaintenanceHandler 保养工单处理器。
type MaintenanceHandler struct {
	svc *service.MaintenanceService
}

func NewMaintenanceHandler(svc *service.MaintenanceService) *MaintenanceHandler {
	return &MaintenanceHandler{svc: svc}
}

// CreateOrder 从保养提醒开单。
func (h *MaintenanceHandler) CreateOrder(c *gin.Context) {
	var req dto.CreateMaintenanceOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Fail(c, 400, 40000, "请求体格式错误")
		return
	}
	if err := req.Validate(); err != nil {
		util.FailError(c, err)
		return
	}
	res, err := h.svc.CreateOrder(c.Request.Context(), req.ReminderID, req.PlanDate, req.RepairPoint)
	if err != nil {
		util.FailError(c, err)
		return
	}
	util.Created(c, res)
}

// CompleteOrder 完工填报。
func (h *MaintenanceHandler) CompleteOrder(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		util.FailError(c, dto.ErrEmptyID)
		return
	}
	var req dto.CompleteMaintenanceOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Fail(c, 400, 40000, "请求体格式错误")
		return
	}
	if err := req.Validate(); err != nil {
		util.FailError(c, err)
		return
	}
	res, err := h.svc.CompleteOrder(c.Request.Context(), id, req.ActualHours, req.Cost, req.NextRemainHour)
	if err != nil {
		util.FailError(c, err)
		return
	}
	util.OK(c, res)
}

// CancelOrder 取消未完工工单。
func (h *MaintenanceHandler) CancelOrder(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		util.FailError(c, dto.ErrEmptyID)
		return
	}
	res, err := h.svc.CancelOrder(c.Request.Context(), id)
	if err != nil {
		util.FailError(c, err)
		return
	}
	util.OK(c, res)
}
