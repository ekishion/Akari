package router

import (
	"github.com/gin-gonic/gin"
	"akari-bridge/internal/auth"
	"akari-bridge/internal/controller/admin"
)

type AdminRouter struct {
	adminAuth *auth.AdminAuthService
	authCtrl  *admin.AuthController
	userCtrl  *admin.UserController
	ruleCtrl  *admin.RuleController
	sysCtrl   *admin.SystemController
	biliCtrl  *admin.BilibiliController
}

func NewAdminRouter(
	adminAuth *auth.AdminAuthService,
	authCtrl *admin.AuthController,
	userCtrl *admin.UserController,
	ruleCtrl *admin.RuleController,
	sysCtrl *admin.SystemController,
	biliCtrl *admin.BilibiliController,
) *AdminRouter {
	return &AdminRouter{
		adminAuth: adminAuth,
		authCtrl:  authCtrl,
		userCtrl:  userCtrl,
		ruleCtrl:  ruleCtrl,
		sysCtrl:   sysCtrl,
		biliCtrl:  biliCtrl,
	}
}

func (r *AdminRouter) RegisterRoutes(apiGroup *gin.RouterGroup) {
	// Public Auth & Telemetry
	apiGroup.POST("/auth/login", r.authCtrl.Login)
	apiGroup.GET("/events", r.sysCtrl.EventsStream)

	// Protected Admin Routes
	protected := apiGroup.Group("")
	protected.Use(AdminAuthMiddleware(r.adminAuth))
	{
		// Admin Profile & Password
		protected.POST("/auth/logout", r.authCtrl.Logout)
		protected.GET("/auth/me", r.authCtrl.GetProfile)
		protected.POST("/auth/change-password", r.authCtrl.ChangePassword)

		// Security & Audit
		protected.GET("/security/audit", r.sysCtrl.GetAuditLogs)
		protected.DELETE("/security/audit", r.sysCtrl.ClearAuditLogs)
		protected.GET("/security/bans", r.sysCtrl.GetBannedIPs)
		protected.POST("/security/unban", r.sysCtrl.UnbanIP)

		// System
		protected.GET("/system/status", r.sysCtrl.GetStatus)
		protected.GET("/system/config", r.sysCtrl.GetConfig)
		protected.POST("/system/config", r.sysCtrl.UpdateConfig)
		protected.PUT("/system/config", r.sysCtrl.UpdateConfig)
		protected.POST("/system/clean-cache", r.sysCtrl.CleanCache)
		protected.POST("/system/bangumi/test", r.sysCtrl.TestBangumiEndpoint)
		protected.POST("/system/bangumi-image/test", r.sysCtrl.TestBangumiImageEndpoint)

		// Bilibili
		protected.GET("/bilibili/status", r.biliCtrl.GetBilibiliStatus)
		protected.POST("/bilibili/config", r.biliCtrl.UpdateBilibiliConfig)
		protected.POST("/bilibili/qr/generate", r.biliCtrl.GenerateBilibiliQR)
		protected.GET("/bilibili/qr/poll", r.biliCtrl.PollBilibiliQR)
		protected.POST("/bilibili/logout", r.biliCtrl.LogoutBilibili)

		// Users
		protected.GET("/users", r.userCtrl.ListUsers)
		protected.POST("/users", r.userCtrl.CreateUser)
		protected.DELETE("/users/:id", r.userCtrl.DeleteUser)
		protected.POST("/users/:id/password", r.userCtrl.SetUserPassword)
		protected.GET("/users/:id/tokens", r.userCtrl.GetUserTokens)
		protected.POST("/users/:id/tokens", r.userCtrl.CreateUserToken)
		protected.DELETE("/users/:id/tokens/:token", r.userCtrl.RevokeUserToken)
		protected.POST("/users/:id/bangumi", r.userCtrl.BindUserBangumi)
		protected.DELETE("/users/:id/bangumi", r.userCtrl.UnbindUserBangumi)

		// Rules
		protected.GET("/rules", r.ruleCtrl.ListRules)
		protected.GET("/rules/:name", r.ruleCtrl.GetRule)
		protected.POST("/rules", r.ruleCtrl.SaveRule)
		protected.PUT("/rules/:name", r.ruleCtrl.SaveRule)
		protected.PUT("/rules/:name/toggle", r.ruleCtrl.ToggleRule)
		protected.DELETE("/rules/:name", r.ruleCtrl.DeleteRule)
		protected.POST("/rules/import-url", r.ruleCtrl.ImportRulesFromURL)
		protected.POST("/rules/update-all", r.ruleCtrl.UpdateAllRules)
		protected.POST("/rules/test", r.ruleCtrl.TestRule)

		// Aliases & Synonyms
		protected.GET("/aliases/synonyms", r.sysCtrl.ListGlobalSynonyms)
		protected.POST("/aliases/synonyms", r.sysCtrl.UpsertGlobalSynonym)
		protected.DELETE("/aliases/synonyms/:pattern", r.sysCtrl.DeleteGlobalSynonym)
		protected.POST("/aliases/synonyms/reset", r.sysCtrl.ResetGlobalSynonyms)

		protected.GET("/aliases/subjects", r.sysCtrl.ListSubjectAliases)
		protected.POST("/aliases/subjects", r.sysCtrl.UpsertSubjectAliases)
		protected.DELETE("/aliases/subjects/:subject_id", r.sysCtrl.DeleteSubjectAliases)

		// History
		protected.GET("/history", r.sysCtrl.ListHistory)
	}
}
