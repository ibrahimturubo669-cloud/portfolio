package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
)

type AssignDepartmentRequest struct {
	MemberID     string `json:"member_id"`
	DepartmentID string `json:"department_id"`
}

func assignMemberDepartmentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "Method not allowed",
		})
		return
	}

	var req AssignDepartmentRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
		return
	}

	req.MemberID = strings.TrimSpace(req.MemberID)
	req.DepartmentID = strings.TrimSpace(req.DepartmentID)

	if req.MemberID == "" || req.DepartmentID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Member ID and department ID are required",
		})
		return
	}

	var congregationID string
	err := db.QueryRow(r.Context(),
		`SELECT congregation_id::text
		 FROM members
		 WHERE id = $1 AND status = 'active'`,
		req.MemberID,
	).Scan(&congregationID)

	if err == pgx.ErrNoRows {
		respondJSON(w, http.StatusNotFound, map[string]string{
			"error": "Active member not found",
		})
		return
	}
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not find member",
		})
		return
	}

	var departmentName string
	err = db.QueryRow(r.Context(),
		`SELECT name FROM departments WHERE id = $1`,
		req.DepartmentID,
	).Scan(&departmentName)

	if err == pgx.ErrNoRows {
		respondJSON(w, http.StatusNotFound, map[string]string{
			"error": "Department not found",
		})
		return
	}
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not find department",
		})
		return
	}

	_, err = db.Exec(r.Context(),
		`INSERT INTO department_members
		 (member_id, department_id, congregation_id, role)
		 SELECT $1, $2, $3, 'member'
		 WHERE NOT EXISTS (
		   SELECT 1 FROM department_members
		   WHERE member_id = $1 AND department_id = $2
		     AND congregation_id = $3
		 )`,
		req.MemberID, req.DepartmentID, congregationID,
	)

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not assign department",
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message":    "Department assignment saved",
		"department": departmentName,
	})
}
