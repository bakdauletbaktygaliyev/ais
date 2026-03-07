"""Unit tests for IndexingService."""
import pytest
from unittest.mock import AsyncMock, MagicMock, patch

from src.application.indexing_service import FileInput, IndexingService
from src.domain.chunk import CodeChunk


class FakeEmbedder:
    dimension = 1536
    calls: list

    def __init__(self):
        self.calls = []

    async def embed_texts(self, texts):
        self.calls.append(texts)
        return [[0.0] * self.dimension for _ in texts]

    async def embed_query(self, query):
        return [0.0] * self.dimension


class FakeVectorStore:
    upserted: list

    def __init__(self):
        self.upserted = []

    async def upsert_chunks(self, repo_id, chunks, vectors):
        self.upserted.extend(chunks)
        return len(chunks)

    async def delete_repo(self, repo_id):
        pass

    async def collection_exists(self, name):
        return True

    async def ensure_collection(self, name, vector_size):
        pass


@pytest.mark.asyncio
async def test_index_empty_files_returns_zero() -> None:
    svc = IndexingService(FakeEmbedder(), FakeVectorStore())
    count = await svc.index_repository("repo-1", [])
    assert count == 0


@pytest.mark.asyncio
async def test_index_produces_chunks() -> None:
    embedder = FakeEmbedder()
    store = FakeVectorStore()
    svc = IndexingService(embedder, store)

    files = [
        FileInput(
            path="src/math.ts",
            content="export function add(a: number, b: number): number { return a + b; }\n"
                    "export function sub(a: number, b: number): number { return a - b; }",
            language="typescript",
        )
    ]

    count = await svc.index_repository("repo-1", files)
    assert count > 0
    assert len(store.upserted) > 0


@pytest.mark.asyncio
async def test_index_calls_embedder_with_prefix() -> None:
    embedder = FakeEmbedder()
    store = FakeVectorStore()
    svc = IndexingService(embedder, store)

    files = [
        FileInput(
            path="src/service.go",
            content="package service\n\nfunc Hello() string { return \"hello\" }",
            language="go",
        )
    ]

    await svc.index_repository("repo-1", files)

    assert len(embedder.calls) > 0
    # Verify texts have the structured prefix
    first_batch = embedder.calls[0]
    assert len(first_batch) > 0
    for text in first_batch:
        assert "File:" in text or "function" in text.lower() or "module" in text.lower()


@pytest.mark.asyncio
async def test_index_multiple_files() -> None:
    store = FakeVectorStore()
    svc = IndexingService(FakeEmbedder(), store)

    files = [
        FileInput(path=f"src/file_{i}.ts",
                  content=f"export function fn{i}() {{ return {i}; }}",
                  language="typescript")
        for i in range(5)
    ]

    count = await svc.index_repository("repo-multi", files)
    assert count >= 5  # at least one chunk per file


@pytest.mark.asyncio
async def test_index_handles_parse_error_gracefully() -> None:
    """A file that fails to parse should not crash the whole indexing."""
    class BrokenChunker:
        def chunk_file(self, *args, **kwargs):
            raise RuntimeError("Parse exploded")

    embedder = FakeEmbedder()
    store = FakeVectorStore()
    svc = IndexingService(embedder, store)
    svc._chunker = BrokenChunker()

    files = [FileInput(path="broken.ts", content="???", language="typescript")]
    count = await svc.index_repository("repo-1", files)
    assert count == 0  # graceful — no crash


@pytest.mark.asyncio
async def test_index_handles_embedding_failure() -> None:
    """Embedding failure for a batch should be logged but not crash."""
    call_count = 0

    class PartiallyBrokenEmbedder:
        dimension = 1536
        async def embed_texts(self, texts):
            nonlocal call_count
            call_count += 1
            if call_count == 1:
                raise RuntimeError("Rate limit")
            return [[0.0] * self.dimension for _ in texts]
        async def embed_query(self, q):
            return [0.0] * self.dimension

    store = FakeVectorStore()
    svc = IndexingService(PartiallyBrokenEmbedder(), store)

    files = [FileInput(path="src/a.ts",
                       content="export function test() { return 1; }",
                       language="typescript")]
    # Should not raise
    await svc.index_repository("repo-1", files)


@pytest.mark.asyncio
async def test_format_chunk_for_embedding() -> None:
    from src.domain.chunk import ChunkType, CodeChunk

    chunk = CodeChunk(
        id="c1", repo_id="r1", file_path="src/utils.ts",
        chunk_type=ChunkType.FUNCTION, name="formatDate",
        content="function formatDate(d: Date) { return d.toISOString(); }",
        start_line=1, end_line=3, language="typescript",
    )
    text = IndexingService._format_chunk_for_embedding(chunk)
    assert "formatDate" in text
    assert "src/utils.ts" in text
    assert "function" in text
    assert chunk.content in text
