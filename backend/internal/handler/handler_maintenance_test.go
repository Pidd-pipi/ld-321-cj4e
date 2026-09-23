package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/agridispatch/agridispatch/internal/constants"
	"github.com/agridispatch/agridispatch/internal/model"
	"github.com/agridispatch/agridispatch/internal/repository"
	"github.com/agridispatch/agridispatch/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

var httpFixtureSeq int64

type apiResp struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func newMaintenanceRouter(t *testing.T) (*gorm.DB, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dsn := fmt.Sprintf("file:http_test_%d?mode=memory&cache=shared", atomic.AddInt64(&httpFixtureSeq, 1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Machine{}, &model.MaintenanceReminder{}, &model.MaintenanceOrder{}, &model.MaintenanceCost{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	repo := repository.NewMaintenanceRepository(db)
	svc := service.NewMaintenanceService(repo, rdb, slog.Default())
	h := NewMaintenanceHandler(svc)

	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.POST("/maintenance/orders", h.CreateOrder)
	v1.POST("/maintenance/orders/:id/complete", h.CompleteOrder)
	v1.POST("/maintenance/orders/:id/cancel", h.CancelOrder)
	return db, r
}

func doJSON(t *testing.T, r *gin.Engine, method, path string, body interface{}) (apiResp, int) {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp apiResp
	if w.Body.Len() > 0 {
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response %q: %v", w.Body.String(), err)
		}
	}
	return resp, w.Code
}

func dataField(t *testing.T, data json.RawMessage, key string) interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("decode data: %v", err)
	}
	return m[key]
}

// 全生命周期：成功开单 → 完工 → 重复完工返回冲突且只有一笔费用。
func TestMaintenanceOrderHTTPLifecycle(t *testing.T) {
	db, r := newMaintenanceRouter(t)
	db.Create(&model.Machine{ID: "m1", Code: "NJ-H-001", Status: constants.MachineIdle})
	db.Create(&model.MaintenanceReminder{ID: "s1", MachineCode: "NJ-H-001", Title: "换机油", Status: constants.ReminderPending})

	// 1. 开单成功
	resp, status := doJSON(t, r, http.MethodPost, "/api/v1/maintenance/orders", map[string]string{
		"reminderId":  "s1",
		"planDate":    "2026-09-25",
		"repairPoint": "北岭维修点",
	})
	if status != http.StatusCreated || resp.Code != 0 {
		t.Fatalf("create: status=%d resp=%+v", status, resp)
	}
	orderID, _ := dataField(t, resp.Data, "orderId").(string)
	if got := dataField(t, resp.Data, "status"); got != constants.OrderProcessing {
		t.Fatalf("order status = %v", got)
	}

	// 农机已转维修中
	var machine model.Machine
	db.First(&machine, "code = ?", "NJ-H-001")
	if machine.Status != constants.MachineRepair {
		t.Fatalf("machine = %s", machine.Status)
	}

	// 2. 再开一单应被拒绝（已有未结束工单），201 且留拒绝原因
	resp, status = doJSON(t, r, http.MethodPost, "/api/v1/maintenance/orders", map[string]string{
		"reminderId":  "s1",
		"planDate":    "2026-09-26",
		"repairPoint": "南湾维修点",
	})
	if status != http.StatusCreated {
		t.Fatalf("reject should still be 201 with persisted record, got %d", status)
	}
	if got := dataField(t, resp.Data, "status"); got != constants.OrderRejected {
		t.Fatalf("status = %v, want 已拒绝", got)
	}
	if reason, _ := dataField(t, resp.Data, "rejectReason").(string); reason == "" {
		t.Fatal("reject reason empty")
	}

	// 3. 参数校验失败
	resp, status = doJSON(t, r, http.MethodPost, "/api/v1/maintenance/orders", map[string]string{
		"reminderId": "s1",
	})
	if status != http.StatusBadRequest || resp.Code != constants.CodeBadRequest {
		t.Fatalf("validation: status=%d resp=%+v", status, resp)
	}

	// 4. 完工
	resp, status = doJSON(t, r, http.MethodPost, fmt.Sprintf("/api/v1/maintenance/orders/%s/complete", orderID), map[string]float64{
		"actualHours":     4.5,
		"cost":            680,
		"nextRemainHours": 100,
	})
	if status != http.StatusOK || resp.Code != 0 {
		t.Fatalf("complete: status=%d resp=%+v", status, resp)
	}

	// 5. 重复完工 → 409 冲突，费用记录仍只有一笔
	resp, status = doJSON(t, r, http.MethodPost, fmt.Sprintf("/api/v1/maintenance/orders/%s/complete", orderID), map[string]float64{
		"actualHours":     9,
		"cost":            999,
		"nextRemainHours": 50,
	})
	if status != http.StatusConflict || resp.Code != constants.CodeConflict {
		t.Fatalf("repeat complete: status=%d resp=%+v", status, resp)
	}
	var costCount int64
	db.Model(&model.MaintenanceCost{}).Where("order_id = ?", orderID).Count(&costCount)
	if costCount != 1 {
		t.Fatalf("cost count = %d, want 1", costCount)
	}

	// 农机已空闲
	db.First(&machine, "code = ?", "NJ-H-001")
	if machine.Status != constants.MachineIdle {
		t.Fatalf("machine after complete = %s", machine.Status)
	}
}

// 作业中农机开单被拒绝，状态不发生任何变化。
func TestCreateOrderRejectedWhenWorkingHTTP(t *testing.T) {
	db, r := newMaintenanceRouter(t)
	db.Create(&model.Machine{ID: "m1", Code: "NJ-H-002", Status: constants.MachineWorking, CurrentTask: "春耕"})
	db.Create(&model.MaintenanceReminder{ID: "s2", MachineCode: "NJ-H-002", Title: "滤芯", Status: constants.ReminderPending})

	resp, status := doJSON(t, r, http.MethodPost, "/api/v1/maintenance/orders", map[string]string{
		"reminderId":  "s2",
		"planDate":    "2026-09-25",
		"repairPoint": "北岭维修点",
	})
	if status != http.StatusCreated {
		t.Fatalf("status = %d", status)
	}
	if got := dataField(t, resp.Data, "rejectReason"); got != constants.RejectReasonWorking {
		t.Fatalf("reject reason = %v", got)
	}

	var reminder model.MaintenanceReminder
	db.First(&reminder, "id = ?", "s2")
	if reminder.Status != constants.ReminderPending {
		t.Fatalf("reminder changed to %s", reminder.Status)
	}
}

// 取消未完工工单释放农机、恢复提醒，已取消的工单不可再次取消。
func TestCancelOrderHTTP(t *testing.T) {
	db, r := newMaintenanceRouter(t)
	db.Create(&model.Machine{ID: "m1", Code: "NJ-H-003", Status: constants.MachineIdle})
	db.Create(&model.MaintenanceReminder{ID: "s3", MachineCode: "NJ-H-003", Title: "刀盘", Status: constants.ReminderPending})

	resp, _ := doJSON(t, r, http.MethodPost, "/api/v1/maintenance/orders", map[string]string{
		"reminderId":  "s3",
		"planDate":    "2026-09-25",
		"repairPoint": "北岭维修点",
	})
	orderID, _ := dataField(t, resp.Data, "orderId").(string)

	resp, status := doJSON(t, r, http.MethodPost, fmt.Sprintf("/api/v1/maintenance/orders/%s/cancel", orderID), nil)
	if status != http.StatusOK || resp.Code != 0 {
		t.Fatalf("cancel: status=%d resp=%+v", status, resp)
	}

	var machine model.Machine
	db.First(&machine, "code = ?", "NJ-H-003")
	if machine.Status != constants.MachineIdle {
		t.Fatalf("machine = %s", machine.Status)
	}

	_, status = doJSON(t, r, http.MethodPost, fmt.Sprintf("/api/v1/maintenance/orders/%s/cancel", orderID), nil)
	if status != http.StatusConflict {
		t.Fatalf("second cancel status = %d, want 409", status)
	}
}
