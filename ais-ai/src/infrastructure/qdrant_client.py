"""Qdrant vector store implementing the VectorStorePort."""
from __future__ import annotations

import logging
import uuid

from qdrant_client import AsyncQdrantClient
from qdrant_client.models import (
    Distance,
    FieldCondition,
    Filter,
    MatchValue,
    PointStruct,
    VectorParams,
)

from ..domain.chunk import CodeChunk, SearchResult
from ..domain.ports import VectorStorePort

logger = logging.getLogger(__name__)

COLLECTION_NAME = "ais_code_chunks"


class QdrantVectorStore(VectorStorePort):
    """
    Implements VectorStorePort using Qdrant.

    All chunks for all repositories are stored in a single collection
    and filtered by repo_id at search time.
    """

    def __init__(self, url: str, api_key: str | None = None) -> None:
        self._client = AsyncQdrantClient(url=url, api_key=api_key)
        self._collection = COLLECTION_NAME
        logger.info("QdrantVectorStore initialized: %s", url)

    async def collection_exists(self, collection_name: str) -> bool:
        try:
            existing = await self._client.get_collections()
            return any(c.name == collection_name for c in existing.collections)
        except Exception as exc:
            logger.warning("Failed to list Qdrant collections: %s", exc)
            return False

    async def ensure_collection(self, collection_name: str, vector_size: int) -> None:
        """Create the collection if it does not already exist."""
        if await self.collection_exists(collection_name):
            return

        logger.info(
            "Creating Qdrant collection '%s' with vector_size=%d",
            collection_name, vector_size,
        )
        try:
            await self._client.create_collection(
                collection_name=collection_name,
                vectors_config=VectorParams(
                    size=vector_size,
                    distance=Distance.COSINE,
                ),
            )
            # Create payload index on repo_id for fast filtering
            await self._client.create_payload_index(
                collection_name=collection_name,
                field_name="repo_id",
                field_schema="keyword",
            )
            await self._client.create_payload_index(
                collection_name=collection_name,
                field_name="file_path",
                field_schema="keyword",
            )
            logger.info("Qdrant collection '%s' created successfully", collection_name)
        except Exception as exc:
            logger.error("Failed to create Qdrant collection: %s", exc)
            raise

    async def upsert_chunks(
        self,
        repo_id: str,
        chunks: list[CodeChunk],
        vectors: list[list[float]],
    ) -> int:
        """Upsert chunks and their embeddings into Qdrant."""
        if len(chunks) != len(vectors):
            raise ValueError(
                f"Chunk count ({len(chunks)}) != vector count ({len(vectors)})"
            )

        if not chunks:
            return 0

        points = []
        for chunk, vector in zip(chunks, vectors):
            # Use a deterministic UUID based on chunk ID for idempotent upserts
            point_id = str(uuid.uuid5(uuid.NAMESPACE_DNS, chunk.id))
            payload = chunk.to_qdrant_payload()
            points.append(PointStruct(id=point_id, vector=vector, payload=payload))

        try:
            await self._client.upsert(
                collection_name=self._collection,
                points=points,
                wait=True,
            )
            return len(points)
        except Exception as exc:
            logger.error(
                "Qdrant upsert failed for repo %s (%d points): %s",
                repo_id, len(points), exc,
            )
            raise

    async def search(
        self,
        repo_id: str,
        query_vector: list[float],
        top_k: int,
        score_threshold: float = 0.0,
    ) -> list[SearchResult]:
        """Search for similar chunks filtered to a specific repository."""
        try:
            results = await self._client.search(
                collection_name=self._collection,
                query_vector=query_vector,
                query_filter=Filter(
                    must=[
                        FieldCondition(
                            key="repo_id",
                            match=MatchValue(value=repo_id),
                        )
                    ]
                ),
                limit=top_k,
                score_threshold=score_threshold,
                with_payload=True,
            )
        except Exception as exc:
            logger.error("Qdrant search failed for repo %s: %s", repo_id, exc)
            raise

        search_results: list[SearchResult] = []
        for hit in results:
            try:
                chunk = CodeChunk.from_qdrant_payload(hit.payload or {})
                search_results.append(SearchResult(chunk=chunk, score=hit.score))
            except Exception as exc:
                logger.warning("Failed to deserialize Qdrant hit: %s", exc)

        return search_results

    async def delete_repo(self, repo_id: str) -> None:
        """Remove all chunks belonging to a repository."""
        try:
            await self._client.delete(
                collection_name=self._collection,
                points_selector=Filter(
                    must=[
                        FieldCondition(
                            key="repo_id",
                            match=MatchValue(value=repo_id),
                        )
                    ]
                ),
            )
            logger.info("Deleted all chunks for repo %s from Qdrant", repo_id)
        except Exception as exc:
            logger.error("Failed to delete repo %s from Qdrant: %s", repo_id, exc)
            raise
