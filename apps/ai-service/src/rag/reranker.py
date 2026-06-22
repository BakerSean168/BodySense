"""Reranker module for refining retrieval results."""

import os
from dataclasses import dataclass
from typing import Optional

from openai import AsyncOpenAI

from .retriever import RetrievalResult


@dataclass
class RerankResult:
    """Result after reranking."""

    result: RetrievalResult
    relevance_score: float


class Reranker:
    """Rerank retrieval results using LLM for better relevance."""

    def __init__(
        self,
        model: Optional[str] = None,
        api_key: Optional[str] = None,
        base_url: Optional[str] = None,
    ):
        self.model = model or os.getenv("LLM_MODEL", "gpt-4o-mini")
        self.api_key = (
            api_key
            or os.getenv("LLM_API_KEY")
            or os.getenv("OPENAI_API_KEY")
        )
        self.base_url = base_url or os.getenv("LLM_BASE_URL")
        self._client: Optional[AsyncOpenAI] = None

    @property
    def client(self) -> AsyncOpenAI:
        """Lazy initialization of OpenAI-compatible client."""
        if self._client is None:
            if not self.api_key:
                raise ValueError("LLM_API_KEY or OPENAI_API_KEY is required")
            kwargs = {"api_key": self.api_key}
            if self.base_url:
                kwargs["base_url"] = self.base_url
            self._client = AsyncOpenAI(**kwargs)
        return self._client

    async def rerank(
        self,
        query: str,
        candidates: list[RetrievalResult],
        top_n: int = 3,
    ) -> list[RetrievalResult]:
        """
        Rerank candidates based on relevance to query.

        Args:
            query: The original search query.
            candidates: List of candidates from initial retrieval.
            top_n: Number of top results to return.

        Returns:
            Reranked list of results.
        """
        if not candidates:
            return []

        if len(candidates) <= top_n:
            # No need to rerank if we have fewer candidates than requested
            return candidates

        # Prepare candidates text for LLM
        candidates_text = ""
        for i, candidate in enumerate(candidates):
            candidates_text += f"\n--- Candidate {i + 1} ---\n"
            candidates_text += f"Title: {candidate.title}\n"
            candidates_text += f"Category: {candidate.category}\n"
            candidates_text += f"Content: {candidate.content[:500]}...\n"

        prompt = f"""You are a relevance ranking assistant.
Given a user query and a list of candidates,
rank them by relevance to the query.

User Query: {query}

Candidates:
{candidates_text}

Return ONLY a JSON array of candidate numbers (1-indexed)
in order of relevance, most relevant first.
Return exactly {top_n} numbers.
Example: [3, 1, 5]

Do not include any other text, just the JSON array."""

        try:
            response = await self.client.chat.completions.create(
                model=self.model,
                messages=[
                    {
                        "role": "system",
                        "content": (
                            "You are a helpful assistant that ranks candidates by relevance."
                        ),
                    },
                    {"role": "user", "content": prompt},
                ],
                temperature=0,
                max_tokens=100,
            )

            # Parse response
            content = response.choices[0].message.content.strip()
            # Extract JSON array from response
            import json
            # Handle potential markdown code blocks
            if "```" in content:
                content = content.split("```")[1]
                if content.startswith("json"):
                    content = content[4:]
                content = content.strip()

            ranked_indices = json.loads(content)

            # Validate and convert to 0-indexed
            ranked_indices = [i - 1 for i in ranked_indices if 1 <= i <= len(candidates)]

            # Return reranked results
            reranked = []
            for idx in ranked_indices[:top_n]:
                if 0 <= idx < len(candidates):
                    reranked.append(candidates[idx])

            return reranked if reranked else candidates[:top_n]

        except Exception:
            # Fallback: return original order if reranking fails
            return candidates[:top_n]

    async def rerank_with_scores(
        self,
        query: str,
        candidates: list[RetrievalResult],
        top_n: int = 3,
    ) -> list[RerankResult]:
        """
        Rerank candidates and return with relevance scores.

        Args:
            query: The original search query.
            candidates: List of candidates from initial retrieval.
            top_n: Number of top results to return.

        Returns:
            List of RerankResult with relevance scores.
        """
        reranked = await self.rerank(query, candidates, top_n)

        results = []
        for i, result in enumerate(reranked):
            # Assign decreasing scores based on rank
            score = 1.0 - (i * 0.1)
            results.append(RerankResult(result=result, relevance_score=score))

        return results


# Singleton instance
_default_reranker: Optional[Reranker] = None


def get_reranker() -> Reranker:
    """Get or create the default reranker."""
    global _default_reranker
    if _default_reranker is None:
        _default_reranker = Reranker()
    return _default_reranker
