package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
)

type assignRoleRequest struct {
	Email          string  `json:"email"`
	CongregationID string  `json:"congregation_id"`
	DepartmentID   *string `json:"department_id"`
	Role           string  `json:"role"`
}

var uuidPattern = regexp.MustCompile(
	`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`,
)

func assignRoleHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "Method not allowed",
		})
		return
	}

	var input assignRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid JSON request",
		})
		return
	}

	input.Email = strings.TrimSpace(strings.ToLower(input.Email))
	input.Role = strings.TrimSpace(input.Role)
	input.CongregationID = strings.TrimSpace(input.CongregationID)

	if input.Email == "" || !uuidPattern.MatchString(input.CongregationID) {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "A valid email and congregation_id are required",
		})
		return
	}

	allowedRoles := map[string]bool{
		"church_admin":      true,
		"treasurer":         true,
		"secretary":         true,
		"department_leader": true,
		"member":            true,
	}

	if !allowedRoles[input.Role] {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid role",
		})
		return
	}

	if input.Role == "department_leader" {
		if input.DepartmentID == nil ||
			!uuidPattern.MatchString(strings.TrimSpace(*input.DepartmentID)) {
			respondJSON(w, http.StatusBadRequest, map[string]string{
				"error": "A valid department_id is required for department leaders",
			})
			return
		}
	}

	ctx := r.Context()

	// Find the active account being assigned the role.
	var userID string
	err := db.QueryRow(ctx, `
		SELECT id::text
		FROM user_accounts
		WHERE LOWER(email) = $1
		  AND is_active = TRUE
	`, input.Email).Scan(&userID)

	if errors.Is(err, pgx.ErrNoRows) {
		respondJSON(w, http.StatusNotFound, map[string]string{
			"error": "No active user account found with that email",
		})
		return
	}
	if err != nil {
		log.Printf("Could not find account: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not find user account",
		})
		return
	}

	// Confirm that the congregation exists.
	var congregationExists bool
	err = db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM congregations WHERE id = $1::uuid
		)
	`, input.CongregationID).Scan(&congregationExists)

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not check congregation",
		})
		return
	}
	if !congregationExists {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Congregation not found",
		})
		return
	}

	// If a department was supplied, confirm that it exists.
	var departmentID interface{}
	if input.DepartmentID != nil &&
		strings.TrimSpace(*input.DepartmentID) != "" {

		department := strings.TrimSpace(*input.DepartmentID)
		if !uuidPattern.MatchString(department) {
			respondJSON(w, http.StatusBadRequest, map[string]string{
				"error": "Invalid department_id",
			})
			return
		}

		var departmentExists bool
		err = db.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM departments WHERE id = $1::uuid
			)
		`, department).Scan(&departmentExists)

		if err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "Could not check department",
			})
			return
		}
		if !departmentExists {
			respondJSON(w, http.StatusBadRequest, map[string]string{
				"error": "Department not found",
			})
			return
		}

		departmentID = department
	}

	if input.Role == "department_leader" && departmentID == nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Department leaders must be assigned to a department",
		})
		return
	}

	_, err = db.Exec(ctx, `
		INSERT INTO account_roles (
			user_account_id,
			congregation_id,
			department_id,
			role
		)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4)
		ON CONFLICT DO NOTHING
	`, userID, input.CongregationID, departmentID, input.Role)

	if err != nil {
		log.Printf("Could not assign role: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not assign role",
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Role assignment saved",
	})
}
