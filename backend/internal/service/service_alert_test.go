package service

import (
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/cygreenenv/greenhouse-panel/internal/constants"
	apperrors "github.com/cygreenenv/greenhouse-panel/internal/errors"
	"github.com/cygreenenv/greenhouse-panel/internal/model"
	"github.com/cygreenenv/greenhouse-panel/internal/repository"
	ws "github.com/cygreenenv/greenhouse-panel/internal/websocket"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func newAlertServiceDB(t *testing.T) (*gorm.DB, *AlertService) {
	t.Helper()
	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(model.All()...); err != nil {
		t.Fatal(err)
	}
	svc := NewAlertService(repository.NewAlertRepository(db), testLogger(), ws.NewHub())
	return db, svc
}

func TestAlertHandleRecordsOperatorNoteAndTime(t *testing.T) {
	db, svc := newAlertServiceDB(t)
	alert := model.Alert{GreenhouseID: 1, SensorID: 1, Level: "warning", Message: "温度超限", Status: constants.AlertPending}
	if err := db.Create(&alert).Error; err != nil {
		t.Fatal(err)
	}
	before := time.Now()
	handled, err := svc.Handle(alert.ID, "admin", "  已开启风机降温并复测正常  ")
	if err != nil {
		t.Fatal(err)
	}
	if handled.Status != constants.AlertHandled {
		t.Fatalf("want handled, got %s", handled.Status)
	}
	if handled.HandledBy != "admin" {
		t.Fatalf("want operator admin, got %q", handled.HandledBy)
	}
	if handled.HandleNote != "已开启风机降温并复测正常" {
		t.Fatalf("note not trimmed/stored: %q", handled.HandleNote)
	}
	if handled.HandledAt == nil || handled.HandledAt.Before(before) {
		t.Fatalf("handledAt not set: %v", handled.HandledAt)
	}
}

func TestAlertHandleRejectsEmptyNoteAndKeepsPending(t *testing.T) {
	db, svc := newAlertServiceDB(t)
	alert := model.Alert{GreenhouseID: 1, SensorID: 1, Level: "warning", Message: "湿度超限", Status: constants.AlertPending}
	if err := db.Create(&alert).Error; err != nil {
		t.Fatal(err)
	}
	for _, note := range []string{"", "   ", "\t\n"} {
		if _, err := svc.Handle(alert.ID, "admin", note); err != apperrors.ErrValidation {
			t.Fatalf("note %q: want ErrValidation, got %v", note, err)
		}
	}
	var fresh model.Alert
	if err := db.First(&fresh, alert.ID).Error; err != nil {
		t.Fatal(err)
	}
	if fresh.Status != constants.AlertPending || fresh.HandledAt != nil || fresh.HandledBy != "" || fresh.HandleNote != "" {
		t.Fatalf("alert must stay untouched, got %#v", fresh)
	}
}

func TestAlertHandleRejectsAlreadyHandled(t *testing.T) {
	db, svc := newAlertServiceDB(t)
	alert := model.Alert{GreenhouseID: 1, SensorID: 1, Level: "warning", Message: "CO2 超限", Status: constants.AlertPending}
	if err := db.Create(&alert).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Handle(alert.ID, "admin", "第一次处理：通风换气"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Handle(alert.ID, "other", "第二次尝试覆盖"); err != apperrors.ErrAlertHandled {
		t.Fatalf("want ErrAlertHandled, got %v", err)
	}
	var fresh model.Alert
	if err := db.First(&fresh, alert.ID).Error; err != nil {
		t.Fatal(err)
	}
	if fresh.HandledBy != "admin" || fresh.HandleNote != "第一次处理：通风换气" {
		t.Fatalf("handled record must stay immutable, got %#v", fresh)
	}
}

func TestAlertHandleWithoutOperatorRejected(t *testing.T) {
	db, svc := newAlertServiceDB(t)
	alert := model.Alert{GreenhouseID: 1, SensorID: 1, Level: "warning", Message: "光照超限", Status: constants.AlertPending}
	if err := db.Create(&alert).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Handle(alert.ID, "  ", "采取了措施"); err != apperrors.ErrUnauthorized {
		t.Fatalf("want ErrUnauthorized, got %v", err)
	}
}

func TestReportCountsPendingAndHandled(t *testing.T) {
	db, _ := newAlertServiceDB(t)
	repo := repository.NewAlertRepository(db)
	now := time.Now()
	mk := func(status string, age time.Duration) {
		a := model.Alert{GreenhouseID: 7, Level: "warning", Message: "x", Status: status, CreatedAt: now.Add(-age)}
		if err := db.Create(&a).Error; err != nil {
			t.Fatal(err)
		}
	}
	mk(constants.AlertPending, time.Hour)
	mk(constants.AlertPending, 2*time.Hour)
	mk(constants.AlertHandled, 3*time.Hour)
	// 窗口外（8 天前）的报警不应计入日范围
	mk(constants.AlertPending, 8*24*time.Hour)

	counts, err := repo.CountByStatusBetween(7, now.Add(-24*time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if counts["pending"] != 2 || counts["handled"] != 1 {
		t.Fatalf("want pending=2 handled=1, got %#v", counts)
	}
}
