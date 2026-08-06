package admin

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"dsvpn-backend/internal/response"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AnnouncementAdminHandler struct {
	db *pgxpool.Pool
}

func NewAnnouncementAdminHandler(db *pgxpool.Pool) *AnnouncementAdminHandler {
	return &AnnouncementAdminHandler{db: db}
}

func (h *AnnouncementAdminHandler) List(c *gin.Context) {
	rows, err := h.db.Query(c.Request.Context(),
		`SELECT id, title, message, is_active, created_at, updated_at
		 FROM announcements ORDER BY created_at DESC`)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to query announcements")
		return
	}
	defer rows.Close()

	var announcements []gin.H
	for rows.Next() {
		var (
			id, title, message string
			isActive           bool
			createdAt          time.Time
			updatedAt          time.Time
		)
		err := rows.Scan(&id, &title, &message, &isActive, &createdAt, &updatedAt)
		if err != nil {
			continue
		}
		announcements = append(announcements, gin.H{
			"id": id, "title": title, "message": message,
			"is_active": isActive, "created_at": createdAt, "updated_at": updatedAt,
		})
	}
	if announcements == nil {
		announcements = []gin.H{}
	}
	response.OK(c, announcements)
}

type createAnnouncementReq struct {
	Title   string `json:"title" binding:"required"`
	Message string `json:"message" binding:"required"`
}

func (h *AnnouncementAdminHandler) Create(c *gin.Context) {
	var req createAnnouncementReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	var id string
	var isActive bool
	var createdAt, updatedAt time.Time
	err := h.db.QueryRow(c.Request.Context(),
		`INSERT INTO announcements (title, message)
		 VALUES ($1, $2) RETURNING id, is_active, created_at, updated_at`,
		req.Title, req.Message,
	).Scan(&id, &isActive, &createdAt, &updatedAt)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to create announcement")
		return
	}

	response.Created(c, gin.H{
		"id": id, "title": req.Title, "message": req.Message,
		"is_active": isActive, "created_at": createdAt, "updated_at": updatedAt,
	})
}

type updateAnnouncementReq struct {
	Title    *string `json:"title"`
	Message  *string `json:"message"`
	IsActive *bool   `json:"is_active"`
}

func (h *AnnouncementAdminHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var req updateAnnouncementReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	setClauses := []string{}
	args := []interface{}{}
	argIdx := 1

	if req.Title != nil {
		setClauses = append(setClauses, fmt.Sprintf("title=$%d", argIdx))
		args = append(args, *req.Title)
		argIdx++
	}
	if req.Message != nil {
		setClauses = append(setClauses, fmt.Sprintf("message=$%d", argIdx))
		args = append(args, *req.Message)
		argIdx++
	}
	if req.IsActive != nil {
		setClauses = append(setClauses, fmt.Sprintf("is_active=$%d", argIdx))
		args = append(args, *req.IsActive)
		argIdx++
	}

	if len(setClauses) == 0 {
		response.Error(c, http.StatusBadRequest, "no fields to update")
		return
	}

	setClauses = append(setClauses, "updated_at=NOW()")
	args = append(args, id)
	query := fmt.Sprintf("UPDATE announcements SET %s WHERE id=$%d", strings.Join(setClauses, ", "), argIdx)

	result, err := h.db.Exec(c.Request.Context(), query, args...)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to update announcement")
		return
	}
	if result.RowsAffected() == 0 {
		response.Error(c, http.StatusNotFound, "announcement not found")
		return
	}
	response.OK(c, gin.H{"message": "announcement updated"})
}

func (h *AnnouncementAdminHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	result, err := h.db.Exec(c.Request.Context(), "DELETE FROM announcements WHERE id=$1", id)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to delete announcement")
		return
	}
	if result.RowsAffected() == 0 {
		response.Error(c, http.StatusNotFound, "announcement not found")
		return
	}
	response.OK(c, nil)
}
