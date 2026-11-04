package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"ktfc-chms-backend/auth"
)

type EventReviewRequest struct {
	EventID string `json:"event_id"`
	Action  string `json:"action"`
}

func reviewEventHandler(w http.ResponseWriter, r *http.Request) {
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

	var input EventReviewRequest
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

	if !validUUID(input.EventID) {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid event ID",
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
		UPDATE events e
		SET status = $1,
		    reviewed_by = $2,
		    reviewed_at = NOW()
		WHERE e.id = $3
		  AND e.status = 'pending'
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
					      AND ar.congregation_id = e.congregation_id
					      AND ar.role = 'church_admin'
				    )
			    )
		  )
	`, status, claims.UserID, input.EventID)
	if err != nil {
		log.Printf("Could not review event %s: %v", input.EventID, err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not review event",
		})
		return
	}

	if result.RowsAffected() == 0 {
		respondJSON(w, http.StatusNotFound, map[string]string{
			"error": "Pending event not found",
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Event review completed",
		"status":  status,
	})
}
