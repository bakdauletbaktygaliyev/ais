from __future__ import annotations

from pydantic import Field
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """All configuration for the ais-ai service, loaded from environment variables."""

    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        case_sensitive=False,
        extra="ignore",
    )

    # gRPC
    grpc_port: int = Field(default=50051, description="Port for the gRPC server")
    grpc_max_workers: int = Field(default=10, description="Thread pool size for gRPC")

    # Qdrant
    qdrant_url: str = Field(default="http://localhost:6333")
    qdrant_api_key: str | None = Field(default=None)

    # Neo4j
    neo4j_uri: str = Field(default="bolt://localhost:7687")
    neo4j_user: str = Field(default="neo4j")
    neo4j_password: str = Field(default="password")
    neo4j_database: str = Field(default="neo4j")

    # Voyage AI
    voyage_api_key: str = Field(..., description="Voyage AI API key (required)")
    embedding_model: str = Field(default="voyage-code-2")

    # Anthropic
    anthropic_api_key: str = Field(..., description="Anthropic API key (required)")
    claude_model: str = Field(default="claude-opus-4-20250514")

    # Logging
    log_level: str = Field(default="INFO")
    log_format: str = Field(default="json")


_settings: Settings | None = None


def get_settings() -> Settings:
    global _settings
    if _settings is None:
        _settings = Settings()
    return _settings
