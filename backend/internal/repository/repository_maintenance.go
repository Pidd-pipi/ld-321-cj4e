package repository

import (
	"errors"
	"fmt"

	"github.com/agridispatch/agridispatch/internal/constants"
	"github.com/agridispatch/agridispatch/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// clauseLocking 行级锁，保证工单状态并发安全。
var clauseLocking = clause.Locking{Strength: "UPDATE"}

// openOrderStatus 未结束工单状态。
const openOrderStatus = constants.OrderProcessing

// MaintenanceRepository 保养工单数据访问。
type MaintenanceRepository struct {
	db *gorm.DB
}

func NewMaintenanceRepository(db *gorm.DB) *MaintenanceRepository {
	return &MaintenanceRepository{db: db}
}

// ListOrders 查询全部保养工单（最近创建在前）。
func (r *MaintenanceRepository) ListOrders() ([]model.MaintenanceOrder, error) {
	var orders []model.MaintenanceOrder
	if err := r.db.Order("created_at DESC").Find(&orders).Error; err != nil {
		return nil, fmt.Errorf("list maintenance orders: %w", err)
	}
	return orders, nil
}

// ListCosts 查询全部保养费用记录。
func (r *MaintenanceRepository) ListCosts() ([]model.MaintenanceCost, error) {
	var costs []model.MaintenanceCost
	if err := r.db.Order("created_at DESC").Find(&costs).Error; err != nil {
		return nil, fmt.Errorf("list maintenance costs: %w", err)
	}
	return costs, nil
}

// FindReminder 按 ID 查询保养提醒节点。
func (r *MaintenanceRepository) FindReminder(tx *gorm.DB, id string) (*model.MaintenanceReminder, error) {
	session := r.session(tx)
	var reminder model.MaintenanceReminder
	err := session.First(&reminder, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find reminder: %w", err)
	}
	return &reminder, nil
}

// FindMachineByCodeForUpdate 行级锁定查询农机，避免并发开单/完工竞态。
func (r *MaintenanceRepository) FindMachineByCodeForUpdate(tx *gorm.DB, code string) (*model.Machine, error) {
	session := r.withLock(r.session(tx))
	var machine model.Machine
	err := session.First(&machine, "code = ?", code).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find machine for update: %w", err)
	}
	return &machine, nil
}

// FindOrderByIDForUpdate 行级锁定查询工单。
func (r *MaintenanceRepository) FindOrderByIDForUpdate(tx *gorm.DB, id string) (*model.MaintenanceOrder, error) {
	session := r.withLock(r.session(tx))
	var order model.MaintenanceOrder
	err := session.First(&order, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find order for update: %w", err)
	}
	return &order, nil
}

// HasOpenOrderByMachine 判断农机是否存在未结束工单（处理中）。
func (r *MaintenanceRepository) HasOpenOrderByMachine(tx *gorm.DB, code string) (bool, error) {
	session := r.session(tx)
	var count int64
	if err := session.Model(&model.MaintenanceOrder{}).
		Where("machine_code = ? AND status = ?", code, openOrderStatus).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("count open orders: %w", err)
	}
	return count > 0, nil
}

// CreateOrder 创建工单。
func (r *MaintenanceRepository) CreateOrder(tx *gorm.DB, order *model.MaintenanceOrder) error {
	if err := r.session(tx).Create(order).Error; err != nil {
		return fmt.Errorf("create order: %w", err)
	}
	return nil
}

// CreateCost 创建费用记录。
func (r *MaintenanceRepository) CreateCost(tx *gorm.DB, cost *model.MaintenanceCost) error {
	if err := r.session(tx).Create(cost).Error; err != nil {
		return fmt.Errorf("create cost: %w", err)
	}
	return nil
}

// SaveReminder 保存保养提醒节点。
func (r *MaintenanceRepository) SaveReminder(tx *gorm.DB, reminder *model.MaintenanceReminder) error {
	if err := r.session(tx).Save(reminder).Error; err != nil {
		return fmt.Errorf("save reminder: %w", err)
	}
	return nil
}

// SaveMachine 保存农机。
func (r *MaintenanceRepository) SaveMachine(tx *gorm.DB, machine *model.Machine) error {
	if err := r.session(tx).Save(machine).Error; err != nil {
		return fmt.Errorf("save machine: %w", err)
	}
	return nil
}

// SaveOrder 保存工单。
func (r *MaintenanceRepository) SaveOrder(tx *gorm.DB, order *model.MaintenanceOrder) error {
	if err := r.session(tx).Save(order).Error; err != nil {
		return fmt.Errorf("save order: %w", err)
	}
	return nil
}

// WithTx 在事务中执行操作。
func (r *MaintenanceRepository) WithTx(fn func(tx *gorm.DB) error) error {
	if err := r.db.Transaction(fn); err != nil {
		return err
	}
	return nil
}

func (r *MaintenanceRepository) session(tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx
	}
	return r.db
}

// withLock 在支持的方言（MySQL）上追加行级锁；SQLite 等测试方言会忽略。
func (r *MaintenanceRepository) withLock(db *gorm.DB) *gorm.DB {
	if db.Dialector.Name() == "mysql" {
		return db.Clauses(clauseLocking)
	}
	return db
}
