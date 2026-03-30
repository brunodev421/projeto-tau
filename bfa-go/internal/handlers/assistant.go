package handlers

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	apierrors "github.com/itau-ai-assistant/bfa-go/internal/errors"
	"github.com/itau-ai-assistant/bfa-go/internal/middleware"
	"github.com/itau-ai-assistant/bfa-go/internal/models"
	"github.com/itau-ai-assistant/bfa-go/internal/observability"
	"github.com/itau-ai-assistant/bfa-go/internal/service"
)

var customerIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,64}$`)

type AssistantHandler struct {
	service *service.AssistantService
}

func NewAssistantHandler(service *service.AssistantService) AssistantHandler {
	return AssistantHandler{service: service}
}

func (h AssistantHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	customerID := chi.URLParam(r, "customerId")
	if !customerIDPattern.MatchString(customerID) {
		writeError(w, apierrors.ErrBadRequest)
		return
	}

	ctx := middleware.WithRoute(r.Context(), "/v1/assistant/{customerId}")
	ctx = middleware.WithCustomerID(ctx, customerID)
	r = r.WithContext(ctx)

	response, err := h.service.GetAssistant(r.Context(), customerID, r.URL.Query().Get("question"), r.URL.Query().Get("scenario"), observability.RequestID(r.Context()))
	if err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Downstream", downstreamHeader(response.Dependencies))
	_ = json.NewEncoder(w).Encode(response)
}

func writeError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	var payload any = apierrors.ErrInternalFailure
	if apiErr, ok := err.(apierrors.APIError); ok {
		payload = apiErr
		w.Header().Set("X-Error-Code", apiErr.Code)
		if apiErr.Code == apierrors.ErrDependencyFailure.Code || apiErr.Code == apierrors.ErrUpstreamTimeout.Code {
			w.Header().Set("X-Downstream", "profile_api,transactions_api")
		}
	} else {
		w.Header().Set("X-Error-Code", apierrors.ErrInternalFailure.Code)
	}
	w.WriteHeader(apierrors.StatusCode(err))
	_ = json.NewEncoder(w).Encode(payload)
}

func downstreamHeader(dependencies []models.DependencySummary) string {
	names := make([]string, 0, len(dependencies)+1)
	seen := map[string]struct{}{}
	for _, dependency := range dependencies {
		if dependency.Name == "" {
			continue
		}
		if _, ok := seen[dependency.Name]; ok {
			continue
		}
		seen[dependency.Name] = struct{}{}
		names = append(names, dependency.Name)
	}
	if _, ok := seen["agent_service"]; !ok {
		names = append(names, "agent_service")
	}
	return strings.Join(names, ",")
}
