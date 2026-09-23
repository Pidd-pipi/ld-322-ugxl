package service

import (
	"github.com/cygreenenv/greenhouse-panel/internal/constants"
	apperrors "github.com/cygreenenv/greenhouse-panel/internal/errors"
	"github.com/cygreenenv/greenhouse-panel/internal/model"
	"github.com/cygreenenv/greenhouse-panel/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"io"
	"log/slog"
	"testing"
)

func setupAlertService(t *testing.T, name string) (*AlertService, *repository.AlertRepository, model.Alert) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(model.All()...); err != nil {
		t.Fatal(err)
	}
	alert := model.Alert{GreenhouseID: 1, SensorID: 1, Level: "warning", Message: "温度超限", Value: 35, Status: constants.AlertPending}
	if err = db.Create(&alert).Error; err != nil {
		t.Fatal(err)
	}
	repo := repository.NewAlertRepository(db)
	svc := NewAlertService(repo, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return svc, repo, alert
}

func TestHandleRejectsEmptyNote(t *testing.T) {
	svc, repo, alert := setupAlertService(t, "alert_empty_note")
	for _, note := range []string{"", "   "} {
		if _, err := svc.Handle(alert.ID, note, "admin"); err != apperrors.ErrAlertNoteRequired {
			t.Fatalf("note %q: want ErrAlertNoteRequired, got %v", note, err)
		}
	}
	rows, err := repo.List(alert.GreenhouseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Status != constants.AlertPending || rows[0].HandledAt != nil {
		t.Fatalf("alert must stay pending when note is empty: %#v", rows)
	}
}

func TestHandleRecordsTrace(t *testing.T) {
	svc, _, alert := setupAlertService(t, "alert_trace")
	row, err := svc.Handle(alert.ID, " 已开启风机降温 ", "admin")
	if err != nil {
		t.Fatal(err)
	}
	if row.Status != constants.AlertHandled || row.HandledBy != "admin" || row.HandleNote != "已开启风机降温" || row.HandledAt == nil {
		t.Fatalf("handle trace incomplete: %#v", row)
	}
	if _, err = svc.Handle(alert.ID, "重复处理", "admin"); err != apperrors.ErrAlertAlreadyHandled {
		t.Fatalf("want ErrAlertAlreadyHandled, got %v", err)
	}
}
