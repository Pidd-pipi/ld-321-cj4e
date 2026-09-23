package model

import "time"

// MaintenanceOrder 保养工单。
type MaintenanceOrder struct {
	ID              string     `gorm:"primaryKey;size:32" json:"id"`
	ReminderID      string     `gorm:"size:32;index" json:"reminderId"`
	MachineCode     string     `gorm:"size:32;index" json:"machineCode"`
	Title           string     `gorm:"size:128" json:"title"`
	PlanDate        string     `gorm:"size:32" json:"planDate"`
	ServicePoint    string     `gorm:"size:128" json:"servicePoint"`
	Status          string     `gorm:"size:20;index" json:"status"`
	RejectReason    string     `gorm:"size:255" json:"rejectReason"`
	ActualHours     float64    `json:"actualHours"`
	Cost            float64    `json:"cost"`
	NextRemainHours float64    `json:"nextRemainHours"`
	CompletedAt     *time.Time `json:"completedAt"`
	CreatedAt       time.Time  `json:"createdAt"`
}

// MaintenanceExpense 保养费用记录。
type MaintenanceExpense struct {
	ID              string    `gorm:"primaryKey;size:32" json:"id"`
	OrderID         string    `gorm:"size:32;uniqueIndex" json:"orderId"`
	MachineCode     string    `gorm:"size:32;index" json:"machineCode"`
	ReminderID      string    `gorm:"size:32;index" json:"reminderId"`
	Title           string    `gorm:"size:128" json:"title"`
	ServicePoint    string    `gorm:"size:128" json:"servicePoint"`
	ActualHours     float64   `json:"actualHours"`
	Cost            float64   `json:"cost"`
	NextRemainHours float64   `json:"nextRemainHours"`
	PaidAt          string    `gorm:"size:32" json:"paidAt"`
	CreatedAt       time.Time `json:"createdAt"`
}
