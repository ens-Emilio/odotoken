"""In-memory token bucket rate limiter per project_id."""
from __future__ import annotations

import time


class TokenBucket:
    """Per-project token bucket. Refill continuously."""

    def __init__(self, requests_per_min: int = 0, tokens_per_min: int = 0):
        # capacity = max configured value; prefer requests_per_min if set
        if requests_per_min and requests_per_min > 0:
            self.capacity = float(requests_per_min)
            self.refill_per_sec = requests_per_min / 60.0
        elif tokens_per_min and tokens_per_min > 0:
            self.capacity = float(tokens_per_min)
            self.refill_per_sec = tokens_per_min / 60.0
        else:
            self.capacity = 0  # 0 means disabled (no limit)
            self.refill_per_sec = 0
        self._buckets: dict[str, list[float]] = {}  # project_id -> [tokens, last_ts]

    @property
    def enabled(self) -> bool:
        return self.capacity > 0

    def _refill(self, project_id: str, now: float) -> float:
        tokens, last = self._buckets.get(project_id, [self.capacity, now])
        elapsed = now - last
        tokens = min(self.capacity, tokens + elapsed * self.refill_per_sec)
        return tokens

    def consume(self, project_id: str = "default", tokens: float = 1) -> bool:
        """Try to consume tokens. Returns True if allowed, False if rate-limited."""
        if not self.enabled:
            return True
        now = time.monotonic()
        cur = self._refill(project_id, now)
        if cur >= tokens:
            self._buckets[project_id] = [cur - tokens, now]
            return True
        # not enough — update refill but don't deduct
        self._buckets[project_id] = [cur, now]
        return False

    def check(self, project_id: str = "default") -> bool:
        """Alias for consume(1)."""
        return self.consume(project_id, 1)

    def retry_after(self, project_id: str = "default") -> float:
        """Seconds until 1 token available."""
        if not self.enabled:
            return 0
        tokens, _ = self._buckets.get(project_id, [self.capacity, time.monotonic()])
        if tokens >= 1:
            return 0
        return (1 - tokens) / self.refill_per_sec if self.refill_per_sec else 0
