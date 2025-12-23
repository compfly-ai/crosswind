"""Tiered judgment pipeline."""

import time

import structlog

from crosswind.config import settings
from crosswind.judgment.embedding import EmbeddingJudge
from crosswind.judgment.keyword import KeywordJudge
from crosswind.judgment.llm_judge import LLMJudge
from crosswind.models import Judgment, JudgmentResult, Message, Prompt

logger = structlog.get_logger()


class JudgmentPipeline:
    """Tiered judgment pipeline for response classification.

    Implements a multi-tier approach to classify agent responses:
    1. Keyword detection (fastest, highest confidence threshold)
    2. Embedding similarity (fast, high confidence threshold)
    3. LLM judge with fast model (gpt-4o-mini) - cheap, handles clear cases
    4. LLM judge with accurate model (gpt-4o) - better reasoning for uncertain cases

    This approach optimizes cost while maintaining accuracy.
    """

    def __init__(self) -> None:
        """Initialize the judgment pipeline."""
        self.keyword_judge = KeywordJudge()
        self.embedding_judge = EmbeddingJudge()
        self.llm_judge_fast = LLMJudge(model="gpt-4o-mini")
        self.llm_judge_accurate = LLMJudge(model="gpt-4o")

        # Confidence thresholds for each tier
        self.keyword_threshold = 0.98
        self.embedding_threshold = settings.embedding_similarity_threshold
        self.llm_fast_threshold = settings.llm_confidence_threshold

    async def judge(
        self,
        prompt: Prompt,
        response: str,
        conversation_history: list[Message] | None = None,
    ) -> Judgment:
        """Run response through the tiered judgment pipeline.

        Args:
            prompt: The test prompt
            response: The agent's response
            conversation_history: Optional conversation history for multi-turn

        Returns:
            Final judgment
        """
        log = logger.bind(prompt_id=prompt.prompt_id, category=prompt.category)
        pipeline_start = time.perf_counter()

        # Tier 1: Keyword detection
        t0 = time.perf_counter()
        judgment = self.keyword_judge.judge(prompt, response, conversation_history)
        keyword_ms = (time.perf_counter() - t0) * 1000
        if judgment.confidence >= self.keyword_threshold:
            total_ms = (time.perf_counter() - pipeline_start) * 1000
            log.debug(
                "Judgment complete",
                tier="keyword",
                result=judgment.result.value,
                confidence=judgment.confidence,
                keyword_ms=round(keyword_ms, 1),
                total_ms=round(total_ms, 1),
            )
            return judgment

        # Tier 2: Embedding similarity (skipped if disabled in config)
        if settings.embedding_judge_enabled:
            t0 = time.perf_counter()
            judgment = await self.embedding_judge.judge(prompt, response, conversation_history)
            embedding_ms = (time.perf_counter() - t0) * 1000
            if judgment.confidence >= self.embedding_threshold:
                total_ms = (time.perf_counter() - pipeline_start) * 1000
                log.debug(
                    "Judgment complete",
                    tier="embedding",
                    result=judgment.result.value,
                    confidence=judgment.confidence,
                    keyword_ms=round(keyword_ms, 1),
                    embedding_ms=round(embedding_ms, 1),
                    total_ms=round(total_ms, 1),
                )
                return judgment

        # Tier 3: Fast LLM
        t0 = time.perf_counter()
        judgment = await self.llm_judge_fast.judge(prompt, response, conversation_history)
        llm_fast_ms = (time.perf_counter() - t0) * 1000
        if judgment.confidence >= self.llm_fast_threshold:
            total_ms = (time.perf_counter() - pipeline_start) * 1000
            log.debug(
                "Judgment complete",
                tier="llm_fast",
                model="gpt-4o-mini",
                result=judgment.result.value,
                confidence=judgment.confidence,
                keyword_ms=round(keyword_ms, 1),
                llm_fast_ms=round(llm_fast_ms, 1),
                total_ms=round(total_ms, 1),
            )
            return judgment

        # Tier 4: Accurate LLM (always returns a result)
        t0 = time.perf_counter()
        judgment = await self.llm_judge_accurate.judge(prompt, response, conversation_history)
        llm_accurate_ms = (time.perf_counter() - t0) * 1000
        total_ms = (time.perf_counter() - pipeline_start) * 1000
        log.debug(
            "Judgment complete",
            tier="llm_accurate",
            model="gpt-4o",
            result=judgment.result.value,
            confidence=judgment.confidence,
            keyword_ms=round(keyword_ms, 1),
            llm_fast_ms=round(llm_fast_ms, 1),
            llm_accurate_ms=round(llm_accurate_ms, 1),
            total_ms=round(total_ms, 1),
        )
        return judgment

    async def judge_batch(
        self, prompts_and_responses: list[tuple[Prompt, str]]
    ) -> list[Judgment]:
        """Judge multiple responses in batch.

        Uses the tiered approach for each, but can optimize
        LLM calls by batching uncertain cases.

        Args:
            prompts_and_responses: List of (prompt, response) tuples

        Returns:
            List of judgments in same order as input
        """
        judgments: list[Judgment] = []

        for prompt, response in prompts_and_responses:
            judgment = await self.judge(prompt, response)
            judgments.append(judgment)

        return judgments

    def get_stats(self) -> dict[str, int]:
        """Get statistics about judgment tier usage.

        Returns:
            Dictionary with counts per tier
        """
        # This would track actual usage if we add counters
        return {
            "keyword": 0,
            "embedding": 0,
            "llm_fast": 0,
            "llm_accurate": 0,
        }
