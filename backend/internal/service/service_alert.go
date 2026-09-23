package service

import (
	"github.com/cygreenenv/greenhouse-panel/internal/constants"
	apperrors "github.com/cygreenenv/greenhouse-panel/internal/errors"
	"github.com/cygreenenv/greenhouse-panel/internal/model"
	"github.com/cygreenenv/greenhouse-panel/internal/repository"
	ws "github.com/cygreenenv/greenhouse-panel/internal/websocket"
	"log/slog"
	"strings"
)

type AlertService struct {
	repo   *repository.AlertRepository
	logger *slog.Logger
	hub    *ws.Hub
}

func NewAlertService(r *repository.AlertRepository, l *slog.Logger, h *ws.Hub) *AlertService {
	return &AlertService{r, l, h}
}
func (s *AlertService) List(id uint) ([]model.Alert, error) { return s.repo.List(id) }

// Handle 记录一次可追溯的报警处理：operator 为当前登录账号，note 为处理说明。
// 说明为空（含纯空白）时拒绝，报警保持原状。
func (s *AlertService) Handle(id uint, operator, note string) (*model.Alert, error) {
	if strings.TrimSpace(operator) == "" {
		return nil, apperrors.ErrUnauthorized
	}
	note = strings.TrimSpace(note)
	if note == "" {
		return nil, apperrors.ErrValidation
	}
	row, err := s.repo.Handle(id, operator, note)
	if err != nil {
		return nil, err
	}
	s.hub.Broadcast(constants.EventAlertDone, row)
	return row, nil
}
