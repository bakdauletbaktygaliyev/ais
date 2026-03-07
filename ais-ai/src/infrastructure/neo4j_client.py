"""Neo4j client implementing the GraphContextPort for the AI service."""
from __future__ import annotations

import logging

from neo4j import AsyncGraphDatabase, AsyncDriver

from ..domain.chunk import GraphContext
from ..domain.ports import GraphContextPort

logger = logging.getLogger(__name__)


class Neo4jGraphContextClient(GraphContextPort):
    """
    Reads graph context from Neo4j for use in RAG prompt construction.

    The AI service only READS from Neo4j — it never writes.
    All writes are done by ais-back.
    """

    def __init__(self, uri: str, user: str, password: str, database: str = "neo4j") -> None:
        self._driver: AsyncDriver = AsyncGraphDatabase.driver(
            uri, auth=(user, password)
        )
        self._database = database
        logger.info("Neo4jGraphContextClient initialized: %s", uri)

    async def close(self) -> None:
        await self._driver.close()

    async def get_context(self, repo_id: str, node_id: str) -> GraphContext | None:
        """Fetch graph context for a node identified by its graph ID."""
        query = """
        MATCH (n {id: $nodeId})
        OPTIONAL MATCH (n)-[:IMPORTS]->(dep:File)
        OPTIONAL MATCH (importer:File)-[:IMPORTS]->(n)
        OPTIONAL MATCH (caller:Function)-[:CALLS]->(fn:Function)
            WHERE (n)-[:HAS_FUNCTION]->(fn)
        RETURN
            n.id AS nodeId,
            n.path AS path,
            n.fanIn AS fanIn,
            n.fanOut AS fanOut,
            n.hasCycle AS hasCycle,
            collect(DISTINCT dep.path) AS imports,
            collect(DISTINCT importer.path) AS importedBy,
            collect(DISTINCT caller.name) AS callers
        LIMIT 1
        """
        try:
            async with self._driver.session(database=self._database) as session:
                result = await session.run(query, nodeId=node_id)
                record = await result.single()
                if not record:
                    return None
                return self._record_to_context(record)
        except Exception as exc:
            logger.error("Neo4j get_context failed for node %s: %s", node_id, exc)
            return None

    async def get_context_by_path(self, repo_id: str, file_path: str) -> GraphContext | None:
        """Fetch graph context for a file identified by its path."""
        query = """
        MATCH (n:File {repoId: $repoId, path: $path})
        OPTIONAL MATCH (n)-[:IMPORTS]->(dep:File)
        OPTIONAL MATCH (importer:File)-[:IMPORTS]->(n)
        OPTIONAL MATCH (fn:Function)<-[:HAS_FUNCTION]-(n)
        OPTIONAL MATCH (caller:Function)-[:CALLS]->(fn)
        RETURN
            n.id AS nodeId,
            n.path AS path,
            n.fanIn AS fanIn,
            n.fanOut AS fanOut,
            n.hasCycle AS hasCycle,
            collect(DISTINCT dep.path) AS imports,
            collect(DISTINCT importer.path) AS importedBy,
            collect(DISTINCT caller.name) AS callers
        LIMIT 1
        """
        try:
            async with self._driver.session(database=self._database) as session:
                result = await session.run(query, repoId=repo_id, path=file_path)
                record = await result.single()
                if not record:
                    return None
                return self._record_to_context(record)
        except Exception as exc:
            logger.error(
                "Neo4j get_context_by_path failed for %s/%s: %s",
                repo_id, file_path, exc,
            )
            return None

    @staticmethod
    def _record_to_context(record: object) -> GraphContext:
        """Convert a Neo4j record to a GraphContext domain object."""
        return GraphContext(
            node_id=record["nodeId"] or "",
            node_path=record["path"] or "",
            imports=[p for p in (record["imports"] or []) if p],
            imported_by=[p for p in (record["importedBy"] or []) if p],
            callers=[c for c in (record["callers"] or []) if c],
            cycle_member=record["hasCycle"] or False,
            fan_in=record["fanIn"] or 0,
            fan_out=record["fanOut"] or 0,
        )
