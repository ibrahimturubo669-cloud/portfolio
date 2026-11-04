package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"ktfc-chms-backend/auth"
)

type ReportReviewRequest struct {
	ReportID string `json:"report_id"`
	Action   string `json:"action"`
}

func reviewReportHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
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

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var input ReportReviewRequest
	if err := decoder.Decode(&input); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid JSON",
		})
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid JSON",
		})
		return
	}

	if !validUUID(input.ReportID) {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid report ID",
		})
		return
	}

	if input.Action != "approve" && input.Action != "reject" {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Action must be approve or reject",
		})
		return
	}

	status := "approved"
	if input.Action == "reject" {
		status = "rejected"
	}

	result, err := db.Exec(r.Context(), `
		UPDATE department_reports dr
		SET status = $1,
		    reviewed_by = $2,
		    reviewed_at = NOW()
		WHERE dr.id = $3
		  AND dr.status = 'pending'
		  AND EXISTS (
			SELECT 1
			FROM user_accounts ua
			WHERE ua.id = $2
			  AND ua.is_active = TRUE
			  AND (
				ua.system_role = 'super_admin'
				OR EXISTS (
					SELECT 1
					FROM account_roles ar
					WHERE ar.user_account_id = ua.id
					  AND ar.congregation_id = dr.congregation_id
					  AND ar.role = 'church_admin'
				)
			  )
		  )
	`, status, claims.UserID, input.ReportID)
	if err != nil {
		log.Printf("Could not review report %s: %v", input.ReportID, err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not review report",
		})
		return
	}

	if result.RowsAffected() == 0 {
		respondJSON(w, http.StatusNotFound, map[string]string{
			"error": "Pending report not found or you do not have permission to review it",
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Report review completed",
		"status":  status,
	})
}
