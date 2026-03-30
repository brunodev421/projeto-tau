package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/itau-ai-assistant/mock-services/internal/scenarios"
	"github.com/itau-ai-assistant/mock-services/internal/service"
)

type Handlers struct {
	repository *service.Repository
}

func New(repository *service.Repository) Handlers {
	return Handlers{repository: repository}
}

func (h Handlers) Profile(w http.ResponseWriter, r *http.Request, customerID string) {
	switch r.URL.Query().Get("scenario") {
	case scenarios.ScenarioProfileTimeout:
		time.Sleep(3 * time.Second)
	case scenarios.ScenarioProfileError:
		http.Error(w, `{"code":"profile_failure","message":"simulated profile failure"}`, http.StatusServiceUnavailable)
		return
	}

	profile, ok := h.repository.Profile(customerID)
	if !ok {
		http.Error(w, `{"code":"not_found"}`, http.StatusNotFound)
		return
	}

	switch r.URL.Query().Get("scenario") {
	case scenarios.ScenarioProfilePartial:
		profile.CompanyName = ""
		profile.KYCStatus = ""
		profile.Degraded = true
	case scenarios.ScenarioDegraded:
		profile.Degraded = true
	}

	writeJSON(w, profile)
}

func (h Handlers) Transactions(w http.ResponseWriter, r *http.Request, customerID string) {
	switch r.URL.Query().Get("scenario") {
	case scenarios.ScenarioTransactionsTimeout:
		time.Sleep(3 * time.Second)
	case scenarios.ScenarioTransactionsError:
		http.Error(w, `{"code":"transactions_failure","message":"simulated transactions failure"}`, http.StatusServiceUnavailable)
		return
	}

	transactions, ok := h.repository.Transactions(customerID)
	if !ok {
		http.Error(w, `{"code":"not_found"}`, http.StatusNotFound)
		return
	}

	switch r.URL.Query().Get("scenario") {
	case scenarios.ScenarioTransactionsPartial:
		transactions.RecentTransactions = transactions.RecentTransactions[:1]
		transactions.Degraded = true
	case scenarios.ScenarioDegraded:
		transactions.Degraded = true
	}

	writeJSON(w, transactions)
}

func Healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]string{"status": "ok"})
}

func Readyz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]string{"status": "ready"})
}

func Metrics(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]string{"status": "metrics_available_via_prometheus"})
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func BuildRoute(handler func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		customerID := r.PathValue("customerId")
		handler(w, r, customerID)
	}
}
