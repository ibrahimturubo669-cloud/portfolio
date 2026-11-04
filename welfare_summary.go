package main

import (
	"net/http"
)

type WelfareSummary struct {
	IncidentID      string `json:"incident_id"`
	Title           string `json:"title"`
	TargetAmount    string `json:"target_amount"`
	VerifiedAmount  string `json:"verified_amount"`
	PendingAmount   string `json:"pending_amount"`
	RemainingAmount string `json:"remaining_amount"`
	Status          string `json:"status"`
}

func welfareSummaryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "Method not allowed",
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

	rows, err := db.Query(r.Context(), `
		SELECT
			wi.id::text,
			wi.title,
			wi.target_amount::text,

			COALESCE(
				SUM(wc.amount) FILTER (
					WHERE wc.status = 'verified'
				), 0
			)::text,

			COALESCE(
				SUM(wc.amount) FILTER (
					WHERE wc.status = 'pending'
				), 0
			)::text,

			GREATEST(
				wi.target_amount - COALESCE(
					SUM(wc.amount) FILTER (
						WHERE wc.status = 'verified'
					), 0
				), 0
			)::text,

			wi.status

		FROM welfare_incidents wi

		LEFT JOIN welfare_contributions wc
			ON wc.incident_id = wi.id

		WHERE wi.congregation_id = $1

		GROUP BY wi.id

		ORDER BY wi.created_at DESC
	`, congregationID)

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not load welfare summary",
		})
		return
	}

	defer rows.Close()

	summaries := []WelfareSummary{}

	for rows.Next() {
		var item WelfareSummary

		err := rows.Scan(
			&item.IncidentID,
			&item.Title,
			&item.TargetAmount,
			&item.VerifiedAmount,
			&item.PendingAmount,
			&item.RemainingAmount,
			&item.Status,
		)

		if err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "Could not read welfare summary",
			})
			return
		}

		summaries = append(summaries, item)
	}

	if err := rows.Err(); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not finish reading welfare summary",
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"incidents": summaries,
		"total":     len(summaries),
	})
}
