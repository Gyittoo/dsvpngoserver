package admin

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"dsvpn-backend/internal/response"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

type VoucherAdminHandler struct {
	db *pgxpool.Pool
}

func NewVoucherAdminHandler(db *pgxpool.Pool) *VoucherAdminHandler {
	return &VoucherAdminHandler{db: db}
}

func (h *VoucherAdminHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	var total int
	err := h.db.QueryRow(c.Request.Context(), "SELECT COUNT(*) FROM vouchers").Scan(&total)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to count vouchers")
		return
	}

	rows, err := h.db.Query(c.Request.Context(),
		`SELECT id, code, duration_in_days, status, used_by_email, used_at, created_at
		 FROM vouchers ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to query vouchers")
		return
	}
	defer rows.Close()

	var vouchers []gin.H
	for rows.Next() {
		var (
			id, code, status string
			durationInDays   int
			usedByEmail      *string
			usedAt           *time.Time
			createdAt        time.Time
		)
		err := rows.Scan(&id, &code, &durationInDays, &status, &usedByEmail, &usedAt, &createdAt)
		if err != nil {
			continue
		}
		vouchers = append(vouchers, gin.H{
			"id": id, "code": code, "duration_in_days": durationInDays,
			"status": status, "used_by_email": usedByEmail,
			"used_at": usedAt, "created_at": createdAt,
		})
	}
	if vouchers == nil {
		vouchers = []gin.H{}
	}

	response.OK(c, gin.H{"vouchers": vouchers, "total": total, "page": page, "limit": limit})
}

type createVoucherReq struct {
	Count          int    `json:"count" binding:"required,min=1,max=1000"`
	DurationInDays int    `json:"duration_in_days" binding:"required,min=1"`
	Prefix         string `json:"prefix" binding:"required"`
}

func (h *VoucherAdminHandler) Create(c *gin.Context) {
	var req createVoucherReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	var codes []string
	for i := 0; i < req.Count; i++ {
		b := make([]byte, 4)
		rand.Read(b)
		code := fmt.Sprintf("%s-%s", req.Prefix, hex.EncodeToString(b))
		codes = append(codes, code)
	}

	for _, code := range codes {
		_, err := h.db.Exec(c.Request.Context(),
			`INSERT INTO vouchers (code, duration_in_days) VALUES ($1, $2)`,
			code, req.DurationInDays)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, "failed to create vouchers")
			return
		}
	}

	response.Created(c, gin.H{"created": req.Count, "codes": codes})
}

func (h *VoucherAdminHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	result, err := h.db.Exec(c.Request.Context(), "DELETE FROM vouchers WHERE id=$1", id)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to delete voucher")
		return
	}
	if result.RowsAffected() == 0 {
		response.Error(c, http.StatusNotFound, "voucher not found")
		return
	}
	response.OK(c, nil)
}
