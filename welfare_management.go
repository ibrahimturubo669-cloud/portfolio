package main

import (
	"encoding/json"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ktfc-chms-backend/auth"
)

type CreateWelfareIncidentRequest struct {
	CongregationID string `json:"congregation_id"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	TargetAmount   string `json:"target_amount"`
	IncidentDate   string `json:"incident_date"`
}
type RecordWelfareContributionRequest struct {
	MemberID         string `json:"member_id"`
	IncidentID       string `json:"incident_id"`
	Amount           string `json:"amount"`
	PaymentMethod    string `json:"payment_method"`
	PaymentReference string `json:"payment_reference"`
}

func createWelfareIncidentHandler(w http.ResponseWriter, r *http.Request) {
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
			"error": "Authentication required",
		})
		return
	}

	var req CreateWelfareIncidentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
		return
	}

	req.CongregationID = strings.TrimSpace(req.CongregationID)
	req.Title = strings.TrimSpace(req.Title)
	req.Description = strings.TrimSpace(req.Description)
	req.TargetAmount = strings.TrimSpace(req.TargetAmount)
	req.IncidentDate = strings.TrimSpace(req.IncidentDate)

	targetAmount, err := strconv.ParseFloat(req.TargetAmount, 64)
	if !validUUID(req.CongregationID) ||
		req.Title == "" ||
		req.TargetAmount == "" ||
		err != nil ||
		targetAmount <= 0 ||
		math.IsNaN(targetAmount) ||
		math.IsInf(targetAmount, 0) {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Provide a valid congregation ID, title, and positive target amount",
		})
		return
	}

	incidentDate, err := time.Parse("2006-01-02", req.IncidentDate)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Incident date must use YYYY-MM-DD format",
		})
		return
	}

	var incidentID string
	err = db.QueryRow(r.Context(), `
		INSERT INTO welfare_incidents (
			congregation_id,
			title,
			description,
			target_amount,
			incident_date,
			status,
			created_at
		)
		VALUES ($1::uuid, $2, $3, $4::numeric, $5::date, 'open', NOW())
		RETURNING id::text
	`,
		req.CongregationID,
		req.Title,
		req.Description,
		req.TargetAmount,
		incidentDate,
	).Scan(&incidentID)
	if err != nil {
		log.Printf("Could not create welfare incident: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not create welfare incident",
		})
		return
	}

	respondJSON(w, http.StatusCreated, map[string]string{
		"message":     "Welfare incident created",
		"incident_id": incidentID,
		"status":      "open",
	})
}

func recordWelfareContributionHandler(w http.ResponseWriter, r *http.Request) {
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

	var req RecordWelfareContributionRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
		return
	}

	req.MemberID = strings.TrimSpace(req.MemberID)
	req.IncidentID = strings.TrimSpace(req.IncidentID)
	req.Amount = strings.TrimSpace(req.Amount)
	req.PaymentMethod = strings.TrimSpace(req.PaymentMethod)
	req.PaymentReference = strings.TrimSpace(req.PaymentReference)

	if req.MemberID == "" || req.Amount == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Member ID and amount are required",
		})
		return
	}

	var congregationID string

	err := db.QueryRow(r.Context(), `
		SELECT congregation_id::text
		FROM members
		WHERE id = $1 AND status = 'active'
	`, req.MemberID).Scan(&congregationID)

	if err != nil {
		respondJSON(w, http.StatusNotFound, map[string]string{
			"error": "Active member not found",
		})
		return
	}

	contributionType := "general"

	if req.IncidentID != "" {
		var incidentStatus string

		err = db.QueryRow(r.Context(), `
			SELECT status
			FROM welfare_incidents
			WHERE id = $1 AND congregation_id = $2
		`, req.IncidentID, congregationID).Scan(&incidentStatus)

		if err != nil {
			respondJSON(w, http.StatusNotFound, map[string]string{
				"error": "Welfare incident not found in this congregation",
			})
			return
		}

		if incidentStatus != "open" {
			respondJSON(w, http.StatusBadRequest, map[string]string{
				"error": "This welfare incident is closed",
			})
			return
		}

		contributionType = "incident"
	}

	var contributionID string

	err = db.QueryRow(r.Context(), `
		INSERT INTO welfare_contributions (
			incident_id,
			congregation_id,
			member_id,
			amount,
			payment_method,
			payment_reference,
			contribution_type,
			status,
			recorded_by
		)
		VALUES (
			NULLIF($1, '')::uuid,
			$2,
			$3,
			$4::numeric,
			NULLIF($5, ''),
			NULLIF($6, ''),
			$7,
			'pending',
			$8
		)
		RETURNING id::text
	`,
		req.IncidentID,
		congregationID,
		req.MemberID,
		req.Amount,
		req.PaymentMethod,
		req.PaymentReference,
		contributionType,
		claims.UserID,
	).Scan(&contributionID)

	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Could not record contribution. Check the amount and details.",
		})
		return
	}

	respondJSON(w, http.StatusCreated, map[string]string{
		"message":         "Contribution recorded and awaiting verification",
		"contribution_id": contributionID,
		"type":            contributionType,
		"status":          "pending",
	})
}
