#!/usr/bin/env python3
from __future__ import annotations

import json
from pathlib import Path

from fastapi.testclient import TestClient

from app.main import app


def base_payload(customer_id: str, question: str) -> dict:
    profile = {
        "customer_id": customer_id,
        "segment": "upper_smb",
        "industry": "servicos",
        "annual_revenue_brl": 1200000,
        "kyc_status": "complete",
        "risk_tier": "low",
    }
    transactions = {
        "customer_id": customer_id,
        "current_balance_brl": 120000,
        "average_monthly_inflow_brl": 200000,
        "average_monthly_outflow_brl": 150000,
        "overdraft_usage_days": 0,
        "late_payment_events": 0,
        "top_categories": ["folha", "cloud"],
        "recent_transactions": [],
    }
    if customer_id == "pj-risk":
        profile["risk_tier"] = "high"
        transactions.update(
            {
                "current_balance_brl": 10000,
                "average_monthly_inflow_brl": 50000,
                "average_monthly_outflow_brl": 65000,
                "overdraft_usage_days": 12,
                "late_payment_events": 3,
            }
        )
    if customer_id == "pj-incomplete":
        profile["kyc_status"] = "pending_documents"
    return {
        "request_id": f"eval-{customer_id}",
        "customer_id": customer_id,
        "question": question,
        "profile": profile,
        "transactions": transactions,
        "dependency_status": [
            {"name": "profile_api", "status": "ok", "source": "network"},
            {"name": "transactions_api", "status": "ok", "source": "network"},
        ],
    }


def main() -> None:
    cases = json.loads(Path("tests/evaluation/agent_eval_cases.json").read_text(encoding="utf-8"))
    results = []
    with TestClient(app) as client:
        for case in cases:
            payload = base_payload(case["customer_id"], case["question"])
            response = client.post("/v1/agent/analyze", json=payload)
            body = response.json()
            combined = f"{body['answer']} {' '.join(body['recommendations'])}".lower()
            passed = response.status_code == 200 and body["fallback_used"] == case["expect_fallback"] and any(
                keyword.lower() in combined for keyword in case["expect_keywords"]
            )
            results.append(
                {
                    "case": case["name"],
                    "status_code": response.status_code,
                    "passed": passed,
                    "fallback_used": body.get("fallback_used"),
                    "risk_flags": body.get("risk_flags", []),
                }
            )

    print(json.dumps({"results": results}, indent=2, ensure_ascii=True))


if __name__ == "__main__":
    main()
