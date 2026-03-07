"""Unit tests for the AST-based code chunker."""
import pytest
from src.application.chunker import CodeChunker
from src.domain.chunk import ChunkType


@pytest.fixture
def chunker() -> CodeChunker:
    return CodeChunker()


# ---------------------------------------------------------------------------
# TypeScript
# ---------------------------------------------------------------------------

TS_SIMPLE = """
export function add(a: number, b: number): number {
  return a + b;
}

export const multiply = (a: number, b: number): number => a * b;

export class Calculator {
  private history: number[] = [];

  add(a: number, b: number): number {
    const result = a + b;
    this.history.push(result);
    return result;
  }
}

export interface Serializable {
  serialize(): string;
  deserialize(data: string): void;
}
"""


def test_ts_extracts_function(chunker: CodeChunker) -> None:
    chunks = chunker.chunk_file("repo-1", "math.ts", TS_SIMPLE, "typescript")
    names = [c.name for c in chunks]
    assert "add" in names or any("add" in n for n in names), f"'add' not found in {names}"


def test_ts_extracts_class(chunker: CodeChunker) -> None:
    chunks = chunker.chunk_file("repo-1", "math.ts", TS_SIMPLE, "typescript")
    class_chunks = [c for c in chunks if c.chunk_type == ChunkType.CLASS]
    assert len(class_chunks) >= 1, "Expected at least one class chunk"
    assert any("Calculator" in c.name for c in class_chunks)


def test_ts_chunk_has_content(chunker: CodeChunker) -> None:
    chunks = chunker.chunk_file("repo-1", "math.ts", TS_SIMPLE, "typescript")
    for chunk in chunks:
        assert chunk.content.strip(), f"Empty content for chunk {chunk.name}"


def test_ts_chunk_has_line_numbers(chunker: CodeChunker) -> None:
    chunks = chunker.chunk_file("repo-1", "math.ts", TS_SIMPLE, "typescript")
    for chunk in chunks:
        assert chunk.start_line > 0, f"start_line not set for {chunk.name}"
        assert chunk.end_line >= chunk.start_line, f"end_line < start_line for {chunk.name}"


def test_ts_chunk_ids_are_unique(chunker: CodeChunker) -> None:
    chunks = chunker.chunk_file("repo-1", "math.ts", TS_SIMPLE, "typescript")
    ids = [c.id for c in chunks]
    assert len(ids) == len(set(ids)), "Duplicate chunk IDs"


def test_ts_chunk_ids_are_stable(chunker: CodeChunker) -> None:
    """Same input → same IDs (deterministic hashing)."""
    chunks_a = chunker.chunk_file("repo-1", "math.ts", TS_SIMPLE, "typescript")
    chunks_b = chunker.chunk_file("repo-1", "math.ts", TS_SIMPLE, "typescript")
    assert [c.id for c in chunks_a] == [c.id for c in chunks_b]


# ---------------------------------------------------------------------------
# Go
# ---------------------------------------------------------------------------

GO_SIMPLE = """
package service

import (
\t"context"
\t"fmt"
)

type UserService struct {
\tdb Database
}

func NewUserService(db Database) *UserService {
\treturn &UserService{db: db}
}

func (s *UserService) GetUser(ctx context.Context, id string) (*User, error) {
\treturn s.db.FindByID(ctx, id)
}

func (s *UserService) CreateUser(ctx context.Context, user *User) error {
\tif user.Name == "" {
\t\treturn fmt.Errorf("name is required")
\t}
\treturn s.db.Save(ctx, user)
}
"""


def test_go_extracts_functions(chunker: CodeChunker) -> None:
    chunks = chunker.chunk_file("repo-1", "service.go", GO_SIMPLE, "go")
    names = {c.name for c in chunks}
    assert "NewUserService" in names or any("NewUserService" in n for n in names)


def test_go_extracts_methods(chunker: CodeChunker) -> None:
    chunks = chunker.chunk_file("repo-1", "service.go", GO_SIMPLE, "go")
    method_chunks = [c for c in chunks if c.chunk_type == ChunkType.METHOD]
    assert len(method_chunks) >= 2, f"Expected ≥2 methods, got {len(method_chunks)}"


def test_go_struct_detected(chunker: CodeChunker) -> None:
    chunks = chunker.chunk_file("repo-1", "service.go", GO_SIMPLE, "go")
    # Either ChunkType.STRUCT or ChunkType.TYPE depending on parser
    struct_or_type = [c for c in chunks if c.chunk_type in (ChunkType.STRUCT, ChunkType.TYPE)]
    assert len(struct_or_type) >= 1, "Expected at least one struct/type chunk"


# ---------------------------------------------------------------------------
# Edge cases
# ---------------------------------------------------------------------------

def test_empty_file_returns_no_chunks(chunker: CodeChunker) -> None:
    chunks = chunker.chunk_file("repo-1", "empty.ts", "", "typescript")
    assert chunks == []


def test_whitespace_only_returns_no_chunks(chunker: CodeChunker) -> None:
    chunks = chunker.chunk_file("repo-1", "blank.ts", "   \n\n\t\n   ", "typescript")
    assert chunks == []


def test_short_file_produces_module_chunk(chunker: CodeChunker) -> None:
    src = "const x = 42;\nconst y = x + 1;"
    chunks = chunker.chunk_file("repo-1", "small.ts", src, "typescript")
    assert len(chunks) == 1
    assert chunks[0].chunk_type == ChunkType.MODULE


def test_oversized_chunk_is_split(chunker: CodeChunker) -> None:
    long_fn = "export function bigFn() {\n" + "  console.log('x');\n" * 500 + "}"
    chunks = chunker.chunk_file("repo-1", "big.ts", long_fn, "typescript")
    for c in chunks:
        assert len(c.content) <= 8500, f"Chunk too large: {len(c.content)} chars"


def test_unknown_language_falls_back_to_line_chunker(chunker: CodeChunker) -> None:
    src = "# Some random config\nkey = value\nanother_key = another_value\n" * 20
    chunks = chunker.chunk_file("repo-1", "config.ini", src, "ini")
    assert len(chunks) >= 1


def test_chunk_repo_isolation(chunker: CodeChunker) -> None:
    """Same file in different repos must produce different chunk IDs."""
    chunks_a = chunker.chunk_file("repo-A", "math.ts", TS_SIMPLE, "typescript")
    chunks_b = chunker.chunk_file("repo-B", "math.ts", TS_SIMPLE, "typescript")
    ids_a = {c.id for c in chunks_a}
    ids_b = {c.id for c in chunks_b}
    assert ids_a.isdisjoint(ids_b), "Repo isolation violated: chunk IDs overlap"
