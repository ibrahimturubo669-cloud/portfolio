package main

import (
	"net/http"
)

type WelfareContributionRecord struct {
	MemberName       string `json:"member_name"`
	Amount           string `json:"amount"`
	ContributionType string `json:"contribution_type"`
	IncidentTitle    string `json:"incident_title"`
	PaymentMethod    string `json:"payment_method"`
	ContributionDate string `json:"contribution_date"`
	Status           string `json:"status"`
}

type WelfareReport struct {
	Records       []WelfareContributionRecord `json:"records"`
	VerifiedTotal string                      `json:"verified_total"`
	PendingTotal  string                      `json:"pending_total"`
	RejectedTotal string                      `json:"rejected_total"`
	RecordCount   int                         `json:"record_count"`
}

func welfareReportHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "Method not allowed",
		})
		return
	}

	congregationID := r.URL.Query().Get("congregation_id")
	memberID := r.URL.Query().Get("member_id")
	startDate := r.URL.Query().Get("start_date")
	endDate := r.URL.Query().Get("end_date")

	if congregationID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "congregation_id is required",
		})
		return
	}

	if startDate != "" && endDate != "" && startDate > endDate {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "start_date cannot be after end_date",
		})
		return
	}

	rows, err := db.Query(r.Context(), `
		SELECT
			m.full_name,
			wc.amount::text,
			wc.contribution_type,
			COALESCE(wi.title, 'General Welfare'),
			COALESCE(wc.payment_method, ''),
			wc.contribution_date::text,
			wc.status
		FROM welfare_contributions wc
		JOIN members m ON m.id = wc.member_id
		LEFT JOIN welfare_incidents wi ON wi.id = wc.incident_id
		WHERE wc.congregation_id = $1
		  AND ($2 = '' OR wc.member_id::text = $2)
		  AND ($3 = '' OR wc.contribution_date >= $3::date)
		  AND ($4 = '' OR wc.contribution_date <= $4::date)
		ORDER BY wc.contribution_date DESC, m.full_name
	`, congregationID, memberID, startDate, endDate)

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not load Welfare report",
		})
		return
	}
	defer rows.Close()

	report := WelfareReport{
		Records: []WelfareContributionRecord{},
	}

	for rows.Next() {
		var item WelfareContributionRecord

		err := rows.Scan(
			&item.MemberName,
			&item.Amount,
			&item.ContributionType,
			&item.IncidentTitle,
			&item.PaymentMethod,
			&item.ContributionDate,
			&item.Status,
		)

		if err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "Could not read Welfare report",
			})
			return
		}

		report.Records = append(report.Records, item)
	}

	if err := rows.Err(); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not finish reading Welfare report",
		})
		return
	}

	err = db.QueryRow(r.Context(), `
		SELECT
			COALESCE(SUM(amount) FILTER (WHERE status = 'verified'), 0)::text,
			COALESCE(SUM(amount) FILTER (WHERE status = 'pending'), 0)::text,
			COALESCE(SUM(amount) FILTER (WHERE status = 'rejected'), 0)::text,
			COUNT(*)
		FROM welfare_contributions
		WHERE congregation_id = $1
		  AND ($2 = '' OR member_id::text = $2)
		  AND ($3 = '' OR contribution_date >= $3::date)
		  AND ($4 = '' OR contribution_date <= $4::date)
	`, congregationID, memberID, startDate, endDate).Scan(
		&report.VerifiedTotal,
		&report.PendingTotal,
		&report.RejectedTotal,
		&report.RecordCount,
	)

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not calculate Welfare totals",
		})
		return
	}

	respondJSON(w, http.StatusOK, report)
}
