package admin

import (
	"net/http"

	"dsvpn-backend/internal/response"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AppConfigHandler struct {
	db *pgxpool.Pool
}

func NewAppConfigHandler(db *pgxpool.Pool) *AppConfigHandler {
	return &AppConfigHandler{db: db}
}

// Get returns the current app configuration (used by both admin and client).
func (h *AppConfigHandler) Get(c *gin.Context) {
	var (
		latestVersion  string
		versionCode    int
		minVersionCode int
		forceUpdate    bool
		updateTitle    string
		updateMessage  string
		downloadUrl    string
	)

	err := h.db.QueryRow(c.Request.Context(),
		`SELECT latest_version, version_code, min_version_code,
		        force_update, update_title, update_message, download_url
		 FROM app_config WHERE id = 1`,
	).Scan(&latestVersion, &versionCode, &minVersionCode,
		&forceUpdate, &updateTitle, &updateMessage, &downloadUrl)

	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to get app config")
		return
	}

	response.OK(c, gin.H{
		"latest_version":   latestVersion,
		"version_code":     versionCode,
		"min_version_code": minVersionCode,
		"force_update":     forceUpdate,
		"update_title":     updateTitle,
		"update_message":   updateMessage,
		"download_url":     downloadUrl,
	})
}

type updateAppConfigReq struct {
	LatestVersion  *string `json:"latest_version"`
	VersionCode    *int    `json:"version_code"`
	MinVersionCode *int    `json:"min_version_code"`
	ForceUpdate    *bool   `json:"force_update"`
	UpdateTitle    *string `json:"update_title"`
	UpdateMessage  *string `json:"update_message"`
	DownloadUrl    *string `json:"download_url"`
}

// Update saves the app configuration (admin only).
func (h *AppConfigHandler) Update(c *gin.Context) {
	var req updateAppConfigReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	_, err := h.db.Exec(c.Request.Context(),
		`UPDATE app_config SET
			latest_version   = COALESCE($1, latest_version),
			version_code     = COALESCE($2, version_code),
			min_version_code = COALESCE($3, min_version_code),
			force_update     = COALESCE($4, force_update),
			update_title     = COALESCE($5, update_title),
			update_message   = COALESCE($6, update_message),
			download_url     = COALESCE($7, download_url),
			updated_at       = NOW()
		 WHERE id = 1`,
		req.LatestVersion, req.VersionCode, req.MinVersionCode,
		req.ForceUpdate, req.UpdateTitle, req.UpdateMessage, req.DownloadUrl,
	)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to update app config")
		return
	}

	// Return updated config
	h.Get(c)
}
