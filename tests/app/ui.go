package main

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strings"
)

// The web UI the browser resource's feature drives: a contract template
// edited as one continuous document. Each paragraph is a clause; editing
// the master and applying it updates the countries' versions, except where
// a country has its own. Everything lives in Postgres, so two browsers see
// the same document and tomato's reset clears it between scenarios.

//go:embed ui.html
var uiPage string

// defaultClauses is the master an empty database starts from.
var defaultClauses = []clause{
	{ID: "7.1", Body: "Zinsen werden monatlich berechnet."},
	{ID: "7.2", Body: "Die Zinsen werden jährlich gezahlt"},
}

type clause struct {
	ID   string `json:"clause_id"`
	Body string `json:"body"`
}

func initUIDB() error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS master_clauses (
			clause_id TEXT PRIMARY KEY,
			body TEXT NOT NULL,
			position INT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS country_clauses (
			country TEXT NOT NULL,
			clause_id TEXT NOT NULL,
			body TEXT NOT NULL,
			local_override BOOLEAN NOT NULL DEFAULT false,
			effective TEXT NOT NULL DEFAULT 'now',
			PRIMARY KEY (country, clause_id)
		)
	`)
	return err
}

func registerUI(mux *http.ServeMux) {
	mux.HandleFunc("/ui/master", uiMasterHandler)
	mux.HandleFunc("/ui/history", uiHistoryHandler)
	mux.HandleFunc("/ui/api/document", uiDocumentHandler)
	mux.HandleFunc("/ui/api/apply", uiApplyHandler)
}

func uiMasterHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	lang := r.Header.Get("Accept-Language")
	if i := strings.IndexByte(lang, ','); i >= 0 {
		lang = lang[:i]
	}
	fmt.Fprint(w, strings.Replace(uiPage, "{{ACCEPT_LANGUAGE}}", html.EscapeString(lang), 1))
}

func uiHistoryHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(`SELECT country, clause_id, body FROM country_clauses WHERE NOT local_override ORDER BY country, clause_id`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<!doctype html><html><head><meta charset="utf-8"><title>History</title></head><body><h1>Change history</h1><ul>`)
	for rows.Next() {
		var country, id, body string
		if err := rows.Scan(&country, &id, &body); err != nil {
			return
		}
		fmt.Fprintf(w, "<li>%s %s: %s</li>", html.EscapeString(country), html.EscapeString(id), html.EscapeString(body))
	}
	fmt.Fprint(w, `</ul><a href="/ui/master">Back to the master</a></body></html>`)
}

// masterClauses reads the master, seeding it when the database is empty.
func masterClauses() ([]clause, error) {
	for i, c := range defaultClauses {
		if _, err := db.Exec(`INSERT INTO master_clauses (clause_id, body, position) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`, c.ID, c.Body, i); err != nil {
			return nil, err
		}
	}
	rows, err := db.Query(`SELECT clause_id, body FROM master_clauses ORDER BY position`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []clause
	for rows.Next() {
		var c clause
		if err := rows.Scan(&c.ID, &c.Body); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func uiDocumentHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
	case http.MethodPut:
		var req struct {
			Clauses []clause `json:"clauses"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		for _, c := range req.Clauses {
			if _, err := db.Exec(`UPDATE master_clauses SET body = $2 WHERE clause_id = $1`, c.ID, c.Body); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	clauses, err := masterClauses()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"clauses": clauses})
}

func uiApplyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Clauses   []string `json:"clauses"`
		Countries []string `json:"countries"`
		Effective string   `json:"effective"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	type skip struct {
		Country string `json:"country"`
		Clause  string `json:"clause_id"`
	}
	applied := []string{}
	skipped := []skip{}
	for _, country := range req.Countries {
		changed := false
		for _, id := range req.Clauses {
			var override bool
			err := db.QueryRow(`SELECT local_override FROM country_clauses WHERE country = $1 AND clause_id = $2`, country, id).Scan(&override)
			if err != nil && err != sql.ErrNoRows {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if override {
				skipped = append(skipped, skip{country, id})
				continue
			}
			if _, err := db.Exec(`
				INSERT INTO country_clauses (country, clause_id, body, effective)
				SELECT $1, clause_id, body, $3 FROM master_clauses WHERE clause_id = $2
				ON CONFLICT (country, clause_id) DO UPDATE SET body = EXCLUDED.body, effective = EXCLUDED.effective`,
				country, id, req.Effective); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			changed = true
		}
		if changed {
			applied = append(applied, country)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"applied": applied, "skipped": skipped, "effective": req.Effective})
}
