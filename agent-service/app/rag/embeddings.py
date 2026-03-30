from __future__ import annotations

from dataclasses import dataclass
from typing import Sequence

import numpy as np
from openai import OpenAI
from sklearn.feature_extraction.text import TfidfVectorizer

from app.core.config import Settings


@dataclass
class EmbeddingProvider:
    settings: Settings

    def build_local(self, texts: Sequence[str]) -> tuple[str, TfidfVectorizer, object]:
        vectorizer = TfidfVectorizer(ngram_range=(1, 2), lowercase=True)
        matrix = vectorizer.fit_transform(texts)
        return "local", vectorizer, matrix

    def build(self, texts: Sequence[str]) -> dict[str, object]:
        if self.settings.rag_embedding_mode == "openai" and self.settings.openai_api_key:
            client_kwargs = {"api_key": self.settings.openai_api_key}
            if self.settings.openai_base_url:
                client_kwargs["base_url"] = self.settings.openai_base_url
            client = OpenAI(**client_kwargs)
            response = client.embeddings.create(model=self.settings.rag_embedding_model, input=list(texts))
            vectors = [item.embedding for item in response.data]
            return {
                "embedding_mode": "openai",
                "embedding_model": self.settings.rag_embedding_model,
                "vectors": np.array(vectors, dtype=np.float32),
            }

        mode, vectorizer, matrix = self.build_local(texts)
        return {
            "embedding_mode": mode,
            "embedding_model": "tfidf-ngram",
            "vectorizer": vectorizer,
            "matrix": matrix,
        }

    def embed_query(self, query: str) -> np.ndarray | object:
        if self.settings.rag_embedding_mode == "openai" and self.settings.openai_api_key:
            client_kwargs = {"api_key": self.settings.openai_api_key}
            if self.settings.openai_base_url:
                client_kwargs["base_url"] = self.settings.openai_base_url
            client = OpenAI(**client_kwargs)
            response = client.embeddings.create(model=self.settings.rag_embedding_model, input=[query])
            return np.array(response.data[0].embedding, dtype=np.float32)
        _, vectorizer, _ = self.build_local([query, query + " contexto"])
        return vectorizer.transform([query])

