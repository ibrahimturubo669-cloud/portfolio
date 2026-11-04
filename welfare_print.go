package main

import (
	"html/template"
	"net/http"
	"strings"
)

type WelfarePrintRecord struct {
	MemberName       string
	Amount           string
	ContributionType string
	IncidentTitle    string
	ContributionDate string
	Status           string
}

type WelfarePrintData struct {
	ChurchName    string
	StartDate     string
	EndDate       string
	Records       []WelfarePrintRecord
	VerifiedTotal string
	PendingTotal  string
	RejectedTotal string
}

var welfarePrintTemplate = template.Must(template.New("welfare").Parse(`
<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<title>Welfare Contribution Report</title>
<style>
	body {
		font-family: Arial, sans-serif;
		margin: 30px;
		color: #222;
	}
	h1, h2, p { text-align: center; }
	table {
		width: 100%;
		border-collapse: collapse;
		margin-top: 25px;
	}
	th, td {
		border: 1px solid #999;
		padding: 9px;
		text-align: left;
	}
	th { background: #eee; }
	.totals {
		margin-top: 25px;
		width: 50%;
		margin-left: auto;
	}
	.signatures {
		margin-top: 60px;
		display: flex;
		justify-content: space-between;
	}
	.print-button {
		padding: 10px 20px;
		cursor: pointer;
	}
	@media print {
		.print-button { display: none; }
		body { margin: 10mm; }
	}
</style>
</head>
<body>

<button class="print-button" onclick="window.print()">
	Print Report
</button>

<h1>{{.ChurchName}}</h1>
<h2>WELFARE CONTRIBUTION REPORT</h2>

<p>
	Reporting period:
	{{if .StartDate}}{{.StartDate}}{{else}}All dates{{end}}
	to
	{{if .EndDate}}{{.EndDate}}{{else}}Present{{end}}
</p>

<table>
	<thead>
		<tr>
			<th>Member</th>
			<th>Contribution Type</th>
			<th>Date</th>
			<th>Amount (KSh)</th>
			<th>Status</th>
		</tr>
	</thead>
	<tbody>
	{{range .Records}}
		<tr>
			<td>{{.MemberName}}</td>
			<td>{{.IncidentTitle}}</td>
			<td>{{.ContributionDate}}</td>
			<td>{{.Amount}}</td>
			<td>{{.Status}}</td>
		</tr>
	{{else}}
		<tr>
			<td colspan="5">No contributions found.</td>
		</tr>
	{{end}}
	</tbody>
</table>

<table class="totals">
	<tr>
		<th>Verified contributions</th>
		<td>KSh {{.VerifiedTotal}}</td>
	</tr>
	<tr>
		<th>Pending contributions</th>
		<td>KSh {{.PendingTotal}}</td>
	</tr>
	<tr>
		<th>Rejected contributions</th>
		<td>KSh {{.RejectedTotal}}</td>
	</tr>
</table>

<div class="signatures">
	<p>Prepared by: __________________</p>
	<p>Verified by: __________________</p>
</div>

</body>
</html>
`))

func welfarePrintHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "Method not allowed",
		})
		return
	}

	congregationID := strings.TrimSpace(r.URL.Query().Get("congregation_id"))
	memberID := strings.TrimSpace(r.URL.Query().Get("member_id"))
	startDate := strings.TrimSpace(r.URL.Query().Get("start_date"))
	endDate := strings.TrimSpace(r.URL.Query().Get("end_date"))

	if congregationID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "congregation_id is required",
		})
		return
	}

	if startDate != "" && endDate != "" && startDate > endDate {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid reporting date range",
		})
		return
	}

	data := WelfarePrintData{
		ChurchName: "Kimilili Town Friends Church",
		StartDate:  startDate,
		EndDate:    endDate,
		Records:    []WelfarePrintRecord{},
	}

	rows, err := db.Query(r.Context(), `
		SELECT
			m.full_name,
			wc.amount::text,
			wc.contribution_type,
			COALESCE(wi.title, 'General Welfare'),
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
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Could not load report. Check the filters.",
		})
		return
	}
	defer rows.Close()

	for rows.Next() {
		var item WelfarePrintRecord

		if err := rows.Scan(
			&item.MemberName,
			&item.Amount,
			&item.ContributionType,
			&item.IncidentTitle,
			&item.ContributionDate,
			&item.Status,
		); err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "Could not read report records",
			})
			return
		}

		data.Records = append(data.Records, item)
	}

	if err := rows.Err(); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not finish reading report",
		})
		return
	}

	err = db.QueryRow(r.Context(), `
		SELECT
			COALESCE(SUM(amount) FILTER (WHERE status = 'verified'), 0)::text,
			COALESCE(SUM(amount) FILTER (WHERE status = 'pending'), 0)::text,
			COALESCE(SUM(amount) FILTER (WHERE status = 'rejected'), 0)::text
		FROM welfare_contributions
		WHERE congregation_id = $1
		  AND ($2 = '' OR member_id::text = $2)
		  AND ($3 = '' OR contribution_date >= $3::date)
		  AND ($4 = '' OR contribution_date <= $4::date)
	`, congregationID, memberID, startDate, endDate).Scan(
		&data.VerifiedTotal,
		&data.PendingTotal,
		&data.RejectedTotal,
	)

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not calculate report totals",
		})
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := welfarePrintTemplate.Execute(w, data); err != nil {
		return
	}
}
