"""Voyage AI embedding client implementing the EmbeddingPort."""
from __future__ import annotations

import asyncio
import logging
from functools import lru_cache

import voyageai

from ..domain.ports import EmbeddingPort

logger = logging.getLogger(__name__)

# voyage-code-2 produces 1536-dimensional vectors
VOYAGE_CODE_MODEL = "voyage-code-2"
VOYAGE_QUERY_MODEL = "voyage-code-2"
VECTOR_DIMENSION = 1536

# Voyage AI rate limits: 300 RPM, 1M tokens/min for voyage-code-2
# Batch size chosen to stay safely within limits
MAX_BATCH_SIZE = 128


class VoyageEmbeddingClient(EmbeddingPort):
    """
    Implements EmbeddingPort using Voyage AI's voyage-code-2 model.

    voyage-code-2 is purpose-trained for code retrieval and outperforms
    general text models by 15–30% on code search benchmarks.
    """

    def __init__(self, api_key: str, model: str = VOYAGE_CODE_MODEL) -> None:
        self._client = voyageai.AsyncClient(api_key=api_key)
        self._model = model
        self._dimension = VECTOR_DIMENSION
        logger.info("VoyageEmbeddingClient initialized with model %s", model)

    @property
    def dimension(self) -> int:
        return self._dimension

    async def embed_texts(self, texts: list[str]) -> list[list[float]]:
        """
        Embed a batch of code texts.

        Args:
            texts: List of text strings to embed. Should be ≤128 items.

        Returns:
            List of float vectors, one per input text.
        """
        if not texts:
            return []

        # Truncate individual texts that are too long (Voyage limit: ~16k tokens)
        truncated = [t[:32_000] for t in texts]

        try:
            result = await self._client.embed(
                texts=truncated,
                model=self._model,
                input_type="document",  # "document" for indexing, "query" for search
            )
            return result.embeddings
        except Exception as exc:
            logger.error("Voyage AI embed_texts failed: %s", exc)
            raise

    async def embed_query(self, query: str) -> list[float]:
        """
        Embed a single search query with query-optimized settings.

        Uses input_type="query" which Voyage optimizes differently from documents.
        """
        if not query:
            raise ValueError("Query must not be empty")

        truncated = query[:4_000]  # queries are shorter

        try:
            result = await self._client.embed(
                texts=[truncated],
                model=self._model,
                input_type="query",
            )
            return result.embeddings[0]
        except Exception as exc:
            logger.error("Voyage AI embed_query failed: %s", exc)
            raise

    async def embed_texts_batched(
        self,
        texts: list[str],
        batch_size: int = MAX_BATCH_SIZE,
    ) -> list[list[float]]:
        """
        Embed a large list of texts by splitting into batches with rate limiting.
        """
        all_vectors: list[list[float]] = []

        for i in range(0, len(texts), batch_size):
            batch = texts[i: i + batch_size]
            vectors = await self.embed_texts(batch)
            all_vectors.extend(vectors)

            # Small delay between batches to respect rate limits
            if i + batch_size < len(texts):
                await asyncio.sleep(0.2)

        return all_vectors
