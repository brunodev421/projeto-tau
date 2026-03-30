FROM python:3.11-slim AS runtime
WORKDIR /app
COPY agent-service/pyproject.toml /app/pyproject.toml
COPY agent-service/README.md /app/README.md
COPY agent-service/app /app/app
COPY knowledge-base /app/knowledge-base
RUN pip install --upgrade pip && pip install .
EXPOSE 8090
CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8090"]

