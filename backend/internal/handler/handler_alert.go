package handler

import (
	"github.com/cygreenenv/greenhouse-panel/internal/constants"
	"github.com/cygreenenv/greenhouse-panel/internal/dto"
	apperrors "github.com/cygreenenv/greenhouse-panel/internal/errors"
	"github.com/cygreenenv/greenhouse-panel/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"strconv"
)

type AlertHandler struct {
	service   *service.AlertService
	validator *validator.Validate
}

func NewAlertHandler(s *service.AlertService, v *validator.Validate) *AlertHandler {
	return &AlertHandler{s, v}
}
func (h *AlertHandler) List(c *gin.Context) {
	var id uint
	if raw := c.Query("greenhouse_id"); raw != "" {
		value, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || value == 0 {
			Fail(c, apperrors.ErrValidation)
			return
		}
		id = uint(value)
	}
	rows, err := h.service.List(id)
	if err != nil {
		Fail(c, err)
		return
	}
	Success(c, rows)
}
func (h *AlertHandler) Handle(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req dto.AlertHandleRequest
	if err := c.ShouldBindJSON(&req); err != nil || h.validator.Struct(req) != nil {
		Fail(c, apperrors.ErrValidation)
		return
	}
	username, _ := c.Get(constants.ContextUsername)
	handledBy, _ := username.(string)
	row, err := h.service.Handle(id, req.Note, handledBy)
	if err != nil {
		Fail(c, err)
		return
	}
	Success(c, row)
}
