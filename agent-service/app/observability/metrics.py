from __future__ import annotations

import time
from contextlib import contextmanager
from dataclasses import dataclass

from prometheus_client import Counter, Histogram, generate_latest, CONTENT_TYPE_LATEST


REQUEST_LATENCY = Histogram(
    "agent_http_request_latency_seconds",
    "Latency of agent HTTP requests",
    labelnames=("route",),
)
WORKFLOW_STEP_LATENCY = Histogram(
    "agent_workflow_step_latency_seconds",
    "Latency per LangGraph workflow step",
    labelnames=("step",),
)
TOOL_CALLS = Counter(
    "agent_tool_calls_total",
    "Number of tool invocations",
    labelnames=("tool", "outcome"),
)
TOOL_ERRORS = Counter(
    "agent_tool_errors_total",
    "Tool errors by tool name",
    labelnames=("tool",),
)
MODEL_ERRORS = Counter(
    "agent_model_errors_total",
    "LLM/model errors by mode",
    labelnames=("mode",),
)
FALLBACK_COUNT = Counter(
    "agent_fallback_total",
    "Fallback responses by reason",
    labelnames=("reason",),
)
RAG_RESULTS = Counter(
    "agent_rag_results_total",
    "RAG retrieval outcomes",
    labelnames=("outcome",),
)
TOKENS = Counter(
    "agent_tokens_total",
    "Token usage by direction",
    labelnames=("direction",),
)
ESTIMATED_COST_USD = Counter(
    "agent_estimated_cost_usd_total",
    "Estimated cost in USD",
)


@dataclass
class MetricsHelper:
    @contextmanager
    def time_request(self, route: str):
        start = time.perf_counter()
        try:
            yield
        finally:
            REQUEST_LATENCY.labels(route=route).observe(time.perf_counter() - start)

    @contextmanager
    def time_step(self, step: str):
        start = time.perf_counter()
        try:
            yield
        finally:
            WORKFLOW_STEP_LATENCY.labels(step=step).observe(time.perf_counter() - start)


def metrics_response() -> tuple[bytes, str]:
    return generate_latest(), CONTENT_TYPE_LATEST

