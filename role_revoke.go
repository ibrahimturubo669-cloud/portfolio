package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5"
)

type revokeRoleRequest struct {
	AssignmentID string `json:"assignment_id"`
}

func revokeRoleHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "Method not allowed",
		})
		return
	}

	var input revokeRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.AssignmentID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "A valid assignment_id is required",
		})
		return
	}

	tx, err := db.Begin(r.Context())
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not start transaction",
		})
		return
	}
	defer tx.Rollback(r.Context())

	var congregationID string
	var role string

	err = tx.QueryRow(r.Context(), `
		SELECT congregation_id::text, role
		FROM account_roles
		WHERE id = $1::uuid
		FOR UPDATE
	`, input.AssignmentID).Scan(&congregationID, &role)

	if errors.Is(err, pgx.ErrNoRows) {
		respondJSON(w, http.StatusNotFound, map[string]string{
			"error": "Role assignment not found",
		})
		return
	}

	if err != nil {
		log.Printf("Could not find role assignment: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not find role assignment",
		})
		return
	}

	// Protect the last active church administrator.
	if role == "church_admin" {
		var adminCount int

		err = tx.QueryRow(r.Context(), `
			SELECT COUNT(*)
			FROM account_roles ar
			JOIN user_accounts ua ON ua.id = ar.user_account_id
			WHERE ar.congregation_id = $1::uuid
			  AND ar.role = 'church_admin'
			  AND ua.is_active = TRUE
		`, congregationID).Scan(&adminCount)

		if err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "Could not check administrator assignments",
			})
			return
		}

		if adminCount <= 1 {
			respondJSON(w, http.StatusConflict, map[string]string{
				"error": "Cannot remove the last active church administrator",
			})
			return
		}
	}

	_, err = tx.Exec(r.Context(), `
		DELETE FROM account_roles
		WHERE id = $1::uuid
	`, input.AssignmentID)

	if err != nil {
		log.Printf("Could not revoke role: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not revoke role",
		})
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		log.Printf("Could not commit role revocation: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not complete role revocation",
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Role assignment revoked successfully",
	})
}
