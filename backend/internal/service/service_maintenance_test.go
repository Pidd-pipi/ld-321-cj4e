package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"testing"

	"github.com/agridispatch/agridispatch/internal/constants"
	apperrors "github.com/agridispatch/agridispatch/internal/errors"
	"github.com/agridispatch/agridispatch/internal/model"
	"github.com/agridispatch/agridispatch/internal/repository"
	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

var fixtureSeq int64

// newMaintenanceFixture 构造内存 SQLite + miniredis 测试夹具。
func newMaintenanceFixture(t *testing.T) (*gorm.DB, *MaintenanceService) {
	t.Helper()
	dsn := fmt.Sprintf("file:maint_test_%d?mode=memory&cache=shared", atomic.AddInt64(&fixtureSeq, 1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	// SQLite 内存库单连接，避免事务与查询落在不同连接上。
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(
		&model.Machine{},
		&model.MaintenanceReminder{},
		&model.MaintenanceOrder{},
		&model.MaintenanceCost{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	repo := repository.NewMaintenanceRepository(db)
	svc := NewMaintenanceService(repo, rdb, slog.Default())
	return db, svc
}

func seedMachineAndReminder(t *testing.T, db *gorm.DB, machineStatus, reminderStatus string) (string, string) {
	t.Helper()
	machine := &model.Machine{ID: "m-1", Code: "NJ-T-001", Name: "测试拖拉机", Status: machineStatus, CurrentTask: ""}
	reminder := &model.MaintenanceReminder{
		ID: "s-1", MachineCode: "NJ-T-001", Title: "换机油", DueDate: "2026-10-01",
		RemainingHours: 10, Level: "warning", Status: reminderStatus,
	}
	if err := db.Create(machine).Error; err != nil {
		t.Fatalf("seed machine: %v", err)
	}
	if err := db.Create(reminder).Error; err != nil {
		t.Fatalf("seed reminder: %v", err)
	}
	return machine.Code, reminder.ID
}

func reloadMachine(t *testing.T, db *gorm.DB, code string) model.Machine {
	t.Helper()
	var m model.Machine
	if err := db.First(&m, "code = ?", code).Error; err != nil {
		t.Fatalf("reload machine: %v", err)
	}
	return m
}

func reloadReminder(t *testing.T, db *gorm.DB, id string) model.MaintenanceReminder {
	t.Helper()
	var r model.MaintenanceReminder
	if err := db.First(&r, "id = ?", id).Error; err != nil {
		t.Fatalf("reload reminder: %v", err)
	}
	return r
}

// 开单成功：提醒转处理中、农机转维修中。
func TestCreateOrderSuccess(t *testing.T) {
	db, svc := newMaintenanceFixture(t)
	code, reminderID := seedMachineAndReminder(t, db, constants.MachineIdle, constants.ReminderPending)

	res, err := svc.CreateOrder(context.Background(), reminderID, "2026-09-25", "北岭维修点")
	if err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}
	if res.Status != constants.OrderProcessing || res.RejectReason != "" {
		t.Fatalf("unexpected result: %+v", res)
	}

	machine := reloadMachine(t, db, code)
	if machine.Status != constants.MachineRepair {
		t.Errorf("machine status = %s, want %s", machine.Status, constants.MachineRepair)
	}
	reminder := reloadReminder(t, db, reminderID)
	if reminder.Status != constants.ReminderProcessed {
		t.Errorf("reminder status = %s, want %s", reminder.Status, constants.ReminderProcessed)
	}

	var order model.MaintenanceOrder
	if err := db.First(&order, "id = ?", res.OrderID).Error; err != nil {
		t.Fatalf("load order: %v", err)
	}
	if order.PlanDate != "2026-09-25" || order.RepairPoint != "北岭维修点" {
		t.Errorf("order fields wrong: %+v", order)
	}
}

// 农机作业中开单：拒绝并记录拒绝原因，状态不变。
func TestCreateOrderRejectedWhenWorking(t *testing.T) {
	db, svc := newMaintenanceFixture(t)
	code, reminderID := seedMachineAndReminder(t, db, constants.MachineWorking, constants.ReminderPending)

	res, err := svc.CreateOrder(context.Background(), reminderID, "2026-09-25", "北岭维修点")
	if err != nil {
		t.Fatalf("CreateOrder should not hard-fail: %v", err)
	}
	if res.Status != constants.OrderRejected || res.RejectReason != constants.RejectReasonWorking {
		t.Fatalf("unexpected result: %+v", res)
	}

	machine := reloadMachine(t, db, code)
	if machine.Status != constants.MachineWorking {
		t.Errorf("machine status changed to %s", machine.Status)
	}
	reminder := reloadReminder(t, db, reminderID)
	if reminder.Status != constants.ReminderPending {
		t.Errorf("reminder status changed to %s", reminder.Status)
	}

	var order model.MaintenanceOrder
	if err := db.First(&order, "id = ?", res.OrderID).Error; err != nil {
		t.Fatalf("rejected order not persisted: %v", err)
	}
	if order.RejectReason == "" {
		t.Error("reject reason should be persisted")
	}
}

// 已有未结束工单再开单：拒绝。
func TestCreateOrderRejectedWhenOpenOrderExists(t *testing.T) {
	db, svc := newMaintenanceFixture(t)
	code, reminderID := seedMachineAndReminder(t, db, constants.MachineIdle, constants.ReminderPending)
	// 先成功开一单
	if _, err := svc.CreateOrder(context.Background(), reminderID, "2026-09-25", "北岭维修点"); err != nil {
		t.Fatalf("first CreateOrder: %v", err)
	}
	// 再补一个同农机的待处理提醒，尝试开单
	second := &model.MaintenanceReminder{ID: "s-2", MachineCode: code, Title: "滤芯", Status: constants.ReminderPending}
	if err := db.Create(second).Error; err != nil {
		t.Fatalf("seed second reminder: %v", err)
	}
	res, err := svc.CreateOrder(context.Background(), second.ID, "2026-09-26", "北岭维修点")
	if err != nil {
		t.Fatalf("CreateOrder should not hard-fail: %v", err)
	}
	if res.Status != constants.OrderRejected || res.RejectReason != constants.RejectReasonOpenOrder {
		t.Fatalf("unexpected result: %+v", res)
	}
	// 第二个提醒不应被占用
	if got := reloadReminder(t, db, second.ID).Status; got != constants.ReminderPending {
		t.Errorf("second reminder status = %s, want 待处理", got)
	}
}

// 提醒处于处理中（已开过单）再开单：拒绝。
func TestCreateOrderRejectedWhenReminderProcessing(t *testing.T) {
	db, svc := newMaintenanceFixture(t)
	_, reminderID := seedMachineAndReminder(t, db, constants.MachineRepair, constants.ReminderProcessed)
	existing := &model.MaintenanceOrder{ID: "mo-old", ReminderID: reminderID, MachineCode: "NJ-T-001", Status: constants.OrderProcessing}
	if err := db.Create(existing).Error; err != nil {
		t.Fatalf("seed order: %v", err)
	}

	res, err := svc.CreateOrder(context.Background(), reminderID, "2026-09-25", "北岭维修点")
	if err != nil {
		t.Fatalf("CreateOrder should not hard-fail: %v", err)
	}
	if res.Status != constants.OrderRejected || res.RejectReason != constants.RejectReasonOpenOrder {
		t.Fatalf("unexpected result: %+v", res)
	}
}

// 完工：工单/费用/提醒/农机空闲同时生效；重复完工不产生第二笔费用。
func TestCompleteOrderAndIdempotency(t *testing.T) {
	db, svc := newMaintenanceFixture(t)
	code, reminderID := seedMachineAndReminder(t, db, constants.MachineIdle, constants.ReminderPending)

	created, err := svc.CreateOrder(context.Background(), reminderID, "2026-09-25", "北岭维修点")
	if err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}

	res, err := svc.CompleteOrder(context.Background(), created.OrderID, 6.5, 820.0, 100.0)
	if err != nil {
		t.Fatalf("CompleteOrder: %v", err)
	}
	if res["status"] != constants.OrderCompleted {
		t.Fatalf("unexpected: %+v", res)
	}

	machine := reloadMachine(t, db, code)
	if machine.Status != constants.MachineIdle {
		t.Errorf("machine status = %s, want 空闲", machine.Status)
	}
	reminder := reloadReminder(t, db, reminderID)
	if reminder.Status != constants.ReminderDone || reminder.RemainingHours != 100.0 {
		t.Errorf("reminder wrong: %+v", reminder)
	}
	if reminder.Level != constants.ReminderLevelNormal {
		t.Errorf("reminder level = %s, want normal", reminder.Level)
	}

	var costCount int64
	db.Model(&model.MaintenanceCost{}).Where("order_id = ?", created.OrderID).Count(&costCount)
	if costCount != 1 {
		t.Fatalf("cost count = %d, want 1", costCount)
	}

	// 重复完工：报冲突错误，费用仍是一笔
	_, err = svc.CompleteOrder(context.Background(), created.OrderID, 9.9, 999.0, 50.0)
	var bizErr *apperrors.BusinessError
	if !errors.As(err, &bizErr) || bizErr.Code != constants.CodeConflict {
		t.Fatalf("expect conflict business error, got %v", err)
	}
	db.Model(&model.MaintenanceCost{}).Where("order_id = ?", created.OrderID).Count(&costCount)
	if costCount != 1 {
		t.Errorf("cost count after repeat = %d, want 1", costCount)
	}
}

// 取消未完工工单：农机释放、提醒恢复待处理；已完工工单不可取消。
func TestCancelOrder(t *testing.T) {
	db, svc := newMaintenanceFixture(t)
	code, reminderID := seedMachineAndReminder(t, db, constants.MachineIdle, constants.ReminderPending)

	created, err := svc.CreateOrder(context.Background(), reminderID, "2026-09-25", "北岭维修点")
	if err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}

	if _, err := svc.CancelOrder(context.Background(), created.OrderID); err != nil {
		t.Fatalf("CancelOrder: %v", err)
	}
	if got := reloadMachine(t, db, code).Status; got != constants.MachineIdle {
		t.Errorf("machine status = %s, want 空闲", got)
	}
	if got := reloadReminder(t, db, reminderID).Status; got != constants.ReminderPending {
		t.Errorf("reminder status = %s, want 待处理", got)
	}

	// 再次取消应报冲突
	if _, err := svc.CancelOrder(context.Background(), created.OrderID); err == nil {
		t.Error("second cancel should fail")
	}

	// 取消后应能用同一提醒重新开单
	recreated, err := svc.CreateOrder(context.Background(), reminderID, "2026-09-27", "南湾维修点")
	if err != nil {
		t.Fatalf("reopen after cancel: %v", err)
	}
	if recreated.Status != constants.OrderProcessing {
		t.Fatalf("reopen result = %+v", recreated)
	}
}
