package repository

import (
	"testing"

	"github.com/agridispatch/agridispatch/internal/constants"
	apperrors "github.com/agridispatch/agridispatch/internal/errors"
	"github.com/agridispatch/agridispatch/internal/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	// 内存库需要单连接，避免连接池里各连接看到不同的内存数据库。
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(
		&model.Machine{},
		&model.MaintenanceReminder{},
		&model.MaintenanceOrder{},
		&model.MaintenanceExpense{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})
	return db
}

func seedMachineAndReminder(t *testing.T, db *gorm.DB, machineStatus, reminderStatus string) (*model.Machine, *model.MaintenanceReminder) {
	t.Helper()
	m := &model.Machine{ID: "m-1", Code: "NJ-T-001", Name: "测试拖拉机", Status: machineStatus, CurrentTask: "测试任务"}
	r := &model.MaintenanceReminder{ID: "r-1", MachineCode: m.Code, Title: "换机油", Status: reminderStatus, RemainingHours: 8}
	if err := db.Create(m).Error; err != nil {
		t.Fatalf("seed machine: %v", err)
	}
	if err := db.Create(r).Error; err != nil {
		t.Fatalf("seed reminder: %v", err)
	}
	return m, r
}

// 开单成功：提醒转处理中、农机转维修中。
func TestCreateOrder_Success(t *testing.T) {
	db := newTestDB(t)
	seedMachineAndReminder(t, db, constants.MachineIdle, constants.ReminderOpen)
	repo := NewMaintenanceRepository(db)

	order := &model.MaintenanceOrder{ID: "wo-1", ReminderID: "r-1", PlanDate: "2026-09-25", ServicePoint: "县农机站"}
	res, err := repo.CreateOrderTx(order)
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if res.RejectReason != "" || res.Order.Status != constants.MaintenanceOrderProcessing {
		t.Fatalf("unexpected order: %+v", res.Order)
	}

	var reminder model.MaintenanceReminder
	db.First(&reminder, "id = ?", "r-1")
	if reminder.Status != constants.ReminderProcessing || reminder.ActiveOrderID != "wo-1" {
		t.Errorf("reminder = %+v", reminder)
	}
	var machine model.Machine
	db.First(&machine, "code = ?", "NJ-T-001")
	if machine.Status != constants.MachineRepair {
		t.Errorf("machine status = %s, want %s", machine.Status, constants.MachineRepair)
	}
}

// 农机作业中开单被拒绝，并记录拒绝原因，状态不变。
func TestCreateOrder_RejectWhenWorking(t *testing.T) {
	db := newTestDB(t)
	seedMachineAndReminder(t, db, constants.MachineWorking, constants.ReminderOpen)
	repo := NewMaintenanceRepository(db)

	order := &model.MaintenanceOrder{ID: "wo-2", ReminderID: "r-1", PlanDate: "2026-09-25", ServicePoint: "县农机站"}
	res, err := repo.CreateOrderTx(order)
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if res.Order.Status != constants.MaintenanceOrderRejected || res.RejectReason != constants.RejectReasonMachineWorking {
		t.Fatalf("order = %+v reason=%q", res.Order, res.RejectReason)
	}
	if res.Order.MachineCode != "NJ-T-001" || res.Order.Title != "换机油" {
		t.Errorf("rejected order missing machine/title: %+v", res.Order)
	}
	var machine model.Machine
	db.First(&machine, "code = ?", "NJ-T-001")
	if machine.Status != constants.MachineWorking {
		t.Errorf("machine status changed to %s", machine.Status)
	}
	var reminder model.MaintenanceReminder
	db.First(&reminder, "id = ?", "r-1")
	if reminder.Status != constants.ReminderOpen {
		t.Errorf("reminder status changed to %s", reminder.Status)
	}
}

// 已有未结束工单再次开单被拒绝。
func TestCreateOrder_RejectWhenOpenOrderExists(t *testing.T) {
	db := newTestDB(t)
	seedMachineAndReminder(t, db, constants.MachineIdle, constants.ReminderOpen)
	existing := &model.MaintenanceOrder{
		ID: "wo-exist", ReminderID: "r-1", MachineCode: "NJ-T-001", Title: "换机油",
		PlanDate: "2026-09-24", ServicePoint: "县农机站", Status: constants.MaintenanceOrderProcessing,
	}
	if err := db.Create(existing).Error; err != nil {
		t.Fatalf("seed existing order: %v", err)
	}
	repo := NewMaintenanceRepository(db)

	order := &model.MaintenanceOrder{ID: "wo-3", ReminderID: "r-1", PlanDate: "2026-09-26", ServicePoint: "维修点 B"}
	res, err := repo.CreateOrderTx(order)
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if res.Order.Status != constants.MaintenanceOrderRejected || res.RejectReason != constants.RejectReasonOrderOpen {
		t.Fatalf("order = %+v", res.Order)
	}
}

// 提醒非待开单状态时拒绝。
func TestCreateOrder_RejectWhenReminderNotOpen(t *testing.T) {
	db := newTestDB(t)
	seedMachineAndReminder(t, db, constants.MachineIdle, constants.ReminderProcessing)
	repo := NewMaintenanceRepository(db)

	order := &model.MaintenanceOrder{ID: "wo-4", ReminderID: "r-1", PlanDate: "2026-09-26", ServicePoint: "维修点 B"}
	res, err := repo.CreateOrderTx(order)
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if res.Order.Status != constants.MaintenanceOrderRejected || res.RejectReason != constants.RejectReasonReminderState {
		t.Fatalf("order = %+v", res.Order)
	}
}

// 完工：工单/费用/提醒/农机一起生效，重复完工不再产生第二笔费用。
func TestCompleteOrder_AtomicAndIdempotent(t *testing.T) {
	db := newTestDB(t)
	seedMachineAndReminder(t, db, constants.MachineRepair, constants.ReminderProcessing)
	order := &model.MaintenanceOrder{
		ID: "wo-c", ReminderID: "r-1", MachineCode: "NJ-T-001", Title: "换机油",
		PlanDate: "2026-09-25", ServicePoint: "县农机站", Status: constants.MaintenanceOrderProcessing,
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("seed order: %v", err)
	}
	repo := NewMaintenanceRepository(db)

	expense := &model.MaintenanceExpense{ID: "fee-1", ActualHours: 3.5, Cost: 860, NextRemainHours: 100}
	res, err := repo.CompleteOrderTx("wo-c", expense)
	if err != nil {
		t.Fatalf("complete order: %v", err)
	}
	if res.Duplicated || res.Expense.OrderID != "wo-c" {
		t.Fatalf("unexpected complete result: %+v", res)
	}

	var savedOrder model.MaintenanceOrder
	db.First(&savedOrder, "id = ?", "wo-c")
	if savedOrder.Status != constants.MaintenanceOrderDone || savedOrder.CompletedAt == nil {
		t.Errorf("order = %+v", savedOrder)
	}
	var feeCount int64
	db.Model(&model.MaintenanceExpense{}).Where("order_id = ?", "wo-c").Count(&feeCount)
	if feeCount != 1 {
		t.Errorf("expense count = %d, want 1", feeCount)
	}
	var reminder model.MaintenanceReminder
	db.First(&reminder, "id = ?", "r-1")
	if reminder.Status != constants.ReminderDone || reminder.RemainingHours != 100 || reminder.ActiveOrderID != "" {
		t.Errorf("reminder = %+v", reminder)
	}
	var machine model.Machine
	db.First(&machine, "code = ?", "NJ-T-001")
	if machine.Status != constants.MachineIdle || machine.CurrentTask != constants.MaintenanceIdleCurrentTask {
		t.Errorf("machine = %+v", machine)
	}

	// 重复完工：幂等返回，不产生第二笔费用。
	res2, err := repo.CompleteOrderTx("wo-c", &model.MaintenanceExpense{ID: "fee-2", ActualHours: 9, Cost: 9999, NextRemainHours: 5})
	if err != nil {
		t.Fatalf("duplicate complete: %v", err)
	}
	if !res2.Duplicated {
		t.Errorf("expect duplicated=true")
	}
	db.Model(&model.MaintenanceExpense{}).Where("order_id = ?", "wo-c").Count(&feeCount)
	if feeCount != 1 {
		t.Errorf("expense count after duplicate = %d, want 1", feeCount)
	}
}

// 取消未完工工单：释放农机、恢复提醒。
func TestCancelOrder_ReleasesMachineAndRestoresReminder(t *testing.T) {
	db := newTestDB(t)
	seedMachineAndReminder(t, db, constants.MachineRepair, constants.ReminderProcessing)
	order := &model.MaintenanceOrder{
		ID: "wo-x", ReminderID: "r-1", MachineCode: "NJ-T-001", Title: "换机油",
		PlanDate: "2026-09-25", ServicePoint: "县农机站", Status: constants.MaintenanceOrderProcessing,
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("seed order: %v", err)
	}
	repo := NewMaintenanceRepository(db)

	canceled, err := repo.CancelOrderTx("wo-x")
	if err != nil {
		t.Fatalf("cancel order: %v", err)
	}
	if canceled.Status != constants.MaintenanceOrderCanceled {
		t.Errorf("order status = %s", canceled.Status)
	}
	var reminder model.MaintenanceReminder
	db.First(&reminder, "id = ?", "r-1")
	if reminder.Status != constants.ReminderOpen || reminder.ActiveOrderID != "" {
		t.Errorf("reminder = %+v", reminder)
	}
	var machine model.Machine
	db.First(&machine, "code = ?", "NJ-T-001")
	if machine.Status != constants.MachineIdle {
		t.Errorf("machine status = %s, want 空闲", machine.Status)
	}

	// 已完工工单取消应报错。
	done := &model.MaintenanceOrder{
		ID: "wo-done", ReminderID: "r-1", MachineCode: "NJ-T-001", Title: "换机油",
		Status: constants.MaintenanceOrderDone,
	}
	if err := db.Create(done).Error; err != nil {
		t.Fatalf("seed done: %v", err)
	}
	if _, err := repo.CancelOrderTx("wo-done"); err != apperrors.ErrOrderAlreadyCompleted {
		t.Errorf("done order cancel err = %v", err)
	}
}

// 对已取消/已拒绝工单完工应返回哨兵错误。
func TestCompleteOrder_InvalidStates(t *testing.T) {
	db := newTestDB(t)
	seedMachineAndReminder(t, db, constants.MachineRepair, constants.ReminderOpen)
	canceled := &model.MaintenanceOrder{
		ID: "wo-canceled", ReminderID: "r-1", MachineCode: "NJ-T-001", Title: "换机油",
		Status: constants.MaintenanceOrderCanceled,
	}
	rejected := &model.MaintenanceOrder{
		ID: "wo-rejected", ReminderID: "r-1", MachineCode: "NJ-T-001", Title: "换机油",
		Status: constants.MaintenanceOrderRejected, RejectReason: constants.RejectReasonMachineWorking,
	}
	if err := db.Create(canceled).Error; err != nil {
		t.Fatalf("seed canceled: %v", err)
	}
	if err := db.Create(rejected).Error; err != nil {
		t.Fatalf("seed rejected: %v", err)
	}
	repo := NewMaintenanceRepository(db)

	if _, err := repo.CompleteOrderTx("wo-canceled", &model.MaintenanceExpense{ID: "f-c", NextRemainHours: 100}); err != apperrors.ErrOrderAlreadyCanceled {
		t.Errorf("canceled order complete err = %v", err)
	}
	if _, err := repo.CompleteOrderTx("wo-rejected", &model.MaintenanceExpense{ID: "f-r", NextRemainHours: 100}); err != apperrors.ErrOrderRejected {
		t.Errorf("rejected order complete err = %v", err)
	}
}
