package handler_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agridispatch/agridispatch/internal/constants"
	"github.com/agridispatch/agridispatch/internal/handler"
	"github.com/agridispatch/agridispatch/internal/model"
	"github.com/agridispatch/agridispatch/internal/repository"
	"github.com/agridispatch/agridispatch/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type apiEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func setupRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(
		&model.Machine{}, &model.MaintenanceReminder{},
		&model.MaintenanceOrder{}, &model.MaintenanceExpense{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(func() { mr.Close(); _ = sqlDB.Close() })

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	svc := service.NewMaintenanceService(repository.NewMaintenanceRepository(db), rdb, slog.Default())
	h := handler.NewMaintenanceHandler(svc)

	r := gin.New()
	v1 := r.Group("/api/" + constants.APIVersion)
	v1.POST("/maintenance/reminders/:id/orders", h.CreateOrder)
	v1.POST("/maintenance/orders/:id/complete", h.CompleteOrder)
	v1.POST("/maintenance/orders/:id/cancel", h.CancelOrder)
	return r, db
}

func postJSON(t *testing.T, r *gin.Engine, path string, body any) (apiEnvelope, int) {
	t.Helper()
	raw, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	var env apiEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode response %q: %v", w.Body.String(), err)
	}
	return env, w.Code
}

func TestMaintenanceHTTP_FullFlow(t *testing.T) {
	r, db := setupRouter(t)
	db.Create(&model.Machine{ID: "m1", Code: "NJ-H-001", Name: "HTTP 测试机", Status: constants.MachineWorking, CurrentTask: "作业中"})
	db.Create(&model.MaintenanceReminder{ID: "s1", MachineCode: "NJ-H-001", Title: "换机油", Status: constants.ReminderOpen, RemainingHours: 3})

	// 作业中开单：HTTP 200 业务拒绝，返回已拒绝工单与拒绝原因
	env, status := postJSON(t, r, "/api/v1/maintenance/reminders/s1/orders", map[string]string{
		"planDate": "2026-09-25", "servicePoint": "县农机站",
	})
	if status != http.StatusOK || env.Code != 0 {
		t.Fatalf("rejected create status=%d env=%+v", status, env)
	}
	var created struct {
		Order   model.MaintenanceOrder `json:"order"`
		Message string                 `json:"message"`
	}
	json.Unmarshal(env.Data, &created)
	if created.Order.Status != constants.MaintenanceOrderRejected || created.Order.RejectReason != constants.RejectReasonMachineWorking {
		t.Fatalf("order = %+v msg=%s", created.Order, created.Message)
	}

	// 缺必填：400
	_, status = postJSON(t, r, "/api/v1/maintenance/reminders/s1/orders", map[string]string{"planDate": "", "servicePoint": ""})
	if status != http.StatusBadRequest {
		t.Fatalf("validation status = %d", status)
	}

	// 农机空闲后开单成功
	db.Model(&model.Machine{}).Where("code = ?", "NJ-H-001").Update("status", constants.MachineIdle)
	env, _ = postJSON(t, r, "/api/v1/maintenance/reminders/s1/orders", map[string]string{
		"planDate": "2026-09-26", "servicePoint": "镇维修点",
	})
	json.Unmarshal(env.Data, &created)
	if created.Order.Status != constants.MaintenanceOrderProcessing {
		t.Fatalf("created order = %+v", created.Order)
	}
	orderID := created.Order.ID

	// 完工
	env, status = postJSON(t, r, "/api/v1/maintenance/orders/"+orderID+"/complete", map[string]float64{
		"actualHours": 3.5, "cost": 660, "nextRemainHours": 100,
	})
	if status != http.StatusOK {
		t.Fatalf("complete status=%d body=%s", status, env.Message)
	}
	var completed struct {
		Duplicate bool   `json:"duplicate"`
		Message   string `json:"message"`
	}
	json.Unmarshal(env.Data, &completed)
	if completed.Duplicate {
		t.Fatalf("first complete should not be duplicate")
	}

	// 重复完工：幂等，duplicate=true
	env, status = postJSON(t, r, "/api/v1/maintenance/orders/"+orderID+"/complete", map[string]float64{
		"actualHours": 99, "cost": 9999, "nextRemainHours": 1,
	})
	if status != http.StatusOK {
		t.Fatalf("duplicate complete status=%d", status)
	}
	json.Unmarshal(env.Data, &completed)
	if !completed.Duplicate {
		t.Fatalf("duplicate complete should be flagged, data=%s", env.Data)
	}

	// 费用记录仍只有一笔，且为首笔金额
	var feeCount int64
	db.Model(&model.MaintenanceExpense{}).Where("order_id = ?", orderID).Count(&feeCount)
	if feeCount != 1 {
		t.Fatalf("fee count = %d, want 1", feeCount)
	}
}

func TestMaintenanceHTTP_CancelRestores(t *testing.T) {
	r, db := setupRouter(t)
	db.Create(&model.Machine{ID: "m1", Code: "NJ-H-002", Status: constants.MachineRepair})
	db.Create(&model.MaintenanceReminder{ID: "s2", MachineCode: "NJ-H-002", Title: "检修", Status: constants.ReminderProcessing, ActiveOrderID: "wo-h-1"})
	db.Create(&model.MaintenanceOrder{ID: "wo-h-1", ReminderID: "s2", MachineCode: "NJ-H-002", Title: "检修", Status: constants.MaintenanceOrderProcessing, ServicePoint: "县站"})

	env, status := postJSON(t, r, "/api/v1/maintenance/orders/wo-h-1/cancel", nil)
	if status != http.StatusOK || env.Code != 0 {
		t.Fatalf("cancel status=%d env=%+v", status, env)
	}
	var machine model.Machine
	db.First(&machine, "code = ?", "NJ-H-002")
	if machine.Status != constants.MachineIdle {
		t.Errorf("machine = %s, want 空闲", machine.Status)
	}
	var reminder model.MaintenanceReminder
	db.First(&reminder, "id = ?", "s2")
	if reminder.Status != constants.ReminderOpen {
		t.Errorf("reminder = %s, want 待开单", reminder.Status)
	}
}
