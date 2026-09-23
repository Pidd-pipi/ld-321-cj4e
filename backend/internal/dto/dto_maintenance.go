package dto

// CreateMaintenanceOrderRequest 从保养提醒开单请求。
type CreateMaintenanceOrderRequest struct {
	PlanDate     string `json:"planDate" binding:"required"`
	ServicePoint string `json:"servicePoint" binding:"required"`
}

// CompleteMaintenanceOrderRequest 保养工单完工请求。
type CompleteMaintenanceOrderRequest struct {
	ActualHours     float64 `json:"actualHours" binding:"gte=0"`
	Cost            float64 `json:"cost" binding:"gte=0"`
	NextRemainHours float64 `json:"nextRemainHours" binding:"gt=0"`
}
