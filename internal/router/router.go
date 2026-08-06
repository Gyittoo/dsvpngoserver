package router

import (
	"dsvpn-backend/internal/handlers"
	adminHandlers "dsvpn-backend/internal/handlers/admin"
	"dsvpn-backend/internal/middleware"
	"dsvpn-backend/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func New(db *pgxpool.Pool, authService *services.AuthService, googleClientID string) *gin.Engine {
	r := gin.Default()

	authHandler := handlers.NewAuthHandler(db, authService, googleClientID)
	userHandler := handlers.NewUserHandler(db)
	serverHandler := handlers.NewServerHandler(db)
	voucherHandler := handlers.NewVoucherHandler(db)
	announcementHandler := handlers.NewAnnouncementHandler(db)

	adminAuthHandler := adminHandlers.NewAuthHandler(db, authService)
	adminUserHandler := adminHandlers.NewUserAdminHandler(db)
	adminServerHandler := adminHandlers.NewServerAdminHandler(db)
	adminVoucherHandler := adminHandlers.NewVoucherAdminHandler(db)
	adminAnnouncementHandler := adminHandlers.NewAnnouncementAdminHandler(db)

	api := r.Group("/api/v1")
	{
		auth := api.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
			auth.POST("/google", authHandler.Google)
			auth.POST("/refresh", authHandler.Refresh)
			auth.POST("/logout", middleware.AuthRequired(authService), authHandler.Logout)
		}

		secured := api.Group("", middleware.AuthRequired(authService))
		{
			secured.GET("/users/me", userHandler.Me)
			secured.PATCH("/users/me/data-usage", userHandler.UpdateDataUsage)
			secured.POST("/users/me/reset-plan", userHandler.ResetPlan)
			secured.DELETE("/users/me", userHandler.DeleteAccount)

			secured.GET("/servers", serverHandler.ListActive)
			secured.POST("/vouchers/redeem", voucherHandler.Redeem)
			secured.GET("/announcements", announcementHandler.Active)
			secured.POST("/connections/log", announcementHandler.LogConnection)
		}

		admin := api.Group("/admin")
		{
			admin.POST("/auth/login", adminAuthHandler.Login)

			adminSecured := admin.Group("", middleware.AuthRequired(authService), middleware.AdminRequired())
			{
				adminSecured.GET("/users", adminUserHandler.List)
				adminSecured.GET("/users/:id", adminUserHandler.Get)
				adminSecured.PUT("/users/:id", adminUserHandler.Update)
				adminSecured.POST("/users/:id/reset-device", adminUserHandler.ResetDevice)
				adminSecured.POST("/users/:id/reset-plan", adminUserHandler.ResetPlan)
				adminSecured.DELETE("/users/:id", adminUserHandler.Delete)

				adminSecured.GET("/servers", adminServerHandler.List)
				adminSecured.POST("/servers", adminServerHandler.Create)
				adminSecured.PUT("/servers/:id", adminServerHandler.Update)
				adminSecured.DELETE("/servers/:id", adminServerHandler.Delete)

				adminSecured.GET("/vouchers", adminVoucherHandler.List)
				adminSecured.POST("/vouchers", adminVoucherHandler.Create)
				adminSecured.DELETE("/vouchers/:id", adminVoucherHandler.Delete)

				adminSecured.GET("/announcements", adminAnnouncementHandler.List)
				adminSecured.POST("/announcements", adminAnnouncementHandler.Create)
				adminSecured.PUT("/announcements/:id", adminAnnouncementHandler.Update)
				adminSecured.DELETE("/announcements/:id", adminAnnouncementHandler.Delete)
			}
		}
	}

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	return r
}
