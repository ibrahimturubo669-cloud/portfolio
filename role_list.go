package main

import (
	"log"
	"net/http"
	"time"
)

type RoleAssignment struct {
	ID             string    `json:"id"`
	FullName       string    `json:"full_name"`
	Email          string    `json:"email"`
	Role           string    `json:"role"`
	CongregationID string    `json:"congregation_id"`
	Congregation   string    `json:"congregation"`
	DepartmentID   *string   `json:"department_id"`
	Department     *string   `json:"department"`
	CreatedAt      time.Time `json:"created_at"`
}

func listRoleAssignmentsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "Method not allowed",
		})
		return
	}

	rows, err := db.Query(r.Context(), `
		SELECT
			ar.id::text,
			COALESCE(m.full_name, ''),
			ua.email,
			ar.role,
			ar.congregation_id::text,
			c.name,
			ar.department_id::text,
			d.name,
			ar.created_at
		FROM account_roles ar
		JOIN user_accounts ua ON ua.id = ar.user_account_id
		JOIN congregations c ON c.id = ar.congregation_id
		LEFT JOIN members m ON m.id = ua.member_id
		LEFT JOIN departments d ON d.id = ar.department_id
		ORDER BY c.name, ar.role, m.full_name
	`)
	if err != nil {
		log.Printf("Could not list role assignments: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not load role assignments",
		})
		return
	}
	defer rows.Close()

	assignments := []RoleAssignment{}

	for rows.Next() {
		var assignment RoleAssignment

		err := rows.Scan(
			&assignment.ID,
			&assignment.FullName,
			&assignment.Email,
			&assignment.Role,
			&assignment.CongregationID,
			&assignment.Congregation,
			&assignment.DepartmentID,
			&assignment.Department,
			&assignment.CreatedAt,
		)
		if err != nil {
			log.Printf("Could not read role assignment: %v", err)
			respondJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "Could not read role assignments",
			})
			return
		}

		assignments = append(assignments, assignment)
	}

	if err := rows.Err(); err != nil {
		log.Printf("Role assignment query failed: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not finish loading role assignments",
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"total":       len(assignments),
		"assignments": assignments,
	})
}
