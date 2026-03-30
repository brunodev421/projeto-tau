package models

import "time"

type Profile struct {
	CustomerID       string    `json:"customer_id"`
	Segment          string    `json:"segment,omitempty"`
	CompanyName      string    `json:"company_name,omitempty"`
	Industry         string    `json:"industry,omitempty"`
	AnnualRevenueBRL float64   `json:"annual_revenue_brl,omitempty"`
	AccountManager   string    `json:"account_manager,omitempty"`
	KYCStatus        string    `json:"kyc_status,omitempty"`
	RiskTier         string    `json:"risk_tier,omitempty"`
	LastUpdatedAt    time.Time `json:"last_updated_at,omitempty"`
	Degraded         bool      `json:"degraded,omitempty"`
}

type Transaction struct {
	Date        time.Time `json:"date"`
	Type        string    `json:"type"`
	AmountBRL   float64   `json:"amount_brl"`
	Counterpart string    `json:"counterpart,omitempty"`
	Category    string    `json:"category,omitempty"`
}

type TransactionsSnapshot struct {
	CustomerID            string        `json:"customer_id"`
	CurrentBalanceBRL     float64       `json:"current_balance_brl"`
	AverageMonthlyInflow  float64       `json:"average_monthly_inflow_brl"`
	AverageMonthlyOutflow float64       `json:"average_monthly_outflow_brl"`
	OverdraftUsageDays    int           `json:"overdraft_usage_days"`
	LatePaymentEvents     int           `json:"late_payment_events"`
	TopCategories         []string      `json:"top_categories,omitempty"`
	RecentTransactions    []Transaction `json:"recent_transactions,omitempty"`
	Degraded              bool          `json:"degraded,omitempty"`
}

type DependencyStatus string

const (
	DependencyStatusOK       DependencyStatus = "ok"
	DependencyStatusCached   DependencyStatus = "cached"
	DependencyStatusDegraded DependencyStatus = "degraded"
	DependencyStatusFailed   DependencyStatus = "failed"
)

type DependencySummary struct {
	Name         string           `json:"name"`
	Status       DependencyStatus `json:"status"`
	Source       string           `json:"source"`
	ErrorCode    string           `json:"error_code,omitempty"`
	ErrorMessage string           `json:"error_message,omitempty"`
	LatencyMS    int64            `json:"latency_ms,omitempty"`
}

type AgentRequest struct {
	RequestID        string                `json:"request_id"`
	CustomerID       string                `json:"customer_id"`
	Question         string                `json:"question"`
	Profile          *Profile              `json:"profile,omitempty"`
	Transactions     *TransactionsSnapshot `json:"transactions,omitempty"`
	DependencyStatus []DependencySummary   `json:"dependency_status"`
}

type CostEstimate struct {
	InputTokens      int     `json:"input_tokens"`
	OutputTokens     int     `json:"output_tokens"`
	EstimatedCostUSD float64 `json:"estimated_cost_usd"`
}

type AgentResponse struct {
	Answer           string       `json:"answer"`
	ReasoningSummary string       `json:"reasoning_summary"`
	Recommendations  []string     `json:"recommendations"`
	Sources          []string     `json:"sources"`
	ToolsUsed        []string     `json:"tools_used"`
	RiskFlags        []string     `json:"risk_flags"`
	FallbackUsed     bool         `json:"fallback_used"`
	CostEstimate     CostEstimate `json:"cost_estimate"`
}

type AssistantResponse struct {
	RequestID    string              `json:"request_id"`
	CustomerID   string              `json:"customer_id"`
	Question     string              `json:"question"`
	Dependencies []DependencySummary `json:"dependencies"`
	Assistant    AgentResponse       `json:"assistant"`
}
