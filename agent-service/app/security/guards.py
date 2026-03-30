from __future__ import annotations

import re
from dataclasses import dataclass, field
from threading import Lock
from typing import Any

from app.models.schemas import Profile, RetrievedChunk, TransactionsSnapshot

PROMPT_VERSION = "v1.0.0"

INJECTION_PATTERNS = [
    re.compile(pattern, re.IGNORECASE)
    for pattern in [
        r"ignore (all|any|previous) instructions",
        r"ignore .*instruc",
        r"system prompt",
        r"prompt do sistema",
        r"developer message",
        r"mensagem do desenvolvedor",
        r"reveal .*prompt",
        r"revele .*prompt",
        r"execute .*tool",
        r"execute .*ferramenta",
        r"change .*policy",
        r"altere .*politica",
        r"desconsidere .*instruc",
    ]
]


def sanitize_text(text: str, max_chars: int = 600) -> str:
    normalized = " ".join(text.replace("\n", " ").split())
    return normalized[:max_chars]


def detect_prompt_injection(text: str) -> list[str]:
    flags: list[str] = []
    for pattern in INJECTION_PATTERNS:
        if pattern.search(text):
            flags.append("prompt_injection_detected")
            break
    return flags


def redact_pii(text: str) -> str:
    text = re.sub(r"[\w\.-]+@[\w\.-]+", "[redacted-email]", text)
    text = re.sub(r"\b\d{10,16}\b", "[redacted-number]", text)
    return text


def filter_retrieved_chunks(chunks: list[RetrievedChunk]) -> tuple[list[RetrievedChunk], list[str]]:
    safe_chunks: list[RetrievedChunk] = []
    flags: list[str] = []
    for chunk in chunks:
        combined_text = f"{chunk.title}\n{chunk.content}"
        if detect_prompt_injection(combined_text):
            flags.append("kb_prompt_injection_detected")
            continue
        sanitized_content = redact_pii(sanitize_text(chunk.content, max_chars=1200))
        safe_chunks.append(chunk.model_copy(update={"content": sanitized_content}))
    return safe_chunks, list(dict.fromkeys(flags))


def allowed_profile_context(profile: Profile | None) -> dict[str, Any] | None:
    if profile is None:
        return None
    return {
        "customer_id": profile.customer_id,
        "segment": profile.segment,
        "industry": profile.industry,
        "annual_revenue_brl": profile.annual_revenue_brl,
        "kyc_status": profile.kyc_status,
        "risk_tier": profile.risk_tier,
        "degraded": profile.degraded,
    }


def allowed_transactions_context(transactions: TransactionsSnapshot | None) -> dict[str, Any] | None:
    if transactions is None:
        return None
    return {
        "customer_id": transactions.customer_id,
        "current_balance_brl": transactions.current_balance_brl,
        "average_monthly_inflow_brl": transactions.average_monthly_inflow_brl,
        "average_monthly_outflow_brl": transactions.average_monthly_outflow_brl,
        "overdraft_usage_days": transactions.overdraft_usage_days,
        "late_payment_events": transactions.late_payment_events,
        "top_categories": transactions.top_categories,
        "recent_transactions": [
            {
                "date": item.date.isoformat(),
                "type": item.type,
                "amount_brl": item.amount_brl,
                "category": item.category,
            }
            for item in transactions.recent_transactions[:5]
        ],
        "degraded": transactions.degraded,
    }


@dataclass
class CustomerBudgetTracker:
    max_cost_usd: float
    max_requests: int
    request_count: dict[str, int] = field(default_factory=dict)
    accumulated_cost: dict[str, float] = field(default_factory=dict)
    lock: Lock = field(default_factory=Lock, repr=False)

    def check(self, customer_id: str) -> tuple[bool, str | None]:
        with self.lock:
            if self.request_count.get(customer_id, 0) >= self.max_requests:
                return False, "request_budget_exceeded"
            if self.accumulated_cost.get(customer_id, 0.0) >= self.max_cost_usd:
                return False, "cost_budget_exceeded"
            return True, None

    def register(self, customer_id: str, estimated_cost: float) -> None:
        with self.lock:
            self.request_count[customer_id] = self.request_count.get(customer_id, 0) + 1
            self.accumulated_cost[customer_id] = self.accumulated_cost.get(customer_id, 0.0) + estimated_cost


def enforce_response_guardrails(answer: str, reasoning_summary: str) -> tuple[str, list[str]]:
    violations: list[str] = []
    sanitized = answer
    if "system prompt" in answer.lower():
        violations.append("prompt_leakage")
        sanitized = "Nao posso expor instrucoes internas. Segue uma resposta segura e focada no seu contexto financeiro."
    if len(sanitized) > 1400:
        violations.append("answer_too_long")
        sanitized = sanitized[:1397] + "..."
    if len(reasoning_summary) > 500:
        violations.append("reasoning_summary_too_long")
    return sanitized, violations
