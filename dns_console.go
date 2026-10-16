package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
)

var consolePage = template.Must(template.New("console").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Nonprofit DNS console</title><style>
body{font:16px system-ui;margin:2rem auto;max-width:46rem;padding:0 1rem;color:#17211b}fieldset{display:grid;gap:.8rem;border:1px solid #aab5ad;padding:1rem}label{display:grid;gap:.25rem}input,button{font:inherit;padding:.6rem}button{width:max-content}pre{white-space:pre-wrap;background:#f2f5f3;padding:1rem}
</style></head><body><h1>Nonprofit DNS console</h1>
<form method="post" action="/sync"><fieldset><legend>Program delivery records</legend>
<label>Domain<input name="domain" required></label><label>Receipt mail server<input name="receipt_mail_server" required></label>
<label>Volunteer portal target<input name="volunteer_portal" required></label><label>Campaign report token<input name="campaign_report_token" required></label>
<button type="submit">Sync records</button></fieldset></form><p><a href="/records?domain=">Inspect records by adding a domain to the URL</a></p></body></html>`))

type consoleServer struct{ client *InfraiClient }

func (s consoleServer) home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" || r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	if err := consolePage.Execute(w, nil); err != nil {
		http.Error(w, "render page", http.StatusInternalServerError)
	}
}

func (s consoleServer) sync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	result, err := SyncProgramDNS(r.Context(), s.client, ProgramDNS{
		Domain: r.FormValue("domain"), ReceiptMailServer: r.FormValue("receipt_mail_server"),
		VolunteerPortal: r.FormValue("volunteer_portal"), CampaignReportToken: r.FormValue("campaign_report_token"),
	})
	if err != nil {
		writeClientError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"zone_id": result.ZoneID, "records_synced": result.Count})
}

func (s consoleServer) records(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodDelete {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	zoneID, err := s.client.ZoneID(r.Context(), r.URL.Query().Get("domain"))
	if err != nil {
		writeClientError(w, err)
		return
	}
	if r.Method == http.MethodDelete {
		err = s.client.DeleteRecords(r.Context(), zoneID)
		if err == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}
	} else {
		var records json.RawMessage
		records, err = s.client.ListRecords(r.Context(), zoneID)
		if err == nil {
			w.Header().Set("Content-Type", "application/json")
			w.Write(records)
			return
		}
	}
	writeClientError(w, err)
}

func writeClientError(w http.ResponseWriter, err error) {
	status := http.StatusBadGateway
	if apiErr, ok := err.(*APIError); ok && apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500 {
		status = apiErr.HTTPStatus
	}
	http.Error(w, err.Error(), status)
}

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	server := consoleServer{client: NewInfraiClient(key)}
	mux := http.NewServeMux()
	mux.HandleFunc("/", server.home)
	mux.HandleFunc("/sync", server.sync)
	mux.HandleFunc("/records", server.records)
	addr := ":8080"
	fmt.Printf("nonprofit DNS console listening on http://localhost%s\n", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
