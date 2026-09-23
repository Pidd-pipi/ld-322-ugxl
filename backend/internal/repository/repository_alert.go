package repository

import (
	"errors"
	"fmt"
	"github.com/cygreenenv/greenhouse-panel/internal/constants"
	apperrors "github.com/cygreenenv/greenhouse-panel/internal/errors"
	"github.com/cygreenenv/greenhouse-panel/internal/model"
	"gorm.io/gorm"
	"time"
)

type AlertRepository struct{ db *gorm.DB }

func NewAlertRepository(db *gorm.DB) *AlertRepository { return &AlertRepository{db: db} }
func (r *AlertRepository) Create(row *model.Alert) error {
	if err := r.db.Create(row).Error; err != nil {
		return fmt.Errorf("create alert: %w", err)
	}
	return nil
}
func (r *AlertRepository) List(greenhouseID uint) ([]model.Alert, error) {
	var rows []model.Alert
	q := r.db.Preload("Sensor.Threshold").Order("created_at desc")
	if greenhouseID > 0 {
		q = q.Where("greenhouse_id = ?", greenhouseID)
	}
	if err := q.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list alerts: %w", err)
	}
	return rows, nil
}
func (r *AlertRepository) Handle(id uint, note, handledBy string) (*model.Alert, error) {
	var row model.Alert
	err := r.db.First(&row, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperrors.ErrRecordNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get alert: %w", err)
	}
	if row.Status == constants.AlertHandled {
		return nil, apperrors.ErrAlertAlreadyHandled
	}
	now := time.Now()
	row.Status = constants.AlertHandled
	row.HandledAt = &now
	row.HandledBy = handledBy
	row.HandleNote = note
	if err = r.db.Save(&row).Error; err != nil {
		return nil, fmt.Errorf("handle alert: %w", err)
	}
	return &row, nil
}
func (r *AlertRepository) CountByStatusBetween(greenhouseID uint, start, end time.Time) (map[string]int64, error) {
	type statusCount struct {
		Status string
		Total  int64
	}
	var rows []statusCount
	if err := r.db.Model(&model.Alert{}).Select("status, count(*) as total").Where("greenhouse_id = ? AND created_at BETWEEN ? AND ?", greenhouseID, start, end).Group("status").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("count alerts by status: %w", err)
	}
	totals := map[string]int64{constants.AlertPending: 0, constants.AlertHandled: 0}
	for _, row := range rows {
		totals[row.Status] = row.Total
	}
	return totals, nil
}
