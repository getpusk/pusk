// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestNewJWTService_PanicsOnEmptySecret(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on empty secret")
		}
	}()
	NewJWTService("", 168)
}

func TestGenerateAndValidate(t *testing.T) {
	j := NewJWTService("test-secret-key", 168)
	tok, err := j.Generate(42, "myorg", "admin")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	claims, err := j.Validate(tok)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if claims.UserID != 42 {
		t.Errorf("UserID = %d, want 42", claims.UserID)
	}
	if claims.OrgID != "myorg" {
		t.Errorf("OrgID = %q, want myorg", claims.OrgID)
	}
	if claims.Username != "admin" {
		t.Errorf("Username = %q, want admin", claims.Username)
	}
}

func TestValidate_InvalidToken(t *testing.T) {
	j := NewJWTService("secret", 168)
	_, err := j.Validate("not-a-jwt")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
	if err != ErrInvalidToken {
		t.Errorf("err = %v, want ErrInvalidToken", err)
	}
}

func TestValidate_WrongSecret(t *testing.T) {
	j1 := NewJWTService("secret1", 168)
	j2 := NewJWTService("secret2", 168)
	tok, _ := j1.Generate(1, "org", "user")
	_, err := j2.Validate(tok)
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func TestValidate_ExpiredToken(t *testing.T) {
	j := NewJWTService("secret", 1)
	claims := Claims{
		UserID:   1,
		OrgID:    "org",
		Username: "user",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			Subject:   "user",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString([]byte("secret"))
	_, err := j.Validate(tokenStr)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestValidate_NoneSigningMethod(t *testing.T) {
	claims := Claims{
		UserID:   1,
		OrgID:    "org",
		Username: "user",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: "user",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	tokenStr, _ := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	j := NewJWTService("secret", 168)
	_, err := j.Validate(tokenStr)
	if err == nil {
		t.Fatal("expected error for none signing method")
	}
}

func TestGenerate_SetsTTL(t *testing.T) {
	j := NewJWTService("secret", 24) // 24 hours
	tok, _ := j.Generate(1, "org", "user")
	claims, err := j.Validate(tok)
	if err != nil {
		t.Fatal(err)
	}
	exp := claims.ExpiresAt.Time
	// Should expire roughly 24h from now (within 1 minute tolerance)
	diff := time.Until(exp)
	if diff < 23*time.Hour || diff > 25*time.Hour {
		t.Errorf("expiry diff = %v, want ~24h", diff)
	}
}

func TestValidate_EmptyString(t *testing.T) {
	j := NewJWTService("secret", 168)
	_, err := j.Validate("")
	if err == nil {
		t.Fatal("expected error for empty token string")
	}
}
