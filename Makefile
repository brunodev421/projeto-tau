GO ?= $(PWD)/.tooling/go/bin/go
PYTHON ?= python3

.PHONY: setup install-python format test test-go test-python run-bfa run-agent run-mocks ingest

setup:
	./scripts/bootstrap.sh

install-python:
	$(PYTHON) -m pip install -e './agent-service[dev]'

format:
	cd bfa-go && $(GO) fmt ./...
	cd mock-services && $(GO) fmt ./...
	$(PYTHON) -m compileall agent-service/app

test: test-go test-python

test-go:
	cd bfa-go && $(GO) test ./...
	cd mock-services && $(GO) test ./...

test-python:
	pytest agent-service/tests

run-bfa:
	./scripts/run_bfa.sh

run-agent:
	./scripts/run_agent.sh

run-mocks:
	./scripts/run_mock_services.sh

ingest:
	cd agent-service && $(PYTHON) -m app.rag.ingest
