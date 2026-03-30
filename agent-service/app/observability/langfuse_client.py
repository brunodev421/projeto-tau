from __future__ import annotations

from contextlib import contextmanager
from dataclasses import dataclass
from typing import Any, Iterator

from app.core.config import Settings

try:
    from langfuse import Langfuse
except Exception:
    Langfuse = None


@dataclass
class LangfuseFacade:
    enabled: bool
    client: Any = None

    @classmethod
    def from_settings(cls, settings: Settings) -> "LangfuseFacade":
        if Langfuse is None:
            return cls(enabled=False)
        if not (settings.langfuse_public_key and settings.langfuse_secret_key and settings.langfuse_host):
            return cls(enabled=False)
        return cls(
            enabled=True,
            client=Langfuse(
                public_key=settings.langfuse_public_key,
                secret_key=settings.langfuse_secret_key,
                host=settings.langfuse_host,
            ),
        )

    @contextmanager
    def trace(self, request_id: str, customer_id: str, input_payload: dict[str, Any]) -> Iterator[Any]:
        if not self.enabled:
            yield None
            return
        trace = self.client.trace(
            id=request_id,
            name="agent-workflow",
            user_id=customer_id,
            input=input_payload,
        )
        try:
            yield trace
        finally:
            try:
                self.client.flush()
            except Exception:
                pass
