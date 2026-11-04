package main

import (
	"net/http"
)

func dashboardCountsHandler(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodGet {
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "Method not allowed",
		})
		return
	}

	query := `
		SELECT
			COUNT(DISTINCT m.id) AS total_members,

			COUNT(DISTINCT m.id) FILTER (
				WHERE d.name = 'USFW'
			) AS usfw_members,

			COUNT(DISTINCT m.id) FILTER (
				WHERE d.name = 'Youth'
			) AS youth_members,

			COUNT(DISTINCT m.id) FILTER (
				WHERE d.name = 'Quakermen'
			) AS quakermen_members

		FROM members m

		LEFT JOIN department_members dm
			ON dm.member_id = m.id
			AND dm.congregation_id = m.congregation_id

		LEFT JOIN departments d
			ON d.id = dm.department_id

		WHERE m.status = 'active';
	`

	var counts struct {
		TotalMembers     int64 `json:"total_members"`
		USFWMembers      int64 `json:"usfw_members"`
		YouthMembers     int64 `json:"youth_members"`
		QuakermenMembers int64 `json:"quakermen_members"`
	}

	err := db.QueryRow(r.Context(), query).Scan(
		&counts.TotalMembers,
		&counts.USFWMembers,
		&counts.YouthMembers,
		&counts.QuakermenMembers,
	)

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not load dashboard counts",
		})
		return
	}

	respondJSON(w, http.StatusOK, counts)
}
