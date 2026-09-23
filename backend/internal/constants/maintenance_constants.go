package constants

// 保养工单状态
const (
	MaintenanceOrderProcessing = "处理中"
	MaintenanceOrderDone       = "已完工"
	MaintenanceOrderCanceled   = "已取消"
	MaintenanceOrderRejected   = "已拒绝"
)

// 保养提醒状态
const (
	ReminderOpen       = "待开单"
	ReminderProcessing = "处理中"
	ReminderDone       = "已完工"
)

// 保养预警等级
const (
	ReminderLevelNormal  = "normal"
	ReminderLevelWarning = "warning"
	ReminderLevelDanger  = "danger"
)

// 开单拒绝原因
const (
	RejectReasonMachineWorking = "农机作业中，暂不能进场保养"
	RejectReasonOrderOpen      = "该农机已有未结束工单，请先完工或取消"
	RejectReasonReminderState  = "提醒当前状态不允许开单"
)

// 保养默认值
const (
	MaintenanceIdleCurrentTask = "可派单"
)

// 保养工单 ID 前缀
const (
	MaintenanceOrderIDPrefix   = "wo"
	MaintenanceExpenseIDPrefix = "fee"
)

// 保养工单消息
const (
	MsgOrderCreated     = "工单已创建，提醒进入处理中，农机转为维修中"
	MsgOrderRejected    = "开单被拒绝，已记录拒绝原因"
	MsgOrderCompleted   = "工单完工，费用、提醒与农机空闲状态已一并生效"
	MsgOrderAlreadyDone = "工单已完工，未重复生成费用记录"
	MsgOrderCanceled    = "工单已取消，农机已释放，提醒恢复为待开单"
)
