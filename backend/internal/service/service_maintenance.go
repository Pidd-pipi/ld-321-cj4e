package service

import (
	"log/slog"
	"time"

	"github.com/agridispatch/agridispatch/internal/constants"
	"github.com/agridispatch/agridispatch/internal/repository"
	"github.com/redis/go-redis/v9"
)

// MaintenanceService 保养工单业务。
type MaintenanceService struct {
	repo     *repository.MaintenanceRepository
	cache    *redis.Client
	logger   *slog.Logger
	now      func() time.Time
	idPrefix string
}

// NewMaintenanceService 构造保养服务。
func NewMaintenanceService(repo *repository.MaintenanceRepository, cache *redis.Client, logger *slog.Logger) *MaintenanceService {
	return &MaintenanceService{
		repo:     repo,
		cache:    cache,
		logger:   logger,
		now:      time.Now,
		idPrefix: constants.MaintenanceOrderIDPrefix,
	}
}

// CreateOrderResult 开单结果。
type CreateOrderResult struct {
	OrderID      string `json:"orderId"`
	Status       string `json:"status"`
	Message      string `json:"message"`
	RejectReason string `json:"rejectReason,omitempty"`
}
