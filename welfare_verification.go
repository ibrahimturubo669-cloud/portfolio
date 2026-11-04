package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"ktfc-chms-backend/auth"
)

type VerifyWelfareContributionRequest struct {
	ContributionID string `json:"contribution_id"`
	Status         string `json:"status"`
}

func verifyWelfareContributionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "Method not allowed",
		})
		return
	}

	claims, ok := r.Context().Value(authClaimsKey).(*auth.AuthClaims)
	if !ok || claims == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "Authentication required",
		})
		return
	}

	var req VerifyWelfareContributionRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
		return
	}

	req.ContributionID = strings.TrimSpace(req.ContributionID)
	req.Status = strings.ToLower(strings.TrimSpace(req.Status))

	if req.ContributionID == "" ||
		(req.Status != "verified" && req.Status != "rejected") {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Provide a contribution ID and status of verified or rejected",
		})
		return
	}

	congregationID := strings.TrimSpace(r.URL.Query().Get("congregation_id"))
	if congregationID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "congregation_id is required",
		})
		return
	}

	tx, err := db.Begin(r.Context())
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not start verification",
		})
		return
	}
	defer tx.Rollback(r.Context())

	result, err := tx.Exec(r.Context(), `
		UPDATE welfare_contributions
		SET status = $1,
		    verified_by = $2,
		    verified_at = NOW()
		WHERE id = $3
		  AND congregation_id = $4
		  AND status = 'pending'
	`, req.Status, claims.UserID, req.ContributionID, congregationID)

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not update contribution",
		})
		return
	}

	if result.RowsAffected() == 0 {
		respondJSON(w, http.StatusNotFound, map[string]string{
			"error": "Pending contribution not found in this congregation",
		})
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not save verification",
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Contribution review saved",
		"status":  req.Status,
	})
}
