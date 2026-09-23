package errors

import "github.com/agridispatch/agridispatch/internal/constants"

// 保养工单流程哨兵错误，统一由 handler 转换为标准响应。
var (
	// ErrReminderStateConflict 提醒当前状态不允许开单。
	ErrReminderStateConflict = New(constants.CodeReminderStateConflict, "保养提醒当前状态不允许开单")
	// ErrMachineWorking 农机作业中，拒绝保养开单。
	ErrMachineWorking = New(constants.CodeMachineBusy, constants.RejectReasonMachineWorking)
	// ErrMachineHasOpenOrder 农机已有未结束工单，拒绝保养开单。
	ErrMachineHasOpenOrder = New(constants.CodeMachineBusy, constants.RejectReasonOrderOpen)
	// ErrOrderNotProcessing 工单不是处理中状态，无法完工/取消。
	ErrOrderNotProcessing = New(constants.CodeOrderStateConflict, "工单不是处理中状态，无法执行该操作")
	// ErrOrderAlreadyCompleted 工单已经完工，重复完工不再生成费用。
	ErrOrderAlreadyCompleted = New(constants.CodeOrderDuplicateDone, constants.MsgOrderAlreadyDone)
	// ErrOrderAlreadyCanceled 工单已取消。
	ErrOrderAlreadyCanceled = New(constants.CodeOrderStateConflict, "工单已取消，无法重复操作")
	// ErrOrderRejected 工单已被拒绝。
	ErrOrderRejected = New(constants.CodeOrderStateConflict, "工单已被拒绝，无法执行该操作")
)
