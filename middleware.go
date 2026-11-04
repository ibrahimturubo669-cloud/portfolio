package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"ktfc-chms-backend/auth"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const authClaimsKey contextKey = "authClaims"

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")

		if !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, `{"error":"Authentication required"}`,
				http.StatusUnauthorized)
			return
		}

		tokenString := strings.TrimSpace(
			strings.TrimPrefix(authHeader, "Bearer "),
		)

		secret := os.Getenv("JWT_SECRET")
		if len(secret) < 32 {
			http.Error(w, `{"error":"Authentication configuration error"}`,
				http.StatusInternalServerError)
			return
		}

		claims := &auth.AuthClaims{}

		token, err := jwt.ParseWithClaims(
			tokenString,
			claims,
			func(token *jwt.Token) (interface{}, error) {
				if token.Method != jwt.SigningMethodHS256 {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(secret), nil
			},
			jwt.WithIssuer("ktfc-chms"),
		)

		if err != nil || !token.Valid {
			http.Error(w, `{"error":"Invalid or expired token"}`,
				http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), authClaimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func meHandler(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value(authClaimsKey).(*auth.AuthClaims)

	if !ok {
		http.Error(w, `{"error":"User not authenticated"}`,
			http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(map[string]interface{}{
		"authenticated": true,
		"user_id":       claims.UserID,
		"member_id":     claims.MemberID,
	})
}
