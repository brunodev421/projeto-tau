from typing import Any, Dict, List, Optional, TypedDict

from app.models.schemas import AgentRequest, AgentResponse, RetrievedChunk, WorkflowPlan


class AgentState(TypedDict, total=False):
    request: AgentRequest
    request_id: str
    customer_id: str
    normalized_question: str
    safe_question: str
    safe_profile: Optional[Dict[str, Any]]
    safe_transactions: Optional[Dict[str, Any]]
    dependency_status: List[Dict[str, Any]]
    prompt_flags: list[str]
    budget_block_reason: Optional[str]
    plan: WorkflowPlan
    plan_usage: Dict[str, int]
    structured_summary: Dict[str, Any]
    kb_results: List[RetrievedChunk]
    kb_rejected: bool
    recommendations: List[str]
    risk_flags: List[str]
    tools_used: List[str]
    sources: List[str]
    draft_answer: str
    reasoning_summary: str
    validator_output: Dict[str, Any]
    response: AgentResponse
    fallback_used: bool
    model_usage: Dict[str, int]
