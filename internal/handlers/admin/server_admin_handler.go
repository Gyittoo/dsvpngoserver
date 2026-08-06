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

type ServerAdminHandler struct {
	db *pgxpool.Pool
}

func NewServerAdminHandler(db *pgxpool.Pool) *ServerAdminHandler {
	return &ServerAdminHandler{db: db}
}

func (h *ServerAdminHandler) List(c *gin.Context) {
	rows, err := h.db.Query(c.Request.Context(),
		`SELECT id, name, region, flag, country_code, protocol, plan, is_active,
		        host, port, raw_config, note, sort_order, created_at, updated_at
		 FROM servers ORDER BY sort_order`)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to query servers")
		return
	}
	defer rows.Close()

	var servers []gin.H
	for rows.Next() {
		var (
			id, name, region, flag, countryCode, protocol, plan string
			host, rawConfig, note                               string
			isActive                                            bool
			port, sortOrder                                     int
			createdAt, updatedAt                                time.Time
		)
		err := rows.Scan(&id, &name, &region, &flag, &countryCode, &protocol, &plan, &isActive,
			&host, &port, &rawConfig, &note, &sortOrder, &createdAt, &updatedAt)
		if err != nil {
			continue
		}
		servers = append(servers, gin.H{
			"id": id, "name": name, "region": region, "flag": flag,
			"country_code": countryCode, "protocol": protocol, "plan": plan,
			"is_active": isActive, "host": host, "port": port,
			"raw_config": rawConfig, "note": note, "sort_order": sortOrder,
			"created_at": createdAt, "updated_at": updatedAt,
		})
	}
	if servers == nil {
		servers = []gin.H{}
	}
	response.OK(c, servers)
}

type createServerReq struct {
	Name        string `json:"name" binding:"required"`
	Region      string `json:"region"`
	Flag        string `json:"flag"`
	CountryCode string `json:"country_code"`
	Protocol    string `json:"protocol"`
	Plan        string `json:"plan"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	RawConfig   string `json:"raw_config"`
	Note        string `json:"note"`
	SortOrder   int    `json:"sort_order"`
}

func (h *ServerAdminHandler) Create(c *gin.Context) {
	var req createServerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	var id string
	var createdAt, updatedAt time.Time
	err := h.db.QueryRow(c.Request.Context(),
		`INSERT INTO servers (name, region, flag, country_code, protocol, plan, host, port, raw_config, note, sort_order)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		 RETURNING id, created_at, updated_at`,
		req.Name, req.Region, req.Flag, req.CountryCode, req.Protocol,
		req.Plan, req.Host, req.Port, req.RawConfig, req.Note, req.SortOrder,
	).Scan(&id, &createdAt, &updatedAt)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to create server")
		return
	}

	response.Created(c, gin.H{
		"id": id, "name": req.Name, "region": req.Region, "flag": req.Flag,
		"country_code": req.CountryCode, "protocol": req.Protocol, "plan": req.Plan,
		"is_active": true, "host": req.Host, "port": req.Port,
		"raw_config": req.RawConfig, "note": req.Note, "sort_order": req.SortOrder,
		"created_at": createdAt, "updated_at": updatedAt,
	})
}

type updateServerReq struct {
	Name        *string `json:"name"`
	Region      *string `json:"region"`
	Flag        *string `json:"flag"`
	CountryCode *string `json:"country_code"`
	Protocol    *string `json:"protocol"`
	Plan        *string `json:"plan"`
	IsActive    *bool   `json:"is_active"`
	Host        *string `json:"host"`
	Port        *int    `json:"port"`
	RawConfig   *string `json:"raw_config"`
	Note        *string `json:"note"`
	SortOrder   *int    `json:"sort_order"`
}

func (h *ServerAdminHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var req updateServerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	setClauses := []string{}
	args := []interface{}{}
	argIdx := 1

	if req.Name != nil {
		setClauses = append(setClauses, fmt.Sprintf("name=$%d", argIdx))
		args = append(args, *req.Name)
		argIdx++
	}
	if req.Region != nil {
		setClauses = append(setClauses, fmt.Sprintf("region=$%d", argIdx))
		args = append(args, *req.Region)
		argIdx++
	}
	if req.Flag != nil {
		setClauses = append(setClauses, fmt.Sprintf("flag=$%d", argIdx))
		args = append(args, *req.Flag)
		argIdx++
	}
	if req.CountryCode != nil {
		setClauses = append(setClauses, fmt.Sprintf("country_code=$%d", argIdx))
		args = append(args, *req.CountryCode)
		argIdx++
	}
	if req.Protocol != nil {
		setClauses = append(setClauses, fmt.Sprintf("protocol=$%d", argIdx))
		args = append(args, *req.Protocol)
		argIdx++
	}
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
	if req.Host != nil {
		setClauses = append(setClauses, fmt.Sprintf("host=$%d", argIdx))
		args = append(args, *req.Host)
		argIdx++
	}
	if req.Port != nil {
		setClauses = append(setClauses, fmt.Sprintf("port=$%d", argIdx))
		args = append(args, *req.Port)
		argIdx++
	}
	if req.RawConfig != nil {
		setClauses = append(setClauses, fmt.Sprintf("raw_config=$%d", argIdx))
		args = append(args, *req.RawConfig)
		argIdx++
	}
	if req.Note != nil {
		setClauses = append(setClauses, fmt.Sprintf("note=$%d", argIdx))
		args = append(args, *req.Note)
		argIdx++
	}
	if req.SortOrder != nil {
		setClauses = append(setClauses, fmt.Sprintf("sort_order=$%d", argIdx))
		args = append(args, *req.SortOrder)
		argIdx++
	}

	if len(setClauses) == 0 {
		response.Error(c, http.StatusBadRequest, "no fields to update")
		return
	}

	setClauses = append(setClauses, "updated_at=NOW()")
	args = append(args, id)
	query := fmt.Sprintf("UPDATE servers SET %s WHERE id=$%d", strings.Join(setClauses, ", "), argIdx)

	result, err := h.db.Exec(c.Request.Context(), query, args...)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to update server")
		return
	}
	if result.RowsAffected() == 0 {
		response.Error(c, http.StatusNotFound, "server not found")
		return
	}
	response.OK(c, gin.H{"message": "server updated"})
}

func (h *ServerAdminHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	result, err := h.db.Exec(c.Request.Context(), "DELETE FROM servers WHERE id=$1", id)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to delete server")
		return
	}
	if result.RowsAffected() == 0 {
		response.Error(c, http.StatusNotFound, "server not found")
		return
	}
	response.OK(c, nil)
}
