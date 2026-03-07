"""Unit tests for ChatService prompt building logic."""
import pytest
from unittest.mock import AsyncMock, MagicMock
from typing import AsyncIterator

from src.application.chat_service import ChatService, ChatMessage
from src.domain.chunk import ChunkType, CodeChunk, GraphContext, SearchResult


# ---------------------------------------------------------------------------
# Helpers / fakes
# ---------------------------------------------------------------------------

def make_chunk(
    chunk_id: str = "chunk-1",
    file_path: str = "src/service.ts",
    content: str = "export function add(a: number, b: number) { return a + b; }",
    name: str = "add",
    score: float = 0.9,
) -> SearchResult:
    chunk = CodeChunk(
        id=chunk_id,
        repo_id="repo-1",
        file_path=file_path,
        chunk_type=ChunkType.FUNCTION,
        name=name,
        content=content,
        start_line=1,
        end_line=3,
        language="typescript",
    )
    return SearchResult(chunk=chunk, score=score)


def make_graph_context() -> GraphContext:
    return GraphContext(
        node_id="node-1",
        node_path="src/service.ts",
        imports=["src/utils.ts", "src/models.ts"],
        imported_by=["src/app.ts"],
        fan_in=1,
        fan_out=2,
        cycle_member=False,
    )


class FakeEmbedder:
    dimension = 1536

    async def embed_texts(self, texts):
        return [[0.1] * self.dimension for _ in texts]

    async def embed_query(self, query):
        return [0.1] * self.dimension


class FakeVectorStore:
    def __init__(self, results):
        self._results = results

    async def search(self, repo_id, query_vector, top_k, score_threshold=0.0):
        return self._results[:top_k]

    async def upsert_chunks(self, *args, **kwargs):
        return 0

    async def delete_repo(self, *args):
        pass

    async def collection_exists(self, *args):
        return True

    async def ensure_collection(self, *args):
        pass


class FakeGraphContext:
    def __init__(self, context=None):
        self._ctx = context

    async def get_context(self, repo_id, node_id):
        return self._ctx

    async def get_context_by_path(self, repo_id, file_path):
        return self._ctx


class FakeLLM:
    def __init__(self, tokens=None):
        self._tokens = tokens or ["Hello", " world", "!"]

    async def stream_completion(self, system_prompt, messages, max_tokens=4096):
        for t in self._tokens:
            yield t


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------

@pytest.mark.asyncio
async def test_stream_chat_yields_tokens() -> None:
    service = ChatService(
        embedding_port=FakeEmbedder(),
        vector_store_port=FakeVectorStore([make_chunk()]),
        graph_context_port=FakeGraphContext(),
        llm_port=FakeLLM(tokens=["The", " answer", " is", " 42"]),
    )

    tokens = []
    async for token in service.stream_chat("repo-1", "What does add do?"):
        tokens.append(token)

    assert tokens == ["The", " answer", " is", " 42"]


@pytest.mark.asyncio
async def test_stream_chat_no_results_still_answers() -> None:
    """When vector search finds nothing, LLM should still be called."""
    called = False

    class CheckingLLM:
        async def stream_completion(self, system_prompt, messages, max_tokens=4096):
            nonlocal called
            called = True
            yield "I don't see relevant code"

    service = ChatService(
        embedding_port=FakeEmbedder(),
        vector_store_port=FakeVectorStore([]),  # empty results
        graph_context_port=FakeGraphContext(),
        llm_port=CheckingLLM(),
    )

    tokens = []
    async for t in service.stream_chat("repo-1", "What does nonexistent do?"):
        tokens.append(t)

    assert called
    assert len(tokens) > 0


@pytest.mark.asyncio
async def test_stream_chat_includes_graph_context() -> None:
    """Graph context should be included in the prompt when node_id is given."""
    received_messages = []

    class CaptureLLM:
        async def stream_completion(self, system_prompt, messages, max_tokens=4096):
            received_messages.extend(messages)
            yield "ok"

    service = ChatService(
        embedding_port=FakeEmbedder(),
        vector_store_port=FakeVectorStore([make_chunk()]),
        graph_context_port=FakeGraphContext(context=make_graph_context()),
        llm_port=CaptureLLM(),
    )

    async for _ in service.stream_chat("repo-1", "Tell me about this file", node_id="node-1"):
        pass

    assert len(received_messages) > 0
    last_user_msg = next(m for m in reversed(received_messages) if m["role"] == "user")
    assert "Graph Context" in last_user_msg["content"]
    assert "src/service.ts" in last_user_msg["content"]


@pytest.mark.asyncio
async def test_stream_chat_respects_history() -> None:
    """Conversation history should be included in the messages list."""
    received_messages = []

    class CaptureLLM:
        async def stream_completion(self, system_prompt, messages, max_tokens=4096):
            received_messages.extend(messages)
            yield "ok"

    service = ChatService(
        embedding_port=FakeEmbedder(),
        vector_store_port=FakeVectorStore([]),
        graph_context_port=FakeGraphContext(),
        llm_port=CaptureLLM(),
    )

    history = [
        ChatMessage(role="user", content="What is this project?"),
        ChatMessage(role="assistant", content="It's an AIS project."),
    ]

    async for _ in service.stream_chat("repo-1", "Tell me more", history=history):
        pass

    roles = [m["role"] for m in received_messages]
    assert "user" in roles
    assert "assistant" in roles


@pytest.mark.asyncio
async def test_stream_chat_handles_embedding_error() -> None:
    """If embedding fails, service should yield an error message without crashing."""
    class BrokenEmbedder:
        dimension = 1536
        async def embed_texts(self, texts):
            raise RuntimeError("Embedding service down")
        async def embed_query(self, query):
            raise RuntimeError("Embedding service down")

    service = ChatService(
        embedding_port=BrokenEmbedder(),
        vector_store_port=FakeVectorStore([]),
        graph_context_port=FakeGraphContext(),
        llm_port=FakeLLM(),
    )

    tokens = []
    async for t in service.stream_chat("repo-1", "Will this crash?"):
        tokens.append(t)

    # Should yield an error message, not raise
    full = "".join(tokens)
    assert len(full) > 0


def test_build_context_block_empty_results() -> None:
    service = ChatService(
        embedding_port=FakeEmbedder(),
        vector_store_port=FakeVectorStore([]),
        graph_context_port=FakeGraphContext(),
        llm_port=FakeLLM(),
    )
    block = service._build_context_block([], None)
    assert block.strip() == ""


def test_build_context_block_with_chunks() -> None:
    service = ChatService(
        embedding_port=FakeEmbedder(),
        vector_store_port=FakeVectorStore([]),
        graph_context_port=FakeGraphContext(),
        llm_port=FakeLLM(),
    )
    results = [make_chunk(content="function add() {}")]
    block = service._build_context_block(results, None)
    assert "add" in block
    assert "Relevant Code Chunks" in block


def test_build_context_block_with_graph_ctx() -> None:
    service = ChatService(
        embedding_port=FakeEmbedder(),
        vector_store_port=FakeVectorStore([]),
        graph_context_port=FakeGraphContext(),
        llm_port=FakeLLM(),
    )
    ctx = make_graph_context()
    block = service._build_context_block([], ctx)
    assert "Graph Context" in block
    assert "src/service.ts" in block
