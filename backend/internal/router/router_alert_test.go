package router_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/cygreenenv/greenhouse-panel/internal/constants"
	"github.com/cygreenenv/greenhouse-panel/internal/model"
	"github.com/cygreenenv/greenhouse-panel/internal/repository"
	"github.com/cygreenenv/greenhouse-panel/internal/router"
	"github.com/cygreenenv/greenhouse-panel/internal/service"
	ws "github.com/cygreenenv/greenhouse-panel/internal/websocket"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type apiEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func setupEngine(t *testing.T) (*gorm.DB, http.Handler) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:router_alert_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(model.All()...); err != nil {
		t.Fatal(err)
	}
	g := model.Greenhouse{Name: "测试温室", Location: "A-01", Area: 100}
	if err = db.Create(&g).Error; err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	greenhouseRepo := repository.NewGreenhouseRepository(db)
	sensorRepo := repository.NewSensorRepository(db)
	alertRepo := repository.NewAlertRepository(db)
	deviceRepo := repository.NewDeviceRepository(db)
	hub := ws.NewHub()
	auth := service.NewAuthService("test-secret")
	monitoring := service.NewMonitoringService(greenhouseRepo, sensorRepo, alertRepo, logger, hub)
	control := service.NewControlService(deviceRepo, logger, hub)
	alerts := service.NewAlertService(alertRepo, logger, hub)
	reports := service.NewReportService(sensorRepo, alertRepo)
	engine := router.New(router.Dependencies{Logger: logger, Auth: auth, Monitoring: monitoring, Alerts: alerts, Control: control, Reports: reports, Hub: hub})
	return db, engine
}

func doRequest(t *testing.T, h http.Handler, method, path, token string, body any) (int, apiEnvelope) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var env apiEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("invalid json response: %v body=%s", err, rec.Body.String())
	}
	return rec.Code, env
}

func TestAlertHandleFlowAndReportCounts(t *testing.T) {
	db, engine := setupEngine(t)

	// 登录换取当前账号令牌
	status, env := doRequest(t, engine, http.MethodPost, "/api/v1/auth/login", "", map[string]string{"username": "admin", "password": "admin123"})
	if status != http.StatusOK {
		t.Fatalf("login status = %d", status)
	}
	var session struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(env.Data, &session); err != nil || session.Token == "" {
		t.Fatalf("missing token: %s", string(env.Data))
	}

	// 准备两条报警：一条待处理，一条已处理
	now := time.Now()
	pending := model.Alert{GreenhouseID: 1, Level: "warning", Message: "温度超限", Status: constants.AlertPending, CreatedAt: now.Add(-time.Hour)}
	handled := model.Alert{GreenhouseID: 1, Level: "critical", Message: "湿度超限", Status: constants.AlertHandled, CreatedAt: now.Add(-2 * time.Hour), HandledAt: &now, HandledBy: "admin", HandleNote: "历史处理记录"}
	if err := db.Create(&pending).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&handled).Error; err != nil {
		t.Fatal(err)
	}

	// 未登录处理被拒
	if code, _ := doRequest(t, engine, http.MethodPatch, "/api/v1/alerts/"+itoa(pending.ID)+"/handle", "", map[string]string{"note": "措施"}); code != http.StatusUnauthorized {
		t.Fatalf("want 401 without token, got %d", code)
	}

	// 空说明被拒（JSON 空串 / 纯空白），报警保持原状
	for _, note := range []string{"", "   "} {
		code, env := doRequest(t, engine, http.MethodPatch, "/api/v1/alerts/"+itoa(pending.ID)+"/handle", session.Token, map[string]string{"note": note})
		if code != http.StatusBadRequest {
			t.Fatalf("note %q want 400, got %d (%s)", note, code, env.Message)
		}
	}
	var fresh model.Alert
	if err := db.First(&fresh, pending.ID).Error; err != nil {
		t.Fatal(err)
	}
	if fresh.Status != constants.AlertPending || fresh.HandledBy != "" {
		t.Fatalf("pending alert changed after empty note: %#v", fresh)
	}

	// 带说明处理成功，处理人取登录账号
	code, env := doRequest(t, engine, http.MethodPatch, "/api/v1/alerts/"+itoa(pending.ID)+"/handle", session.Token, map[string]string{"note": "已开启风机并复测"})
	if code != http.StatusOK {
		t.Fatalf("handle want 200, got %d (%s)", code, env.Message)
	}
	var result model.Alert
	if err := json.Unmarshal(env.Data, &result); err != nil {
		t.Fatal(err)
	}
	if result.Status != constants.AlertHandled || result.HandledBy != "admin" || result.HandleNote != "已开启风机并复测" || result.HandledAt == nil {
		t.Fatalf("trace fields wrong: %+v", result)
	}

	// 重复处理返回 409，原记录不变
	if code, _ := doRequest(t, engine, http.MethodPatch, "/api/v1/alerts/"+itoa(pending.ID)+"/handle", session.Token, map[string]string{"note": "尝试覆盖"}); code != http.StatusConflict {
		t.Fatalf("want 409 on duplicate handle, got %d", code)
	}

	// 列表中可看到说明、处理人、时间
	code, env = doRequest(t, engine, http.MethodGet, "/api/v1/alerts?greenhouse_id=1", session.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("list want 200, got %d", code)
	}
	var list []model.Alert
	if err := json.Unmarshal(env.Data, &list); err != nil {
		t.Fatal(err)
	}
	var listed *model.Alert
	for i := range list {
		if list[i].ID == pending.ID {
			listed = &list[i]
		}
	}
	if listed == nil || listed.HandledBy != "admin" || listed.HandleNote != "已开启风机并复测" || listed.HandledAt == nil {
		t.Fatalf("list missing trace record: %#v", list)
	}

	// 报告同时给出待处理与已处理数量
	code, env = doRequest(t, engine, http.MethodGet, "/api/v1/reports/environment?greenhouse_id=1&range=day", session.Token, nil)
	if code != http.StatusOK {
		t.Fatalf("report want 200, got %d", code)
	}
	var report struct {
		Alerts        int64 `json:"alerts"`
		PendingAlerts int64 `json:"pendingAlerts"`
		HandledAlerts int64 `json:"handledAlerts"`
	}
	if err := json.Unmarshal(env.Data, &report); err != nil {
		t.Fatal(err)
	}
	if report.PendingAlerts != 0 || report.HandledAlerts != 2 || report.Alerts != 2 {
		t.Fatalf("report counts wrong: %+v", report)
	}
}

func itoa(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}
