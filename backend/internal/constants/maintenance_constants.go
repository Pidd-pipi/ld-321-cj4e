package constants

// 时间格式
const (
	DateFormatDay = "2006-01-02"
)

// 保养工单状态
const (
	OrderProcessing = "处理中"
	OrderCompleted  = "已完工"
	OrderCancelled  = "已取消"
	OrderRejected   = "已拒绝"
)

// 保养提醒节点状态
const (
	ReminderPending   = "待处理"
	ReminderProcessed = "处理中"
	ReminderDone      = "已完工"
)

// 保养预警等级
const (
	ReminderLevelNormal  = "normal"
	ReminderLevelWarning = "warning"
	ReminderLevelDanger  = "danger"
)

// 农机空闲时的当前任务标识
const MachineIdleTask = "空闲"

// 工单开单拒绝原因
const (
	RejectReasonWorking   = "农机作业中，暂不开单"
	RejectReasonOpenOrder = "该农机已有未结束工单"
)

// 保养周期默认值
const (
	DefaultNextRemainingHours = 100.0
	DangerRemainingHours      = 0.0
	WarningRemainingHours     = 20.0
)

// 工单与费用记录 ID 前缀
const (
	MaintenanceOrderIDPrefix = "mo"
	MaintenanceCostIDPrefix  = "mc"
)
