package main

import (
	"context"
	"errors"
	"log"
	"net/http"

	"ktfc-chms-backend/auth"

	"github.com/jackc/pgx/v5"
)

type UserRole struct {
	Role           string  `json:"role"`
	CongregationID string  `json:"congregation_id"`
	DepartmentID   *string `json:"department_id"`
}

func getUserRoles(ctx context.Context, userID string) ([]UserRole, error) {
	rows, err := db.Query(ctx, `
		SELECT
			role,
			congregation_id::text,
			department_id::text
		FROM account_roles
		WHERE user_account_id = $1
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	roles := []UserRole{}
	for rows.Next() {
		var role UserRole
		if err := rows.Scan(
			&role.Role,
			&role.CongregationID,
			&role.DepartmentID,
		); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}

	return roles, rows.Err()
}

func rolesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "Method not allowed",
		})
		return
	}

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
		log.Printf("Could not load user account role: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not load account",
		})
		return
	}

	roles, err := getUserRoles(r.Context(), claims.UserID)
	if err != nil {
		log.Printf("Could not load user roles: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not load user roles",
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"system_role": systemRole,
		"roles":       roles,
	})
}
