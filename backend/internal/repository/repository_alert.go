package repository

import (
	"errors"
	"fmt"
	apperrors "github.com/cygreenenv/greenhouse-panel/internal/errors"
	"github.com/cygreenenv/greenhouse-panel/internal/model"
	"gorm.io/gorm"
	"strings"
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
func (r *AlertRepository) Get(id uint) (*model.Alert, error) {
	var row model.Alert
	err := r.db.First(&row, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperrors.ErrRecordNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get alert: %w", err)
	}
	return &row, nil
}

// Handle 将一条待处理报警标记为已处理，并写入处理人与处理说明。
// 仅当报警仍为 pending 时更新生效，已处理的报警保持原记录不变。
func (r *AlertRepository) Handle(id uint, operator, note string) (*model.Alert, error) {
	now := time.Now()
	result := r.db.Model(&model.Alert{}).
		Where("id = ? AND status = ?", id, "pending").
		Updates(map[string]any{
			"status":      "handled",
			"handled_at":  now,
			"handled_by":  operator,
			"handle_note": note,
		})
	if result.Error != nil {
		return nil, fmt.Errorf("handle alert: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		// 区分“报警不存在”与“报警已处理”，便于上层返回不同错误。
		var row model.Alert
		if err := r.db.First(&row, id).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrRecordNotFound
		} else if err != nil {
			return nil, fmt.Errorf("get alert: %w", err)
		}
		return nil, apperrors.ErrAlertHandled
	}
	return r.Get(id)
}
func (r *AlertRepository) CountBetween(greenhouseID uint, start, end time.Time) (int64, error) {
	var count int64
	if err := r.db.Model(&model.Alert{}).Where("greenhouse_id=? AND created_at BETWEEN ? AND ?", greenhouseID, start, end).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count alerts: %w", err)
	}
	return count, nil
}

// CountByStatusBetween 统计时间窗口内各状态（pending/handled）的报警数量。
func (r *AlertRepository) CountByStatusBetween(greenhouseID uint, start, end time.Time) (map[string]int64, error) {
	type statusCount struct {
		Status string
		Total  int64
	}
	var rows []statusCount
	if err := r.db.Model(&model.Alert{}).
		Select("status, count(*) as total").
		Where("greenhouse_id = ? AND created_at BETWEEN ? AND ?", greenhouseID, start, end).
		Group("status").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("count alerts by status: %w", err)
	}
	counts := map[string]int64{"pending": 0, "handled": 0}
	for _, row := range rows {
		counts[strings.ToLower(row.Status)] = row.Total
	}
	return counts, nil
}
