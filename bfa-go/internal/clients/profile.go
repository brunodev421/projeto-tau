package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/itau-ai-assistant/bfa-go/internal/models"
	"github.com/itau-ai-assistant/bfa-go/internal/observability"
	"github.com/itau-ai-assistant/bfa-go/internal/resilience"
	"github.com/sony/gobreaker/v2"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type ProfileClient interface {
	GetProfile(ctx context.Context, customerID, requestID, scenario string) (*models.Profile, models.DependencySummary, error)
}

type profileClient struct {
	baseURL  string
	client   *http.Client
	retry    resilience.RetryPolicy
	breaker  *gobreaker.CircuitBreaker[any]
	bulkhead *resilience.Bulkhead
	metrics  *observability.Metrics
	timeout  time.Duration
}

func NewProfileClient(baseURL string, timeout time.Duration, retry resilience.RetryPolicy, breaker *gobreaker.CircuitBreaker[any], bulkhead *resilience.Bulkhead, metrics *observability.Metrics) ProfileClient {
	return &profileClient{
		baseURL:  strings.TrimSuffix(baseURL, "/"),
		client:   &http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)},
		retry:    retry,
		breaker:  breaker,
		bulkhead: bulkhead,
		metrics:  metrics,
		timeout:  timeout,
	}
}

func (c *profileClient) GetProfile(ctx context.Context, customerID, requestID, scenario string) (*models.Profile, models.DependencySummary, error) {
	start := time.Now()
	summary := models.DependencySummary{Name: "profile_api", Source: "network"}
	requestCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	var profile models.Profile
	err := c.bulkhead.Run(requestCtx, "profile_api", func() error {
		_, breakerErr := c.breaker.Execute(func() (any, error) {
			operation := func() error {
				endpoint, err := url.Parse(fmt.Sprintf("%s/profile/%s", c.baseURL, customerID))
				if err != nil {
					return err
				}
				if scenario != "" {
					query := endpoint.Query()
					query.Set("scenario", scenario)
					endpoint.RawQuery = query.Encode()
				}

				req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, endpoint.String(), nil)
				if err != nil {
					return err
				}
				req.Header.Set("X-Request-ID", requestID)

				resp, err := c.client.Do(req)
				if err != nil {
					return err
				}
				defer resp.Body.Close()

				if resp.StatusCode >= http.StatusBadRequest {
					body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
					return &resilience.HTTPStatusError{StatusCode: resp.StatusCode, Body: string(body)}
				}

				if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
					return err
				}
				return nil
			}
			if err := c.retry.Do(requestCtx, "profile_api", operation); err != nil {
				return nil, err
			}
			return nil, nil
		})
		return breakerErr
	})

	summary.LatencyMS = time.Since(start).Milliseconds()
	if err != nil {
		summary.Status = models.DependencyStatusFailed
		summary.ErrorCode = "profile_api_unavailable"
		summary.ErrorMessage = err.Error()
		c.metrics.DownstreamCalls.WithLabelValues("profile_api", "error").Inc()
		return nil, summary, err
	}

	if profile.Degraded {
		summary.Status = models.DependencyStatusDegraded
	} else {
		summary.Status = models.DependencyStatusOK
	}
	c.metrics.DownstreamCalls.WithLabelValues("profile_api", "success").Inc()
	return &profile, summary, nil
}
