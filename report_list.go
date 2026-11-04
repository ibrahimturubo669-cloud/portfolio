package main

import (
	"encoding/json"
	"net/http"

	"ktfc-chms-backend/auth"
)

type DepartmentReport struct {
	ID              string  `json:"id"`
	CongregationID  string  `json:"congregation_id"`
	DepartmentID    string  `json:"department_id"`
	ReportTitle     string  `json:"report_title"`
	PeriodStart     string  `json:"report_period_start"`
	PeriodEnd       string  `json:"report_period_end"`
	Activities      *string `json:"activities"`
	Achievements    *string `json:"achievements"`
	Challenges      *string `json:"challenges"`
	Recommendations *string `json:"recommendations"`
	Status          string  `json:"status"`
	CreatedAt       string  `json:"created_at"`
}

func listReportsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	claims, ok := r.Context().Value(authClaimsKey).(*auth.AuthClaims)
	if !ok || claims == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	rows, err := db.Query(r.Context(), `
		SELECT
			dr.id::text,
			dr.congregation_id::text,
			dr.department_id::text,
			dr.report_title,
			dr.report_period_start::text,
			dr.report_period_end::text,
			dr.activities,
			dr.achievements,
			dr.challenges,
			dr.recommendations,
			dr.status,
			dr.created_at::text
		FROM department_reports dr
		WHERE
			dr.status = 'approved'
			OR EXISTS (
				SELECT 1
				FROM user_accounts ua
				WHERE ua.id = $1
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
					OR EXISTS (
						SELECT 1
						FROM account_roles ar
						WHERE ar.user_account_id = ua.id
						  AND ar.congregation_id = dr.congregation_id
						  AND ar.department_id = dr.department_id
						  AND ar.role = 'department_leader'
					)
				  )
			)
		ORDER BY dr.created_at DESC
	`, claims.UserID)

	if err != nil {
		http.Error(w, "Could not retrieve reports", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	reports := []DepartmentReport{}

	for rows.Next() {
		var report DepartmentReport

		err := rows.Scan(
			&report.ID,
			&report.CongregationID,
			&report.DepartmentID,
			&report.ReportTitle,
			&report.PeriodStart,
			&report.PeriodEnd,
			&report.Activities,
			&report.Achievements,
			&report.Challenges,
			&report.Recommendations,
			&report.Status,
			&report.CreatedAt,
		)

		if err != nil {
			http.Error(w, "Could not read report", http.StatusInternalServerError)
			return
		}

		reports = append(reports, report)
	}

	if err := rows.Err(); err != nil {
		http.Error(w, "Report retrieval failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(reports)
}
