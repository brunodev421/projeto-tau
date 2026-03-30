package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/itau-ai-assistant/bfa-go/internal/api"
	"github.com/itau-ai-assistant/bfa-go/internal/cache"
	"github.com/itau-ai-assistant/bfa-go/internal/clients"
	bhandlers "github.com/itau-ai-assistant/bfa-go/internal/handlers"
	"github.com/itau-ai-assistant/bfa-go/internal/models"
	"github.com/itau-ai-assistant/bfa-go/internal/observability"
	"github.com/itau-ai-assistant/bfa-go/internal/resilience"
	"github.com/itau-ai-assistant/bfa-go/internal/service"
)

func TestAssistantEndpointSuccess(t *testing.T) {
	router := newTestRouter(t, dependencyHandlers{
		profile: func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(models.Profile{CustomerID: "pj-healthy", Segment: "smb", KYCStatus: "complete", RiskTier: "low"})
		},
		transactions: func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(models.TransactionsSnapshot{CustomerID: "pj-healthy", CurrentBalanceBRL: 100000, AverageMonthlyInflow: 200000, AverageMonthlyOutflow: 150000})
		},
		agent: func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(models.AgentResponse{
				Answer:           "Cliente com fluxo saudavel.",
				ReasoningSummary: "Analise com perfil e transacoes.",
				Recommendations:  []string{"Manter reserva de caixa."},
			})
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/assistant/pj-healthy?question=Analise+financeira", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var response models.AssistantResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if response.Assistant.Answer == "" {
		t.Fatal("expected assistant answer")
	}
	if len(response.Dependencies) != 2 {
		t.Fatalf("expected 2 dependencies, got %d", len(response.Dependencies))
	}
}

func TestAssistantEndpointDegradesWhenProfileFails(t *testing.T) {
	router := newTestRouter(t, dependencyHandlers{
		profile: func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "profile error", http.StatusServiceUnavailable)
		},
		transactions: func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(models.TransactionsSnapshot{CustomerID: "pj-risk", CurrentBalanceBRL: 10000, AverageMonthlyInflow: 50000, AverageMonthlyOutflow: 60000})
		},
		agent: func(w http.ResponseWriter, r *http.Request) {
			var payload models.AgentRequest
			_ = json.NewDecoder(r.Body).Decode(&payload)
			if payload.Profile != nil {
				t.Fatal("expected missing profile in degraded scenario")
			}
			_ = json.NewEncoder(w).Encode(models.AgentResponse{Answer: "Resposta degradada.", ReasoningSummary: "Sem perfil completo."})
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/assistant/pj-risk?question=Analise+financeira", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var response models.AssistantResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &response)
	if response.Dependencies[0].Status != models.DependencyStatusFailed {
		t.Fatalf("expected failed profile dependency, got %s", response.Dependencies[0].Status)
	}
}

func TestAssistantEndpointTimeoutWhenAllDependenciesTimeout(t *testing.T) {
	router := newTestRouter(t, dependencyHandlers{
		profile: func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(250 * time.Millisecond)
			_ = json.NewEncoder(w).Encode(models.Profile{CustomerID: "pj-timeout"})
		},
		transactions: func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(250 * time.Millisecond)
			_ = json.NewEncoder(w).Encode(models.TransactionsSnapshot{CustomerID: "pj-timeout"})
		},
		agent: func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(models.AgentResponse{Answer: "na", ReasoningSummary: "na"})
		},
		timeout: 100 * time.Millisecond,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/assistant/pj-timeout?question=Analise+financeira", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("expected 504, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAssistantEndpointUsesFallbackWhenAgentFails(t *testing.T) {
	router := newTestRouter(t, dependencyHandlers{
		profile: func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(models.Profile{CustomerID: "pj-healthy", KYCStatus: "complete"})
		},
		transactions: func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(models.TransactionsSnapshot{CustomerID: "pj-healthy", CurrentBalanceBRL: 100000})
		},
		agent: func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "agent down", http.StatusServiceUnavailable)
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/assistant/pj-healthy?question=Analise+financeira", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var response models.AssistantResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &response)
	if !response.Assistant.FallbackUsed {
		t.Fatal("expected fallback response")
	}
}

func TestAssistantEndpointCachesProfile(t *testing.T) {
	var profileCalls int32
	router := newTestRouter(t, dependencyHandlers{
		profile: func(w http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&profileCalls, 1)
			_ = json.NewEncoder(w).Encode(models.Profile{CustomerID: "pj-cache", KYCStatus: "complete"})
		},
		transactions: func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(models.TransactionsSnapshot{CustomerID: "pj-cache", CurrentBalanceBRL: 50000})
		},
		agent: func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(models.AgentResponse{Answer: "ok", ReasoningSummary: "ok"})
		},
	})

	for range 2 {
		req := httptest.NewRequest(http.MethodGet, "/v1/assistant/pj-cache?question=Analise+financeira", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	}

	if atomic.LoadInt32(&profileCalls) != 1 {
		t.Fatalf("expected profile endpoint to be called once, got %d", profileCalls)
	}
}

func TestAssistantEndpointRetriesTransientProfileError(t *testing.T) {
	var profileCalls int32
	router := newTestRouter(t, dependencyHandlers{
		profile: func(w http.ResponseWriter, _ *http.Request) {
			call := atomic.AddInt32(&profileCalls, 1)
			if call < 3 {
				http.Error(w, "transient", http.StatusServiceUnavailable)
				return
			}
			_ = json.NewEncoder(w).Encode(models.Profile{CustomerID: "pj-retry", KYCStatus: "complete"})
		},
		transactions: func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(models.TransactionsSnapshot{CustomerID: "pj-retry", CurrentBalanceBRL: 50000})
		},
		agent: func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(models.AgentResponse{Answer: "ok", ReasoningSummary: "ok"})
		},
		timeout: 700 * time.Millisecond,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/assistant/pj-retry?question=Analise+financeira", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if atomic.LoadInt32(&profileCalls) != 3 {
		t.Fatalf("expected 3 profile calls, got %d", profileCalls)
	}
}

type dependencyHandlers struct {
	profile      http.HandlerFunc
	transactions http.HandlerFunc
	agent        http.HandlerFunc
	timeout      time.Duration
}

func newTestRouter(t *testing.T, handlers dependencyHandlers) http.Handler {
	t.Helper()
	if handlers.timeout == 0 {
		handlers.timeout = 150 * time.Millisecond
	}

	downstreamMux := http.NewServeMux()
	downstreamMux.HandleFunc("/profile/", handlers.profile)
	downstreamMux.HandleFunc("/transactions/", handlers.transactions)
	downstreamServer := httptest.NewServer(downstreamMux)
	t.Cleanup(downstreamServer.Close)

	agentServer := httptest.NewServer(http.HandlerFunc(handlers.agent))
	t.Cleanup(agentServer.Close)

	logger := observability.NewLogger("test-bfa")
	registry := prometheus.NewRegistry()
	metrics := observability.NewMetrics(registry)
	retryPolicy := resilience.NewRetryPolicy(3, metrics)
	bulkhead := resilience.NewBulkhead(4, metrics)

	profileClient := clients.NewProfileClient(
		downstreamServer.URL,
		handlers.timeout,
		retryPolicy,
		resilience.NewCircuitBreaker("profile_api", 0.5, 2, 50*time.Millisecond, metrics),
		bulkhead,
		metrics,
	)
	transactionsClient := clients.NewTransactionsClient(
		downstreamServer.URL,
		handlers.timeout,
		retryPolicy,
		resilience.NewCircuitBreaker("transactions_api", 0.5, 2, 50*time.Millisecond, metrics),
		bulkhead,
		metrics,
	)
	agentClient := clients.NewAgentClient(
		agentServer.URL,
		handlers.timeout,
		retryPolicy,
		resilience.NewCircuitBreaker("agent_service", 0.5, 2, 50*time.Millisecond, metrics),
		bulkhead,
		metrics,
	)

	assistantService := service.NewAssistantService(
		profileClient,
		transactionsClient,
		agentClient,
		cache.New[models.Profile](),
		5*time.Minute,
		logger,
		metrics,
	)
	return api.NewRouter(logger, metrics, registry, bhandlers.NewAssistantHandler(assistantService))
}
