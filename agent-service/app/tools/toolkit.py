from __future__ import annotations

from typing import Any

from langchain_core.tools import tool
from pydantic import BaseModel, Field

from app.models.schemas import RetrievedChunk
from app.observability.metrics import TOOL_CALLS, TOOL_ERRORS
from app.rag.store import RAGStore


class StructuredProfileInput(BaseModel):
    profile: dict[str, Any] | None = None


class StructuredTransactionsInput(BaseModel):
    transactions: dict[str, Any] | None = None


class KBInput(BaseModel):
    query: str
    tags: list[str] = Field(default_factory=list)


class RecommendationInput(BaseModel):
    structured_summary: dict[str, Any]
    rag_context: list[dict[str, Any]] = Field(default_factory=list)


class ValidationInput(BaseModel):
    answer: str
    reasoning_summary: str


def create_tools(rag_store: RAGStore):
    @tool("StructuredProfileTool", args_schema=StructuredProfileInput)
    def structured_profile_tool(profile: dict[str, Any] | None = None) -> dict[str, Any]:
        """Summarize profile risk and data completeness signals."""
        TOOL_CALLS.labels(tool="StructuredProfileTool", outcome="success").inc()
        if not profile:
            return {"profile_status": "missing", "risk_flags": ["profile_missing"]}
        health_notes: list[str] = []
        risk_flags: list[str] = []
        if profile.get("kyc_status") not in {None, "", "complete"}:
            risk_flags.append("kyc_pending")
            health_notes.append("Cadastro com pendencias pode restringir ofertas e limites.")
        if profile.get("risk_tier") in {"high", "medium"}:
            risk_flags.append(f"risk_tier_{profile.get('risk_tier')}")
        return {
            "profile_status": "available",
            "segment": profile.get("segment"),
            "industry": profile.get("industry"),
            "annual_revenue_brl": profile.get("annual_revenue_brl"),
            "risk_flags": risk_flags,
            "health_notes": health_notes,
        }

    @tool("StructuredTransactionsTool", args_schema=StructuredTransactionsInput)
    def structured_transactions_tool(transactions: dict[str, Any] | None = None) -> dict[str, Any]:
        """Summarize transaction-derived liquidity and cash-flow signals."""
        TOOL_CALLS.labels(tool="StructuredTransactionsTool", outcome="success").inc()
        if not transactions:
            return {"transactions_status": "missing", "risk_flags": ["transactions_missing"]}
        inflow = transactions.get("average_monthly_inflow_brl") or 0.0
        outflow = transactions.get("average_monthly_outflow_brl") or 0.0
        balance = transactions.get("current_balance_brl") or 0.0
        overdraft_days = transactions.get("overdraft_usage_days") or 0
        late_events = transactions.get("late_payment_events") or 0
        margin = inflow - outflow
        health = "healthy" if margin >= 0 and overdraft_days == 0 and late_events == 0 else "attention"
        if balance < outflow * 0.25:
            health = "risk"
        risk_flags: list[str] = []
        if margin < 0:
            risk_flags.append("negative_cashflow")
        if overdraft_days > 0:
            risk_flags.append("overdraft_usage")
        if late_events > 0:
            risk_flags.append("late_payments")
        return {
            "transactions_status": "available",
            "balance_brl": balance,
            "monthly_margin_brl": margin,
            "cashflow_health": health,
            "top_categories": transactions.get("top_categories", []),
            "risk_flags": risk_flags,
        }

    @tool("KnowledgeBaseRAGTool", args_schema=KBInput)
    def knowledge_base_tool(query: str, tags: list[str] | None = None) -> list[dict[str, Any]]:
        """Retrieve relevant knowledge-base chunks for the current financial question."""
        try:
            results = rag_store.search(query=query, tags=tags or [])
            TOOL_CALLS.labels(tool="KnowledgeBaseRAGTool", outcome="success").inc()
            return [result.model_dump() for result in results]
        except Exception:
            TOOL_ERRORS.labels(tool="KnowledgeBaseRAGTool").inc()
            TOOL_CALLS.labels(tool="KnowledgeBaseRAGTool", outcome="error").inc()
            return []

    @tool("PolicyOrRecommendationTool", args_schema=RecommendationInput)
    def policy_tool(structured_summary: dict[str, Any], rag_context: list[dict[str, Any]] | None = None) -> dict[str, Any]:
        """Generate recommendations grounded on structured signals and retrieved policies."""
        TOOL_CALLS.labels(tool="PolicyOrRecommendationTool", outcome="success").inc()
        rag_context = rag_context or []
        recommendations: list[str] = []
        risk_flags: list[str] = []
        if structured_summary.get("cashflow_health") == "risk":
            recommendations.append("Priorizar reforco de caixa e revisar compromissos de curto prazo antes de solicitar novas linhas.")
            risk_flags.append("cashflow_risk")
        elif structured_summary.get("cashflow_health") == "attention":
            recommendations.append("Acompanhar fluxo de caixa semanalmente e renegociar saidas concentradas.")
        else:
            recommendations.append("Fluxo de caixa indica espaco para planejamento de capital de giro ou reserva operacional.")

        for chunk in rag_context[:2]:
            title = chunk.get("title", "base-de-conhecimento")
            if "credito" in title.lower():
                recommendations.append("Validar criterios de elegibilidade de credito antes de submeter proposta.")
            if "fluxo" in title.lower():
                recommendations.append("Aplicar a politica de reserva minima de caixa sugerida na base de conhecimento.")

        return {"recommendations": recommendations[:4], "risk_flags": risk_flags}

    @tool("OptionalFinalValidatorTool", args_schema=ValidationInput)
    def validator_tool(answer: str, reasoning_summary: str) -> dict[str, Any]:
        """Validate the final answer for unsafe patterns and formatting issues."""
        TOOL_CALLS.labels(tool="OptionalFinalValidatorTool", outcome="success").inc()
        violations: list[str] = []
        if "ignore" in answer.lower():
            violations.append("instruction_echo")
        if len(answer) > 1400:
            violations.append("answer_too_long")
        return {"accepted": not violations, "violations": violations}

    return {
        "StructuredProfileTool": structured_profile_tool,
        "StructuredTransactionsTool": structured_transactions_tool,
        "KnowledgeBaseRAGTool": knowledge_base_tool,
        "PolicyOrRecommendationTool": policy_tool,
        "OptionalFinalValidatorTool": validator_tool,
    }


def chunk_sources(chunks: list[RetrievedChunk]) -> list[str]:
    return [f"kb://{chunk.document_id}#{chunk.chunk_id}" for chunk in chunks]
