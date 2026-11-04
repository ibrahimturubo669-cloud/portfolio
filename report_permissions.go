package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"ktfc-chms-backend/auth"
)

type CreateReportRequest struct {
	CongregationID  string `json:"congregation_id"`
	DepartmentID    string `json:"department_id"`
	ReportTitle     string `json:"report_title"`
	PeriodStart     string `json:"report_period_start"`
	PeriodEnd       string `json:"report_period_end"`
	Activities      string `json:"activities"`
	Achievements    string `json:"achievements"`
	Challenges      string `json:"challenges"`
	Recommendations string `json:"recommendations"`
}

func canCreateReport(
	ctx context.Context,
	userID string,
	congregationID string,
	departmentID string,
) (bool, error) {
	var allowed bool

	err := db.QueryRow(
		ctx,
		`
		SELECT EXISTS (
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
					  AND ar.congregation_id = $2
					  AND ar.role = 'church_admin'
				)
				OR EXISTS (
					SELECT 1
					FROM account_roles ar
					WHERE ar.user_account_id = ua.id
					  AND ar.congregation_id = $2
					  AND ar.department_id = $3
					  AND ar.role = 'department_leader'
				)
			  )
		)
		`,
		userID,
		congregationID,
		departmentID,
	).Scan(&allowed)

	return allowed, err
}

func createReportHandler(w http.ResponseWriter, r *http.Request) {
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

	var input CreateReportRequest
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

	input.CongregationID = strings.TrimSpace(input.CongregationID)
	input.DepartmentID = strings.TrimSpace(input.DepartmentID)
	if !validUUID(input.CongregationID) || !validUUID(input.DepartmentID) {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid congregation or department ID",
		})
		return
	}

	input.ReportTitle = strings.TrimSpace(input.ReportTitle)
	if input.ReportTitle == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Report title is required",
		})
		return
	}

	start, err := time.Parse("2006-01-02", input.PeriodStart)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid report start date. Use YYYY-MM-DD",
		})
		return
	}

	end, err := time.Parse("2006-01-02", input.PeriodEnd)
	if err != nil || end.Before(start) {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid report end date",
		})
		return
	}

	allowed, err := canCreateReport(
		r.Context(),
		claims.UserID,
		input.CongregationID,
		input.DepartmentID,
	)
	if err != nil {
		log.Printf("Could not check report creation permission: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Permission check failed",
		})
		return
	}
	if !allowed {
		respondJSON(w, http.StatusForbidden, map[string]string{
			"error": "You cannot submit a report for this department",
		})
		return
	}

	var departmentExists bool
	err = db.QueryRow(r.Context(), `
		SELECT EXISTS (
			SELECT 1 FROM departments WHERE id = $1
		)
	`, input.DepartmentID).Scan(&departmentExists)
	if err != nil {
		log.Printf("Could not verify report department: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not verify department",
		})
		return
	}
	if !departmentExists {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Department not found",
		})
		return
	}

	var reportID string
	err = db.QueryRow(r.Context(), `
		INSERT INTO department_reports (
			id,
			congregation_id,
			department_id,
			report_title,
			report_period_start,
			report_period_end,
			activities,
			achievements,
			challenges,
			recommendations,
			prepared_by,
			status,
			created_at
		)
		VALUES (
			gen_random_uuid(),
			$1, $2, $3, $4, $5, $6, $7, $8, $9,
			$10, 'pending', NOW()
		)
		RETURNING id::text
	`,
		input.CongregationID,
		input.DepartmentID,
		input.ReportTitle,
		start,
		end,
		input.Activities,
		input.Achievements,
		input.Challenges,
		input.Recommendations,
		claims.MemberID,
	).Scan(&reportID)
	if err != nil {
		log.Printf("Could not save department report: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not save report",
		})
		return
	}

	respondJSON(w, http.StatusCreated, map[string]string{
		"message":   "Report submitted for approval",
		"report_id": reportID,
		"status":    "pending",
	})
}
