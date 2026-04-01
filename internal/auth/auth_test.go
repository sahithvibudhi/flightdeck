package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestHashPassword(t *testing.T) {
	hash, err := HashPassword("testpass123")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if len(hash) == 0 {
		t.Fatal("hash is empty")
	}
	if hash == "testpass123" {
		t.Fatal("hash should not equal plaintext")
	}
}

func TestCheckPassword(t *testing.T) {
	hash, _ := HashPassword("correct-password")

	if !CheckPassword(hash, "correct-password") {
		t.Error("expected true for correct password")
	}
	if CheckPassword(hash, "wrong-password") {
		t.Error("expected false for wrong password")
	}
	if CheckPassword("not-a-hash", "anything") {
		t.Error("expected false for invalid hash")
	}
}

func TestGenerateSecret(t *testing.T) {
	s1, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret failed: %v", err)
	}
	if len(s1) != 64 {
		t.Errorf("expected 64-char hex, got %d chars", len(s1))
	}

	s2, _ := GenerateSecret()
	if s1 == s2 {
		t.Error("two calls should produce different secrets")
	}
}

func TestIssueAndVerifyToken(t *testing.T) {
	secret := "my-test-secret"
	token, err := IssueToken("admin", secret)
	if err != nil {
		t.Fatalf("IssueToken failed: %v", err)
	}

	username, err := VerifyToken(token, secret)
	if err != nil {
		t.Fatalf("VerifyToken failed: %v", err)
	}
	if username != "admin" {
		t.Errorf("expected username 'admin', got '%s'", username)
	}
}

func TestVerifyToken_WrongSecret(t *testing.T) {
	token, _ := IssueToken("admin", "secret-one")

	_, err := VerifyToken(token, "secret-two")
	if err == nil {
		t.Error("expected error for wrong secret")
	}
}

func TestVerifyToken_Malformed(t *testing.T) {
	_, err := VerifyToken("not-a-jwt", "secret")
	if err == nil {
		t.Error("expected error for malformed token")
	}
}

func TestVerifyToken_Expired(t *testing.T) {
	secret := "test-secret"
	claims := jwt.MapClaims{
		"sub": "admin",
		"exp": time.Now().Add(-1 * time.Hour).Unix(),
		"iat": time.Now().Add(-2 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString([]byte(secret))

	_, err := VerifyToken(tokenStr, secret)
	if err == nil {
		t.Error("expected error for expired token")
	}
}
