"""
ChatService implements the RAG pipeline:
  1. Embed user question with Voyage AI
  2. Search Qdrant for top-k relevant code chunks
  3. Fetch graph context from Neo4j for the selected node
  4. Build a structured prompt
  5. Stream Claude's response token by token
"""
from __future__ import annotations

import logging
from dataclasses import dataclass
from typing import AsyncIterator

from ..domain.chunk import GraphContext, SearchResult
from ..domain.ports import EmbeddingPort, GraphContextPort, LLMPort, VectorStorePort

logger = logging.getLogger(__name__)

TOP_K_CHUNKS = 5
MAX_CONTEXT_CHARS = 40_000  # ~10k tokens, well within Claude's 200k window
SYSTEM_PROMPT = """You are an expert software architect assistant for the AIS (Architecture Insight System) platform.

You have been given context about a specific GitHub repository including:
1. Relevant code chunks retrieved via semantic search
2. Graph context showing how files relate to each other (imports, callers, dependencies)

Your role is to answer questions about the codebase accurately and concisely. You should:
- Reference specific file paths and line numbers when relevant
- Explain architectural patterns you observe
- Identify potential issues like circular dependencies or high coupling
- Suggest improvements based on clean architecture principles
- Be precise about what the code actually does, not what it might do

Always ground your answers in the provided code context. If you cannot find relevant information in the context, say so clearly rather than speculating.
"""


@dataclass
class ChatMessage:
    role: str  # "user" or "assistant"
    content: str


class ChatService:
    """Implements the full RAG chat pipeline with streaming output."""

    def __init__(
        self,
        embedding_port: EmbeddingPort,
        vector_store_port: VectorStorePort,
        graph_context_port: GraphContextPort,
        llm_port: LLMPort,
    ) -> None:
        self._embedder = embedding_port
        self._vector_store = vector_store_port
        self._graph_ctx = graph_context_port
        self._llm = llm_port

    async def stream_chat(
        self,
        repo_id: str,
        message: str,
        node_id: str | None = None,
        history: list[ChatMessage] | None = None,
    ) -> AsyncIterator[str]:
        """
        Execute the RAG pipeline and stream response tokens.

        Args:
            repo_id:  Repository identifier for context scoping.
            message:  User's natural language question.
            node_id:  Optional node ID for focused graph context.
            history:  Previous conversation turns.

        Yields:
            Individual string tokens from Claude's response.
        """
        history = history or []

        # Step 1: Embed the question
        logger.info("Embedding query for repo %s: %s...", repo_id, message[:80])
        try:
            query_vector = await self._embedder.embed_query(message)
        except Exception as exc:
            logger.error("Failed to embed query: %s", exc)
            yield "I'm unable to process your question right now due to an embedding service error."
            return

        # Step 2: Vector search — top-k similar chunks
        try:
            search_results: list[SearchResult] = await self._vector_store.search(
                repo_id=repo_id,
                query_vector=query_vector,
                top_k=TOP_K_CHUNKS,
                score_threshold=0.3,
            )
        except Exception as exc:
            logger.error("Vector search failed: %s", exc)
            search_results = []

        logger.info("Found %d relevant chunks for repo %s", len(search_results), repo_id)

        # Step 3: Graph context
        graph_context: GraphContext | None = None
        if node_id:
            try:
                graph_context = await self._graph_ctx.get_context(repo_id, node_id)
            except Exception as exc:
                logger.warning("Graph context fetch failed: %s", exc)

        # Step 4: Build prompt
        context_block = self._build_context_block(search_results, graph_context)
        user_message = self._build_user_message(message, context_block)

        # Step 5: Build messages list for Claude
        messages = []
        for turn in history[-6:]:  # Last 3 turns (6 messages)
            messages.append({"role": turn.role, "content": turn.content})
        messages.append({"role": "user", "content": user_message})

        # Step 6: Stream from Claude
        try:
            async for token in self._llm.stream_completion(
                system_prompt=SYSTEM_PROMPT,
                messages=messages,
                max_tokens=4096,
            ):
                yield token
        except Exception as exc:
            logger.error("LLM streaming failed: %s", exc)
            yield f"\n\n[Error: Unable to complete response — {exc}]"

    def _build_context_block(
        self,
        search_results: list[SearchResult],
        graph_context: GraphContext | None,
    ) -> str:
        """Assembles retrieved context into a structured block."""
        parts: list[str] = []
        total_chars = 0

        # Code chunks from vector search
        if search_results:
            parts.append("## Relevant Code Chunks\n")
            for i, result in enumerate(search_results, start=1):
                chunk = result.chunk
                header = (
                    f"### Chunk {i}: `{chunk.name}` "
                    f"({chunk.chunk_type.value}) — "
                    f"`{chunk.file_path}` "
                    f"lines {chunk.start_line}–{chunk.end_line} "
                    f"(relevance: {result.score:.2f})\n"
                )
                body = f"```{chunk.language}\n{chunk.content}\n```\n"
                chunk_text = header + body

                if total_chars + len(chunk_text) > MAX_CONTEXT_CHARS:
                    break

                parts.append(chunk_text)
                total_chars += len(chunk_text)

        # Graph context
        if graph_context:
            graph_text = "\n## Graph Context\n" + graph_context.to_context_string() + "\n"
            if total_chars + len(graph_text) <= MAX_CONTEXT_CHARS:
                parts.append(graph_text)

        return "\n".join(parts)

    @staticmethod
    def _build_user_message(question: str, context: str) -> str:
        """Wraps the question with retrieved context."""
        if not context.strip():
            return question

        return (
            f"Here is relevant context from the repository:\n\n"
            f"{context}\n\n"
            f"---\n\n"
            f"Question: {question}"
        )
