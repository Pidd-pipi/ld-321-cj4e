package service

import (
	"context"
	"fmt"

	"github.com/agridispatch/agridispatch/internal/constants"
	apperrors "github.com/agridispatch/agridispatch/internal/errors"
	"github.com/agridispatch/agridispatch/internal/model"
	"gorm.io/gorm"
)

// CompleteOrder 完工填报：
// 工时、费用、下次剩余小时落库；工单、费用记录、提醒节点、农机空闲状态在同一事务内一起生效；
// 重复完工不产生第二笔费用。
func (s *MaintenanceService) CompleteOrder(ctx context.Context, orderID string, actualHours, cost, nextRemain float64) (map[string]interface{}, error) {
	var res map[string]interface{}
	if err := s.repo.WithTx(func(tx *gorm.DB) error {
		order, err := s.repo.FindOrderByIDForUpdate(tx, orderID)
		if err != nil {
			return err
		}
		if order.Status != constants.OrderProcessing {
			return apperrors.New(constants.CodeConflict, fmt.Sprintf("工单 %s 当前状态为 %s，无法完工", orderID, order.Status))
		}

		now := s.now()
		order.Status = constants.OrderCompleted
		order.ActualHours = actualHours
		order.Cost = cost
		order.NextRemainHour = nextRemain
		order.CompletedAt = &now
		order.UpdatedAt = now
		if err := s.repo.SaveOrder(tx, order); err != nil {
			return err
		}

		// 费用记录与工单 1:1（order_id 唯一索引兜底），重复完工不会写入第二笔。
		fee := &model.MaintenanceCost{
			ID:             s.newID(constants.MaintenanceCostIDPrefix),
			OrderID:        order.ID,
			MachineCode:    order.MachineCode,
			Amount:         cost,
			ActualHours:    actualHours,
			NextRemainHour: nextRemain,
			RecordedAt:     now.Format(constants.DateFormatDay),
			CreatedAt:      now,
		}
		if err := s.repo.CreateCost(tx, fee); err != nil {
			return fmt.Errorf("record maintenance cost: %w", err)
		}

		reminder, err := s.repo.FindReminder(tx, order.ReminderID)
		if err != nil {
			return err
		}
		reminder.Status = constants.ReminderDone
		reminder.RemainingHours = nextRemain
		reminder.Level = levelForHours(nextRemain)
		reminder.LastServiceRecord = fmt.Sprintf("%s 已完工，工时 %.1f，费用 %.2f 元",
			now.Format(constants.DateFormatDay), actualHours, cost)
		if err := s.repo.SaveReminder(tx, reminder); err != nil {
			return err
		}

		machine, err := s.repo.FindMachineByCodeForUpdate(tx, order.MachineCode)
		if err != nil {
			return err
		}
		machine.Status = constants.MachineIdle
		machine.CurrentTask = constants.MachineIdleTask
		if err := s.repo.SaveMachine(tx, machine); err != nil {
			return err
		}

		res = map[string]interface{}{
			"orderId":         order.ID,
			"status":          constants.OrderCompleted,
			"costId":          fee.ID,
			"message":         "工单已完工，费用、提醒节点与农机空闲状态已同步生效",
			"cost":            cost,
			"nextRemainHours": nextRemain,
		}
		return nil
	}); err != nil {
		return nil, err
	}

	s.invalidateOverview(ctx)
	s.logger.Info("maintenance order completed", "orderId", orderID, "cost", cost)
	return res, nil
}

// CancelOrder 取消未完工工单：释放农机为空闲，并将提醒恢复为待处理。
func (s *MaintenanceService) CancelOrder(ctx context.Context, orderID string) (map[string]interface{}, error) {
	var res map[string]interface{}
	if err := s.repo.WithTx(func(tx *gorm.DB) error {
		order, err := s.repo.FindOrderByIDForUpdate(tx, orderID)
		if err != nil {
			return err
		}
		if order.Status != constants.OrderProcessing {
			return apperrors.New(constants.CodeConflict, fmt.Sprintf("工单 %s 当前状态为 %s，仅处理中的工单可取消", orderID, order.Status))
		}

		now := s.now()
		order.Status = constants.OrderCancelled
		order.UpdatedAt = now
		order.CancelledAt = &now
		if err := s.repo.SaveOrder(tx, order); err != nil {
			return err
		}

		reminder, err := s.repo.FindReminder(tx, order.ReminderID)
		if err != nil {
			return err
		}
		reminder.Status = constants.ReminderPending
		if err := s.repo.SaveReminder(tx, reminder); err != nil {
			return err
		}

		machine, err := s.repo.FindMachineByCodeForUpdate(tx, order.MachineCode)
		if err != nil {
			return err
		}
		machine.Status = constants.MachineIdle
		machine.CurrentTask = constants.MachineIdleTask
		if err := s.repo.SaveMachine(tx, machine); err != nil {
			return err
		}

		res = map[string]interface{}{
			"orderId": order.ID,
			"status":  constants.OrderCancelled,
			"message": "工单已取消，农机已释放，提醒已恢复待处理",
		}
		return nil
	}); err != nil {
		return nil, err
	}

	s.invalidateOverview(ctx)
	s.logger.Info("maintenance order cancelled", "orderId", orderID)
	return res, nil
}

// levelForHours 按剩余小时数计算提醒等级。
func levelForHours(remaining float64) string {
	switch {
	case remaining <= constants.DangerRemainingHours:
		return constants.ReminderLevelDanger
	case remaining <= constants.WarningRemainingHours:
		return constants.ReminderLevelWarning
	default:
		return constants.ReminderLevelNormal
	}
}

// invalidateOverview 失效看板缓存。
func (s *MaintenanceService) invalidateOverview(ctx context.Context) {
	if err := s.cache.Del(ctx, constants.OverviewCacheKey).Err(); err != nil {
		s.logger.Warn("invalidate overview cache failed", "err", err)
	}
}
