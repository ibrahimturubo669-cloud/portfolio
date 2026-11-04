package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

type CreateMemberRequest struct {
	FullName        string  `json:"full_name"`
	Email           *string `json:"email"`
	Phone           *string `json:"phone"`
	CongregationID  string  `json:"congregation_id"`
	ProfileImageURL *string `json:"profile_image_url"`
}

func createMemberHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "Method not allowed",
		})
		return
	}

	var input CreateMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid JSON",
		})
		return
	}

	input.FullName = strings.TrimSpace(input.FullName)
	input.CongregationID = strings.TrimSpace(input.CongregationID)

	if input.FullName == "" || input.CongregationID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Full name and congregation ID are required",
		})
		return
	}

	cleanOptional := func(value *string) *string {
		if value == nil {
			return nil
		}
		cleaned := strings.TrimSpace(*value)
		if cleaned == "" {
			return nil
		}
		return &cleaned
	}

	input.Email = cleanOptional(input.Email)
	input.Phone = cleanOptional(input.Phone)
	input.ProfileImageURL = cleanOptional(input.ProfileImageURL)

	var memberID string

	err := db.QueryRow(r.Context(), `
		INSERT INTO members (
			congregation_id,
			full_name,
			email,
			phone,
			profile_image_url,
			status
		)
		VALUES ($1::uuid, $2, $3, $4, $5, 'active')
		RETURNING id::text
	`,
		input.CongregationID,
		input.FullName,
		input.Email,
		input.Phone,
		input.ProfileImageURL,
	).Scan(&memberID)

	if err != nil {
		log.Printf("Could not create member: %v", err)

		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not register member. Check the supplied details.",
		})
		return
	}

	respondJSON(w, http.StatusCreated, map[string]string{
		"message":   "Member registered successfully",
		"member_id": memberID,
	})
}

func listMembersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "Method not allowed",
		})
		return
	}

	congregationID := strings.TrimSpace(
		r.URL.Query().Get("congregation_id"),
	)

	if congregationID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "congregation_id is required",
		})
		return
	}

	rows, err := db.Query(r.Context(), `
		SELECT
			m.id::text,
			m.full_name,
			m.email,
			m.phone,
			m.status,
			c.name
		FROM members m
		JOIN congregations c ON c.id = m.congregation_id
		WHERE m.congregation_id = $1::uuid
		ORDER BY m.full_name
	`, congregationID)

	if err != nil {
		log.Printf("Could not list members: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not load members",
		})
		return
	}
	defer rows.Close()

	type Member struct {
		ID           string  `json:"id"`
		FullName     string  `json:"full_name"`
		Email        *string `json:"email"`
		Phone        *string `json:"phone"`
		Status       string  `json:"status"`
		Congregation string  `json:"congregation"`
	}

	members := []Member{}

	for rows.Next() {
		var member Member

		if err := rows.Scan(
			&member.ID,
			&member.FullName,
			&member.Email,
			&member.Phone,
			&member.Status,
			&member.Congregation,
		); err != nil {
			log.Printf("Could not read member: %v", err)
			respondJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "Could not read member records",
			})
			return
		}

		members = append(members, member)
	}

	if err := rows.Err(); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not finish reading members",
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"total":   len(members),
		"members": members,
	})
}

func updateMemberStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		w.Header().Set("Allow", http.MethodPatch)
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "Method not allowed",
		})
		return
	}

	var input struct {
		MemberID string `json:"member_id"`
		Status   string `json:"status"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid JSON",
		})
		return
	}

	input.MemberID = strings.TrimSpace(input.MemberID)
	input.Status = strings.TrimSpace(input.Status)

	if input.MemberID == "" ||
		(input.Status != "active" && input.Status != "inactive") {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Member ID and valid status are required",
		})
		return
	}

	result, err := db.Exec(r.Context(), `
		UPDATE members
		SET status = $1
		WHERE id = $2::uuid
	`, input.Status, input.MemberID)

	if err != nil {
		log.Printf("Could not update member status: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not update member status",
		})
		return
	}

	if result.RowsAffected() == 0 {
		respondJSON(w, http.StatusNotFound, map[string]string{
			"error": "Member not found",
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Member status updated successfully",
	})
}

type EditMemberRequest struct {
	MemberID       string  `json:"member_id"`
	FullName       string  `json:"full_name"`
	Email          *string `json:"email"`
	Phone          *string `json:"phone"`
	CongregationID string  `json:"congregation_id"`
}

func editMemberHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		w.Header().Set("Allow", http.MethodPatch)
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "Method not allowed",
		})
		return
	}

	var input EditMemberRequest

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid JSON",
		})
		return
	}

	input.MemberID = strings.TrimSpace(input.MemberID)
	input.FullName = strings.TrimSpace(input.FullName)
	input.CongregationID = strings.TrimSpace(input.CongregationID)

	if input.MemberID == "" ||
		input.FullName == "" ||
		input.CongregationID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Member ID, full name and congregation ID are required",
		})
		return
	}

	cleanOptional := func(value *string) *string {
		if value == nil {
			return nil
		}

		cleaned := strings.TrimSpace(*value)
		if cleaned == "" {
			return nil
		}

		return &cleaned
	}

	input.Email = cleanOptional(input.Email)
	input.Phone = cleanOptional(input.Phone)

	result, err := db.Exec(r.Context(), `
		UPDATE members
		SET
			full_name = $1,
			email = $2,
			phone = $3
		WHERE
			id = $4::uuid
			AND congregation_id = $5::uuid
	`,
		input.FullName,
		input.Email,
		input.Phone,
		input.MemberID,
		input.CongregationID,
	)

	if err != nil {
		log.Printf("Could not edit member: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not update member details",
		})
		return
	}

	if result.RowsAffected() == 0 {
		respondJSON(w, http.StatusNotFound, map[string]string{
			"error": "Member not found in this congregation",
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Member details updated successfully",
	})
}
