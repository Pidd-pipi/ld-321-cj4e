package dto

import (
	"errors"

	apperrors "github.com/agridispatch/agridispatch/internal/errors"
)

// CreateMaintenanceOrderRequest 从保养提醒开单。
type CreateMaintenanceOrderRequest struct {
	ReminderID  string `json:"reminderId"`
	PlanDate    string `json:"planDate"`
	RepairPoint string `json:"repairPoint"`
}

// Validate 校验开单入参。
func (r *CreateMaintenanceOrderRequest) Validate() error {
	if r.ReminderID == "" {
		return apperrors.NewValidationError("reminderId 不能为空")
	}
	if r.PlanDate == "" {
		return apperrors.NewValidationError("planDate 计划日期不能为空")
	}
	if r.RepairPoint == "" {
		return apperrors.NewValidationError("repairPoint 维修点不能为空")
	}
	return nil
}

// CompleteMaintenanceOrderRequest 完工填报。
type CompleteMaintenanceOrderRequest struct {
	ActualHours    float64 `json:"actualHours"`
	Cost           float64 `json:"cost"`
	NextRemainHour float64 `json:"nextRemainHours"`
}

// Validate 校验完工入参。
func (r *CompleteMaintenanceOrderRequest) Validate() error {
	if r.ActualHours <= 0 {
		return apperrors.NewValidationError("actualHours 工时必须大于 0")
	}
	if r.Cost < 0 {
		return apperrors.NewValidationError("cost 费用不能为负数")
	}
	if r.NextRemainHour < 0 {
		return apperrors.NewValidationError("nextRemainHours 下次剩余小时不能为负数")
	}
	return nil
}

// ErrEmptyID 路径参数缺失。
var ErrEmptyID = errors.New("id required")
