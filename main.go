package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"ktfc-chms-backend/auth"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

var db *pgxpool.Pool

func main() {
	// Load environment configuration
	envLoaded, err := loadEnvironment()
	if err != nil {
		log.Fatal("Environment configuration error:", err)
	}
	if !envLoaded {
		log.Println("No .env file found; using system environment")
	}

	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}

	// PostgreSQL connection
	config, err := databaseConfig()
	if err != nil {
		log.Fatal("Database configuration error:", err)
	}

	db, err = pgxpool.NewWithConfig(
		context.Background(), config,
	)
	if err != nil {
		log.Fatal("Database connection error:", err)
	}
	defer db.Close()

	// Verify database connection
	ctx, cancel := context.WithTimeout(
		context.Background(), 5*time.Second,
	)
	defer cancel()

	if err := db.Ping(ctx); err != nil {
		log.Fatal("Cannot connect to PostgreSQL:", err)
	}

	log.Println("Connected to ktfc_chms successfully!")

	mux := http.NewServeMux()

	mux.HandleFunc("/api/health", healthHandler)
	mux.HandleFunc("/api/dashboard/counts", dashboardCountsHandler)
	mux.Handle(
		"/api/admin/members",
		requireSystemRole(
			"super_admin",
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					createMemberHandler(w, r)
					return
				}
				if r.Method == http.MethodGet {
					listMembersHandler(w, r)
					return
				}
				w.Header().Set("Allow", "GET, POST")
				respondJSON(w, http.StatusMethodNotAllowed, map[string]string{
					"error": "Method not allowed",
				})
			}),
		),
	)
	mux.Handle(
		"/api/admin/members/edit",
		requireSystemRole("super_admin", http.HandlerFunc(editMemberHandler)),
	)

	mux.Handle(
		"/api/admin/members/status",
		requireSystemRole(
			"super_admin",
			http.HandlerFunc(updateMemberStatusHandler),
		),
	)
	mux.Handle("/api/auth/login", auth.LoginHandler(db))
	mux.Handle(
		"/api/auth/me",
		authMiddleware(http.HandlerFunc(meHandler)),
	)
	mux.Handle(
		"/api/admin/members/department",
		requireSystemRole(
			"super_admin",
			http.HandlerFunc(assignMemberDepartmentHandler),
		),
	)
	mux.Handle(
		"/api/admin/welfare/contributions/verify",
		requireCongregationRole(
			"treasurer",
			http.HandlerFunc(verifyWelfareContributionHandler),
		),
	)
	mux.Handle(
		"/api/admin/members/departments",
		requireSystemRole(
			"super_admin",
			http.HandlerFunc(memberDepartmentsHandler),
		),
	)
	mux.Handle(
		"/api/admin/welfare/incidents",
		requireSystemRole(
			"super_admin",
			http.HandlerFunc(createWelfareIncidentHandler),
		),
	)

	mux.Handle(
		"/api/admin/welfare/contributions",
		requireSystemRole(
			"super_admin",
			http.HandlerFunc(recordWelfareContributionHandler),
		),
	)
	mux.Handle(
		"/api/admin/welfare/summary",
		requireCongregationRole(
			"treasurer",
			http.HandlerFunc(welfareSummaryHandler),
		),
	)
	mux.Handle(
		"/api/admin/welfare/reports",
		requireCongregationRole(
			"treasurer",
			http.HandlerFunc(welfareReportHandler),
		),
	)
	mux.Handle(
		"/api/admin/welfare/reports/print",
		requireCongregationRole(
			"treasurer",
			http.HandlerFunc(welfarePrintHandler),
		),
	)
	mux.Handle(
		"/api/auth/roles",
		authMiddleware(http.HandlerFunc(rolesHandler)),
	)
	mux.Handle(
		"/api/admin/check",
		requireSystemRole(
			"church_admin",
			http.HandlerFunc(adminCheckHandler),
		),
	)
	mux.Handle(
		"/api/admin/roles/assign",
		requireSystemRole(
			"super_admin",
			http.HandlerFunc(assignRoleHandler),
		),
	)
	mux.Handle(
		"/api/admin/roles/list",
		requireSystemRole(
			"super_admin",
			http.HandlerFunc(listRoleAssignmentsHandler),
		),
	)
	mux.Handle(
		"/api/admin/roles/revoke",
		requireSystemRole(
			"super_admin",
			http.HandlerFunc(revokeRoleHandler),
		),
	)

	mux.HandleFunc("/api/congregations", congregationsHandler)
	mux.HandleFunc("/api/departments", departmentsHandler)
	mux.HandleFunc("/api/services", servicesHandler)
	mux.HandleFunc("/api/events", eventsHandler)
	mux.HandleFunc("/api/leadership", leadershipHandler)
	mux.Handle(
		"/api/reports",
		authMiddleware(http.HandlerFunc(listReportsHandler)),
	)

	mux.Handle(
		"/api/reports/review",
		authMiddleware(http.HandlerFunc(reviewReportHandler)),
	)
	mux.Handle(
		"/api/events/review",
		authMiddleware(http.HandlerFunc(reviewEventHandler)),
	)
	mux.Handle(
		"/api/events/create",
		authMiddleware(http.HandlerFunc(createEventPermissionHandler)),
	)
	mux.Handle(
		"/api/admin/events/test",
		requireSystemRole(
			"church_admin",
			http.HandlerFunc(createEventPermissionHandler),
		),
	)

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           corsMiddleware(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Println("KTFC CHMS API running on port", port)

	log.Fatal(server.ListenAndServe())
}

func loadEnvironment() (bool, error) {
	loaded := false
	for _, path := range []string{
		".env",
		"backend/.env",
		"../.env",
		"../../.env",
	} {
		_, err := os.Stat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return false, fmt.Errorf("check %q: %w", path, err)
		}

		if err := godotenv.Load(path); err != nil {
			return false, fmt.Errorf("load %q: %w", path, err)
		}
		loaded = true
	}
	return loaded, nil
}

func databaseConfig() (*pgxpool.Config, error) {
	required := []string{
		"DB_HOST",
		"DB_PORT",
		"DB_USER",
		"DB_PASSWORD",
		"DB_NAME",
		"DB_SSLMODE",
	}
	missing := make([]string, 0)
	for _, key := range required {
		if os.Getenv(key) == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf(
			"missing required settings: %s",
			strings.Join(missing, ", "),
		)
	}

	dsn := url.URL{
		Scheme: "postgres",
		User: url.UserPassword(
			os.Getenv("DB_USER"),
			os.Getenv("DB_PASSWORD"),
		),
		Host: net.JoinHostPort(
			os.Getenv("DB_HOST"),
			os.Getenv("DB_PORT"),
		),
		Path: "/" + os.Getenv("DB_NAME"),
		RawQuery: url.Values{
			"sslmode": {os.Getenv("DB_SSLMODE")},
		}.Encode(),
	}
	return pgxpool.ParseConfig(dsn.String())
}

// JSON response helper
func respondJSON(
	w http.ResponseWriter,
	status int,
	data interface{},
) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// CORS for local frontend development.
// Restrict allowed origins before production deployment.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		origin := r.Header.Get("Origin")

		if origin == "http://localhost:8081" ||
			origin == "http://127.0.0.1:8081" {
			w.Header().Set(
				"Access-Control-Allow-Origin", origin,
			)
		}

		w.Header().Set(
			"Access-Control-Allow-Methods",
			"GET, POST, OPTIONS",
		)
		w.Header().Set(
			"Access-Control-Allow-Headers",
			"Content-Type, Authorization",
		)

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// 1. Health check
func healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondJSON(w, 405, map[string]string{
			"error": "Method not allowed",
		})
		return
	}

	ctx, cancel := context.WithTimeout(
		r.Context(), 3*time.Second,
	)
	defer cancel()

	if err := db.Ping(ctx); err != nil {
		respondJSON(w, 503, map[string]string{
			"status": "database unavailable",
		})
		return
	}

	respondJSON(w, 200, map[string]string{
		"status": "healthy",
		"app":    "Kimilili Town Friends Church CHMS",
	})
}

// 2. Congregations
func congregationsHandler(
	w http.ResponseWriter, r *http.Request,
) {
	if r.Method != http.MethodGet {
		respondJSON(w, 405, map[string]string{
			"error": "Method not allowed",
		})
		return
	}

	rows, err := db.Query(
		r.Context(),
		`SELECT id, name, location
		 FROM congregations ORDER BY name`,
	)
	if err != nil {
		respondJSON(w, 500, map[string]string{
			"error": "Could not load congregations",
		})
		return
	}
	defer rows.Close()

	type Congregation struct {
		ID       string  `json:"id"`
		Name     string  `json:"name"`
		Location *string `json:"location"`
	}

	results := []Congregation{}

	for rows.Next() {
		var c Congregation

		if err := rows.Scan(
			&c.ID, &c.Name, &c.Location,
		); err != nil {
			respondJSON(w, 500, map[string]string{
				"error": "Could not read congregation",
			})
			return
		}

		results = append(results, c)
	}

	if err := rows.Err(); err != nil {
		respondJSON(w, 500, map[string]string{
			"error": "Could not read congregations",
		})
		return
	}

	respondJSON(w, 200, results)
}

// 3. Departments
func departmentsHandler(
	w http.ResponseWriter, r *http.Request,
) {
	if r.Method != http.MethodGet {
		respondJSON(w, 405, map[string]string{
			"error": "Method not allowed",
		})
		return
	}

	rows, err := db.Query(
		r.Context(),
		`SELECT id, name, description
		 FROM departments ORDER BY name`,
	)
	if err != nil {
		respondJSON(w, 500, map[string]string{
			"error": "Could not load departments",
		})
		return
	}
	defer rows.Close()

	type Department struct {
		ID          string  `json:"id"`
		Name        string  `json:"name"`
		Description *string `json:"description"`
	}

	results := []Department{}

	for rows.Next() {
		var d Department

		if err := rows.Scan(
			&d.ID, &d.Name, &d.Description,
		); err != nil {
			respondJSON(w, 500, map[string]string{
				"error": "Could not read department",
			})
			return
		}

		results = append(results, d)
	}

	if err := rows.Err(); err != nil {
		respondJSON(w, 500, map[string]string{
			"error": "Could not read departments",
		})
		return
	}

	respondJSON(w, 200, results)
}

// 4. Upcoming Sunday services
func servicesHandler(
	w http.ResponseWriter, r *http.Request,
) {
	if r.Method != http.MethodGet {
		respondJSON(w, 405, map[string]string{
			"error": "Method not allowed",
		})
		return
	}

	rows, err := db.Query(
		r.Context(),
		`SELECT
			id, congregation_id, service_date,
			service_topic, preacher, service_verse,
			service_program, venue
		 FROM sunday_services
		 WHERE status = 'approved'
		   AND service_date >= CURRENT_DATE
		 ORDER BY service_date ASC
		 LIMIT 20`,
	)
	if err != nil {
		respondJSON(w, 500, map[string]string{
			"error": "Could not load services",
		})
		return
	}
	defer rows.Close()

	type Service struct {
		ID             string    `json:"id"`
		CongregationID string    `json:"congregation_id"`
		ServiceDate    time.Time `json:"service_date"`
		Topic          string    `json:"topic"`
		Preacher       *string   `json:"preacher"`
		Verse          *string   `json:"verse"`
		Program        *string   `json:"program"`
		Venue          *string   `json:"venue"`
	}

	results := []Service{}

	for rows.Next() {
		var s Service

		if err := rows.Scan(
			&s.ID,
			&s.CongregationID,
			&s.ServiceDate,
			&s.Topic,
			&s.Preacher,
			&s.Verse,
			&s.Program,
			&s.Venue,
		); err != nil {
			respondJSON(w, 500, map[string]string{
				"error": "Could not read service",
			})
			return
		}

		results = append(results, s)
	}

	if err := rows.Err(); err != nil {
		respondJSON(w, 500, map[string]string{
			"error": "Could not read services",
		})
		return
	}

	respondJSON(w, 200, results)
}

// 5. Upcoming events and conferences
func eventsHandler(
	w http.ResponseWriter, r *http.Request,
) {
	if r.Method != http.MethodGet {
		respondJSON(w, 405, map[string]string{
			"error": "Method not allowed",
		})
		return
	}

	rows, err := db.Query(
		r.Context(),
		`SELECT
			id, congregation_id, department_id,
			title, description, event_type,
			venue, start_date, end_date
		 FROM events
		 WHERE status = 'approved'
		   AND start_date >= NOW()
		 ORDER BY start_date ASC
		 LIMIT 20`,
	)
	if err != nil {
		respondJSON(w, 500, map[string]string{
			"error": "Could not load events",
		})
		return
	}
	defer rows.Close()

	type Event struct {
		ID             string     `json:"id"`
		CongregationID string     `json:"congregation_id"`
		DepartmentID   *string    `json:"department_id"`
		Title          string     `json:"title"`
		Description    *string    `json:"description"`
		EventType      string     `json:"event_type"`
		Venue          *string    `json:"venue"`
		StartDate      time.Time  `json:"start_date"`
		EndDate        *time.Time `json:"end_date"`
	}

	results := []Event{}

	for rows.Next() {
		var e Event

		if err := rows.Scan(
			&e.ID,
			&e.CongregationID,
			&e.DepartmentID,
			&e.Title,
			&e.Description,
			&e.EventType,
			&e.Venue,
			&e.StartDate,
			&e.EndDate,
		); err != nil {
			respondJSON(w, 500, map[string]string{
				"error": "Could not read event",
			})
			return
		}

		results = append(results, e)
	}

	if err := rows.Err(); err != nil {
		respondJSON(w, 500, map[string]string{
			"error": "Could not read events",
		})
		return
	}

	respondJSON(w, 200, results)
}

// 6. Church leadership
func leadershipHandler(
	w http.ResponseWriter, r *http.Request,
) {
	if r.Method != http.MethodGet {
		respondJSON(w, 405, map[string]string{
			"error": "Method not allowed",
		})
		return
	}

	rows, err := db.Query(
		r.Context(),
		`SELECT
			l.id,
			m.full_name,
			l.position,
			m.profile_image_url,
			c.name AS congregation,
			COALESCE(d.name, 'General') AS department
		 FROM church_leadership l
		 JOIN members m ON m.id = l.member_id
		 JOIN congregations c ON c.id = l.congregation_id
		 LEFT JOIN departments d ON d.id = l.department_id
		 WHERE l.end_date IS NULL
		    OR l.end_date >= CURRENT_DATE
		 ORDER BY c.name, l.display_order, l.position`,
	)
	if err != nil {
		respondJSON(w, 500, map[string]string{
			"error": "Could not load church leadership",
		})
		return
	}
	defer rows.Close()

	type Leader struct {
		ID           string  `json:"id"`
		FullName     string  `json:"full_name"`
		Position     string  `json:"position"`
		ImageURL     *string `json:"image_url"`
		Congregation string  `json:"congregation"`
		Department   string  `json:"department"`
	}

	results := []Leader{}

	for rows.Next() {
		var l Leader

		if err := rows.Scan(
			&l.ID,
			&l.FullName,
			&l.Position,
			&l.ImageURL,
			&l.Congregation,
			&l.Department,
		); err != nil {
			respondJSON(w, 500, map[string]string{
				"error": "Could not read leadership",
			})
			return
		}

		results = append(results, l)
	}

	if err := rows.Err(); err != nil {
		respondJSON(w, 500, map[string]string{
			"error": "Could not read leadership",
		})
		return
	}

	respondJSON(w, 200, results)
}
