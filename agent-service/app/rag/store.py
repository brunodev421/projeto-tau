from __future__ import annotations

from dataclasses import dataclass
from typing import Any

import joblib
import numpy as np
from sklearn.metrics.pairwise import cosine_similarity

from app.core.config import Settings
from app.models.schemas import RetrievedChunk
from app.observability.metrics import RAG_RESULTS
from app.rag.embeddings import EmbeddingProvider
from app.rag.ingest import build_index


@dataclass
class RAGStore:
    settings: Settings
    payload: dict[str, Any] | None = None

    def load(self) -> None:
        path = self.settings.processed_dir / "vector_store.joblib"
        if not path.exists():
            self.payload = build_index()
        else:
            self.payload = joblib.load(path)

    def search(self, query: str, top_k: int | None = None, score_threshold: float | None = None, tags: list[str] | None = None) -> list[RetrievedChunk]:
        if self.payload is None:
            self.load()
        assert self.payload is not None

        chunks = self.payload["chunks"]
        raw_scores = self._score(query)

        results: list[RetrievedChunk] = []
        top_k = top_k or self.settings.rag_top_k
        threshold = score_threshold if score_threshold is not None else self.settings.rag_score_threshold
        indices = np.argsort(raw_scores)[::-1]
        seen_documents: set[str] = set()
        for index in indices:
            if len(results) >= top_k:
                break
            score = float(raw_scores[index])
            chunk = chunks[index]
            metadata = chunk.get("metadata", {})
            if score < threshold:
                continue
            if tags:
                chunk_tags = set(metadata.get("tags", []))
                if chunk_tags and not chunk_tags.intersection(tags):
                    continue
            dedupe_key = f"{chunk['document_id']}:{chunk['content'][:80]}"
            if dedupe_key in seen_documents:
                continue
            seen_documents.add(dedupe_key)
            rerank_score = score + lexical_overlap_bonus(query, chunk["content"])
            results.append(
                RetrievedChunk(
                    chunk_id=chunk["chunk_id"],
                    document_id=chunk["document_id"],
                    title=chunk["title"],
                    content=chunk["content"],
                    metadata=metadata,
                    score=round(score, 4),
                    rerank_score=round(rerank_score, 4),
                )
            )

        results.sort(key=lambda item: (item.rerank_score or item.score), reverse=True)
        if results:
            RAG_RESULTS.labels(outcome="hit").inc()
        else:
            RAG_RESULTS.labels(outcome="miss").inc()
        return results

    def _score(self, query: str) -> np.ndarray:
        embedding_mode = self.payload.get("embedding_mode", "local")
        if embedding_mode == "openai" and "vectors" in self.payload:
            provider = EmbeddingProvider(self.settings)
            query_vector = provider.embed_query(query)
            vectors = self.payload["vectors"]
            query_norm = np.linalg.norm(query_vector)
            vector_norm = np.linalg.norm(vectors, axis=1)
            denominator = np.maximum(query_norm * vector_norm, 1e-9)
            return np.dot(vectors, query_vector) / denominator

        vectorizer = self.payload["vectorizer"]
        matrix = self.payload["matrix"]
        query_vector = vectorizer.transform([query])
        return cosine_similarity(query_vector, matrix).flatten()


def lexical_overlap_bonus(query: str, content: str) -> float:
    query_terms = {term for term in query.lower().split() if len(term) > 3}
    content_terms = set(content.lower().split())
    if not query_terms:
        return 0.0
    overlap = len(query_terms & content_terms) / len(query_terms)
    return overlap * 0.15
