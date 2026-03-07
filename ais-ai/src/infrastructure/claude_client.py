"""Anthropic Claude API client implementing the LLMPort with streaming."""
from __future__ import annotations

import logging
from typing import AsyncIterator

import anthropic

from ..domain.ports import LLMPort

logger = logging.getLogger(__name__)

DEFAULT_MODEL = "claude-opus-4-20250514"
DEFAULT_MAX_TOKENS = 4096


class ClaudeClient(LLMPort):
    """
    Implements LLMPort using the Anthropic Claude API with streaming.

    Uses claude-opus-4 for its large context window (200k tokens) and
    excellent code understanding capabilities.
    """

    def __init__(
        self,
        api_key: str,
        model: str = DEFAULT_MODEL,
        max_retries: int = 2,
    ) -> None:
        self._client = anthropic.AsyncAnthropic(
            api_key=api_key,
            max_retries=max_retries,
        )
        self._model = model
        logger.info("ClaudeClient initialized with model %s", model)

    async def stream_completion(
        self,
        system_prompt: str,
        messages: list[dict],
        max_tokens: int = DEFAULT_MAX_TOKENS,
    ) -> AsyncIterator[str]:
        """
        Stream completion tokens from Claude.

        Args:
            system_prompt: System-level instruction prompt.
            messages:      Conversation history as list of {"role": ..., "content": ...}.
            max_tokens:    Maximum tokens to generate.

        Yields:
            Individual text tokens as they are streamed from the API.
        """
        logger.debug(
            "Streaming Claude completion: model=%s, messages=%d, max_tokens=%d",
            self._model, len(messages), max_tokens,
        )

        try:
            async with self._client.messages.stream(
                model=self._model,
                max_tokens=max_tokens,
                system=system_prompt,
                messages=messages,
            ) as stream:
                async for text in stream.text_stream:
                    yield text

        except anthropic.RateLimitError as exc:
            logger.error("Claude API rate limit exceeded: %s", exc)
            yield "\n\n[Rate limit reached. Please try again in a moment.]"

        except anthropic.APIStatusError as exc:
            logger.error("Claude API status error %d: %s", exc.status_code, exc.message)
            yield f"\n\n[Claude API error: {exc.message}]"

        except anthropic.APIConnectionError as exc:
            logger.error("Claude API connection error: %s", exc)
            yield "\n\n[Unable to connect to Claude API. Please check your connection.]"

        except Exception as exc:
            logger.error("Unexpected Claude API error: %s", exc)
            yield f"\n\n[Unexpected error: {exc}]"

    async def count_tokens(self, messages: list[dict], system: str = "") -> int:
        """Estimate token count for a set of messages (for logging/monitoring)."""
        try:
            result = await self._client.messages.count_tokens(
                model=self._model,
                system=system,
                messages=messages,
            )
            return result.input_tokens
        except Exception as exc:
            logger.warning("Token counting failed: %s", exc)
            # Rough estimate: 4 chars per token
            total_chars = sum(len(str(m.get("content", ""))) for m in messages)
            return total_chars // 4
