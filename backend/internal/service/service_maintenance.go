package service

import (
	"context"
	"log/slog"
	"strings"

	"github.com/agridispatch/agridispatch/internal/constants"
	"github.com/agridispatch/agridispatch/internal/dto"
	apperrors "github.com/agridispatch/agridispatch/internal/errors"
	"github.com/agridispatch/agridispatch/internal/model"
	"github.com/agridispatch/agridispatch/internal/repository"
	"github.com/agridispatch/agridispatch/internal/util"
	"github.com/redis/go-redis/v9"
)

// MaintenanceService 保养工单业务流程。
type MaintenanceService struct {
	repo   *repository.MaintenanceRepository
	redis  *redis.Client
	logger *slog.Logger
}

func NewMaintenanceService(repo *repository.MaintenanceRepository, redis *redis.Client, logger *slog.Logger) *MaintenanceService {
	return &MaintenanceService{repo: repo, redis: redis, logger: logger}
}

// CreateOrder 从保养提醒开单。
// 农机作业中或已有未结束工单时返回被拒绝的工单（含拒绝原因）；
// 成功时提醒进入处理中、农机转为维修中。
func (s *MaintenanceService) CreateOrder(ctx context.Context, req dto.CreateMaintenanceOrderRequest, reminderID string) (*model.MaintenanceOrder, error) {
	if strings.TrimSpace(req.PlanDate) == "" {
		return nil, apperrors.New(constants.CodeBadRequest, "计划日期不能为空")
	}
	if strings.TrimSpace(req.ServicePoint) == "" {
		return nil, apperrors.New(constants.CodeBadRequest, "维修点不能为空")
	}

	order := &model.MaintenanceOrder{
		ID:           util.NewBusinessID(constants.MaintenanceOrderIDPrefix),
		ReminderID:   reminderID,
		PlanDate:     strings.TrimSpace(req.PlanDate),
		ServicePoint: strings.TrimSpace(req.ServicePoint),
	}
	result, err := s.repo.CreateOrderTx(order)
	if err != nil {
		return nil, err
	}
	s.invalidate(ctx)
	if result.RejectReason != "" {
		s.logger.Info("maintenance order rejected",
			"orderId", result.Order.ID, "reminderId", reminderID, "reason", result.RejectReason)
		return result.Order, nil
	}
	s.logger.Info("maintenance order created",
		"orderId", result.Order.ID, "reminderId", reminderID, "machine", result.Order.MachineCode)
	return result.Order, nil
}

// CompleteOrder 工单完工：工时、费用、下次剩余小时落库，
// 工单、费用记录、提醒节点和农机空闲状态一起生效。
// 重复完工幂等返回，不再生成第二笔费用。
func (s *MaintenanceService) CompleteOrder(ctx context.Context, req dto.CompleteMaintenanceOrderRequest, orderID string) (map[string]interface{}, error) {
	if req.NextRemainHours <= 0 {
		return nil, apperrors.New(constants.CodeBadRequest, "下次剩余小时必须大于 0")
	}
	if req.ActualHours < 0 || req.Cost < 0 {
		return nil, apperrors.New(constants.CodeBadRequest, "工时与费用不能为负")
	}

	expense := &model.MaintenanceExpense{
		ID:              util.NewBusinessID(constants.MaintenanceExpenseIDPrefix),
		ActualHours:     req.ActualHours,
		Cost:            req.Cost,
		NextRemainHours: req.NextRemainHours,
	}
	result, err := s.repo.CompleteOrderTx(orderID, expense)
	if err != nil {
		return nil, err
	}
	s.invalidate(ctx)
	if result.Duplicated {
		s.logger.Warn("maintenance order already completed, skip duplicate expense", "orderId", orderID)
		return map[string]interface{}{
			"order":     result.Order,
			"duplicate": true,
			"message":   constants.MsgOrderAlreadyDone,
		}, nil
	}
	s.logger.Info("maintenance order completed",
		"orderId", orderID, "machine", result.Order.MachineCode, "cost", result.Expense.Cost)
	return map[string]interface{}{
		"order":     result.Order,
		"expense":   result.Expense,
		"duplicate": false,
		"message":   constants.MsgOrderCompleted,
	}, nil
}

// CancelOrder 取消未完工工单：释放农机并恢复提醒为待开单。
func (s *MaintenanceService) CancelOrder(ctx context.Context, orderID string) (map[string]interface{}, error) {
	order, err := s.repo.CancelOrderTx(orderID)
	if err != nil {
		return nil, err
	}
	s.invalidate(ctx)
	s.logger.Info("maintenance order canceled", "orderId", orderID, "machine", order.MachineCode)
	return map[string]interface{}{
		"order":   order,
		"message": constants.MsgOrderCanceled,
	}, nil
}

// invalidate 使看板缓存失效，保证下次查询看到最新工单/农机/提醒状态。
func (s *MaintenanceService) invalidate(ctx context.Context) {
	if err := s.redis.Del(ctx, constants.OverviewCacheKey).Err(); err != nil {
		s.logger.Warn("invalidate overview cache failed", "err", err)
	}
}
