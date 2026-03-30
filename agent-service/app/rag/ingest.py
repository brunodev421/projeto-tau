from __future__ import annotations

import hashlib
import json
from pathlib import Path

import joblib

from app.core.config import get_settings
from app.rag.embeddings import EmbeddingProvider


def parse_document(path: Path) -> tuple[dict[str, str], str]:
    content = path.read_text(encoding="utf-8")
    metadata: dict[str, str] = {"document_id": path.stem, "title": path.stem.replace("-", " ").title()}
    if content.startswith("---"):
        _, frontmatter, body = content.split("---", 2)
        for raw_line in frontmatter.strip().splitlines():
            if ":" not in raw_line:
                continue
            key, value = raw_line.split(":", 1)
            metadata[key.strip()] = value.strip().strip("[]")
        content = body.strip()
    return metadata, content


def chunk_text(document_id: str, title: str, text: str, max_chars: int = 700, overlap_chars: int = 120) -> list[dict[str, str]]:
    chunks: list[dict[str, str]] = []
    paragraphs = [paragraph.strip() for paragraph in text.split("\n\n") if paragraph.strip()]
    buffer = ""
    index = 0
    for paragraph in paragraphs:
        if len(buffer) + len(paragraph) + 2 <= max_chars:
            buffer = f"{buffer}\n\n{paragraph}".strip()
            continue
        if buffer:
            chunk_id = hashlib.sha1(f"{document_id}:{index}:{buffer}".encode("utf-8")).hexdigest()[:12]
            chunks.append({"chunk_id": chunk_id, "document_id": document_id, "title": title, "content": buffer})
            index += 1
            buffer = buffer[-overlap_chars:] + "\n\n" + paragraph if overlap_chars else paragraph
        else:
            chunk_id = hashlib.sha1(f"{document_id}:{index}:{paragraph}".encode("utf-8")).hexdigest()[:12]
            chunks.append({"chunk_id": chunk_id, "document_id": document_id, "title": title, "content": paragraph[:max_chars]})
            index += 1
            buffer = paragraph[max_chars - overlap_chars :]
    if buffer:
        chunk_id = hashlib.sha1(f"{document_id}:{index}:{buffer}".encode("utf-8")).hexdigest()[:12]
        chunks.append({"chunk_id": chunk_id, "document_id": document_id, "title": title, "content": buffer})
    return chunks


def build_index() -> dict[str, object]:
    settings = get_settings()
    chunks: list[dict[str, object]] = []
    for path in sorted(settings.knowledge_base_dir.glob("*.md")):
        metadata, content = parse_document(path)
        tags = [item.strip() for item in metadata.get("tags", "").replace("[", "").replace("]", "").split(",") if item.strip()]
        for chunk in chunk_text(metadata["document_id"], metadata.get("title", metadata["document_id"]), content):
            chunk["metadata"] = {"source_file": path.name, "tags": tags}
            chunks.append(chunk)

    embedding_provider = EmbeddingProvider(settings)
    embedding_payload = embedding_provider.build([chunk["content"] for chunk in chunks])
    payload = {"chunks": chunks, **embedding_payload}
    joblib.dump(payload, settings.processed_dir / "vector_store.joblib")
    (settings.processed_dir / "vector_store_manifest.json").write_text(
        json.dumps(
            {
                "documents": len(list(settings.knowledge_base_dir.glob("*.md"))),
                "chunks": len(chunks),
                "embedding_mode": payload["embedding_mode"],
                "embedding_model": payload["embedding_model"],
            },
            indent=2,
        ),
        encoding="utf-8",
    )
    return payload


if __name__ == "__main__":
    build_index()
