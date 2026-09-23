package service

import (
	"context"
	"fmt"

	"github.com/agridispatch/agridispatch/internal/constants"
	apperrors "github.com/agridispatch/agridispatch/internal/errors"
	"github.com/agridispatch/agridispatch/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CreateOrder 从保养提醒开单：
// 农机作业中或已有未结束工单则拒绝并留痕；成功后提醒转处理中、农机转维修中。
func (s *MaintenanceService) CreateOrder(ctx context.Context, reminderID, planDate, repairPoint string) (*CreateOrderResult, error) {
	result := &CreateOrderResult{}
	if err := s.repo.WithTx(func(tx *gorm.DB) error {
		reminder, err := s.repo.FindReminder(tx, reminderID)
		if err != nil {
			return err
		}
		if reminder.Status == constants.ReminderProcessed {
			return s.rejectOrder(tx, reminder, constants.RejectReasonOpenOrder, result)
		}
		if reminder.Status == constants.ReminderDone {
			return apperrors.New(constants.CodeConflict, fmt.Sprintf("提醒 %s 已完工，不能重复开单", reminderID))
		}

		machine, err := s.repo.FindMachineByCodeForUpdate(tx, reminder.MachineCode)
		if err != nil {
			return err
		}

		// 作业中的农机直接拒绝。
		if machine.Status == constants.MachineWorking {
			return s.rejectOrder(tx, reminder, constants.RejectReasonWorking, result)
		}

		// 已有未结束工单（处理中）同样拒绝，防止重复占用农机。
		open, err := s.repo.HasOpenOrderByMachine(tx, machine.Code)
		if err != nil {
			return err
		}
		if open {
			return s.rejectOrder(tx, reminder, constants.RejectReasonOpenOrder, result)
		}

		now := s.now()
		order := &model.MaintenanceOrder{
			ID:          s.newID(s.idPrefix),
			ReminderID:  reminder.ID,
			MachineCode: machine.Code,
			Title:       reminder.Title,
			PlanDate:    planDate,
			RepairPoint: repairPoint,
			Status:      constants.OrderProcessing,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := s.repo.CreateOrder(tx, order); err != nil {
			return err
		}

		reminder.Status = constants.ReminderProcessed
		if err := s.repo.SaveReminder(tx, reminder); err != nil {
			return err
		}

		machine.Status = constants.MachineRepair
		machine.CurrentTask = fmt.Sprintf("保养：%s", reminder.Title)
		if err := s.repo.SaveMachine(tx, machine); err != nil {
			return err
		}

		result.OrderID = order.ID
		result.Status = order.Status
		result.Message = "工单已创建，提醒进入处理中，农机转为维修中"
		return nil
	}); err != nil {
		return nil, err
	}

	s.invalidateOverview(ctx)
	s.logger.Info("maintenance order created", "reminderId", reminderID, "orderId", result.OrderID, "rejected", result.RejectReason)
	return result, nil
}

// rejectOrder 记录一条已拒绝工单，保留拒绝原因。
func (s *MaintenanceService) rejectOrder(tx *gorm.DB, reminder *model.MaintenanceReminder, reason string, result *CreateOrderResult) error {
	order := &model.MaintenanceOrder{
		ID:           s.newID(s.idPrefix),
		ReminderID:   reminder.ID,
		MachineCode:  reminder.MachineCode,
		Title:        reminder.Title,
		Status:       constants.OrderRejected,
		RejectReason: reason,
		CreatedAt:    s.now(),
		UpdatedAt:    s.now(),
	}
	if err := s.repo.CreateOrder(tx, order); err != nil {
		return err
	}
	result.OrderID = order.ID
	result.Status = order.Status
	result.RejectReason = reason
	result.Message = reason
	return nil
}

func (s *MaintenanceService) newID(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, uuid.NewString())
}
