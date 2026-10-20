"""Budget guard and alerts system for odotoken.
Allows setting cost and token thresholds (warning and hard cutoff limits).
"""

from __future__ import annotations

import logging
from dataclasses import dataclass
from datetime import UTC

from odotoken.db import Database

logger = logging.getLogger("odotoken.guard")


@dataclass
class GuardLimits:
    daily_cost_limit_usd: float | None = None
    monthly_cost_limit_usd: float | None = None
    daily_tokens_limit: int | None = None
    monthly_tokens_limit: int | None = None
    action_on_breach: str = "alert"  # "alert" or "block"
    webhook_url: str | None = None


@dataclass
class GuardCheckResult:
    allowed: bool = True
    reason: str = ""
    status: str = "ok"  # "ok", "warning", "breached"
    current_daily_cost: float = 0.0
    current_monthly_cost: float = 0.0
    current_daily_tokens: int = 0
    current_monthly_tokens: int = 0


class Guard:
    def __init__(self, db: Database, limits: GuardLimits | None = None):
        self.db = db
        self.limits = limits or GuardLimits()

    def update_limits(self, limits: GuardLimits) -> None:
        self.limits = limits

    def check_allowance(self) -> GuardCheckResult:
        """Evaluate current usage against configured budget limits."""
        res = GuardCheckResult(allowed=True, status="ok")

        # ISO beginning of today (UTC)
        from datetime import datetime

        now = datetime.now(UTC)
        today_start = f"{now.strftime('%Y-%m-%d')}T00:00:00"
        month_start = f"{now.strftime('%Y-%m')}-01T00:00:00"

        daily_stats = self.db.get_summary(since=today_start)
        monthly_stats = self.db.get_summary(since=month_start)

        res.current_daily_cost = float(daily_stats.get("total_cost_usd", 0.0))
        res.current_monthly_cost = float(monthly_stats.get("total_cost_usd", 0.0))
        res.current_daily_tokens = int(daily_stats.get("total_tokens", 0))
        res.current_monthly_tokens = int(monthly_stats.get("total_tokens", 0))

        # Check daily cost
        if self.limits.daily_cost_limit_usd and res.current_daily_cost >= self.limits.daily_cost_limit_usd:
            msg = (
                f"Daily budget limit breached: ${res.current_daily_cost:.2f} "
                f">= ${self.limits.daily_cost_limit_usd:.2f}"
            )
            if self.limits.action_on_breach == "block":
                res.allowed = False
                res.status = "breached"
                res.reason = msg
                return res
            else:
                res.status = "warning"
                res.reason = msg

        # Check monthly cost
        if (
            self.limits.monthly_cost_limit_usd
            and res.current_monthly_cost >= self.limits.monthly_cost_limit_usd
        ):
            msg = (
                f"Monthly budget limit breached: ${res.current_monthly_cost:.2f} "
                f">= ${self.limits.monthly_cost_limit_usd:.2f}"
            )
            if self.limits.action_on_breach == "block":
                res.allowed = False
                res.status = "breached"
                res.reason = msg
                return res
            else:
                res.status = "warning"
                res.reason = msg

        # Check daily tokens
        if self.limits.daily_tokens_limit and res.current_daily_tokens >= self.limits.daily_tokens_limit:
            msg = (
                f"Daily token limit breached: {res.current_daily_tokens} >= {self.limits.daily_tokens_limit}"
            )
            if self.limits.action_on_breach == "block":
                res.allowed = False
                res.status = "breached"
                res.reason = msg
                return res
            else:
                res.status = "warning"
                res.reason = msg

        # Check monthly tokens
        if self.limits.monthly_tokens_limit and res.current_monthly_tokens >= self.limits.monthly_tokens_limit:
            msg = (
                f"Monthly token limit breached: {res.current_monthly_tokens} >= {self.limits.monthly_tokens_limit}"
            )
            if self.limits.action_on_breach == "block":
                res.allowed = False
                res.status = "breached"
                res.reason = msg
                return res
            else:
                res.status = "warning"
                res.reason = msg

        return res
