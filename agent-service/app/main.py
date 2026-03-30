from __future__ import annotations

from contextlib import asynccontextmanager
import os

from fastapi import FastAPI
from opentelemetry import trace
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor, ConsoleSpanExporter

from app.api.routes import router
from app.core.config import get_settings
from app.graph.workflow import build_workflow
from app.observability.langfuse_client import LangfuseFacade
from app.observability.logging import configure_logging
from app.observability.metrics import MetricsHelper
from app.rag.store import RAGStore
from app.security.guards import CustomerBudgetTracker
from app.services.model_gateway import ModelGateway
from app.services.token_cost import TokenCostEstimator
from app.tools.toolkit import create_tools

_TRACING_CONFIGURED = False


def configure_tracing(service_name: str, endpoint: str) -> None:
    global _TRACING_CONFIGURED
    if _TRACING_CONFIGURED or isinstance(trace.get_tracer_provider(), TracerProvider):
        _TRACING_CONFIGURED = True
        return
    resource = Resource.create({"service.name": service_name})
    provider = TracerProvider(resource=resource)
    exporter = OTLPSpanExporter(endpoint=endpoint) if endpoint else ConsoleSpanExporter(out=open(os.devnull, "w", encoding="utf-8"))
    processor = BatchSpanProcessor(exporter)
    provider.add_span_processor(processor)
    trace.set_tracer_provider(provider)
    _TRACING_CONFIGURED = True


@asynccontextmanager
async def lifespan(app: FastAPI):
    settings = get_settings()
    logger = configure_logging(settings.log_level)
    configure_tracing(settings.service_name, settings.otlp_endpoint)
    rag_store = RAGStore(settings=settings)
    rag_store.load()
    tools = create_tools(rag_store)
    model_gateway = ModelGateway(
        mode=settings.llm_mode,
        model_name=settings.model_name,
        api_key=settings.openai_api_key,
        base_url=settings.openai_base_url,
    )
    metrics_helper = MetricsHelper()
    budget_tracker = CustomerBudgetTracker(
        max_cost_usd=settings.customer_budget_max_cost_usd,
        max_requests=settings.customer_budget_max_requests,
    )
    token_estimator = TokenCostEstimator(model_name=settings.model_name if settings.llm_mode == "openai" else "local")
    app.state.settings = settings
    app.state.logger = logger
    app.state.langfuse = LangfuseFacade.from_settings(settings)
    app.state.metrics_helper = metrics_helper
    app.state.workflow = build_workflow(
        tools=tools,
        model_gateway=model_gateway,
        budget_tracker=budget_tracker,
        token_estimator=token_estimator,
        metrics_helper=metrics_helper,
    )
    yield


app = FastAPI(title="Agent Service", version="0.1.0", lifespan=lifespan)
FastAPIInstrumentor.instrument_app(app)
app.include_router(router)
