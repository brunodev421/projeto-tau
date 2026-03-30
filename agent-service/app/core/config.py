from __future__ import annotations

import os
from dataclasses import dataclass
from pathlib import Path


@dataclass(frozen=True)
class Settings:
    service_name: str
    environment: str
    port: int
    llm_mode: str
    model_name: str
    openai_api_key: str
    openai_base_url: str
    langfuse_public_key: str
    langfuse_secret_key: str
    langfuse_host: str
    rag_top_k: int
    rag_score_threshold: float
    rag_embedding_mode: str
    rag_embedding_model: str
    customer_budget_max_cost_usd: float
    customer_budget_max_requests: int
    otlp_endpoint: str
    log_level: str
    repo_root: Path
    knowledge_base_dir: Path
    processed_dir: Path


def get_settings() -> Settings:
    repo_root = Path(__file__).resolve().parents[3]
    kb_dir = repo_root / "knowledge-base"
    processed_dir = kb_dir / "processed"
    processed_dir.mkdir(parents=True, exist_ok=True)
    return Settings(
        service_name=os.getenv("AGENT_SERVICE_NAME", "agent-service"),
        environment=os.getenv("ENVIRONMENT", "local"),
        port=int(os.getenv("AGENT_PORT", "8090")),
        llm_mode=os.getenv("AGENT_LLM_MODE", "local"),
        model_name=os.getenv("AGENT_MODEL_NAME", "gpt-4.1-mini"),
        openai_api_key=os.getenv("OPENAI_API_KEY", ""),
        openai_base_url=os.getenv("OPENAI_BASE_URL", ""),
        langfuse_public_key=os.getenv("LANGFUSE_PUBLIC_KEY", ""),
        langfuse_secret_key=os.getenv("LANGFUSE_SECRET_KEY", ""),
        langfuse_host=os.getenv("LANGFUSE_HOST", ""),
        rag_top_k=int(os.getenv("RAG_TOP_K", "3")),
        rag_score_threshold=float(os.getenv("RAG_SCORE_THRESHOLD", "0.20")),
        rag_embedding_mode=os.getenv("RAG_EMBEDDING_MODE", "local"),
        rag_embedding_model=os.getenv("RAG_EMBEDDING_MODEL", "text-embedding-3-small"),
        customer_budget_max_cost_usd=float(os.getenv("CUSTOMER_BUDGET_MAX_COST_USD", "0.25")),
        customer_budget_max_requests=int(os.getenv("CUSTOMER_BUDGET_MAX_REQUESTS", "20")),
        otlp_endpoint=os.getenv("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
        log_level=os.getenv("LOG_LEVEL", "INFO"),
        repo_root=repo_root,
        knowledge_base_dir=kb_dir / "documents",
        processed_dir=processed_dir,
    )
