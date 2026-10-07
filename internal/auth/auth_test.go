package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"akari-bridge/internal/config"
	"akari-bridge/internal/storage"
)

func TestAuthService_DefaultAdmin(t *testing.T) {
	cfg := &config.Config{
		AdminUsername: "Admin",
		AdminPassword: "",
		ServerId:      "testserver123",
	}

	svc := NewAuthService(cfg, nil)

	// Admin user should have semantic ID 'admin'
	user, ok := svc.GetUser(DefaultAdminID)
	if !ok || user == nil {
		t.Fatalf("expected admin user with ID 'admin'")
	}
	if user.Id != "admin" {
		t.Errorf("expected user ID 'admin', got '%s'", user.Id)
	}

	// Legacy 32-char ID should alias to 'admin'
	legacyUser, ok := svc.GetUser(LegacyAdminID)
	if !ok || legacyUser == nil {
		t.Fatalf("expected legacy 32-char ID to alias to admin user")
	}
	if legacyUser.Id != "admin" {
		t.Errorf("expected legacy alias to resolve to 'admin', got '%s'", legacyUser.Id)
	}
}

func TestAuthService_MultiUserOperations(t *testing.T) {
	cfg := &config.Config{
		DataDir:       t.TempDir(),
		AdminUsername: "Admin",
		AdminPassword: "adminpassword",
		ServerId:      "testserver123",
	}

	db, err := storage.OpenDB(cfg)
	if err != nil {
		t.Fatalf("OpenDB failed: %v", err)
	}
	defer db.Close()

	svc := NewAuthService(cfg, db)

	// 1. Create a second user
	u2, err := svc.CreateUser("timi", "timipassword", false)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	if u2.Id != "timi" || u2.Name != "timi" {
		t.Errorf("unexpected user: %+v", u2)
	}
	if u2.Policy.IsAdministrator {
		t.Errorf("expected regular user, not admin")
	}

	// 2. Authenticate second user
	authRes, err := svc.Authenticate("timi", "timipassword")
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}
	if authRes.User.Id != "timi" || authRes.AccessToken == "" {
		t.Errorf("unexpected auth result: %+v", authRes)
	}

	// 3. Validate token
	validatedUser, ok := svc.ValidateToken(authRes.AccessToken)
	if !ok || validatedUser == nil || validatedUser.Id != "timi" {
		t.Errorf("ValidateToken failed for timi")
	}

	// 4. Update password
	if err := svc.UpdatePassword("timi", "newpassword"); err != nil {
		t.Fatalf("UpdatePassword failed: %v", err)
	}
	if _, err := svc.Authenticate("timi", "timipassword"); err == nil {
		t.Errorf("expected old password to fail")
	}
	if _, err := svc.Authenticate("timi", "newpassword"); err != nil {
		t.Errorf("expected new password to succeed")
	}

	// 5. Delete user
	if err := svc.DeleteUser("timi"); err != nil {
		t.Fatalf("DeleteUser failed: %v", err)
	}
	if _, ok := svc.GetUser("timi"); ok {
		t.Errorf("expected user timi to be deleted")
	}
	if _, ok := svc.ValidateToken(authRes.AccessToken); ok {
		t.Errorf("expected token to be revoked after user deletion")
	}
}

func TestAuthService_Middleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		AdminUsername: "Admin",
		AdminPassword: "securepassword",
		ServerId:      "testserver123",
	}

	svc := NewAuthService(cfg, nil)
	authRes, _ := svc.Authenticate("Admin", "securepassword")

	r := gin.New()
	r.Use(svc.Middleware())
	r.GET("/emby/Users/admin/Views", func(c *gin.Context) {
		userId := c.GetString("userId")
		c.String(http.StatusOK, "ok:"+userId)
	})

	// 1. Unauthenticated request should be 401
	req := httptest.NewRequest(http.MethodGet, "/emby/Users/admin/Views", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", w.Code)
	}

	// 2. Authenticated request with X-Emby-Token should succeed
	req2 := httptest.NewRequest(http.MethodGet, "/emby/Users/admin/Views", nil)
	req2.Header.Set("X-Emby-Token", authRes.AccessToken)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK || w2.Body.String() != "ok:admin" {
		t.Errorf("expected 200 ok:admin, got %d %s", w2.Code, w2.Body.String())
	}
}
