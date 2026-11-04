package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"ktfc-chms-backend/auth"
)

type CreateEventRequest struct {
	CongregationID string     `json:"congregation_id"`
	DepartmentID   *string    `json:"department_id"`
	Title          string     `json:"title"`
	Description    string     `json:"description"`
	EventType      string     `json:"event_type"`
	Venue          string     `json:"venue"`
	StartDate      time.Time  `json:"start_date"`
	EndDate        *time.Time `json:"end_date"`
}

func createEventPermissionHandler(w http.ResponseWriter, r *http.Request) {
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

	var input CreateEventRequest
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
	if !validUUID(input.CongregationID) {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid congregation ID",
		})
		return
	}

	if input.DepartmentID != nil {
		departmentID := strings.TrimSpace(*input.DepartmentID)
		if !validUUID(departmentID) {
			respondJSON(w, http.StatusBadRequest, map[string]string{
				"error": "Invalid department ID",
			})
			return
		}
		input.DepartmentID = &departmentID
	}

	input.Title = strings.TrimSpace(input.Title)
	input.EventType = strings.TrimSpace(input.EventType)
	if input.Title == "" || input.EventType == "" || input.StartDate.IsZero() {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Title, event type and start date are required",
		})
		return
	}

	if input.EndDate != nil && input.EndDate.Before(input.StartDate) {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "End date cannot be before start date",
		})
		return
	}

	allowed, err := canCreateEvent(
		r.Context(),
		claims.UserID,
		input.CongregationID,
		input.DepartmentID,
	)
	if err != nil {
		log.Printf("Could not check event creation permission: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Permission check failed",
		})
		return
	}
	if !allowed {
		respondJSON(w, http.StatusForbidden, map[string]string{
			"error": "You are not allowed to create events for this congregation or department",
		})
		return
	}

	var eventID string
	err = db.QueryRow(r.Context(), `
		INSERT INTO events (
			id, congregation_id, department_id,
			title, description, event_type, venue,
			start_date, end_date, status, created_at
		)
		VALUES (
			gen_random_uuid(), $1, $2, $3, $4, $5, $6,
			$7, $8, 'pending', NOW()
		)
		RETURNING id::text
	`,
		input.CongregationID,
		input.DepartmentID,
		input.Title,
		input.Description,
		input.EventType,
		input.Venue,
		input.StartDate,
		input.EndDate,
	).Scan(&eventID)
	if err != nil {
		log.Printf("Could not create event: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not create event",
		})
		return
	}

	respondJSON(w, http.StatusCreated, map[string]string{
		"message":  "Event submitted for approval",
		"event_id": eventID,
		"status":   "pending",
	})
}

func validUUID(value string) bool {
	if len(value) != 36 ||
		value[8] != '-' ||
		value[13] != '-' ||
		value[18] != '-' ||
		value[23] != '-' {
		return false
	}

	compact := strings.ReplaceAll(value, "-", "")
	decoded := make([]byte, 16)
	_, err := hex.Decode(decoded, []byte(compact))
	return err == nil
}

func canCreateEvent(
	ctx context.Context,
	userID string,
	congregationID string,
	departmentID *string,
) (bool, error) {
	var allowed bool

	err := db.QueryRow(ctx, `
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
				  OR (
					  $3::uuid IS NOT NULL
					  AND EXISTS (
						  SELECT 1
						  FROM account_roles ar
						  WHERE ar.user_account_id = ua.id
						    AND ar.congregation_id = $2
						    AND ar.department_id = $3
						    AND ar.role = 'department_leader'
					  )
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
