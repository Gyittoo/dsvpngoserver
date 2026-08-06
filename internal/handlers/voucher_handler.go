package handlers

import (
	"fmt"
	"net/http"
	"time"

	"dsvpn-backend/internal/middleware"
	"dsvpn-backend/internal/response"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

type VoucherHandler struct {
	db *pgxpool.Pool
}

func NewVoucherHandler(db *pgxpool.Pool) *VoucherHandler {
	return &VoucherHandler{db: db}
}

type redeemReq struct {
	Code string `json:"code" binding:"required"`
}

func (h *VoucherHandler) Redeem(c *gin.Context) {
	var req redeemReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	userID := c.GetString(middleware.CtxUserIDKey)
	email := c.GetString(middleware.CtxEmailKey)
	ctx := c.Request.Context()

	tx, err := h.db.Begin(ctx)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Rollback(ctx)

	// Lock and fetch voucher
	var voucherID string
	var durationDays int
	err = tx.QueryRow(ctx,
		`SELECT id, duration_in_days FROM vouchers WHERE code=$1 AND status='active' FOR UPDATE`,
		req.Code).Scan(&voucherID, &durationDays)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid or already used voucher code")
		return
	}

	// Mark voucher as used
	_, err = tx.Exec(ctx,
		`UPDATE vouchers SET status='used', used_by=$1, used_by_email=$2, used_at=NOW() WHERE id=$3`,
		userID, email, voucherID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to update voucher")
		return
	}

	// Upgrade user plan
	var expiryDate time.Time
	err = tx.QueryRow(ctx,
		`UPDATE users SET plan='premium',
                expiry_date = COALESCE(
                    CASE WHEN expiry_date > NOW() THEN expiry_date ELSE NOW() END
                , NOW()) + ($1 || ' days')::INTERVAL,
                updated_at = NOW()
         WHERE id=$2 RETURNING expiry_date`,
		fmt.Sprintf("%d", durationDays), userID).Scan(&expiryDate)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to upgrade plan")
		return
	}

	if err := tx.Commit(ctx); err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to commit")
		return
	}

	response.OK(c, gin.H{
		"plan":          "premium",
		"expiry_date":   expiryDate,
		"duration_days": durationDays,
		"message":       "Premium Plan activated successfully!",
	})
}
