package admin

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"dsvpn-backend/internal/response"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserAdminHandler struct {
	db *pgxpool.Pool
}

func NewUserAdminHandler(db *pgxpool.Pool) *UserAdminHandler {
	return &UserAdminHandler{db: db}
}

func getPagination(c *gin.Context) (page, limit, offset int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ = strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset = (page - 1) * limit
	return
}

func (h *UserAdminHandler) List(c *gin.Context) {
	page, limit, offset := getPagination(c)

	var total int
	err := h.db.QueryRow(c.Request.Context(), "SELECT COUNT(*) FROM users").Scan(&total)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to count users")
		return
	}

	rows, err := h.db.Query(c.Request.Context(),
		`SELECT id, email, display_name, photo_url, plan, expiry_date, is_active,
		        data_limit_mb, data_used_mb, bound_device_id, last_login, created_at
		 FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to query users")
		return
	}
	defer rows.Close()

	var users []gin.H
	for rows.Next() {
		var (
			id, email, plan string
			displayName     *string
			photoURL        *string
			expiryDate      *time.Time
			isActive        bool
			dataLimitMB     int
			dataUsedMB      int
			boundDeviceID   string
			lastLogin       *time.Time
			createdAt       time.Time
		)
		err := rows.Scan(&id, &email, &displayName, &photoURL, &plan, &expiryDate,
			&isActive, &dataLimitMB, &dataUsedMB, &boundDeviceID, &lastLogin, &createdAt)
		if err != nil {
			continue
		}
		users = append(users, gin.H{
			"id": id, "email": email, "display_name": displayName,
			"photo_url": photoURL, "plan": plan, "expiry_date": expiryDate,
			"is_active": isActive, "data_limit_mb": dataLimitMB,
			"data_used_mb": dataUsedMB, "bound_device_id": boundDeviceID,
			"last_login": lastLogin, "created_at": createdAt,
		})
	}
	if users == nil {
		users = []gin.H{}
	}

	response.OK(c, gin.H{"users": users, "total": total, "page": page, "limit": limit})
}

func (h *UserAdminHandler) Get(c *gin.Context) {
	id := c.Param("id")

	var (
		uid, email, plan string
		displayName      *string
		photoURL         *string
		expiryDate       *time.Time
		isActive         bool
		dataLimitMB      int
		dataUsedMB       int
		boundDeviceID    string
		lastLogin        *time.Time
		createdAt        time.Time
	)
	err := h.db.QueryRow(c.Request.Context(),
		`SELECT id, email, display_name, photo_url, plan, expiry_date, is_active,
		        data_limit_mb, data_used_mb, bound_device_id, last_login, created_at
		 FROM users WHERE id=$1`, id,
	).Scan(&uid, &email, &displayName, &photoURL, &plan, &expiryDate,
		&isActive, &dataLimitMB, &dataUsedMB, &boundDeviceID, &lastLogin, &createdAt)
	if err == pgx.ErrNoRows {
		response.Error(c, http.StatusNotFound, "user not found")
		return
	} else if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to get user")
		return
	}

	response.OK(c, gin.H{
		"id": uid, "email": email, "display_name": displayName,
		"photo_url": photoURL, "plan": plan, "expiry_date": expiryDate,
		"is_active": isActive, "data_limit_mb": dataLimitMB,
		"data_used_mb": dataUsedMB, "bound_device_id": boundDeviceID,
		"last_login": lastLogin, "created_at": createdAt,
	})
}

func (h *UserAdminHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Plan        *string `json:"plan"`
		IsActive    *bool   `json:"is_active"`
		DataLimitMB *int    `json:"data_limit_mb"`
		DataUsedMB  *int    `json:"data_used_mb"`
		ExpiryDate  *string `json:"expiry_date"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	setClauses := []string{}
	args := []interface{}{}
	argIdx := 1

	if req.Plan != nil {
		setClauses = append(setClauses, fmt.Sprintf("plan=$%d", argIdx))
		args = append(args, *req.Plan)
		argIdx++
	}
	if req.IsActive != nil {
		setClauses = append(setClauses, fmt.Sprintf("is_active=$%d", argIdx))
		args = append(args, *req.IsActive)
		argIdx++
	}
	if req.DataLimitMB != nil {
		setClauses = append(setClauses, fmt.Sprintf("data_limit_mb=$%d", argIdx))
		args = append(args, *req.DataLimitMB)
		argIdx++
	}
	if req.DataUsedMB != nil {
		setClauses = append(setClauses, fmt.Sprintf("data_used_mb=$%d", argIdx))
		args = append(args, *req.DataUsedMB)
		argIdx++
	}
	if req.ExpiryDate != nil {
		setClauses = append(setClauses, fmt.Sprintf("expiry_date=$%d", argIdx))
		args = append(args, *req.ExpiryDate)
		argIdx++
	}

	if len(setClauses) == 0 {
		response.Error(c, http.StatusBadRequest, "no fields to update")
		return
	}

	setClauses = append(setClauses, "updated_at=NOW()")
	args = append(args, id)
	query := fmt.Sprintf("UPDATE users SET %s WHERE id=$%d", strings.Join(setClauses, ", "), argIdx)

	result, err := h.db.Exec(c.Request.Context(), query, args...)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to update user")
		return
	}
	if result.RowsAffected() == 0 {
		response.Error(c, http.StatusNotFound, "user not found")
		return
	}
	response.OK(c, gin.H{"message": "user updated"})
}

func (h *UserAdminHandler) ResetDevice(c *gin.Context) {
	id := c.Param("id")
	result, err := h.db.Exec(c.Request.Context(),
		"UPDATE users SET bound_device_id='', updated_at=NOW() WHERE id=$1", id)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to reset device")
		return
	}
	if result.RowsAffected() == 0 {
		response.Error(c, http.StatusNotFound, "user not found")
		return
	}
	response.OK(c, gin.H{"bound_device_id": ""})
}

func (h *UserAdminHandler) ResetPlan(c *gin.Context) {
	id := c.Param("id")
	result, err := h.db.Exec(c.Request.Context(),
		"UPDATE users SET plan='free', expiry_date=NULL, data_limit_mb=0, data_used_mb=0, updated_at=NOW() WHERE id=$1", id)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to reset plan")
		return
	}
	if result.RowsAffected() == 0 {
		response.Error(c, http.StatusNotFound, "user not found")
		return
	}
	response.OK(c, gin.H{"plan": "free"})
}

func (h *UserAdminHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	result, err := h.db.Exec(c.Request.Context(), "DELETE FROM users WHERE id=$1", id)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to delete user")
		return
	}
	if result.RowsAffected() == 0 {
		response.Error(c, http.StatusNotFound, "user not found")
		return
	}
	response.OK(c, nil)
}
