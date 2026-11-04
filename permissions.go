package main

import (
	"context"
	"errors"
	"log"
	"net/http"

	"ktfc-chms-backend/auth"

	"github.com/jackc/pgx/v5"
)

func requireSystemRole(requiredRole string, next http.Handler) http.Handler {
	return authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := r.Context().Value(authClaimsKey).(*auth.AuthClaims)
		if !ok || claims == nil {
			respondJSON(w, http.StatusUnauthorized, map[string]string{
				"error": "Unauthorized",
			})
			return
		}

		var systemRole string
		err := db.QueryRow(
			r.Context(),
			`SELECT system_role
			 FROM user_accounts
			 WHERE id = $1 AND is_active = TRUE`,
			claims.UserID,
		).Scan(&systemRole)
		if errors.Is(err, pgx.ErrNoRows) {
			respondJSON(w, http.StatusUnauthorized, map[string]string{
				"error": "Unauthorized",
			})
			return
		}
		if err != nil {
			log.Printf("Could not load user system role: %v", err)
			respondJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "Could not load account",
			})
			return
		}

		if systemRole != "super_admin" && systemRole != requiredRole {
			respondJSON(w, http.StatusForbidden, map[string]string{
				"error": "Permission denied",
			})
			return
		}

		next.ServeHTTP(w, r)
	}))
}

func hasCongregationRole(
	ctx context.Context,
	userID string,
	congregationID string,
	departmentID *string,
	requiredRole string,
) (bool, error) {
	var allowed bool

	err := db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM user_accounts ua
			WHERE ua.id = $1
			  AND ua.is_active = TRUE
			  AND (
				  ua.system_role = 'super_admin'
				  OR EXISTS (
					  SELECT 1
					  FROM account_roles ar
					  WHERE ar.user_account_id = ua.id
					    AND ar.congregation_id = $2
					    AND ar.role = $4
					    AND ar.department_id
					        IS NOT DISTINCT FROM $3::uuid
				  )
			  )
		)
	`,
		userID,
		congregationID,
		departmentID,
		requiredRole,
	).Scan(&allowed)
	if err != nil {
		return false, err
	}

	return allowed, nil
}

func requireCongregationRole(
	requiredRole string,
	next http.Handler,
) http.Handler {

	return authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := r.Context().Value(authClaimsKey).(*auth.AuthClaims)
		if !ok || claims == nil {
			respondJSON(w, http.StatusUnauthorized, map[string]string{
				"error": "Unauthorized",
			})
			return
		}

		congregationID := r.URL.Query().Get("congregation_id")
		if congregationID == "" {
			respondJSON(w, http.StatusBadRequest, map[string]string{
				"error": "congregation_id is required",
			})
			return
		}

		var departmentID *string
		departmentValue := r.URL.Query().Get("department_id")
		if departmentValue != "" {
			departmentID = &departmentValue
		}

		if requiredRole == "department_leader" && departmentID == nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{
				"error": "department_id is required",
			})
			return
		}

		allowed, err := hasCongregationRole(
			r.Context(),
			claims.UserID,
			congregationID,
			departmentID,
			requiredRole,
		)
		if err != nil {
			log.Printf("Permission check failed: %v", err)
			respondJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "Permission check failed",
			})
			return
		}

		if !allowed {
			respondJSON(w, http.StatusForbidden, map[string]string{
				"error": "Forbidden",
			})
			return
		}

		next.ServeHTTP(w, r)
	}))
}

func adminCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	w.Write([]byte(`{"message":"Admin permission check passed"}`))
}
