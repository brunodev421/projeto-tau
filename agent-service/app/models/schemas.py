from __future__ import annotations

from datetime import datetime
from typing import Any

from pydantic import BaseModel, Field, ConfigDict


class Profile(BaseModel):
    customer_id: str
    segment: str | None = None
    company_name: str | None = None
    industry: str | None = None
    annual_revenue_brl: float | None = None
    account_manager: str | None = None
    kyc_status: str | None = None
    risk_tier: str | None = None
    last_updated_at: datetime | None = None
    degraded: bool = False


class Transaction(BaseModel):
    date: datetime
    type: str
    amount_brl: float
    counterpart: str | None = None
    category: str | None = None


class TransactionsSnapshot(BaseModel):
    customer_id: str
    current_balance_brl: float | None = None
    average_monthly_inflow_brl: float | None = None
    average_monthly_outflow_brl: float | None = None
    overdraft_usage_days: int = 0
    late_payment_events: int = 0
    top_categories: list[str] = Field(default_factory=list)
    recent_transactions: list[Transaction] = Field(default_factory=list)
    degraded: bool = False


class DependencyStatus(BaseModel):
    name: str
    status: str
    source: str
    error_code: str | None = None
    error_message: str | None = None
    latency_ms: int | None = None


class AgentRequest(BaseModel):
    model_config = ConfigDict(extra="forbid")

    request_id: str = Field(min_length=3, max_length=128)
    customer_id: str = Field(pattern=r"^[a-zA-Z0-9_-]{3,64}$")
    question: str = Field(min_length=3, max_length=1000)
    profile: Profile | None = None
    transactions: TransactionsSnapshot | None = None
    dependency_status: list[DependencyStatus] = Field(default_factory=list)


class CostEstimate(BaseModel):
    input_tokens: int = 0
    output_tokens: int = 0
    estimated_cost_usd: float = 0.0


class AgentResponse(BaseModel):
    answer: str
    reasoning_summary: str
    recommendations: list[str] = Field(default_factory=list)
    sources: list[str] = Field(default_factory=list)
    tools_used: list[str] = Field(default_factory=list)
    risk_flags: list[str] = Field(default_factory=list)
    fallback_used: bool = False
    cost_estimate: CostEstimate = Field(default_factory=CostEstimate)


class RetrievedChunk(BaseModel):
    chunk_id: str
    document_id: str
    title: str
    content: str
    metadata: dict[str, Any] = Field(default_factory=dict)
    score: float
    rerank_score: float | None = None


class WorkflowPlan(BaseModel):
    plan_steps: list[str]
    use_knowledge_base: bool
    use_validator: bool
    rationale: str


class ValidationResult(BaseModel):
    accepted: bool
    violations: list[str] = Field(default_factory=list)
    sanitized_answer: str
    fallback_used: bool = False

