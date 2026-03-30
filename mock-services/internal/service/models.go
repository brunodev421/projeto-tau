package service

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
