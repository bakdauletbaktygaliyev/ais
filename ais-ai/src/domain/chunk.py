from __future__ import annotations

from dataclasses import dataclass, field
from enum import Enum
from typing import Any


class ChunkType(str, Enum):
    FUNCTION = "function"
    CLASS = "class"
    INTERFACE = "interface"
    MODULE = "module"
    METHOD = "method"
    STRUCT = "struct"
    TYPE = "type"


@dataclass
class CodeChunk:
    """A semantically meaningful unit of code extracted from a source file."""

    id: str
    repo_id: str
    file_path: str
    chunk_type: ChunkType
    name: str
    content: str
    start_line: int
    end_line: int
    language: str
    metadata: dict[str, Any] = field(default_factory=dict)

    def __post_init__(self) -> None:
        if not self.id:
            raise ValueError("CodeChunk.id must not be empty")
        if not self.repo_id:
            raise ValueError("CodeChunk.repo_id must not be empty")
        if not self.content:
            raise ValueError("CodeChunk.content must not be empty")

    @property
    def token_estimate(self) -> int:
        """Rough estimate of tokens in this chunk (4 chars ≈ 1 token)."""
        return len(self.content) // 4

    @property
    def line_count(self) -> int:
        return self.end_line - self.start_line + 1

    def to_qdrant_payload(self) -> dict[str, Any]:
        """Serializes this chunk to a Qdrant point payload."""
        return {
            "chunk_id": self.id,
            "repo_id": self.repo_id,
            "file_path": self.file_path,
            "chunk_type": self.chunk_type.value,
            "name": self.name,
            "content": self.content,
            "start_line": self.start_line,
            "end_line": self.end_line,
            "language": self.language,
            **self.metadata,
        }

    @classmethod
    def from_qdrant_payload(cls, payload: dict[str, Any]) -> "CodeChunk":
        """Reconstructs a CodeChunk from a Qdrant point payload."""
        known_keys = {
            "chunk_id", "repo_id", "file_path", "chunk_type",
            "name", "content", "start_line", "end_line", "language",
        }
        metadata = {k: v for k, v in payload.items() if k not in known_keys}
        return cls(
            id=payload["chunk_id"],
            repo_id=payload["repo_id"],
            file_path=payload["file_path"],
            chunk_type=ChunkType(payload.get("chunk_type", "module")),
            name=payload.get("name", ""),
            content=payload.get("content", ""),
            start_line=payload.get("start_line", 0),
            end_line=payload.get("end_line", 0),
            language=payload.get("language", ""),
            metadata=metadata,
        )


@dataclass
class SearchResult:
    """A semantically similar chunk returned by vector search."""

    chunk: CodeChunk
    score: float

    def __lt__(self, other: "SearchResult") -> bool:
        return self.score > other.score  # higher score = more relevant


@dataclass
class GraphContext:
    """Graph-based context fetched from Neo4j for a specific node."""

    node_id: str
    node_path: str
    imports: list[str] = field(default_factory=list)
    imported_by: list[str] = field(default_factory=list)
    callers: list[str] = field(default_factory=list)
    callees: list[str] = field(default_factory=list)
    cycle_member: bool = False
    fan_in: int = 0
    fan_out: int = 0

    def to_context_string(self) -> str:
        """Formats graph context as a readable string for prompt injection."""
        lines = [f"File: {self.node_path}"]

        if self.imports:
            lines.append(f"Imports: {', '.join(self.imports[:10])}")
        if self.imported_by:
            lines.append(f"Imported by: {', '.join(self.imported_by[:10])}")
        if self.callers:
            lines.append(f"Callers: {', '.join(self.callers[:10])}")
        if self.callees:
            lines.append(f"Calls: {', '.join(self.callees[:10])}")
        if self.cycle_member:
            lines.append("⚠️ This file is part of a circular dependency")
        lines.append(f"Fan-in: {self.fan_in}, Fan-out: {self.fan_out}")

        return "\n".join(lines)
