"""
IndexingService orchestrates the full indexing pipeline:
  1. Chunk each file by AST boundaries
  2. Generate Voyage AI embeddings in batches
  3. Upsert to Qdrant
"""
from __future__ import annotations

import asyncio
import logging
from dataclasses import dataclass

from ..domain.chunk import CodeChunk
from ..domain.ports import EmbeddingPort, VectorStorePort
from .chunker import CodeChunker

logger = logging.getLogger(__name__)

# Voyage AI supports up to 128 texts per batch for code embeddings
EMBEDDING_BATCH_SIZE = 64
# Qdrant upsert batch size
UPSERT_BATCH_SIZE = 128


@dataclass
class FileInput:
    path: str
    content: str
    language: str


class IndexingService:
    """
    Orchestrates the complete indexing pipeline for a repository.

    Lifecycle: one instance per application, stateless across repo calls.
    """

    def __init__(
        self,
        embedding_port: EmbeddingPort,
        vector_store_port: VectorStorePort,
    ) -> None:
        self._embedder = embedding_port
        self._vector_store = vector_store_port
        self._chunker = CodeChunker()

    async def index_repository(
        self,
        repo_id: str,
        files: list[FileInput],
    ) -> int:
        """
        Index all files in a repository.

        Args:
            repo_id: Unique repository identifier.
            files:   List of source files to index.

        Returns:
            Total number of chunks indexed.
        """
        logger.info("Starting indexing for repo %s (%d files)", repo_id, len(files))

        # Ensure Qdrant collection exists
        await self._vector_store.ensure_collection(
            collection_name="ais_code_chunks",
            vector_size=self._embedder.dimension,
        )

        # Step 1: Chunk all files
        all_chunks: list[CodeChunk] = []
        for file_input in files:
            try:
                chunks = self._chunker.chunk_file(
                    repo_id=repo_id,
                    file_path=file_input.path,
                    content=file_input.content,
                    language=file_input.language,
                )
                all_chunks.extend(chunks)
            except Exception as exc:
                logger.warning("Chunking failed for %s: %s", file_input.path, exc)

        if not all_chunks:
            logger.warning("No chunks produced for repo %s", repo_id)
            return 0

        logger.info("Produced %d chunks from %d files for repo %s",
                    len(all_chunks), len(files), repo_id)

        # Step 2: Embed in batches
        total_indexed = 0
        for batch_start in range(0, len(all_chunks), EMBEDDING_BATCH_SIZE):
            batch = all_chunks[batch_start: batch_start + EMBEDDING_BATCH_SIZE]

            texts = [self._format_chunk_for_embedding(c) for c in batch]

            try:
                vectors = await self._embedder.embed_texts(texts)
            except Exception as exc:
                logger.error("Embedding failed for batch starting at %d: %s", batch_start, exc)
                continue

            if len(vectors) != len(batch):
                logger.error(
                    "Embedding returned %d vectors for %d chunks — skipping batch",
                    len(vectors), len(batch),
                )
                continue

            # Step 3: Upsert to Qdrant in sub-batches
            for upsert_start in range(0, len(batch), UPSERT_BATCH_SIZE):
                upsert_batch = batch[upsert_start: upsert_start + UPSERT_BATCH_SIZE]
                upsert_vectors = vectors[upsert_start: upsert_start + UPSERT_BATCH_SIZE]

                try:
                    count = await self._vector_store.upsert_chunks(
                        repo_id=repo_id,
                        chunks=upsert_batch,
                        vectors=upsert_vectors,
                    )
                    total_indexed += count
                except Exception as exc:
                    logger.error("Qdrant upsert failed: %s", exc)

            # Brief pause to respect rate limits
            await asyncio.sleep(0.05)

        logger.info(
            "Indexing complete for repo %s: %d/%d chunks indexed",
            repo_id, total_indexed, len(all_chunks),
        )
        return total_indexed

    @staticmethod
    def _format_chunk_for_embedding(chunk: CodeChunk) -> str:
        """
        Formats a code chunk for optimal embedding quality.

        Voyage AI code embeddings benefit from structured prefixes that provide
        context about what kind of code is being embedded.
        """
        prefix = f"# {chunk.chunk_type.value}: {chunk.name}\n# File: {chunk.file_path}\n"
        return prefix + chunk.content
