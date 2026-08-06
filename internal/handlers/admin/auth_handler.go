package admin

import (
	"net/http"

	"dsvpn-backend/internal/response"
	"dsvpn-backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	db   *pgxpool.Pool
	auth *services.AuthService
}

func NewAuthHandler(db *pgxpool.Pool, auth *services.AuthService) *AuthHandler {
	return &AuthHandler{db: db, auth: auth}
}

type loginReq struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	var id string
	var passwordHash string
	err := h.db.QueryRow(c.Request.Context(),
		"SELECT id, password_hash FROM admins WHERE email=$1", req.Email,
	).Scan(&id, &passwordHash)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		response.Error(c, http.StatusUnauthorized, "invalid credentials")
		return
	}

	access, refresh, err := h.auth.GenerateTokenPair(id, req.Email, "admin")
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to generate token")
		return
	}

	response.OK(c, gin.H{"access_token": access, "refresh_token": refresh})
}
