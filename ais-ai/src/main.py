"""
ais-ai service entry point.

Wires all infrastructure adapters and application services,
then starts the gRPC server.
"""
from __future__ import annotations

import asyncio
import logging
import os
import sys

import structlog


def _configure_logging(level: str, fmt: str) -> None:
    log_level = getattr(logging, level.upper(), logging.INFO)

    if fmt == "json":
        structlog.configure(
            processors=[
                structlog.contextvars.merge_contextvars,
                structlog.processors.add_log_level,
                structlog.processors.TimeStamper(fmt="iso"),
                structlog.processors.JSONRenderer(),
            ],
            wrapper_class=structlog.make_filtering_bound_logger(log_level),
            logger_factory=structlog.PrintLoggerFactory(),
        )
    else:
        logging.basicConfig(
            level=log_level,
            format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
            stream=sys.stdout,
        )

    logging.getLogger("grpc").setLevel(logging.WARNING)
    logging.getLogger("neo4j").setLevel(logging.WARNING)
    logging.getLogger("httpx").setLevel(logging.WARNING)


logger = logging.getLogger(__name__)


async def main() -> None:
    # ---- Load settings ----
    from .config.settings import get_settings
    settings = get_settings()

    _configure_logging(settings.log_level, settings.log_format)

    logger.info(
        "Starting ais-ai service on port %d (model: %s)",
        settings.grpc_port,
        settings.claude_model,
    )

    # ---- Infrastructure: Voyage AI ----
    from .infrastructure.voyage_client import VoyageEmbeddingClient
    embedding_client = VoyageEmbeddingClient(
        api_key=settings.voyage_api_key,
        model=settings.embedding_model,
    )

    # ---- Infrastructure: Qdrant ----
    from .infrastructure.qdrant_client import QdrantVectorStore
    vector_store = QdrantVectorStore(
        url=settings.qdrant_url,
        api_key=settings.qdrant_api_key,
    )

    # ---- Infrastructure: Neo4j ----
    from .infrastructure.neo4j_client import Neo4jGraphContextClient
    graph_client = Neo4jGraphContextClient(
        uri=settings.neo4j_uri,
        user=settings.neo4j_user,
        password=settings.neo4j_password,
        database=settings.neo4j_database,
    )

    # ---- Infrastructure: Claude ----
    from .infrastructure.claude_client import ClaudeClient
    llm_client = ClaudeClient(
        api_key=settings.anthropic_api_key,
        model=settings.claude_model,
    )

    # ---- Application layer ----
    from .application.indexing_service import IndexingService
    from .application.chat_service import ChatService
    from .application.search_service import SearchService

    indexing_service = IndexingService(
        embedding_port=embedding_client,
        vector_store_port=vector_store,
    )

    chat_service = ChatService(
        embedding_port=embedding_client,
        vector_store_port=vector_store,
        graph_context_port=graph_client,
        llm_port=llm_client,
    )

    search_service = SearchService(
        embedding_port=embedding_client,
        vector_store_port=vector_store,
    )

    # ---- Delivery: gRPC server ----
    from .delivery.grpc_server import AIServicer, serve

    servicer = AIServicer(
        indexing_service=indexing_service,
        chat_service=chat_service,
        search_service=search_service,
    )

    try:
        await serve(
            servicer=servicer,
            port=settings.grpc_port,
            max_workers=settings.grpc_max_workers,
        )
    finally:
        await graph_client.close()
        logger.info("ais-ai service stopped")


if __name__ == "__main__":
    asyncio.run(main())
