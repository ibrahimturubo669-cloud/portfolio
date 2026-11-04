package main

import (
	"net/http"
)

func memberDepartmentsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		memberID := r.URL.Query().Get("member_id")

		if memberID == "" {
			respondJSON(w, http.StatusBadRequest, map[string]string{
				"error": "member_id is required",
			})
			return
		}

		rows, err := db.Query(r.Context(), `
			SELECT d.id::text, d.name, dm.role
			FROM department_members dm
			JOIN departments d ON d.id = dm.department_id
			WHERE dm.member_id = $1
			ORDER BY d.name
		`, memberID)

		if err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "Could not load department assignments",
			})
			return
		}
		defer rows.Close()

		departments := []map[string]string{}

		for rows.Next() {
			var id, name, role string

			if err := rows.Scan(&id, &name, &role); err != nil {
				respondJSON(w, http.StatusInternalServerError, map[string]string{
					"error": "Could not read department assignments",
				})
				return
			}

			departments = append(departments, map[string]string{
				"id":   id,
				"name": name,
				"role": role,
			})
		}

		if err := rows.Err(); err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "Could not finish reading assignments",
			})
			return
		}

		respondJSON(w, http.StatusOK, map[string]interface{}{
			"departments": departments,
		})

	case http.MethodDelete:
		memberID := r.URL.Query().Get("member_id")
		departmentID := r.URL.Query().Get("department_id")

		if memberID == "" || departmentID == "" {
			respondJSON(w, http.StatusBadRequest, map[string]string{
				"error": "member_id and department_id are required",
			})
			return
		}

		result, err := db.Exec(r.Context(), `
			DELETE FROM department_members
			WHERE member_id = $1 AND department_id = $2
		`, memberID, departmentID)

		if err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "Could not remove department assignment",
			})
			return
		}

		if result.RowsAffected() == 0 {
			respondJSON(w, http.StatusNotFound, map[string]string{
				"error": "Department assignment not found",
			})
			return
		}

		respondJSON(w, http.StatusOK, map[string]string{
			"message": "Department assignment removed",
		})

	default:
		w.Header().Set("Allow", "GET, DELETE")
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "Method not allowed",
		})
	}
}
