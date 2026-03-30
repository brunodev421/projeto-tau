package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	apierrors "github.com/itau-ai-assistant/bfa-go/internal/errors"
	"github.com/itau-ai-assistant/bfa-go/internal/models"
	"github.com/itau-ai-assistant/bfa-go/internal/observability"
	"github.com/itau-ai-assistant/bfa-go/internal/resilience"
	"github.com/sony/gobreaker/v2"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type AgentClient interface {
	Analyze(ctx context.Context, req models.AgentRequest) (models.AgentResponse, error)
}

type agentClient struct {
	baseURL  string
	client   *http.Client
	retry    resilience.RetryPolicy
	breaker  *gobreaker.CircuitBreaker[any]
	bulkhead *resilience.Bulkhead
	metrics  *observability.Metrics
	timeout  time.Duration
}

func NewAgentClient(baseURL string, timeout time.Duration, retry resilience.RetryPolicy, breaker *gobreaker.CircuitBreaker[any], bulkhead *resilience.Bulkhead, metrics *observability.Metrics) AgentClient {
	return &agentClient{
		baseURL:  strings.TrimSuffix(baseURL, "/"),
		client:   &http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)},
		retry:    retry,
		breaker:  breaker,
		bulkhead: bulkhead,
		metrics:  metrics,
		timeout:  timeout,
	}
}

func (c *agentClient) Analyze(ctx context.Context, payload models.AgentRequest) (models.AgentResponse, error) {
	requestCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	var response models.AgentResponse
	err := c.bulkhead.Run(requestCtx, "agent_service", func() error {
		_, breakerErr := c.breaker.Execute(func() (any, error) {
			body, err := json.Marshal(payload)
			if err != nil {
				return nil, err
			}

			operation := func() error {
				req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, c.baseURL+"/v1/agent/analyze", bytes.NewReader(body))
				if err != nil {
					return err
				}
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-Request-ID", payload.RequestID)

				resp, err := c.client.Do(req)
				if err != nil {
					return err
				}
				defer resp.Body.Close()

				if resp.StatusCode >= http.StatusBadRequest {
					raw, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
					return &resilience.HTTPStatusError{StatusCode: resp.StatusCode, Body: string(raw)}
				}
				if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
					return err
				}
				if response.Answer == "" {
					return apierrors.ErrResponseValidation
				}
				return nil
			}

			if err := c.retry.Do(requestCtx, "agent_service", operation); err != nil {
				return nil, err
			}
			return nil, nil
		})
		return breakerErr
	})
	if err != nil {
		c.metrics.DownstreamCalls.WithLabelValues("agent_service", "error").Inc()
		return models.AgentResponse{}, err
	}
	c.metrics.DownstreamCalls.WithLabelValues("agent_service", "success").Inc()
	return response, nil
}
