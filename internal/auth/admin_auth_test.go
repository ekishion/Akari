package auth

import (
	"os"
	"testing"
	"time"

	"akari-bridge/internal/config"
	"akari-bridge/internal/storage"
)

func TestAdminAuthService_Lifecycle(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "admin_auth_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := &config.Config{DataDir: tempDir}
	db, err := storage.OpenDB(cfg)
	if err != nil {
		t.Fatalf("OpenDB failed: %v", err)
	}
	defer db.Close()

	svc := NewAdminAuthService(cfg, db)

	// Test 1: Default login
	token, exp, err := svc.Login("admin", "admin123", "127.0.0.1", "TestAgent")
	if err != nil {
		t.Fatalf("Login failed for default admin: %v", err)
	}
	if token == "" || exp.Before(time.Now()) {
		t.Fatalf("Invalid token or expiry: %s, %v", token, exp)
	}

	// Test 2: Validate token
	claims, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}
	if claims.Username != "admin" {
		t.Fatalf("Expected username admin, got %s", claims.Username)
	}

	// Test 3: Failed login and brute force locking
	for i := 1; i <= 4; i++ {
		_, _, err := svc.Login("admin", "wrongpassword", "192.168.1.50", "TestAgent")
		if err == nil {
			t.Fatalf("Expected error for wrong password on attempt %d", i)
		}
	}

	// 5th attempt triggers ban
	_, _, err = svc.Login("admin", "wrongpassword", "192.168.1.50", "TestAgent")
	if err == nil {
		t.Fatalf("Expected error on 5th failed attempt")
	}

	// 6th attempt should be blocked due to ban
	_, _, err = svc.Login("admin", "admin123", "192.168.1.50", "TestAgent")
	if err == nil {
		t.Fatalf("Expected blocked login from banned IP 192.168.1.50")
	}

	// Unban
	if err := db.UnbanIP("192.168.1.50"); err != nil {
		t.Fatalf("UnbanIP failed: %v", err)
	}

	// Now should succeed
	_, _, err = svc.Login("admin", "admin123", "192.168.1.50", "TestAgent")
	if err != nil {
		t.Fatalf("Login after unban failed: %v", err)
	}

	// Test 4: Change password
	if err := svc.ChangePassword("admin", "admin123", "newSecret999", "127.0.0.1", "TestAgent"); err != nil {
		t.Fatalf("ChangePassword failed: %v", err)
	}

	// Old password must fail
	_, _, err = svc.Login("admin", "admin123", "127.0.0.1", "TestAgent")
	if err == nil {
		t.Fatalf("Expected old password to fail")
	}

	// New password succeeds
	newToken, _, err := svc.Login("admin", "newSecret999", "127.0.0.1", "TestAgent")
	if err != nil {
		t.Fatalf("Login with new password failed: %v", err)
	}
	if newToken == "" {
		t.Fatalf("Expected valid token")
	}
}
