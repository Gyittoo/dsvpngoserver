package handlers

import (
	"net/http"
	"time"

	"dsvpn-backend/internal/response"
	"dsvpn-backend/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/api/idtoken"
)

type AuthHandler struct {
	db             *pgxpool.Pool
	auth           *services.AuthService
	googleClientID string
}

func NewAuthHandler(db *pgxpool.Pool, auth *services.AuthService, googleClientID string) *AuthHandler {
	return &AuthHandler{db: db, auth: auth, googleClientID: googleClientID}
}

type registerReq struct {
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required,min=8"`
	DisplayName string `json:"display_name"`
	DeviceID    string `json:"device_id" binding:"required"`
}

type loginReq struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
	DeviceID string `json:"device_id" binding:"required"`
}

type googleReq struct {
	IDToken  string `json:"id_token" binding:"required"`
	DeviceID string `json:"device_id" binding:"required"`
}

type refreshReq struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req registerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to hash password")
		return
	}

	var id string
	var createdAt time.Time
	err = h.db.QueryRow(c.Request.Context(),
		`INSERT INTO users (email, password_hash, display_name, bound_device_id)
		 VALUES ($1, $2, $3, $4) RETURNING id, created_at`,
		req.Email, string(hash), req.DisplayName, req.DeviceID,
	).Scan(&id, &createdAt)
	if err != nil {
		response.Error(c, http.StatusConflict, "email already registered")
		return
	}

	access, refresh, err := h.auth.GenerateTokenPair(id, req.Email, "user")
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to generate token")
		return
	}

	response.Created(c, gin.H{
		"access_token":  access,
		"refresh_token": refresh,
		"user": gin.H{
			"id": id, "email": req.Email, "display_name": req.DisplayName,
			"photo_url": nil, "plan": "free", "expiry_date": nil,
			"data_limit_mb": 0, "data_used_mb": 0, "is_active": true,
			"bound_device_id": req.DeviceID, "created_at": createdAt,
		},
	})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	var (
		id           string
		passwordHash string
		displayName  *string
		photoURL     *string
		plan         string
		expiryDate   *time.Time
		isActive     bool
		dataLimitMB  int
		dataUsedMB   int
		createdAt    time.Time
	)
	err := h.db.QueryRow(c.Request.Context(),
		`SELECT id, password_hash, display_name, photo_url, plan, expiry_date,
		        is_active, data_limit_mb, data_used_mb, created_at
		 FROM users WHERE email=$1`, req.Email,
	).Scan(&id, &passwordHash, &displayName, &photoURL, &plan, &expiryDate,
		&isActive, &dataLimitMB, &dataUsedMB, &createdAt)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "invalid email or password")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		response.Error(c, http.StatusUnauthorized, "invalid email or password")
		return
	}

	if !isActive {
		response.Error(c, http.StatusForbidden, "account is deactivated")
		return
	}

	// Update device binding and last login
	h.db.Exec(c.Request.Context(),
		`UPDATE users SET bound_device_id=$1, last_login=NOW() WHERE id=$2`,
		req.DeviceID, id)

	access, refresh, err := h.auth.GenerateTokenPair(id, req.Email, "user")
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to generate token")
		return
	}

	response.OK(c, gin.H{
		"access_token": access, "refresh_token": refresh,
		"user": gin.H{
			"id": id, "email": req.Email, "display_name": displayName,
			"photo_url": photoURL, "plan": plan, "expiry_date": expiryDate,
			"data_limit_mb": dataLimitMB, "data_used_mb": dataUsedMB,
			"is_active": isActive, "bound_device_id": req.DeviceID,
			"created_at": createdAt,
		},
	})
}

func (h *AuthHandler) Google(c *gin.Context) {
	var req googleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	payload, err := idtoken.Validate(c.Request.Context(), req.IDToken, h.googleClientID)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "invalid google token")
		return
	}

	email, _ := payload.Claims["email"].(string)
	name, _ := payload.Claims["name"].(string)
	var picture *string
	if pic, ok := payload.Claims["picture"].(string); ok {
		picture = &pic
	}
	googleID, _ := payload.Claims["sub"].(string)

	var (
		id         string
		plan       string
		expiryDate *time.Time
		isActive   bool
		limitMB    int
		usedMB     int
		createdAt  time.Time
	)
	err = h.db.QueryRow(c.Request.Context(),
		`INSERT INTO users (email, display_name, photo_url, google_id, bound_device_id)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (google_id) DO UPDATE SET
		   display_name=EXCLUDED.display_name,
		   photo_url=EXCLUDED.photo_url,
		   last_login=NOW()
		 RETURNING id, plan, expiry_date, is_active, data_limit_mb, data_used_mb, created_at`,
		email, name, picture, googleID, req.DeviceID,
	).Scan(&id, &plan, &expiryDate, &isActive, &limitMB, &usedMB, &createdAt)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to upsert user")
		return
	}

	access, refresh, err := h.auth.GenerateTokenPair(id, email, "user")
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to generate token")
		return
	}

	response.OK(c, gin.H{
		"access_token": access, "refresh_token": refresh,
		"user": gin.H{
			"id": id, "email": email, "display_name": name,
			"photo_url": picture, "plan": plan, "expiry_date": expiryDate,
			"is_active": isActive, "data_limit_mb": limitMB, "data_used_mb": usedMB,
			"bound_device_id": req.DeviceID, "created_at": createdAt,
		},
	})
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req refreshReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	claims, err := h.auth.ParseToken(req.RefreshToken)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "invalid refresh token")
		return
	}

	access, refresh, err := h.auth.GenerateTokenPair(claims.Subject, claims.Email, claims.Role)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to rotate tokens")
		return
	}

	response.OK(c, gin.H{"access_token": access, "refresh_token": refresh})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	response.OK(c, nil)
}
