package handlers

import (
	"encoding/json"
	"net/http"
	"regexp"

	"github.com/go-chi/chi/v5"
	apierrors "github.com/itau-ai-assistant/bfa-go/internal/errors"
	"github.com/itau-ai-assistant/bfa-go/internal/middleware"
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
	_ = json.NewEncoder(w).Encode(response)
}

func writeError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(apierrors.StatusCode(err))
	var payload any = apierrors.ErrInternalFailure
	if apiErr, ok := err.(apierrors.APIError); ok {
		payload = apiErr
	}
	_ = json.NewEncoder(w).Encode(payload)
}
