from __future__ import annotations

from typing import Any, Callable

from langgraph.graph import END, START, StateGraph

from app.graph.state import AgentState
from app.models.schemas import AgentRequest, AgentResponse, CostEstimate, RetrievedChunk, ValidationResult
from app.observability.metrics import ESTIMATED_COST_USD, FALLBACK_COUNT, TOKENS, MetricsHelper
from app.security.guards import (
    CustomerBudgetTracker,
    allowed_profile_context,
    allowed_transactions_context,
    detect_prompt_injection,
    enforce_response_guardrails,
    redact_pii,
    sanitize_text,
)
from app.services.model_gateway import ModelGateway
from app.services.token_cost import TokenCostEstimator
from app.tools.toolkit import chunk_sources


def build_workflow(
    tools: dict[str, Any],
    model_gateway: ModelGateway,
    budget_tracker: CustomerBudgetTracker,
    token_estimator: TokenCostEstimator,
    metrics_helper: MetricsHelper,
) -> Any:
    graph = StateGraph(AgentState)

    def step(name: str, fn: Callable[[AgentState], dict[str, Any]]) -> Callable[[AgentState], dict[str, Any]]:
        def wrapper(state: AgentState) -> dict[str, Any]:
            with metrics_helper.time_step(name):
                return fn(state)

        return wrapper

    def input_normalizer(state: AgentState) -> dict[str, Any]:
        request = state["request"]
        normalized_question = sanitize_text(request.question)
        prompt_flags = detect_prompt_injection(normalized_question)
        safe_question = redact_pii(normalized_question)
        safe_profile = allowed_profile_context(request.profile)
        safe_transactions = allowed_transactions_context(request.transactions)
        allowed, reason = budget_tracker.check(request.customer_id)
        fallback_used = not allowed or bool(prompt_flags)
        if prompt_flags:
            safe_question = "Forneca uma avaliacao segura e conservadora da saude financeira da empresa com base apenas no contexto permitido."
            FALLBACK_COUNT.labels(reason="prompt_injection").inc()
        if not allowed:
            prompt_flags.append(reason or "budget_blocked")
            FALLBACK_COUNT.labels(reason=reason or "budget_blocked").inc()
        return {
            "request_id": request.request_id,
            "customer_id": request.customer_id,
            "normalized_question": normalized_question,
            "safe_question": safe_question,
            "safe_profile": safe_profile,
            "safe_transactions": safe_transactions,
            "dependency_status": [item.model_dump() for item in request.dependency_status],
            "prompt_flags": prompt_flags,
            "budget_block_reason": reason,
            "fallback_used": fallback_used,
            "tools_used": [],
            "sources": [],
            "recommendations": [],
            "risk_flags": list(dict.fromkeys(prompt_flags)),
        }

    def planner(state: AgentState) -> dict[str, Any]:
        plan, usage = model_gateway.plan(state["safe_question"], state.get("safe_profile"), state.get("safe_transactions"))
        return {"plan": plan, "plan_usage": usage}

    def retrieve_structured_data(state: AgentState) -> dict[str, Any]:
        profile_output = tools["StructuredProfileTool"].invoke({"profile": state.get("safe_profile")})
        transactions_output = tools["StructuredTransactionsTool"].invoke({"transactions": state.get("safe_transactions")})
        risk_flags = list(dict.fromkeys(state["risk_flags"] + profile_output.get("risk_flags", []) + transactions_output.get("risk_flags", [])))
        tools_used = state["tools_used"] + ["StructuredProfileTool", "StructuredTransactionsTool"]
        structured_summary = {
            **profile_output,
            **transactions_output,
            "dependency_status": state["dependency_status"],
        }
        return {"structured_summary": structured_summary, "risk_flags": risk_flags, "tools_used": tools_used}

    def retrieve_knowledge(state: AgentState) -> dict[str, Any]:
        tags = infer_tags(state["safe_question"], state["structured_summary"])
        kb_payload = tools["KnowledgeBaseRAGTool"].invoke({"query": state["safe_question"], "tags": tags})
        kb_results = [RetrievedChunk.model_validate(item) for item in kb_payload]
        sources = state["sources"] + chunk_sources(kb_results)
        return {"kb_results": kb_results, "sources": sources, "tools_used": state["tools_used"] + ["KnowledgeBaseRAGTool"]}

    def evaluate_relevance(state: AgentState) -> dict[str, Any]:
        kb_results = state.get("kb_results", [])
        if not kb_results:
            return {"kb_rejected": True, "risk_flags": list(dict.fromkeys(state["risk_flags"] + ["weak_rag_context"]))}
        strong_hits = [item for item in kb_results if (item.rerank_score or item.score) >= 0.22]
        if not strong_hits:
            return {"kb_rejected": True, "risk_flags": list(dict.fromkeys(state["risk_flags"] + ["weak_rag_context"]))}
        return {"kb_rejected": False}

    def policy_recommendations(state: AgentState) -> dict[str, Any]:
        rag_context = [] if state.get("kb_rejected") else [item.model_dump() for item in state.get("kb_results", [])]
        policy_output = tools["PolicyOrRecommendationTool"].invoke(
            {"structured_summary": state["structured_summary"], "rag_context": rag_context}
        )
        recommendations = list(dict.fromkeys(state["recommendations"] + policy_output.get("recommendations", [])))
        risk_flags = list(dict.fromkeys(state["risk_flags"] + policy_output.get("risk_flags", [])))
        return {"recommendations": recommendations[:5], "risk_flags": risk_flags, "tools_used": state["tools_used"] + ["PolicyOrRecommendationTool"]}

    def synthesize(state: AgentState) -> dict[str, Any]:
        structured_summary = state["structured_summary"]
        kb_results = [] if state.get("kb_rejected") else state.get("kb_results", [])
        fallback_used = state.get("fallback_used", False) or bool(state.get("budget_block_reason"))
        draft_answer = build_draft_answer(state["safe_question"], structured_summary, state["recommendations"], kb_results, fallback_used)
        reasoning_summary = build_reasoning_summary(structured_summary, kb_results, state["risk_flags"])
        llm_output = model_gateway.synthesize(
            state["safe_question"],
            {
                "question": state["safe_question"],
                "structured_summary": structured_summary,
                "recommendations": state["recommendations"],
                "risk_flags": state["risk_flags"],
                "kb_evidence": [item.model_dump() for item in kb_results],
                "draft_answer": draft_answer,
            },
        )
        return {
            "draft_answer": llm_output.text,
            "reasoning_summary": reasoning_summary,
            "model_usage": llm_output.usage,
        }

    def validate(state: AgentState) -> dict[str, Any]:
        validator_output = tools["OptionalFinalValidatorTool"].invoke(
            {"answer": state["draft_answer"], "reasoning_summary": state["reasoning_summary"]}
        )
        sanitized_answer, guardrail_violations = enforce_response_guardrails(state["draft_answer"], state["reasoning_summary"])
        violations = list(dict.fromkeys(validator_output.get("violations", []) + guardrail_violations))
        fallback_used = state.get("fallback_used", False)
        if violations:
            fallback_used = True
            FALLBACK_COUNT.labels(reason="validator_guardrail").inc()
        return {
            "validator_output": ValidationResult(
                accepted=validator_output.get("accepted", True) and not violations,
                violations=violations,
                sanitized_answer=sanitized_answer,
                fallback_used=fallback_used,
            ).model_dump(),
            "tools_used": state["tools_used"] + ["OptionalFinalValidatorTool"],
            "fallback_used": fallback_used,
            "risk_flags": list(dict.fromkeys(state["risk_flags"] + violations)),
        }

    def finalize(state: AgentState) -> dict[str, Any]:
        answer = state["validator_output"]["sanitized_answer"]
        if state["fallback_used"]:
            answer = prepend_fallback_prefix(answer)
        model_input = state["safe_question"] + str(state.get("structured_summary", {})) + str([item.model_dump() for item in state.get("kb_results", [])])
        input_tokens, output_tokens, estimated_cost = token_estimator.estimate(model_input, answer)
        input_tokens += state.get("plan_usage", {}).get("input_tokens", 0) + state.get("model_usage", {}).get("input_tokens", 0)
        output_tokens += state.get("plan_usage", {}).get("output_tokens", 0) + state.get("model_usage", {}).get("output_tokens", 0)
        TOKENS.labels(direction="input").inc(input_tokens)
        TOKENS.labels(direction="output").inc(output_tokens)
        ESTIMATED_COST_USD.inc(estimated_cost)
        budget_tracker.register(state["customer_id"], estimated_cost)

        response = AgentResponse(
            answer=answer,
            reasoning_summary=state["reasoning_summary"],
            recommendations=state["recommendations"],
            sources=state["sources"] or ["fallback://no-kb"],
            tools_used=state["tools_used"],
            risk_flags=state["risk_flags"],
            fallback_used=state["fallback_used"],
            cost_estimate=CostEstimate(
                input_tokens=input_tokens,
                output_tokens=output_tokens,
                estimated_cost_usd=estimated_cost,
            ),
        )
        return {"response": response}

    graph.add_node("input_normalizer", step("input_normalizer", input_normalizer))
    graph.add_node("planner", step("planner", planner))
    graph.add_node("retrieve_structured_data", step("retrieve_structured_data", retrieve_structured_data))
    graph.add_node("retrieve_knowledge", step("retrieve_knowledge", retrieve_knowledge))
    graph.add_node("evaluate_relevance", step("evaluate_relevance", evaluate_relevance))
    graph.add_node("policy_recommendations", step("policy_recommendations", policy_recommendations))
    graph.add_node("synthesize", step("synthesize", synthesize))
    graph.add_node("validate", step("validate", validate))
    graph.add_node("finalize", step("finalize", finalize))

    graph.add_edge(START, "input_normalizer")
    graph.add_edge("input_normalizer", "planner")
    graph.add_edge("planner", "retrieve_structured_data")
    graph.add_conditional_edges(
        "retrieve_structured_data",
        lambda state: "knowledge" if state["plan"].use_knowledge_base else "skip",
        {"knowledge": "retrieve_knowledge", "skip": "policy_recommendations"},
    )
    graph.add_edge("retrieve_knowledge", "evaluate_relevance")
    graph.add_conditional_edges(
        "evaluate_relevance",
        lambda state: "policy" if not state.get("kb_rejected") else "policy",
        {"policy": "policy_recommendations"},
    )
    graph.add_edge("policy_recommendations", "synthesize")
    graph.add_edge("synthesize", "validate")
    graph.add_edge("validate", "finalize")
    graph.add_edge("finalize", END)
    return graph.compile()


def infer_tags(question: str, structured_summary: dict[str, Any]) -> list[str]:
    tags: list[str] = []
    lower_question = question.lower()
    if any(keyword in lower_question for keyword in ["credito", "capital de giro", "limite"]):
        tags.append("credito")
    if any(keyword in lower_question for keyword in ["caixa", "fluxo", "margem"]):
        tags.append("fluxo-caixa")
    if structured_summary.get("risk_flags"):
        tags.append("risco")
    if "kyc_pending" in structured_summary.get("risk_flags", []):
        tags.append("compliance")
    return list(dict.fromkeys(tags))


def build_draft_answer(question: str, structured_summary: dict[str, Any], recommendations: list[str], kb_results: list[RetrievedChunk], fallback_used: bool) -> str:
    health = structured_summary.get("cashflow_health", "unknown")
    margin = structured_summary.get("monthly_margin_brl")
    profile_status = structured_summary.get("profile_status")
    supporting_evidence = ""
    if kb_results:
        top_evidence = "; ".join(f"{item.title} (score={item.rerank_score or item.score})" for item in kb_results[:2])
        supporting_evidence = f" Evidencias relevantes da base: {top_evidence}."
    if fallback_used:
        return (
            "A analise foi concluida em modo conservador por indisponibilidade parcial de contexto ou restricao de budget. "
            f"Pergunta original: {question}. Considere validar a decisao com dados atualizados.{supporting_evidence}"
        )
    if health == "risk":
        return (
            f"O contexto financeiro indica pressao de caixa, com margem mensal estimada em {margin:.2f} BRL. "
            "A prioridade deve ser proteger liquidez, revisar concentracoes de saida e avaliar elegibilidade antes de buscar novas linhas."
            + supporting_evidence
        )
    if health == "attention":
        return (
            f"Ha sinais de atencao no fluxo financeiro, com margem mensal estimada em {margin:.2f} BRL. "
            "Vale acompanhar recebimentos e saidas com maior cadencia, reforcar previsibilidade e usar politicas de credito com cautela."
            + supporting_evidence
        )
    if profile_status == "missing":
        return "O perfil do cliente esta incompleto para uma recomendacao assertiva. A orientacao mais segura e concluir o cadastro e revisar a documentacao antes de qualquer oferta."
    return (
        f"A empresa apresenta sinais de operacao saudavel, com margem mensal estimada em {margin:.2f} BRL. "
        "Existe espaco para planejamento mais proativo de capital de giro, desde que a politica de elegibilidade continue atendida."
        + supporting_evidence
    )


def build_reasoning_summary(structured_summary: dict[str, Any], kb_results: list[RetrievedChunk], risk_flags: list[str]) -> str:
    parts = [
        f"Profile status: {structured_summary.get('profile_status', 'unknown')}.",
        f"Transactions status: {structured_summary.get('transactions_status', 'unknown')}.",
        f"Cashflow health: {structured_summary.get('cashflow_health', 'unknown')}.",
    ]
    if kb_results:
        parts.append(f"Knowledge base used with {len(kb_results)} chunks above threshold.")
    else:
        parts.append("Knowledge base not used or rejected due to weak relevance.")
    if risk_flags:
        parts.append(f"Risk flags: {', '.join(sorted(set(risk_flags)))}.")
    return " ".join(parts)


def prepend_fallback_prefix(answer: str) -> str:
    if answer.startswith("Resposta conservadora:"):
        return answer
    return f"Resposta conservadora: {answer}"
