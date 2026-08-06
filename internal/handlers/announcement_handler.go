package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"dsvpn-backend/internal/middleware"
	"dsvpn-backend/internal/response"
)

type AnnouncementHandler struct {
	db *pgxpool.Pool
}

func NewAnnouncementHandler(db *pgxpool.Pool) *AnnouncementHandler {
	return &AnnouncementHandler{db: db}
}

func (h *AnnouncementHandler) Active(c *gin.Context) {
	rows, err := h.db.Query(c.Request.Context(),
		`SELECT id, title, message, is_active, created_at, updated_at
         FROM announcements WHERE is_active=true ORDER BY created_at DESC`)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to fetch announcements")
		return
	}
	defer rows.Close()

	var announcements []gin.H
	for rows.Next() {
		var id, title, message string
		var isActive bool
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&id, &title, &message, &isActive, &createdAt, &updatedAt); err != nil {
			continue
		}
		announcements = append(announcements, gin.H{
			"id": id, "title": title, "message": message, "is_active": isActive,
			"created_at": createdAt, "updated_at": updatedAt,
		})
	}
	if announcements == nil {
		announcements = []gin.H{}
	}
	response.OK(c, announcements)
}

type logReq struct {
	ServerID string `json:"server_id"`
	Event    string `json:"event"`
}

func (h *AnnouncementHandler) LogConnection(c *gin.Context) {
	var req logReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	userID := c.GetString(middleware.CtxUserIDKey)
	_, err := h.db.Exec(c.Request.Context(),
		`INSERT INTO connection_logs (user_id, server_id, event) VALUES ($1, $2, $3)`,
		userID, req.ServerID, req.Event)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to log connection")
		return
	}
	response.Created(c, nil)
}
