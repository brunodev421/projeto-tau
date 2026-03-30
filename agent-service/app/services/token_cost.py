from __future__ import annotations

from dataclasses import dataclass

try:
    import tiktoken
except Exception:
    tiktoken = None


MODEL_PRICING = {
    "gpt-4.1-mini": {"input": 0.0000004, "output": 0.0000016},
    "local": {"input": 0.0, "output": 0.0},
}


@dataclass
class TokenCostEstimator:
    model_name: str

    def count(self, text: str) -> int:
        if not text:
            return 0
        if tiktoken is not None and self.model_name != "local":
            try:
                encoder = tiktoken.encoding_for_model(self.model_name)
                return len(encoder.encode(text))
            except Exception:
                pass
        return max(1, len(text) // 4)

    def estimate(self, input_text: str, output_text: str) -> tuple[int, int, float]:
        input_tokens = self.count(input_text)
        output_tokens = self.count(output_text)
        rates = MODEL_PRICING.get(self.model_name, MODEL_PRICING["local"])
        cost = input_tokens * rates["input"] + output_tokens * rates["output"]
        return input_tokens, output_tokens, round(cost, 6)
