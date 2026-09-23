package service

import (
	"context"
	"log/slog"
	"testing"

	"github.com/agridispatch/agridispatch/internal/constants"
	"github.com/agridispatch/agridispatch/internal/dto"
	"github.com/agridispatch/agridispatch/internal/model"
	"github.com/agridispatch/agridispatch/internal/repository"
	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func newMaintenanceService(t *testing.T) (*MaintenanceService, *gorm.DB, *miniredis.Miniredis) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(
		&model.Machine{},
		&model.MaintenanceReminder{},
		&model.MaintenanceOrder{},
		&model.MaintenanceExpense{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = rdb.Close()
		mr.Close()
		_ = sqlDB.Close()
	})
	svc := NewMaintenanceService(repository.NewMaintenanceRepository(db), rdb, slog.Default())
	return svc, db, mr
}

// 作业中拒绝 → 农机空闲后开单成功 → 完工全部生效 → 重复完工不产生第二笔费用。
func TestMaintenanceFlow_RejectCreateCompleteIdempotent(t *testing.T) {
	ctx := context.Background()
	svc, db, _ := newMaintenanceService(t)

	machine := &model.Machine{ID: "m1", Code: "NJ-F-001", Name: "测试农机", Status: constants.MachineWorking, CurrentTask: "收割作业"}
	reminder := &model.MaintenanceReminder{ID: "s1", MachineCode: machine.Code, Title: "100 小时换机油", Status: constants.ReminderOpen, RemainingHours: 5}
	if err := db.Create(machine).Error; err != nil {
		t.Fatalf("seed machine: %v", err)
	}
	if err := db.Create(reminder).Error; err != nil {
		t.Fatalf("seed reminder: %v", err)
	}

	// 1. 作业中开单被拒绝
	rejected, err := svc.CreateOrder(ctx, dto.CreateMaintenanceOrderRequest{PlanDate: "2026-09-25", ServicePoint: "县农机站"}, "s1")
	if err != nil {
		t.Fatalf("create order while working: %v", err)
	}
	if rejected.Status != constants.MaintenanceOrderRejected || rejected.RejectReason != constants.RejectReasonMachineWorking {
		t.Fatalf("rejected = %+v", rejected)
	}

	// 2. 作业结束、农机空闲后再次开单成功
	db.Model(&model.Machine{}).Where("code = ?", machine.Code).Update("status", constants.MachineIdle)
	created, err := svc.CreateOrder(ctx, dto.CreateMaintenanceOrderRequest{PlanDate: "2026-09-26", ServicePoint: "镇维修点"}, "s1")
	if err != nil {
		t.Fatalf("create order after idle: %v", err)
	}
	if created.Status != constants.MaintenanceOrderProcessing {
		t.Fatalf("created = %+v", created)
	}
	var m model.Machine
	db.First(&m, "code = ?", machine.Code)
	if m.Status != constants.MachineRepair {
		t.Errorf("machine = %s, want 维修中", m.Status)
	}
	var rm model.MaintenanceReminder
	db.First(&rm, "id = ?", "s1")
	if rm.Status != constants.ReminderProcessing || rm.ActiveOrderID != created.ID {
		t.Errorf("reminder = %+v", rm)
	}

	// 3. 处理中再次开单应被拒绝（提醒节点已流转）
	again, err := svc.CreateOrder(ctx, dto.CreateMaintenanceOrderRequest{PlanDate: "2026-09-27", ServicePoint: "村维修铺"}, "s1")
	if err != nil {
		t.Fatalf("duplicate create: %v", err)
	}
	if again.Status != constants.MaintenanceOrderRejected || again.RejectReason != constants.RejectReasonReminderState {
		t.Fatalf("again = %+v", again)
	}

	// 4. 完工
	res, err := svc.CompleteOrder(ctx, dto.CompleteMaintenanceOrderRequest{ActualHours: 4.5, Cost: 1280, NextRemainHours: 100}, created.ID)
	if err != nil {
		t.Fatalf("complete order: %v", err)
	}
	if res["duplicate"] != false {
		t.Fatalf("complete result = %v", res)
	}
	db.First(&m, "code = ?", machine.Code)
	if m.Status != constants.MachineIdle || m.CurrentTask != constants.MaintenanceIdleCurrentTask {
		t.Errorf("machine after complete = %+v", m)
	}
	db.First(&rm, "id = ?", "s1")
	if rm.Status != constants.ReminderDone || rm.RemainingHours != 100 || rm.ActiveOrderID != "" {
		t.Errorf("reminder after complete = %+v", rm)
	}
	var feeCount int64
	db.Model(&model.MaintenanceExpense{}).Where("order_id = ?", created.ID).Count(&feeCount)
	if feeCount != 1 {
		t.Errorf("fee count = %d, want 1", feeCount)
	}

	// 5. 重复完工不发第二笔
	res2, err := svc.CompleteOrder(ctx, dto.CompleteMaintenanceOrderRequest{ActualHours: 99, Cost: 99999, NextRemainHours: 1}, created.ID)
	if err != nil {
		t.Fatalf("idempotent complete: %v", err)
	}
	if res2["duplicate"] != true {
		t.Errorf("expect duplicate=true, got %v", res2)
	}
	db.Model(&model.MaintenanceExpense{}).Where("order_id = ?", created.ID).Count(&feeCount)
	if feeCount != 1 {
		t.Errorf("fee count after duplicate = %d, want 1", feeCount)
	}
	var totalCost float64
	db.Model(&model.MaintenanceExpense{}).Where("order_id = ?", created.ID).Select("cost").Scan(&totalCost)
	if totalCost != 1280 {
		t.Errorf("cost = %v, want first completion value 1280", totalCost)
	}
}

// 取消未完工工单：释放农机、恢复提醒。
func TestMaintenanceFlow_CancelReleases(t *testing.T) {
	ctx := context.Background()
	svc, db, _ := newMaintenanceService(t)

	machine := &model.Machine{ID: "m1", Code: "NJ-F-002", Name: "测试农机", Status: constants.MachineIdle}
	reminder := &model.MaintenanceReminder{ID: "s2", MachineCode: machine.Code, Title: "刀盘检查", Status: constants.ReminderOpen, RemainingHours: 20}
	db.Create(machine)
	db.Create(reminder)

	order, err := svc.CreateOrder(ctx, dto.CreateMaintenanceOrderRequest{PlanDate: "2026-09-25", ServicePoint: "县农机站"}, "s2")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	res, err := svc.CancelOrder(ctx, order.ID)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if res["message"] != constants.MsgOrderCanceled {
		t.Errorf("cancel message = %v", res["message"])
	}
	var m model.Machine
	db.First(&m, "code = ?", machine.Code)
	if m.Status != constants.MachineIdle {
		t.Errorf("machine = %s, want 空闲", m.Status)
	}
	var rm model.MaintenanceReminder
	db.First(&rm, "id = ?", "s2")
	if rm.Status != constants.ReminderOpen || rm.ActiveOrderID != "" {
		t.Errorf("reminder = %+v", rm)
	}

	// 取消后应可以重新开单
	reopened, err := svc.CreateOrder(ctx, dto.CreateMaintenanceOrderRequest{PlanDate: "2026-09-28", ServicePoint: "县农机站"}, "s2")
	if err != nil {
		t.Fatalf("reopen after cancel: %v", err)
	}
	if reopened.Status != constants.MaintenanceOrderProcessing {
		t.Errorf("reopened = %+v", reopened)
	}
}

// 参数校验：缺维修点、下次剩余小时非正数。
func TestMaintenanceFlow_Validation(t *testing.T) {
	ctx := context.Background()
	svc, db, _ := newMaintenanceService(t)
	db.Create(&model.Machine{ID: "m1", Code: "NJ-F-003", Status: constants.MachineIdle})
	db.Create(&model.MaintenanceReminder{ID: "s3", MachineCode: "NJ-F-003", Title: "保养", Status: constants.ReminderOpen})

	if _, err := svc.CreateOrder(ctx, dto.CreateMaintenanceOrderRequest{PlanDate: "", ServicePoint: "县站"}, "s3"); err == nil {
		t.Error("expect error for empty plan date")
	}
	if _, err := svc.CreateOrder(ctx, dto.CreateMaintenanceOrderRequest{PlanDate: "2026-09-25", ServicePoint: "  "}, "s3"); err == nil {
		t.Error("expect error for blank service point")
	}
	if _, err := svc.CompleteOrder(ctx, dto.CompleteMaintenanceOrderRequest{ActualHours: 1, Cost: 1, NextRemainHours: 0}, "wo-missing"); err == nil {
		t.Error("expect error for non-positive next remaining hours")
	}
}
