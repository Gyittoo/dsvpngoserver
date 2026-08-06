package middleware

import (
	"net/http"
	"strings"

	"dsvpn-backend/internal/response"
	"dsvpn-backend/internal/services"
	"github.com/gin-gonic/gin"
)

const (
	CtxUserIDKey = "user_id"
	CtxRoleKey   = "role"
	CtxEmailKey  = "email"
)

func AuthRequired(auth *services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			response.Error(c, http.StatusUnauthorized, "missing or invalid authorization header")
			c.Abort()
			return
		}

		token := strings.TrimPrefix(header, "Bearer ")
		claims, err := auth.ParseToken(token)
		if err != nil {
			response.Error(c, http.StatusUnauthorized, "invalid or expired token")
			c.Abort()
			return
		}

		c.Set(CtxUserIDKey, claims.Subject)
		c.Set(CtxRoleKey, claims.Role)
		c.Set(CtxEmailKey, claims.Email)
		c.Next()
	}
}

func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, ok := c.Get(CtxRoleKey)
		if !ok || role != "admin" {
			response.Error(c, http.StatusForbidden, "admin role required")
			c.Abort()
			return
		}
		c.Next()
	}
}
