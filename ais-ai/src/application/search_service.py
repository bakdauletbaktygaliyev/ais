"""SearchService handles semantic similarity search over repository code chunks."""
from __future__ import annotations

import logging

from ..domain.chunk import SearchResult
from ..domain.ports import EmbeddingPort, VectorStorePort

logger = logging.getLogger(__name__)


class SearchService:
    """Provides semantic similarity search over indexed code chunks."""

    def __init__(
        self,
        embedding_port: EmbeddingPort,
        vector_store_port: VectorStorePort,
    ) -> None:
        self._embedder = embedding_port
        self._vector_store = vector_store_port

    async def search_similar(
        self,
        repo_id: str,
        query: str,
        top_k: int = 5,
        score_threshold: float = 0.2,
    ) -> list[SearchResult]:
        """
        Search for code chunks semantically similar to a query.

        Args:
            repo_id:         Repository to search within.
            query:           Natural language or code query.
            top_k:           Maximum number of results to return.
            score_threshold: Minimum cosine similarity score (0–1).

        Returns:
            List of SearchResult objects sorted by descending relevance.
        """
        if not query.strip():
            return []

        top_k = max(1, min(top_k, 20))

        logger.info(
            "Searching similar code in repo %s (top_k=%d): %s...",
            repo_id, top_k, query[:80],
        )

        try:
            query_vector = await self._embedder.embed_query(query)
        except Exception as exc:
            logger.error("Failed to embed search query: %s", exc)
            return []

        try:
            results = await self._vector_store.search(
                repo_id=repo_id,
                query_vector=query_vector,
                top_k=top_k,
                score_threshold=score_threshold,
            )
        except Exception as exc:
            logger.error("Vector store search failed: %s", exc)
            return []

        logger.info("Found %d results for repo %s", len(results), repo_id)
        return results
