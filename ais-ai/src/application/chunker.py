"""
AST-based code chunker. Splits source files at function/class/struct boundaries
using Tree-sitter grammars so every chunk is a semantically complete unit.
"""
from __future__ import annotations

import hashlib
import logging
from dataclasses import dataclass
from typing import Iterator

from ..domain.chunk import ChunkType, CodeChunk

logger = logging.getLogger(__name__)

# Maximum characters per chunk before we force-split (safety guard)
MAX_CHUNK_CHARS = 8000
# Minimum characters to bother indexing a chunk
MIN_CHUNK_CHARS = 30


@dataclass
class _RawChunk:
    name: str
    chunk_type: ChunkType
    start_line: int
    end_line: int
    content: str


class CodeChunker:
    """
    Splits source files into semantically meaningful chunks using Tree-sitter.

    Falls back to line-based chunking when Tree-sitter parsing fails or the
    language is not supported.
    """

    def __init__(self) -> None:
        self._ts_parsers: dict[str, object] = {}
        self._init_parsers()

    def _init_parsers(self) -> None:
        """Initialize Tree-sitter language parsers."""
        try:
            import tree_sitter_typescript as ts_ts
            import tree_sitter_javascript as ts_js
            import tree_sitter_go as ts_go
            from tree_sitter import Language, Parser

            self._ts_parsers["typescript"] = Parser(Language(ts_ts.language_typescript()))
            self._ts_parsers["javascript"] = Parser(Language(ts_js.language()))
            self._ts_parsers["go"] = Parser(Language(ts_go.language()))

            logger.info("Tree-sitter parsers initialized: %s", list(self._ts_parsers.keys()))
        except Exception as exc:
            logger.warning("Tree-sitter init failed, falling back to line chunker: %s", exc)

    def chunk_file(
        self,
        repo_id: str,
        file_path: str,
        content: str,
        language: str,
    ) -> list[CodeChunk]:
        """
        Chunk a single source file into CodeChunk objects.

        Args:
            repo_id:   Repository identifier.
            file_path: Relative path of the file within the repository.
            content:   Full source text of the file.
            language:  Detected language ('typescript', 'javascript', 'go').

        Returns:
            List of CodeChunk objects, one per semantic unit.
        """
        if not content or not content.strip():
            return []

        lang_key = language.lower()
        raw_chunks: list[_RawChunk] = []

        if lang_key in self._ts_parsers:
            try:
                raw_chunks = list(self._extract_with_treesitter(content, lang_key))
            except Exception as exc:
                logger.warning("Tree-sitter chunking failed for %s: %s", file_path, exc)

        if not raw_chunks:
            raw_chunks = list(self._fallback_chunk(content, file_path))

        chunks: list[CodeChunk] = []
        for raw in raw_chunks:
            chunk_content = raw.content.strip()
            if len(chunk_content) < MIN_CHUNK_CHARS:
                continue
            # Force-split oversized chunks
            for part_idx, part_content in enumerate(self._split_if_oversized(chunk_content)):
                chunk_id = self._make_id(repo_id, file_path, raw.start_line + part_idx)
                chunks.append(
                    CodeChunk(
                        id=chunk_id,
                        repo_id=repo_id,
                        file_path=file_path,
                        chunk_type=raw.chunk_type,
                        name=raw.name if part_idx == 0 else f"{raw.name}[{part_idx}]",
                        content=part_content,
                        start_line=raw.start_line,
                        end_line=raw.end_line,
                        language=language,
                    )
                )

        # If no semantic chunks found, create one module-level chunk for the whole file
        if not chunks and len(content.strip()) >= MIN_CHUNK_CHARS:
            lines = content.split("\n")
            truncated = "\n".join(lines[:200]) if len(lines) > 200 else content
            chunks.append(
                CodeChunk(
                    id=self._make_id(repo_id, file_path, 0),
                    repo_id=repo_id,
                    file_path=file_path,
                    chunk_type=ChunkType.MODULE,
                    name=file_path.split("/")[-1],
                    content=truncated,
                    start_line=1,
                    end_line=len(lines),
                    language=language,
                )
            )

        return chunks

    def _extract_with_treesitter(
        self, content: str, lang_key: str
    ) -> Iterator[_RawChunk]:
        """Extract function/class chunks using Tree-sitter AST."""
        parser = self._ts_parsers[lang_key]
        tree = parser.parse(content.encode("utf-8", errors="replace"))
        root = tree.root_node
        lines = content.split("\n")

        yield from self._walk_node(root, lines, lang_key)

    def _walk_node(
        self, node: object, lines: list[str], lang: str
    ) -> Iterator[_RawChunk]:
        """Recursively walk AST and yield chunks at function/class/struct nodes."""
        from tree_sitter import Node

        assert isinstance(node, Node)

        chunk_node_types_ts = {
            "function_declaration",
            "arrow_function",
            "method_definition",
            "class_declaration",
            "interface_declaration",
            "type_alias_declaration",
            "lexical_declaration",
        }
        chunk_node_types_go = {
            "function_declaration",
            "method_declaration",
            "type_declaration",
        }
        chunk_node_types = chunk_node_types_ts | chunk_node_types_go

        if node.type in chunk_node_types:
            name = self._extract_node_name(node, lang)
            chunk_type = self._node_type_to_chunk_type(node.type)
            start_line = node.start_point[0] + 1
            end_line = node.end_point[0] + 1
            chunk_content = "\n".join(lines[node.start_point[0]: node.end_point[0] + 1])

            if name and chunk_content.strip():
                yield _RawChunk(
                    name=name,
                    chunk_type=chunk_type,
                    start_line=start_line,
                    end_line=end_line,
                    content=chunk_content,
                )
                return  # Don't recurse into already-chunked node

        for child in node.children:
            yield from self._walk_node(child, lines, lang)

    def _extract_node_name(self, node: object, lang: str) -> str:
        """Extract the identifier name from a declaration node."""
        from tree_sitter import Node
        assert isinstance(node, Node)

        identifier_types = {"identifier", "type_identifier", "field_identifier", "property_identifier"}

        for child in node.children:
            if child.type in identifier_types:
                return child.text.decode("utf-8", errors="replace")

        # For arrow functions assigned to const: look at parent's declarator
        return ""

    def _node_type_to_chunk_type(self, node_type: str) -> ChunkType:
        mapping = {
            "function_declaration": ChunkType.FUNCTION,
            "arrow_function": ChunkType.FUNCTION,
            "method_definition": ChunkType.METHOD,
            "method_declaration": ChunkType.METHOD,
            "class_declaration": ChunkType.CLASS,
            "interface_declaration": ChunkType.INTERFACE,
            "type_alias_declaration": ChunkType.TYPE,
            "type_declaration": ChunkType.TYPE,
            "lexical_declaration": ChunkType.FUNCTION,
        }
        return mapping.get(node_type, ChunkType.MODULE)

    def _fallback_chunk(self, content: str, file_path: str) -> Iterator[_RawChunk]:
        """
        Line-based fallback chunker when Tree-sitter is unavailable.
        Tries to split at blank lines between logical blocks.
        """
        lines = content.split("\n")
        current_lines: list[str] = []
        current_start = 1
        blank_count = 0

        for i, line in enumerate(lines, start=1):
            stripped = line.strip()
            if not stripped:
                blank_count += 1
            else:
                blank_count = 0

            current_lines.append(line)

            # Split at double blank lines or when chunk gets large
            should_split = (
                (blank_count >= 2 and len(current_lines) > 10)
                or len("\n".join(current_lines)) > MAX_CHUNK_CHARS
            )

            if should_split and current_lines:
                chunk_content = "\n".join(current_lines).strip()
                if chunk_content:
                    name = f"{file_path.split('/')[-1]}:L{current_start}"
                    yield _RawChunk(
                        name=name,
                        chunk_type=ChunkType.MODULE,
                        start_line=current_start,
                        end_line=i,
                        content=chunk_content,
                    )
                current_lines = []
                current_start = i + 1
                blank_count = 0

        # Remaining lines
        if current_lines:
            chunk_content = "\n".join(current_lines).strip()
            if chunk_content:
                name = f"{file_path.split('/')[-1]}:L{current_start}"
                yield _RawChunk(
                    name=name,
                    chunk_type=ChunkType.MODULE,
                    start_line=current_start,
                    end_line=len(lines),
                    content=chunk_content,
                )

    def _split_if_oversized(self, content: str) -> list[str]:
        """Split a chunk that exceeds MAX_CHUNK_CHARS into sub-chunks."""
        if len(content) <= MAX_CHUNK_CHARS:
            return [content]

        lines = content.split("\n")
        parts: list[str] = []
        current: list[str] = []
        current_len = 0

        for line in lines:
            line_len = len(line) + 1
            if current_len + line_len > MAX_CHUNK_CHARS and current:
                parts.append("\n".join(current))
                current = [line]
                current_len = line_len
            else:
                current.append(line)
                current_len += line_len

        if current:
            parts.append("\n".join(current))

        return parts if parts else [content]

    @staticmethod
    def _make_id(repo_id: str, file_path: str, line: int) -> str:
        """Generate a stable, unique chunk ID."""
        raw = f"{repo_id}:{file_path}:{line}"
        return hashlib.sha256(raw.encode()).hexdigest()[:24]
