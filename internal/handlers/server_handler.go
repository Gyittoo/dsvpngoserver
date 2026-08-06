package handlers

import (
	"net/http"

	"dsvpn-backend/internal/middleware"
	"dsvpn-backend/internal/response"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ServerHandler struct {
	db *pgxpool.Pool
}

func NewServerHandler(db *pgxpool.Pool) *ServerHandler {
	return &ServerHandler{db: db}
}

func (h *ServerHandler) ListActive(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserIDKey)

	var userPlan string
	err := h.db.QueryRow(c.Request.Context(), "SELECT plan FROM users WHERE id=$1", userID).Scan(&userPlan)
	if err != nil {
		userPlan = "free"
	}

	rows, err := h.db.Query(c.Request.Context(),
		`SELECT id, name, region, flag, country_code, protocol, plan, is_active,
                host, port, raw_config, note, sort_order
         FROM servers WHERE is_active=true ORDER BY sort_order`)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to fetch servers")
		return
	}
	defer rows.Close()

	var servers []gin.H
	for rows.Next() {
		var (
			id, name, region, flag, countryCode, protocol, plan, host, rawConfig, note string
			isActive                                                                   bool
			port, sortOrder                                                            int
		)
		if err := rows.Scan(&id, &name, &region, &flag, &countryCode, &protocol, &plan, &isActive,
			&host, &port, &rawConfig, &note, &sortOrder); err != nil {
			continue
		}

		// Free users only see free servers
		if userPlan == "free" && plan != "free" {
			// Still show server info but hide config
			rawConfig = ""
		}

		servers = append(servers, gin.H{
			"id":           id,
			"name":         name,
			"region":       region,
			"flag":         flag,
			"country_code": countryCode,
			"protocol":     protocol,
			"plan":         plan,
			"is_active":    isActive,
			"raw_config":   rawConfig,
			"sort_order":   sortOrder,
		})
	}
	if servers == nil {
		servers = []gin.H{}
	}
	response.OK(c, servers)
}
