package service

import (
	apperrors "github.com/cygreenenv/greenhouse-panel/internal/errors"
	"github.com/cygreenenv/greenhouse-panel/internal/model"
	"github.com/cygreenenv/greenhouse-panel/internal/repository"
	"log/slog"
	"strings"
)

type AlertService struct {
	repo   *repository.AlertRepository
	logger *slog.Logger
}

func NewAlertService(r *repository.AlertRepository, l *slog.Logger) *AlertService {
	return &AlertService{r, l}
}
func (s *AlertService) List(id uint) ([]model.Alert, error) { return s.repo.List(id) }
func (s *AlertService) Handle(id uint, note, handledBy string) (*model.Alert, error) {
	note = strings.TrimSpace(note)
	if note == "" {
		return nil, apperrors.ErrAlertNoteRequired
	}
	row, err := s.repo.Handle(id, note, handledBy)
	if err != nil {
		return nil, err
	}
	s.logger.Info("alert handled", "alert_id", id, "handled_by", handledBy)
	return row, nil
}
