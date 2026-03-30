from __future__ import annotations

from fastapi import APIRouter, HTTPException, Request, Response, status

from app.models.schemas import AgentRequest, AgentResponse
from app.observability.metrics import FALLBACK_COUNT, MetricsHelper, metrics_response


router = APIRouter()


@router.get("/healthz")
def healthz() -> dict[str, str]:
    return {"status": "ok"}


@router.get("/readyz")
def readyz(request: Request) -> dict[str, str]:
    if request.app.state.workflow is None:
        raise HTTPException(status_code=status.HTTP_503_SERVICE_UNAVAILABLE, detail="workflow not ready")
    return {"status": "ready"}


@router.get("/metrics")
def metrics() -> Response:
    payload, content_type = metrics_response()
    return Response(content=payload, media_type=content_type)


@router.post("/v1/agent/analyze", response_model=AgentResponse)
def analyze(request_model: AgentRequest, request: Request) -> AgentResponse:
    workflow = request.app.state.workflow
    metrics_helper: MetricsHelper = request.app.state.metrics_helper
    logger = request.app.state.logger
    with request.app.state.langfuse.trace(
        request_id=request_model.request_id,
        customer_id=request_model.customer_id,
        input_payload=request_model.model_dump(),
    ) as trace:
        with metrics_helper.time_request("/v1/agent/analyze"):
            try:
                result = workflow.invoke({"request": request_model})
                response = result["response"]
            except Exception as exc:
                logger.exception(
                    "workflow execution failed; serving guarded fallback",
                    extra={"request_id": request_model.request_id, "customer_id": request_model.customer_id},
                )
                FALLBACK_COUNT.labels(reason="workflow_exception").inc()
                response = AgentResponse(
                    answer="Resposta conservadora: nao foi possivel concluir a analise automatizada com seguranca total. Considere revisar novamente com dados atualizados.",
                    reasoning_summary="Fallback interno do agent acionado apos falha inesperada no workflow.",
                    recommendations=[
                        "Validar novamente perfil e transacoes antes de seguir com decisao de credito.",
                        "Usar esta resposta apenas como orientacao conservadora temporaria.",
                    ],
                    sources=["fallback://agent-service"],
                    tools_used=["agent_service_internal_fallback"],
                    risk_flags=["agent_workflow_error"],
                    fallback_used=True,
                )
    if trace is not None:
        trace.update(output=response.model_dump())
    return response
