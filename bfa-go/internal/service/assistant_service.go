package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/itau-ai-assistant/bfa-go/internal/cache"
	"github.com/itau-ai-assistant/bfa-go/internal/clients"
	apierrors "github.com/itau-ai-assistant/bfa-go/internal/errors"
	"github.com/itau-ai-assistant/bfa-go/internal/models"
	"github.com/itau-ai-assistant/bfa-go/internal/observability"
	"golang.org/x/sync/errgroup"
)

type AssistantService struct {
	profileClient      clients.ProfileClient
	transactionsClient clients.TransactionsClient
	agentClient        clients.AgentClient
	profileCache       *cache.TTLCache[models.Profile]
	profileCacheTTL    time.Duration
	logger             *slog.Logger
	metrics            *observability.Metrics
}

func NewAssistantService(profileClient clients.ProfileClient, transactionsClient clients.TransactionsClient, agentClient clients.AgentClient, profileCache *cache.TTLCache[models.Profile], profileCacheTTL time.Duration, logger *slog.Logger, metrics *observability.Metrics) *AssistantService {
	return &AssistantService{
		profileClient:      profileClient,
		transactionsClient: transactionsClient,
		agentClient:        agentClient,
		profileCache:       profileCache,
		profileCacheTTL:    profileCacheTTL,
		logger:             logger,
		metrics:            metrics,
	}
}

func (s *AssistantService) GetAssistant(ctx context.Context, customerID, question, scenario, requestID string) (models.AssistantResponse, error) {
	if strings.TrimSpace(customerID) == "" {
		return models.AssistantResponse{}, apierrors.ErrBadRequest
	}
	if strings.TrimSpace(question) == "" {
		question = "Como está a saúde financeira da minha empresa e quais são as próximas ações recomendadas?"
	}

	var (
		profile             *models.Profile
		transactions        *models.TransactionsSnapshot
		profileSummary      models.DependencySummary
		transactionsSummary models.DependencySummary
		profileErr          error
		transactionsErr     error
	)

	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		if cachedProfile, ok := s.profileCache.Get(customerID); ok {
			s.metrics.CacheLookups.WithLabelValues("profile_cache", "hit").Inc()
			profile = &cachedProfile
			profileSummary = models.DependencySummary{
				Name:      "profile_api",
				Status:    models.DependencyStatusCached,
				Source:    "cache",
				LatencyMS: 0,
			}
			return nil
		}

		s.metrics.CacheLookups.WithLabelValues("profile_cache", "miss").Inc()
		result, summary, err := s.profileClient.GetProfile(groupCtx, customerID, requestID, scenario)
		if err == nil && result != nil {
			s.profileCache.Set(customerID, *result, s.profileCacheTTL)
		}
		profile = result
		profileSummary = summary
		profileErr = err
		return nil
	})

	group.Go(func() error {
		result, summary, err := s.transactionsClient.GetTransactions(groupCtx, customerID, requestID, scenario)
		transactions = result
		transactionsSummary = summary
		transactionsErr = err
		return nil
	})

	if err := group.Wait(); err != nil {
		return models.AssistantResponse{}, apierrors.ErrInternalFailure
	}

	dependencies := []models.DependencySummary{profileSummary, transactionsSummary}
	if profileErr != nil && transactionsErr != nil {
		return models.AssistantResponse{}, mapDependencyError(profileErr, transactionsErr)
	}

	agentPayload := models.AgentRequest{
		RequestID:        requestID,
		CustomerID:       customerID,
		Question:         question,
		Profile:          profile,
		Transactions:     transactions,
		DependencyStatus: dependencies,
	}

	agentResponse, err := s.agentClient.Analyze(ctx, agentPayload)
	if err != nil {
		s.logger.Warn("agent service failed, serving conservative fallback", "request_id", requestID, "customer_id", customerID, "error", err)
		agentResponse = fallbackResponse(profile, transactions, dependencies)
	}

	return models.AssistantResponse{
		RequestID:    requestID,
		CustomerID:   customerID,
		Question:     question,
		Dependencies: dependencies,
		Assistant:    agentResponse,
	}, nil
}

func fallbackResponse(profile *models.Profile, transactions *models.TransactionsSnapshot, dependencies []models.DependencySummary) models.AgentResponse {
	recommendations := []string{
		"Revisar fluxo de caixa dos próximos 30 dias antes de assumir novas obrigações.",
		"Confirmar com o gerente se existem dados cadastrais pendentes que possam afetar elegibilidade.",
	}
	riskFlags := []string{"analysis_degraded"}
	if profile == nil {
		riskFlags = append(riskFlags, "profile_unavailable")
	}
	if transactions == nil {
		riskFlags = append(riskFlags, "transactions_unavailable")
	}

	answer := "No momento não foi possível concluir toda a análise automatizada com confiança total. Recomendo usar esta resposta como orientação conservadora e revisar a situação com dados atualizados antes de qualquer decisão de crédito."
	reasoning := "Fallback local acionado porque o serviço do agente ficou indisponível. A resposta prioriza cautela, transparência e continuidade operacional."
	sources := []string{"fallback://bfa"}
	for _, dep := range dependencies {
		sources = append(sources, fmt.Sprintf("dependency://%s/%s", dep.Name, dep.Status))
	}

	return models.AgentResponse{
		Answer:           answer,
		ReasoningSummary: reasoning,
		Recommendations:  recommendations,
		Sources:          sources,
		ToolsUsed:        []string{"bfa_fallback"},
		RiskFlags:        riskFlags,
		FallbackUsed:     true,
		CostEstimate:     models.CostEstimate{},
	}
}

func mapDependencyError(errs ...error) error {
	for _, err := range errs {
		if err == nil {
			continue
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return apierrors.ErrUpstreamTimeout
		}
	}
	return apierrors.ErrDependencyFailure
}
