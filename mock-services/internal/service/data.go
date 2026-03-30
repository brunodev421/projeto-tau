package service

import "time"

type Repository struct{}

func NewRepository() *Repository {
	return &Repository{}
}

func (r *Repository) Profile(customerID string) (Profile, bool) {
	switch customerID {
	case "pj-risk":
		return Profile{
			CustomerID:       customerID,
			Segment:          "corporate",
			CompanyName:      "Metalurgica Horizonte Ltda",
			Industry:         "industria",
			AnnualRevenueBRL: 5400000,
			AccountManager:   "Renata Silva",
			KYCStatus:        "complete",
			RiskTier:         "high",
			LastUpdatedAt:    time.Now().Add(-24 * time.Hour),
		}, true
	case "pj-incomplete":
		return Profile{
			CustomerID:       customerID,
			Segment:          "midmarket",
			CompanyName:      "Distribuidora Sol Nascente",
			Industry:         "varejo",
			AnnualRevenueBRL: 2100000,
			AccountManager:   "Carlos Ferreira",
			KYCStatus:        "pending_documents",
			RiskTier:         "medium",
			LastUpdatedAt:    time.Now().Add(-14 * 24 * time.Hour),
		}, true
	default:
		return Profile{
			CustomerID:       customerID,
			Segment:          "upper_smb",
			CompanyName:      "Atelie Verde Comercio Digital",
			Industry:         "servicos",
			AnnualRevenueBRL: 1200000,
			AccountManager:   "Marina Costa",
			KYCStatus:        "complete",
			RiskTier:         "low",
			LastUpdatedAt:    time.Now().Add(-48 * time.Hour),
		}, true
	}
}

func (r *Repository) Transactions(customerID string) (TransactionsSnapshot, bool) {
	now := time.Now()
	switch customerID {
	case "pj-risk":
		return TransactionsSnapshot{
			CustomerID:            customerID,
			CurrentBalanceBRL:     12000,
			AverageMonthlyInflow:  320000,
			AverageMonthlyOutflow: 355000,
			OverdraftUsageDays:    11,
			LatePaymentEvents:     4,
			TopCategories:         []string{"folha", "fornecedores", "tributos"},
			RecentTransactions: []Transaction{
				{Date: now.Add(-24 * time.Hour), Type: "debit", AmountBRL: 85000, Counterpart: "Fornecedor A", Category: "fornecedores"},
				{Date: now.Add(-48 * time.Hour), Type: "credit", AmountBRL: 45000, Counterpart: "Cliente Enterprise", Category: "recebiveis"},
			},
		}, true
	case "pj-incomplete":
		return TransactionsSnapshot{
			CustomerID:            customerID,
			CurrentBalanceBRL:     86000,
			AverageMonthlyInflow:  180000,
			AverageMonthlyOutflow: 142000,
			OverdraftUsageDays:    0,
			LatePaymentEvents:     0,
			TopCategories:         []string{"marketing", "fornecedores", "logistica"},
			RecentTransactions: []Transaction{
				{Date: now.Add(-8 * time.Hour), Type: "debit", AmountBRL: 6800, Counterpart: "Transportadora", Category: "logistica"},
				{Date: now.Add(-18 * time.Hour), Type: "credit", AmountBRL: 23000, Counterpart: "Marketplace", Category: "receita"},
			},
		}, true
	default:
		return TransactionsSnapshot{
			CustomerID:            customerID,
			CurrentBalanceBRL:     145000,
			AverageMonthlyInflow:  210000,
			AverageMonthlyOutflow: 168000,
			OverdraftUsageDays:    0,
			LatePaymentEvents:     0,
			TopCategories:         []string{"folha", "marketing", "cloud"},
			RecentTransactions: []Transaction{
				{Date: now.Add(-6 * time.Hour), Type: "credit", AmountBRL: 34000, Counterpart: "Cliente Varejo", Category: "receita"},
				{Date: now.Add(-20 * time.Hour), Type: "debit", AmountBRL: 12000, Counterpart: "Folha", Category: "folha"},
			},
		}, true
	}
}
