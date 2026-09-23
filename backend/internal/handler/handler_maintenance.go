package handler

import (
	"net/http"

	"github.com/agridispatch/agridispatch/internal/constants"
	"github.com/agridispatch/agridispatch/internal/dto"
	"github.com/agridispatch/agridispatch/internal/service"
	"github.com/agridispatch/agridispatch/internal/util"
	"github.com/gin-gonic/gin"
)

// MaintenanceHandler 保养工单处理器。
type MaintenanceHandler struct {
	maintenanceSvc *service.MaintenanceService
}

func NewMaintenanceHandler(maintenanceSvc *service.MaintenanceService) *MaintenanceHandler {
	return &MaintenanceHandler{maintenanceSvc: maintenanceSvc}
}

// CreateOrder 从保养提醒开单（计划日期、维修点）。
func (h *MaintenanceHandler) CreateOrder(c *gin.Context) {
	reminderID := c.Param("id")
	if reminderID == "" {
		util.Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "reminder id required")
		return
	}
	var req dto.CreateMaintenanceOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "计划日期和维修点为必填项")
		return
	}
	order, err := h.maintenanceSvc.CreateOrder(c.Request.Context(), req, reminderID)
	if err != nil {
		util.FailError(c, err)
		return
	}
	// 被拒绝的工单同样 200 返回，前端依据 status/rejectReason 展示拒绝原因。
	message := constants.MsgOrderCreated
	if order.Status == constants.MaintenanceOrderRejected {
		message = constants.MsgOrderRejected + "：" + order.RejectReason
	}
	util.OK(c, gin.H{"order": order, "message": message})
}

// CompleteOrder 完工填写工时、费用和下次剩余小时。
func (h *MaintenanceHandler) CompleteOrder(c *gin.Context) {
	orderID := c.Param("id")
	if orderID == "" {
		util.Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "order id required")
		return
	}
	var req dto.CompleteMaintenanceOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "请填写有效的工时、费用和下次剩余小时")
		return
	}
	res, err := h.maintenanceSvc.CompleteOrder(c.Request.Context(), req, orderID)
	if err != nil {
		util.FailError(c, err)
		return
	}
	util.OK(c, res)
}

// CancelOrder 取消未完工工单。
func (h *MaintenanceHandler) CancelOrder(c *gin.Context) {
	orderID := c.Param("id")
	if orderID == "" {
		util.Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "order id required")
		return
	}
	res, err := h.maintenanceSvc.CancelOrder(c.Request.Context(), orderID)
	if err != nil {
		util.FailError(c, err)
		return
	}
	util.OK(c, res)
}
