"""
gRPC server implementing the AIService contract.

Handles three RPCs:
  - IndexRepository (unary): chunk + embed + store
  - Chat (server-streaming): RAG pipeline with token streaming
  - SearchSimilar (unary): semantic code search
"""
from __future__ import annotations

import asyncio
import logging
from concurrent import futures

import grpc

from ..application.chat_service import ChatMessage, ChatService
from ..application.indexing_service import FileInput, IndexingService
from ..application.search_service import SearchService
from ..proto import ai_pb2, ai_pb2_grpc

logger = logging.getLogger(__name__)


class AIServicer(ai_pb2_grpc.AIServiceServicer):
    """
    gRPC servicer that implements the AIService protobuf contract.
    Delegates to application-layer services.
    """

    def __init__(
        self,
        indexing_service: IndexingService,
        chat_service: ChatService,
        search_service: SearchService,
    ) -> None:
        self._indexing = indexing_service
        self._chat = chat_service
        self._search = search_service

    async def IndexRepository(
        self,
        request: ai_pb2.IndexRequest,
        context: grpc.aio.ServicerContext,
    ) -> ai_pb2.IndexResponse:
        """
        Unary RPC: receive repository files, chunk, embed, store in Qdrant.
        """
        repo_id = request.repo_id
        if not repo_id:
            await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "repo_id is required")
            return ai_pb2.IndexResponse(success=False, error="repo_id is required")

        files = [
            FileInput(
                path=f.path,
                content=f.content,
                language=f.language,
            )
            for f in request.files
        ]

        logger.info("IndexRepository RPC: repo=%s files=%d", repo_id, len(files))

        try:
            chunks_indexed = await self._indexing.index_repository(repo_id, files)
            return ai_pb2.IndexResponse(
                success=True,
                chunks_indexed=chunks_indexed,
            )
        except Exception as exc:
            logger.error("IndexRepository failed for repo %s: %s", repo_id, exc)
            return ai_pb2.IndexResponse(
                success=False,
                chunks_indexed=0,
                error=str(exc),
            )

    async def Chat(
        self,
        request: ai_pb2.ChatRequest,
        context: grpc.aio.ServicerContext,
    ) -> None:
        """
        Server-streaming RPC: stream Claude response tokens.
        """
        repo_id = request.repo_id
        message = request.message

        if not repo_id or not message:
            await context.abort(
                grpc.StatusCode.INVALID_ARGUMENT,
                "repo_id and message are required",
            )
            return

        history = [
            ChatMessage(role=m.role, content=m.content)
            for m in request.history
        ]

        node_id = request.node_id or None

        logger.info(
            "Chat RPC: repo=%s node=%s message=%s...",
            repo_id, node_id, message[:60],
        )

        try:
            async for token in self._chat.stream_chat(
                repo_id=repo_id,
                message=message,
                node_id=node_id,
                history=history,
            ):
                chunk = ai_pb2.ChatChunk(token=token, done=False)
                await context.write(chunk)

                # Check if client disconnected
                if context.done():
                    logger.info("Chat RPC client disconnected mid-stream")
                    return

            # Send final done marker
            await context.write(ai_pb2.ChatChunk(token="", done=True))

        except Exception as exc:
            logger.error("Chat RPC stream error for repo %s: %s", repo_id, exc)
            try:
                await context.write(
                    ai_pb2.ChatChunk(
                        token=f"\n\n[Stream error: {exc}]",
                        done=True,
                    )
                )
            except Exception:
                pass

    async def SearchSimilar(
        self,
        request: ai_pb2.SearchRequest,
        context: grpc.aio.ServicerContext,
    ) -> ai_pb2.SearchResponse:
        """
        Unary RPC: perform semantic similarity search.
        """
        repo_id = request.repo_id
        query = request.query
        top_k = request.top_k or 5

        if not repo_id or not query:
            await context.abort(
                grpc.StatusCode.INVALID_ARGUMENT,
                "repo_id and query are required",
            )
            return ai_pb2.SearchResponse()

        logger.info("SearchSimilar RPC: repo=%s top_k=%d query=%s...", repo_id, top_k, query[:60])

        try:
            results = await self._search.search_similar(
                repo_id=repo_id,
                query=query,
                top_k=top_k,
            )

            pb_results = [
                ai_pb2.SearchResult(
                    chunk_id=r.chunk.id,
                    file_path=r.chunk.file_path,
                    content=r.chunk.content,
                    score=r.score,
                    start_line=r.chunk.start_line,
                    end_line=r.chunk.end_line,
                    chunk_type=r.chunk.chunk_type.value,
                    name=r.chunk.name,
                )
                for r in results
            ]

            return ai_pb2.SearchResponse(results=pb_results)

        except Exception as exc:
            logger.error("SearchSimilar failed for repo %s: %s", repo_id, exc)
            await context.abort(grpc.StatusCode.INTERNAL, str(exc))
            return ai_pb2.SearchResponse()


async def serve(
    servicer: AIServicer,
    port: int = 50051,
    max_workers: int = 10,
) -> None:
    """Start the gRPC server and block until shutdown signal."""
    server = grpc.aio.server(
        options=[
            ("grpc.max_send_message_length", 64 * 1024 * 1024),
            ("grpc.max_receive_message_length", 64 * 1024 * 1024),
            ("grpc.keepalive_time_ms", 30_000),
            ("grpc.keepalive_timeout_ms", 10_000),
            ("grpc.keepalive_permit_without_calls", True),
            ("grpc.http2.max_pings_without_data", 0),
        ]
    )

    ai_pb2_grpc.add_AIServiceServicer_to_server(servicer, server)

    listen_addr = f"[::]:{port}"
    server.add_insecure_port(listen_addr)

    logger.info("Starting gRPC server on %s", listen_addr)
    await server.start()

    logger.info("gRPC server started and ready")

    try:
        await server.wait_for_termination()
    except (KeyboardInterrupt, asyncio.CancelledError):
        logger.info("Shutting down gRPC server...")
        await server.stop(grace=5)
        logger.info("gRPC server stopped")
