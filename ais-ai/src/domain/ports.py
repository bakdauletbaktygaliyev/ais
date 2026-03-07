from __future__ import annotations

from abc import ABC, abstractmethod
from typing import AsyncIterator

from .chunk import CodeChunk, GraphContext, SearchResult


class EmbeddingPort(ABC):
    """Port for generating code embeddings."""

    @abstractmethod
    async def embed_texts(self, texts: list[str]) -> list[list[float]]:
        """Embed a batch of texts. Returns one vector per input text."""
        ...

    @abstractmethod
    async def embed_query(self, query: str) -> list[float]:
        """Embed a single search query."""
        ...

    @property
    @abstractmethod
    def dimension(self) -> int:
        """Vector dimension produced by this model."""
        ...


class VectorStorePort(ABC):
    """Port for vector storage and retrieval."""

    @abstractmethod
    async def upsert_chunks(self, repo_id: str, chunks: list[CodeChunk], vectors: list[list[float]]) -> int:
        """Upsert chunk embeddings into the vector store. Returns count of upserted points."""
        ...

    @abstractmethod
    async def search(
        self,
        repo_id: str,
        query_vector: list[float],
        top_k: int,
        score_threshold: float = 0.0,
    ) -> list[SearchResult]:
        """Search for the most similar chunks to a query vector."""
        ...

    @abstractmethod
    async def delete_repo(self, repo_id: str) -> None:
        """Delete all vectors for a repository."""
        ...

    @abstractmethod
    async def collection_exists(self, collection_name: str) -> bool:
        """Check whether a collection exists."""
        ...

    @abstractmethod
    async def ensure_collection(self, collection_name: str, vector_size: int) -> None:
        """Create collection if it does not exist."""
        ...


class GraphContextPort(ABC):
    """Port for fetching graph context from Neo4j."""

    @abstractmethod
    async def get_context(self, repo_id: str, node_id: str) -> GraphContext | None:
        """Fetch graph context for a given node."""
        ...

    @abstractmethod
    async def get_context_by_path(self, repo_id: str, file_path: str) -> GraphContext | None:
        """Fetch graph context for a file identified by path."""
        ...


class LLMPort(ABC):
    """Port for LLM completions with streaming support."""

    @abstractmethod
    async def stream_completion(
        self,
        system_prompt: str,
        messages: list[dict],
        max_tokens: int = 4096,
    ) -> AsyncIterator[str]:
        """Stream completion tokens from the LLM."""
        ...
