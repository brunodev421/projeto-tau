# AI Assistant PJ Monorepo

Solução completa para uma experiência conversacional PJ com:

- `bfa-go`: BFA em Go com resiliência, cache, métricas, tracing e integração com o agente.
- `agent-service`: serviço Python com FastAPI + LangGraph, tools explícitas, guardrails e RAG funcional.
- `mock-services`: simuladores de `Profile API` e `Transactions API` com cenários de falha.
- `knowledge-base`: base de conhecimento fictícia e realista para o RAG.
- `docs`: diagramas Mermaid e notas arquiteturais.
- `scripts`: bootstrap, execução local e avaliação.

## High Level

```text
.
├── README.md
├── docker-compose.yml
├── go.work
├── bfa-go/
├── agent-service/
├── mock-services/
├── knowledge-base/
├── docs/
├── infra/
├── scripts/
└── tests/
```

Arquitetura detalhada e diagramas em [docs/architecture.md](/Users/bruno/Downloads/projeto%20Itau/docs/architecture.md).

## Main Decisions

| Camada | Escolha | Por que |
| --- | --- | --- |
| BFA | Go + `chi` | Stack enxuta, boa performance, middlewares simples e excelente fit para BFF/BFA. |
| Resiliência | `context`, retry exponencial, `sony/gobreaker`, bulkhead com `semaphore` | Cobertura objetiva dos requisitos sem overengineering. |
| Observabilidade BFA | Prometheus + OpenTelemetry | Métricas e tracing padrão de mercado, fáceis de levar para produção. |
| Agent Service | FastAPI + LangGraph | FastAPI simplifica operação HTTP; LangGraph deixa o workflow do agente explícito e testável. |
| Tools | `langchain-core` tools | Ferramentas declarativas, fáceis de instrumentar e coerentes com LangGraph. |
| RAG | Vetor local transparente + threshold + deduplicação + tags | Reproduzível localmente, simples para take-home e robusto contra contexto fraco. |
| Embeddings | `text-embedding-3-small` opcional via OpenAI; fallback local `TF-IDF` | Permite embeddings densos em ambiente com credenciais, mantendo execução offline por padrão. |
| LLM | OpenAI opcional; modo `local` default | O projeto roda sem chave, mas fica pronto para um provider real. |
| Observabilidade Agent | Prometheus + OTEL + LangFuse opcional | Métricas operacionais locais e caminho claro para tracing de LLM em produção. |

## Features Delivered

- `GET /v1/assistant/{customerId}` no BFA.
- chamadas paralelas para `Profile API` e `Transactions API`.
- timeout e cancelamento propagados por `context`.
- retry com backoff exponencial apenas para falhas transitórias.
- circuit breaker por dependência.
- bulkhead para saturação.
- cache TTL em memória para perfil.
- erros estruturados e sem vazamento de detalhes internos.
- `/healthz`, `/readyz`, `/metrics` em BFA e Agent Service.
- logs JSON estruturados no BFA.
- tracing OTEL entre BFA e Agent via `otelhttp`.
- LangGraph com fluxo explícito `input_normalizer -> planner -> structured -> retrieve_knowledge -> evaluate_relevance -> policy -> synthesize -> validate -> finalize`.
- tools obrigatórias implementadas.
- RAG funcional com chunking, vetor local, score, threshold, top-k, deduplicação, tags e rejeição de contexto fraco.
- guardrails contra prompt injection, redução de PII, contexto permitido, budget por `customerId` e fallback conservador.
- testes unitários e de integração.
- módulo simples de avaliação com dataset e script.

## Running Locally

### Prerequisites

- Python 3.9+
- `curl`
- opcional: Docker/Docker Compose

Observação: o host usado para construir este projeto não tinha `go` nem `docker` instalados. Por isso o bootstrap instala um Go local em `.tooling/go` e o README também traz o caminho sem Docker.

### 1. Bootstrap

```bash
./scripts/bootstrap.sh
```

Isso faz:

- cria `.venv`
- instala dependências Python
- instala um Go local em `.tooling/go` se necessário

### 2. Rodar sem Docker

Em três terminais:

```bash
./scripts/run_mock_services.sh
```

```bash
./scripts/run_agent.sh
```

```bash
./scripts/run_bfa.sh
```

Serviços esperados:

- BFA: `http://localhost:8080`
- Mock services: `http://localhost:8081`
- Agent Service: `http://localhost:8090`

### 3. Rodar com Docker Compose

```bash
docker compose up --build
```

## Environment Variables

Use [.env.example](/Users/bruno/Downloads/projeto%20Itau/.env.example) como referência.

Variáveis mais importantes:

- `BFA_PROFILE_API_URL`, `BFA_TRANSACTIONS_API_URL`, `BFA_AGENT_SERVICE_URL`
- `BFA_DOWNSTREAM_TIMEOUT`, `BFA_PROFILE_CACHE_TTL`
- `AGENT_LLM_MODE=local|openai`
- `OPENAI_API_KEY`
- `RAG_EMBEDDING_MODE=local|openai`
- `RAG_EMBEDDING_MODEL=text-embedding-3-small`
- `RAG_TOP_K`, `RAG_SCORE_THRESHOLD`
- `CUSTOMER_BUDGET_MAX_REQUESTS`, `CUSTOMER_BUDGET_MAX_COST_USD`
- `LANGFUSE_PUBLIC_KEY`, `LANGFUSE_SECRET_KEY`, `LANGFUSE_HOST`
- `OTEL_EXPORTER_OTLP_ENDPOINT`

## API

### BFA

```http
GET /v1/assistant/{customerId}?question=...&scenario=...
```

`scenario` é opcional e existe para simular falhas via mock:

- `profile_timeout`
- `transactions_timeout`
- `profile_error`
- `transactions_error`
- `profile_partial`
- `transactions_partial`
- `degraded`

Exemplo:

```bash
curl --request GET \
  'http://localhost:8080/v1/assistant/pj-risk?question=Tenho%20risco%20no%20fluxo%20de%20caixa%3F%20Vale%20buscar%20credito%20agora%3F' \
  --header 'X-Request-ID: demo-123'
```

Resposta exemplo:

```json
{
  "request_id": "demo-123",
  "customer_id": "pj-risk",
  "question": "Tenho risco no fluxo de caixa? Vale buscar credito agora?",
  "dependencies": [
    {
      "name": "profile_api",
      "status": "ok",
      "source": "network",
      "latency_ms": 12
    },
    {
      "name": "transactions_api",
      "status": "ok",
      "source": "network",
      "latency_ms": 18
    }
  ],
  "assistant": {
    "answer": "O contexto financeiro indica pressao de caixa, com margem mensal estimada em -15000.00 BRL. A prioridade deve ser proteger liquidez, revisar concentracoes de saida e avaliar elegibilidade antes de buscar novas linhas. Evidencias relevantes da base: FAQ de Fluxo de Caixa PJ (score=0.2525).",
    "reasoning_summary": "Profile status: available. Transactions status: available. Cashflow health: risk. Knowledge base used with 1 chunks above threshold. Risk flags: cashflow_risk, late_payments, negative_cashflow, overdraft_usage, risk_tier_high.",
    "recommendations": [
      "Priorizar reforco de caixa e revisar compromissos de curto prazo antes de solicitar novas linhas.",
      "Aplicar a politica de reserva minima de caixa sugerida na base de conhecimento."
    ],
    "sources": [
      "kb://faq-fluxo-caixa#c6810c32348e"
    ],
    "tools_used": [
      "StructuredProfileTool",
      "StructuredTransactionsTool",
      "KnowledgeBaseRAGTool",
      "PolicyOrRecommendationTool",
      "OptionalFinalValidatorTool"
    ],
    "risk_flags": [
      "risk_tier_high",
      "negative_cashflow",
      "overdraft_usage",
      "late_payments",
      "cashflow_risk"
    ],
    "fallback_used": false,
    "cost_estimate": {
      "input_tokens": 434,
      "output_tokens": 224,
      "estimated_cost_usd": 0.0
    }
  }
}
```

### Agent Service

```http
POST /v1/agent/analyze
```

Payload: o BFA envia `customer_id`, `question`, perfil, transações e status das dependências.

## Agent Workflow

Resumo:

1. `input_normalizer`
   sanitiza entrada, detecta prompt injection, reduz PII, filtra contexto permitido e checa budget.
2. `planner`
   define plano multi-step e se o KB/validator serão usados.
3. `retrieve_structured_data`
   usa `StructuredProfileTool` e `StructuredTransactionsTool`.
4. `retrieve_knowledge`
   executa `KnowledgeBaseRAGTool`.
5. `evaluate_relevance`
   rejeita contexto fraco com threshold mínimo.
6. `policy_recommendations`
   usa `PolicyOrRecommendationTool`.
7. `synthesize`
   consolida resposta final.
8. `validate`
   usa `OptionalFinalValidatorTool` e guardrails.
9. `finalize`
   monta o JSON final e calcula custo estimado.

## RAG

### Knowledge base

Arquivos em [knowledge-base/documents](/Users/bruno/Downloads/projeto%20Itau/knowledge-base/documents):

- políticas de crédito PJ
- FAQ de fluxo de caixa
- elegibilidade para produtos
- política interna de risco operacional
- FAQ de KYC/documentação
- recomendações de saúde financeira PJ

### Chunking strategy

- chunk por parágrafos
- alvo de `~700` caracteres
- overlap de `~120` caracteres
- metadata por chunk com `document_id`, `title`, `source_file` e `tags`

### Retrieval strategy

- `top-k` configurável (`RAG_TOP_K`, default `3`)
- `score_threshold` configurável (`RAG_SCORE_THRESHOLD`, default `0.20`)
- filtro por tags inferidas da pergunta e do risco estruturado
- deduplicação por documento + prefixo do chunk
- reranking simples por overlap lexical

### Embedding / vector choice

- default local: `TF-IDF` com `scikit-learn`, para execução 100% offline e transparente
- opcional: `text-embedding-3-small` via OpenAI para embeddings densos reais

### Como evitamos contexto irrelevante

- `top-k` pequeno
- threshold mínimo
- deduplicação
- filtros por metadata/tags
- reranking simples
- rejeição explícita com `weak_rag_context` quando a evidência é fraca

## Observability

### BFA metrics

- `bfa_http_request_latency_seconds`
- `bfa_http_request_errors_total`
- `bfa_downstream_calls_total`
- `bfa_retry_attempts_total`
- `bfa_circuit_breaker_events_total`
- `bfa_cache_lookups_total`
- `bfa_bulkhead_saturation_total`

### Agent metrics

- `agent_http_request_latency_seconds`
- `agent_workflow_step_latency_seconds`
- `agent_tool_calls_total`
- `agent_tool_errors_total`
- `agent_model_errors_total`
- `agent_fallback_total`
- `agent_rag_results_total`
- `agent_tokens_total`
- `agent_estimated_cost_usd_total`

### Tracing

- BFA usa OTEL no servidor e nos clients HTTP.
- BFA propaga trace context para o Agent Service.
- Agent Service publica spans FastAPI/ASGI e pode exportar para OTLP.

### LangFuse

Implementado via `LangfuseFacade`:

- se as credenciais estiverem configuradas, o request do agente vira uma trace
- se não estiverem, o projeto continua funcional sem dependência externa

## Security And Governance

- sanitização de entrada com normalização e limitação de tamanho
- detecção de prompt injection por padrões conhecidos
- troca da pergunta original por uma pergunta segura quando a entrada é maliciosa
- contexto permitido separado do contexto bruto
- redaction de PII básica em perguntas
- logs do BFA sem payload sensível
- budget por `customerId` com limite de requests e custo estimado
- fallback conservador quando budget, tools ou validação falham
- guardrails para evitar vazamento de prompt e respostas longas demais
- versionamento de prompt por `PROMPT_VERSION = v1.0.0`
- resposta nunca usa a base recuperada para alterar regras do sistema

## Testing

### Go

```bash
export PATH="$PWD/.tooling/go/bin:$PATH"
cd bfa-go && go test ./...
cd ../mock-services && go test ./...
```

### Python

```bash
source .venv/bin/activate
pytest agent-service/tests -q
```

### Evaluation

```bash
source .venv/bin/activate
python scripts/evaluate_agent.py
```

Cobertura entregue:

- sucesso ponta a ponta do BFA com dependências simuladas
- falha da Profile API
- timeout em dependências
- fallback do BFA quando o agent falha
- retry com falha transitória
- testes unitários de retry, circuit breaker e bulkhead
- workflow do agente
- schema/contrato da resposta do agente
- cenário adversarial com prompt injection
- budget/fallback por cliente
- cenário de RAG sem contexto relevante
- dataset simples de avaliação

## Assumptions

| O que foi assumido | Por que foi necessário | Impacto potencial |
| --- | --- | --- |
| O `question` chega como query param opcional no BFA | O contrato original definia apenas `customerId` no path | Mantive compatibilidade com o endpoint e preservei flexibilidade para o cliente. |
| O provider de LLM default é `local` | O projeto precisava rodar sem depender de chaves externas | Em produção o ideal é habilitar `openai` ou outro provider enterprise. |
| O budget por `customerId` é in-memory | Era preciso demonstrar governança de custo de forma executável localmente | Em produção isso deve ir para Redis ou store distribuído. |
| Os mocks representam Profile API e Transactions API no mesmo serviço | Simplifica a execução local e os cenários de falha controlados | Em produção seriam serviços separados com SLAs e observabilidade independentes. |
| O schema final do agente segue o JSON exigido no enunciado | O desafio já sugeria o contrato alvo | Extensões futuras podem adicionar metadados ou explicações mais ricas. |
| O fallback do BFA responde `200` quando o agent falha mas há contexto suficiente | Privilegiei continuidade operacional e UX | Alguns times prefeririam `206`, `424` ou `503` dependendo do contrato. |

## Trade-offs

| O que foi simplificado | Por que foi escolhido | Alternativa em produção |
| --- | --- | --- |
| RAG local com vetor transparente | Mantém o desafio executável e fácil de inspecionar | `pgvector`, OpenSearch, Pinecone ou Chroma persistente com embeddings densos. |
| Embedding default local `TF-IDF` | Evita dependência obrigatória de API keys ou downloads pesados | `text-embedding-3-small`, `multilingual-e5-small` ou outro embedding model corporativo. |
| Mock services locais | Permitem provar resiliência e degradação de forma determinística | Integrações reais, contract tests e sandbox corporativo. |
| LangFuse opcional | Evita subir uma stack pesada no take-home | Stack completa LangFuse com Postgres/ClickHouse/worker. |
| Budget simples por processo | Demonstra governança de custo com pouco atrito operacional | Rate limiting distribuído + billing + quotas persistidas. |
| Heurística local para synthesize/plan | Garante execução offline | Planner/reviewer com LLM real, prompt registry e experimentação controlada. |

## README Notes By Requirement

- Workflow do agente: implementado em LangGraph, explícito e testável.
- RAG: funcional, com chunking, score, filtros, threshold e rejeição.
- Métricas: Prometheus em ambos os serviços.
- Segurança: guardrails, prompt injection, redaction, allowed context, budget e fallback.
- Documentação executável: scripts, compose, exemplos e testes incluídos.

## How this would evolve in real production

- HA / escalabilidade horizontal: BFA e Agent Service iriam para ECS/EKS com autoscaling por CPU, latência e fila; múltiplas réplicas behind ALB/API Gateway.
- filas e processamento assíncrono: requests síncronos para consultas curtas; avaliações pesadas, reindexação de conhecimento, feature computation e replay de traces em SQS/EventBridge/Kafka.
- rate limiting distribuído: budget e rate limit em Redis/ElastiCache ou API Gateway usage plans, com limites por cliente, canal e produto.
- gestão de segredos: AWS Secrets Manager ou HashiCorp Vault para chaves do provider, LangFuse e OTEL.
- IAM / segurança em cloud: roles por serviço, IRSA em EKS ou task roles em ECS, política de mínimo privilégio e separação entre ambientes.
- observabilidade centralizada: OTEL Collector, Prometheus/Grafana, logs estruturados em CloudWatch/OpenSearch e LangFuse para traces LLM.
- avaliação contínua do agente: dataset versionado, smoke eval em CI, regressão por release, LLM-as-judge para groundedness/relevância e revisão humana amostral.
- versionamento e rollout de prompts: prompt registry com versionamento, feature flags, canary releases e rollback rápido.
- expansão do RAG: assets em S3, pipeline de ingestão incremental, embeddings densos, metadata rica, ACL por documento e reranker cross-encoder.
- governança de dados: classificação de sensibilidade, minimização de payload, criptografia em trânsito e repouso, lineage e políticas de acesso.
- políticas de retenção e auditoria: retention por tipo de dado, trilha de auditoria para prompts/respostas, masking consistente e logs auditáveis.
- MLOps / LLMOps: catálogo de modelos, benchmark por caso de uso, A/B testing, monitoramento de drift, grounding score e custo por jornada.
- custo e FinOps: budgets por cliente e por produto, dashboards de custo/token/tool, cache semântico, batching de embeddings e escolha dinâmica de modelo por criticidade.

## Useful Commands

```bash
make setup
make test
make run-mocks
make run-agent
make run-bfa
make ingest
```

