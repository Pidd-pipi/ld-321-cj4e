package model

import "time"

// MaintenanceOrder 保养工单。
type MaintenanceOrder struct {
	ID             string     `gorm:"primaryKey;size:32" json:"id"`
	ReminderID     string     `gorm:"size:32;index" json:"reminderId"`
	MachineCode    string     `gorm:"size:32;index" json:"machineCode"`
	Title          string     `gorm:"size:128" json:"title"`
	PlanDate       string     `gorm:"size:32" json:"planDate"`
	RepairPoint    string     `gorm:"size:128" json:"repairPoint"`
	Status         string     `gorm:"size:20;index" json:"status"`
	RejectReason   string     `gorm:"size:255" json:"rejectReason"`
	ActualHours    float64    `json:"actualHours"`
	Cost           float64    `json:"cost"`
	NextRemainHour float64    `json:"nextRemainHours"`
	CompletedAt    *time.Time `json:"completedAt"`
	CancelledAt    *time.Time `json:"cancelledAt"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

// MaintenanceCost 保养费用记录（一张工单仅对应一笔费用）。
type MaintenanceCost struct {
	ID             string    `gorm:"primaryKey;size:32" json:"id"`
	OrderID        string    `gorm:"size:32;uniqueIndex" json:"orderId"`
	MachineCode    string    `gorm:"size:32;index" json:"machineCode"`
	Amount         float64   `json:"amount"`
	ActualHours    float64   `json:"actualHours"`
	NextRemainHour float64   `json:"nextRemainHours"`
	RecordedAt     string    `gorm:"size:32" json:"recordedAt"`
	CreatedAt      time.Time `json:"-"`
}
