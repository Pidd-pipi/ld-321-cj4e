package repository

import (
	"errors"
	"fmt"
	"time"

	"github.com/agridispatch/agridispatch/internal/constants"
	apperrors "github.com/agridispatch/agridispatch/internal/errors"
	"github.com/agridispatch/agridispatch/internal/model"
	"gorm.io/gorm"
)

// MaintenanceRepository 保养工单数据访问。
type MaintenanceRepository struct {
	db *gorm.DB
}

func NewMaintenanceRepository(db *gorm.DB) *MaintenanceRepository {
	return &MaintenanceRepository{db: db}
}

// ListOrders 查询全部保养工单（新单在前）。
func (r *MaintenanceRepository) ListOrders() ([]model.MaintenanceOrder, error) {
	var orders []model.MaintenanceOrder
	if err := r.db.Order("created_at DESC").Find(&orders).Error; err != nil {
		return nil, fmt.Errorf("list maintenance orders: %w", err)
	}
	return orders, nil
}

// ListExpenses 查询全部保养费用记录。
func (r *MaintenanceRepository) ListExpenses() ([]model.MaintenanceExpense, error) {
	var expenses []model.MaintenanceExpense
	if err := r.db.Order("created_at DESC").Find(&expenses).Error; err != nil {
		return nil, fmt.Errorf("list maintenance expenses: %w", err)
	}
	return expenses, nil
}

// FindReminder 按 ID 查询保养提醒。
func (r *MaintenanceRepository) FindReminder(id string) (*model.MaintenanceReminder, error) {
	var reminder model.MaintenanceReminder
	err := r.db.First(&reminder, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find maintenance reminder: %w", err)
	}
	return &reminder, nil
}

// FindOrder 按 ID 查询保养工单。
func (r *MaintenanceRepository) FindOrder(id string) (*model.MaintenanceOrder, error) {
	var order model.MaintenanceOrder
	err := r.db.First(&order, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find maintenance order: %w", err)
	}
	return &order, nil
}

// hasOpenOrderForUpdate 判断农机是否存在未结束（处理中）工单，需在事务内调用。
func hasOpenOrderForUpdate(tx *gorm.DB, machineCode string) (bool, error) {
	var count int64
	if err := tx.Model(&model.MaintenanceOrder{}).
		Where("machine_code = ? AND status = ?", machineCode, constants.MaintenanceOrderProcessing).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("count open maintenance orders: %w", err)
	}
	return count > 0, nil
}

// CreateOrderTxResult 开单事务结果。
type CreateOrderTxResult struct {
	Order        *model.MaintenanceOrder
	RejectReason string
}

// CreateOrderTx 从提醒开单：
// 农机作业中或已有未结束工单时拒绝（落一条已拒绝工单留痕），
// 否则创建处理中工单，提醒转处理中、农机转维修中。
func (r *MaintenanceRepository) CreateOrderTx(order *model.MaintenanceOrder) (*CreateOrderTxResult, error) {
	result := &CreateOrderTxResult{}
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var reminder model.MaintenanceReminder
		if err := firstForUpdate(tx, &reminder, "id = ?", order.ReminderID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return fmt.Errorf("lock maintenance reminder: %w", err)
		}
		if reminder.Status != constants.ReminderOpen {
			order.MachineCode = reminder.MachineCode
			order.Title = reminder.Title
			order.Status = constants.MaintenanceOrderRejected
			order.RejectReason = constants.RejectReasonReminderState
			if err := tx.Create(order).Error; err != nil {
				return fmt.Errorf("create rejected maintenance order: %w", err)
			}
			result.Order = order
			result.RejectReason = order.RejectReason
			return nil
		}

		var machine model.Machine
		if err := firstForUpdate(tx, &machine, "code = ?", reminder.MachineCode).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return fmt.Errorf("lock machine: %w", err)
		}

		rejectReason := ""
		switch {
		case machine.Status == constants.MachineWorking:
			rejectReason = constants.RejectReasonMachineWorking
		default:
			open, err := hasOpenOrderForUpdate(tx, machine.Code)
			if err != nil {
				return err
			}
			if open {
				rejectReason = constants.RejectReasonOrderOpen
			}
		}
		if rejectReason != "" {
			order.MachineCode = machine.Code
			order.Title = reminder.Title
			order.Status = constants.MaintenanceOrderRejected
			order.RejectReason = rejectReason
			if err := tx.Create(order).Error; err != nil {
				return fmt.Errorf("create rejected maintenance order: %w", err)
			}
			result.Order = order
			result.RejectReason = rejectReason
			return nil
		}

		// 开单成功：工单处理中、提醒处理中、农机维修中。
		order.MachineCode = machine.Code
		order.Title = reminder.Title
		order.Status = constants.MaintenanceOrderProcessing
		if err := tx.Create(order).Error; err != nil {
			return fmt.Errorf("create maintenance order: %w", err)
		}
		reminder.Status = constants.ReminderProcessing
		reminder.ActiveOrderID = order.ID
		if err := tx.Save(&reminder).Error; err != nil {
			return fmt.Errorf("update reminder to processing: %w", err)
		}
		machine.Status = constants.MachineRepair
		machine.CurrentTask = "维修保养 " + order.ServicePoint
		if err := tx.Save(&machine).Error; err != nil {
			return fmt.Errorf("update machine to repair: %w", err)
		}
		result.Order = order
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// CompleteOrderTxResult 完工事务结果。
type CompleteOrderTxResult struct {
	Order      *model.MaintenanceOrder
	Expense    *model.MaintenanceExpense
	Duplicated bool
}

// CompleteOrderTx 完工：工单、费用记录、提醒节点、农机空闲状态在同一事务生效；
// 工单已是完工状态时幂等返回，不再生成第二笔费用。
func (r *MaintenanceRepository) CompleteOrderTx(orderID string, expense *model.MaintenanceExpense) (*CompleteOrderTxResult, error) {
	result := &CompleteOrderTxResult{}
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var order model.MaintenanceOrder
		if err := firstForUpdate(tx, &order, "id = ?", orderID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return fmt.Errorf("lock maintenance order: %w", err)
		}
		switch order.Status {
		case constants.MaintenanceOrderDone:
			result.Order = &order
			result.Duplicated = true
			return nil
		case constants.MaintenanceOrderCanceled:
			return apperrors.ErrOrderAlreadyCanceled
		case constants.MaintenanceOrderRejected:
			return apperrors.ErrOrderRejected
		case constants.MaintenanceOrderProcessing:
			// 继续完工流程
		default:
			return apperrors.ErrOrderNotProcessing
		}

		now := time.Now()
		paidAt := now.Format("2006-01-02 15:04")
		order.Status = constants.MaintenanceOrderDone
		order.ActualHours = expense.ActualHours
		order.Cost = expense.Cost
		order.NextRemainHours = expense.NextRemainHours
		order.CompletedAt = &now
		if err := tx.Save(&order).Error; err != nil {
			return fmt.Errorf("complete maintenance order: %w", err)
		}

		expense.OrderID = order.ID
		expense.MachineCode = order.MachineCode
		expense.ReminderID = order.ReminderID
		expense.Title = order.Title
		expense.ServicePoint = order.ServicePoint
		expense.PaidAt = paidAt
		if err := tx.Create(expense).Error; err != nil {
			return fmt.Errorf("create maintenance expense: %w", err)
		}

		var reminder model.MaintenanceReminder
		if err := firstForUpdate(tx, &reminder, "id = ?", order.ReminderID).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("lock reminder on complete: %w", err)
			}
		} else {
			reminder.Status = constants.ReminderDone
			reminder.ActiveOrderID = ""
			reminder.RemainingHours = expense.NextRemainHours
			reminder.Level = levelByRemaining(expense.NextRemainHours)
			reminder.LastServiceRecord = fmt.Sprintf("%s 已完工：%s，工时 %.1fh，费用 ¥%.2f",
				paidAt, order.ServicePoint, expense.ActualHours, expense.Cost)
			if err := tx.Save(&reminder).Error; err != nil {
				return fmt.Errorf("update reminder on complete: %w", err)
			}
		}

		var machine model.Machine
		if err := firstForUpdate(tx, &machine, "code = ?", order.MachineCode).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("lock machine on complete: %w", err)
			}
		} else {
			machine.Status = constants.MachineIdle
			machine.CurrentTask = constants.MaintenanceIdleCurrentTask
			if err := tx.Save(&machine).Error; err != nil {
				return fmt.Errorf("release machine on complete: %w", err)
			}
		}

		result.Order = &order
		result.Expense = expense
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// CancelOrderTx 取消未完工工单：释放农机并恢复提醒。
func (r *MaintenanceRepository) CancelOrderTx(orderID string) (*model.MaintenanceOrder, error) {
	var saved *model.MaintenanceOrder
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var order model.MaintenanceOrder
		if err := firstForUpdate(tx, &order, "id = ?", orderID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return fmt.Errorf("lock maintenance order: %w", err)
		}
		switch order.Status {
		case constants.MaintenanceOrderDone:
			return apperrors.ErrOrderAlreadyCompleted
		case constants.MaintenanceOrderCanceled:
			return apperrors.ErrOrderAlreadyCanceled
		case constants.MaintenanceOrderRejected:
			return apperrors.ErrOrderRejected
		case constants.MaintenanceOrderProcessing:
			// 继续取消流程
		default:
			return apperrors.ErrOrderNotProcessing
		}

		order.Status = constants.MaintenanceOrderCanceled
		if err := tx.Save(&order).Error; err != nil {
			return fmt.Errorf("cancel maintenance order: %w", err)
		}

		var reminder model.MaintenanceReminder
		if err := firstForUpdate(tx, &reminder, "id = ?", order.ReminderID).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("lock reminder on cancel: %w", err)
			}
		} else {
			reminder.Status = constants.ReminderOpen
			reminder.ActiveOrderID = ""
			if err := tx.Save(&reminder).Error; err != nil {
				return fmt.Errorf("restore reminder on cancel: %w", err)
			}
		}

		var machine model.Machine
		if err := firstForUpdate(tx, &machine, "code = ?", order.MachineCode).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("lock machine on cancel: %w", err)
			}
		} else {
			machine.Status = constants.MachineIdle
			machine.CurrentTask = constants.MaintenanceIdleCurrentTask
			if err := tx.Save(&machine).Error; err != nil {
				return fmt.Errorf("release machine on cancel: %w", err)
			}
		}

		saved = &order
		return nil
	})
	if err != nil {
		return nil, err
	}
	return saved, nil
}

// levelByRemaining 根据下次剩余小时数推导预警等级。
func levelByRemaining(hours float64) string {
	switch {
	case hours <= 10:
		return constants.ReminderLevelDanger
	case hours <= 30:
		return constants.ReminderLevelWarning
	default:
		return constants.ReminderLevelNormal
	}
}
