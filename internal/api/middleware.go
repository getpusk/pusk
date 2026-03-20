// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package api

import (
	"context"
	"net/http"

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
		ctx := context.WithValue(r.Context(), ctxKeyUserID, claims.UserID)
		ctx = context.WithValue(ctx, ctxKeyClaims, claims)
		next(w, r.WithContext(ctx))
	}
}
