// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package api

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/pusk-platform/pusk/internal/auth"
)

type ctxKey int

const (
	ctxKeyUserID ctxKey = iota
	ctxKeyClaims
)

// UserIDFromCtx extracts user ID stored by AuthRequired middleware.
func UserIDFromCtx(ctx context.Context) int64 {
	if v, ok := ctx.Value(ctxKeyUserID).(int64); ok {
		return v
	}
	return 0
}

// ClaimsFromCtx extracts JWT claims stored by AuthRequired middleware.
func ClaimsFromCtx(ctx context.Context) *auth.Claims {
	if v, ok := ctx.Value(ctxKeyClaims).(*auth.Claims); ok {
		return v
	}
	return nil
}

// ── Token revocation ──
var revokedUsers sync.Map // "orgID:userID" -> time.Time

// RevokeUser invalidates all existing JWTs for a user (called on delete/password change).
func RevokeUser(orgID string, userID int64) {
	revokedUsers.Store(orgID+":"+strconv.FormatInt(userID, 10), time.Now())
}

// AuthRequired validates JWT from Authorization header or ?token= query param,
// stores userID and claims in context, and returns 401 if invalid.
func (a *ClientAPI) AuthRequired(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tokenStr := r.Header.Get("Authorization")
		if tokenStr == "" {
			tokenStr = r.URL.Query().Get("token")
		}
		if tokenStr == "" {
			jsonErr(w, "unauthorized", 401)
			return
		}
		claims, err := a.jwt.Validate(tokenStr)
		if err != nil {
			jsonErr(w, "unauthorized", 401)
			return
		}
		// Check token revocation
		rKey := claims.OrgID + ":" + strconv.FormatInt(claims.UserID, 10)
		if t, ok := revokedUsers.Load(rKey); ok {
			revokedAt := t.(time.Time)
			if time.Since(revokedAt) > 7*24*time.Hour {
				revokedUsers.Delete(rKey)
			} else if claims.IssuedAt != nil && claims.IssuedAt.Before(revokedAt) {
				jsonErr(w, "token revoked", 401)
				return
			}
		}
		ctx := context.WithValue(r.Context(), ctxKeyUserID, claims.UserID)
		ctx = context.WithValue(ctx, ctxKeyClaims, claims)
		next(w, r.WithContext(ctx))
	}
}
