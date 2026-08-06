package handlers

import (
	"net/http"
	"time"

	"dsvpn-backend/internal/middleware"
	"dsvpn-backend/internal/response"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserHandler struct {
	db *pgxpool.Pool
}

func NewUserHandler(db *pgxpool.Pool) *UserHandler {
	return &UserHandler{db: db}
}

func (h *UserHandler) Me(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserIDKey)

	var (
		id, email, displayName, plan string
		photoURL, boundDeviceID      *string
		expiryDate                   *time.Time
		isActive                     bool
		limit, used                  int64
		createdAt                    time.Time
	)

	err := h.db.QueryRow(c.Request.Context(),
		`SELECT id, email, display_name, photo_url, plan, expiry_date, is_active,
       data_limit_mb, data_used_mb, bound_device_id, created_at
       FROM users WHERE id=$1`, userID).Scan(&id, &email, &displayName, &photoURL, &plan, &expiryDate, &isActive, &limit, &used, &boundDeviceID, &createdAt)
	
	if err == pgx.ErrNoRows {
		response.Error(c, http.StatusNotFound, "user not found")
		return
	} else if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to fetch user")
		return
	}

	response.OK(c, gin.H{
		"id":              id,
		"email":           email,
		"display_name":    displayName,
		"photo_url":       photoURL,
		"plan":            plan,
		"expiry_date":     expiryDate,
		"is_active":       isActive,
		"data_limit_mb":   limit,
		"data_used_mb":    used,
		"bound_device_id": boundDeviceID,
		"created_at":      createdAt,
	})
}

type updateUsageReq struct {
	UsedMB int64 `json:"used_mb" binding:"required"`
}

func (h *UserHandler) UpdateDataUsage(c *gin.Context) {
	var req updateUsageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	userID := c.GetString(middleware.CtxUserIDKey)

	var used, limit int64
	err := h.db.QueryRow(c.Request.Context(),
		"UPDATE users SET data_used_mb = data_used_mb + $1, updated_at = NOW() WHERE id = $2 RETURNING data_used_mb, data_limit_mb",
		req.UsedMB, userID).Scan(&used, &limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to update usage")
		return
	}

	remaining := limit - used
	if remaining < 0 {
		remaining = 0
	}

	response.OK(c, gin.H{
		"data_used_mb": used,
		"data_limit_mb": limit,
		"remaining_mb": remaining,
	})
}

func (h *UserHandler) ResetPlan(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserIDKey)

	_, err := h.db.Exec(c.Request.Context(),
		"UPDATE users SET plan='free', expiry_date=NULL, data_limit_mb=0, data_used_mb=0, updated_at=NOW() WHERE id=$1",
		userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to reset plan")
		return
	}

	response.OK(c, gin.H{"plan": "free"})
}

func (h *UserHandler) DeleteAccount(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserIDKey)

	_, err := h.db.Exec(c.Request.Context(), "DELETE FROM users WHERE id=$1", userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to delete account")
		return
	}

	response.OK(c, nil)
}
