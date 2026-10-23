"""Budget guard and alerts system for odotoken.
Allows setting cost and token thresholds (warning and hard cutoff limits).
Supports native alerts: Slack, Discord, Telegram, Email + legacy webhook_url.
"""

from __future__ import annotations

import logging
import os
import smtplib
from dataclasses import dataclass, field
from datetime import UTC
from email.message import EmailMessage
from typing import Any

from odotoken.db import Database

logger = logging.getLogger("odotoken.guard")


# ---------------------------------------------------------------------------
# Alert configuration
# ---------------------------------------------------------------------------


@dataclass
class TelegramAlertConfig:
    bot_token_env: str | None = None
    bot_token: str | None = None
    chat_id: str | None = None


@dataclass
class EmailAlertConfig:
    smtp_host: str | None = None
    port: int = 587
    user_env: str | None = None
    pass_env: str | None = None
    user: str | None = None
    password: str | None = None
    to: str | None = None
    from_addr: str | None = None


@dataclass
class AlertsConfig:
    slack_url: str | None = None
    discord_url: str | None = None
    telegram: TelegramAlertConfig | None = None
    email: EmailAlertConfig | None = None


def _parse_telegram(d: Any) -> TelegramAlertConfig | None:
    if not isinstance(d, dict):
        return None
    bot_token_env = d.get("bot_token_env")
    bot_token = d.get("bot_token") or d.get("token")
    chat_id = d.get("chat_id")
    if not bot_token_env and not bot_token and not chat_id:
        return None
    return TelegramAlertConfig(
        bot_token_env=bot_token_env,
        bot_token=bot_token,
        chat_id=str(chat_id) if chat_id is not None else None,
    )


def _parse_email_to(v: Any) -> str | None:
    if v is None:
        return None
    if isinstance(v, str):
        return v
    if isinstance(v, list):
        return ", ".join(str(x) for x in v)
    return None


def _parse_email(d: Any) -> EmailAlertConfig | None:
    if not isinstance(d, dict):
        return None
    email_keys = ("smtp_host", "host", "to", "port", "user_env", "pass_env", "user", "password")
    if not any(k in d for k in email_keys):
        return None
    smtp_host = d.get("smtp_host") or d.get("host")
    try:
        port = int(d.get("port", 587))
    except Exception:
        port = 587
    return EmailAlertConfig(
        smtp_host=smtp_host,
        port=port,
        user_env=d.get("user_env"),
        pass_env=d.get("pass_env"),
        user=d.get("user") or d.get("username"),
        password=d.get("password") or d.get("pass"),
        to=_parse_email_to(d.get("to")),
        from_addr=d.get("from_addr") or d.get("from"),
    )


def _parse_alerts(raw: Any) -> AlertsConfig | None:
    if not isinstance(raw, dict) or not raw:
        return None
    telegram = _parse_telegram(raw.get("telegram"))
    email = _parse_email(raw.get("email"))
    slack_url = raw.get("slack_url")
    discord_url = raw.get("discord_url")
    # also support generic keys
    if not slack_url:
        slack_url = raw.get("slack")
    if not discord_url:
        discord_url = raw.get("discord")
    if not any([slack_url, discord_url, telegram, email]):
        return None
    return AlertsConfig(slack_url=slack_url, discord_url=discord_url, telegram=telegram, email=email)


def _alerts_to_dict(a: AlertsConfig) -> dict[str, Any]:
    d: dict[str, Any] = {}
    if a.slack_url:
        d["slack_url"] = a.slack_url
    if a.discord_url:
        d["discord_url"] = a.discord_url
    if a.telegram and (a.telegram.bot_token_env or a.telegram.bot_token or a.telegram.chat_id):
        td: dict[str, Any] = {}
        if a.telegram.bot_token_env:
            td["bot_token_env"] = a.telegram.bot_token_env
        if a.telegram.bot_token:
            td["bot_token"] = a.telegram.bot_token
        if a.telegram.chat_id:
            td["chat_id"] = a.telegram.chat_id
        d["telegram"] = td
    if a.email and (a.email.smtp_host or a.email.to):
        ed: dict[str, Any] = {}
        if a.email.smtp_host:
            ed["smtp_host"] = a.email.smtp_host
        if a.email.port != 587:
            ed["port"] = a.email.port
        else:
            # always include port when host present for clarity
            if a.email.smtp_host:
                ed["port"] = a.email.port
        if a.email.user_env:
            ed["user_env"] = a.email.user_env
        if a.email.pass_env:
            ed["pass_env"] = a.email.pass_env
        if a.email.user:
            ed["user"] = a.email.user
        if a.email.password:
            ed["password"] = a.email.password
        if a.email.to:
            ed["to"] = a.email.to
        if a.email.from_addr:
            ed["from_addr"] = a.email.from_addr
        d["email"] = ed
    return d


@dataclass
class GuardLimits:
    daily_cost_limit_usd: float | None = None
    monthly_cost_limit_usd: float | None = None
    daily_tokens_limit: int | None = None
    monthly_tokens_limit: int | None = None
    action_on_breach: str = "alert"
    webhook_url: str | None = None
    alerts: AlertsConfig | None = None
    projects: dict[str, GuardLimits] = field(default_factory=dict)

    @classmethod
    def from_dict(cls, d: dict[str, Any]) -> GuardLimits:
        projects_raw = d.get("projects") or {}
        projects: dict[str, GuardLimits] = {}
        if isinstance(projects_raw, dict):
            for pid, pd in projects_raw.items():
                if isinstance(pd, dict):
                    projects[pid] = cls(
                        daily_cost_limit_usd=pd.get("daily_cost_limit_usd"),
                        monthly_cost_limit_usd=pd.get("monthly_cost_limit_usd"),
                        daily_tokens_limit=pd.get("daily_tokens_limit"),
                        monthly_tokens_limit=pd.get("monthly_tokens_limit"),
                        action_on_breach=pd.get("action_on_breach") or "alert",
                        webhook_url=pd.get("webhook_url"),
                        alerts=_parse_alerts(pd.get("alerts")),
                    )
        daily = d.get("daily_cost_limit_usd")
        if daily is None:
            daily = d.get("daily_budget_usd")
        monthly = d.get("monthly_cost_limit_usd")
        if monthly is None:
            monthly = d.get("monthly_budget_usd")
        mtok = d.get("monthly_tokens_limit")
        if mtok is None:
            mtok = d.get("monthly_token_budget")
        return cls(
            daily_cost_limit_usd=daily,
            monthly_cost_limit_usd=monthly,
            daily_tokens_limit=d.get("daily_tokens_limit"),
            monthly_tokens_limit=mtok,
            action_on_breach=d.get("action_on_breach") or "alert",
            webhook_url=d.get("webhook_url"),
            alerts=_parse_alerts(d.get("alerts")),
            projects=projects,
        )

    def to_dict(self) -> dict[str, Any]:
        d: dict[str, Any] = {}
        if self.daily_cost_limit_usd is not None:
            d["daily_cost_limit_usd"] = self.daily_cost_limit_usd
        if self.monthly_cost_limit_usd is not None:
            d["monthly_cost_limit_usd"] = self.monthly_cost_limit_usd
        if self.daily_tokens_limit is not None:
            d["daily_tokens_limit"] = self.daily_tokens_limit
        if self.monthly_tokens_limit is not None:
            d["monthly_tokens_limit"] = self.monthly_tokens_limit
        d["action_on_breach"] = self.action_on_breach
        if self.webhook_url:
            d["webhook_url"] = self.webhook_url
        if self.alerts:
            ad = _alerts_to_dict(self.alerts)
            if ad:
                d["alerts"] = ad
        if self.projects:
            d["projects"] = {k: v.to_dict() for k, v in self.projects.items()}
        return d


@dataclass
class GuardCheckResult:
    allowed: bool = True
    reason: str = ""
    status: str = "ok"  # "ok", "warning", "breached"
    current_daily_cost: float = 0.0
    current_monthly_cost: float = 0.0
    current_daily_tokens: int = 0
    current_monthly_tokens: int = 0


# ---------------------------------------------------------------------------
# Alert dispatch — no new deps: httpx for webhooks/slack/discord/telegram,
# smtplib (stdlib) for email. Never crashes caller.
# ---------------------------------------------------------------------------


def _build_alert_text(result: GuardCheckResult) -> str:
    icon = "🚨" if result.status == "breached" else "⚠️"
    return (
        f"{icon} odotoken Guard {result.status}: {result.reason} "
        f"(daily ${result.current_daily_cost:.2f}, monthly ${result.current_monthly_cost:.2f}, "
        f"daily tokens {result.current_daily_tokens}, monthly {result.current_monthly_tokens})"
    )


def _send_generic_webhook(url: str, result: GuardCheckResult, text: str) -> None:
    try:
        import httpx

        httpx.post(
            url,
            json={
                "status": result.status,
                "reason": result.reason,
                "text": text,
                "daily_cost": result.current_daily_cost,
                "monthly_cost": result.current_monthly_cost,
                "daily_tokens": result.current_daily_tokens,
                "monthly_tokens": result.current_monthly_tokens,
            },
            timeout=5,
        )
    except Exception:
        logger.warning("guard webhook alert failed: %s", url, exc_info=True)


def _send_slack(url: str, text: str) -> None:
    try:
        import httpx

        httpx.post(url, json={"text": text}, timeout=5)
    except Exception:
        logger.warning("slack alert failed", exc_info=True)


def _send_discord(url: str, text: str) -> None:
    try:
        import httpx

        httpx.post(url, json={"content": text}, timeout=5)
    except Exception:
        logger.warning("discord alert failed", exc_info=True)


def _send_telegram(cfg: TelegramAlertConfig, text: str) -> None:
    try:
        token = None
        if cfg.bot_token_env:
            token = os.getenv(cfg.bot_token_env)
        if not token:
            token = cfg.bot_token
        if not token or not cfg.chat_id:
            logger.warning("telegram alert skipped: missing token or chat_id")
            return
        import httpx

        url = f"https://api.telegram.org/bot{token}/sendMessage"
        httpx.post(url, json={"chat_id": cfg.chat_id, "text": text}, timeout=5)
    except Exception:
        logger.warning("telegram alert failed", exc_info=True)


def _send_email(cfg: EmailAlertConfig, text: str, result: GuardCheckResult) -> None:
    try:
        if not cfg.smtp_host or not cfg.to:
            logger.warning("email alert skipped: missing smtp_host or to")
            return
        user = os.getenv(cfg.user_env) if cfg.user_env else cfg.user
        password = os.getenv(cfg.pass_env) if cfg.pass_env else cfg.password
        from_addr = cfg.from_addr or user or f"odotoken@{cfg.smtp_host}"
        msg = EmailMessage()
        msg["Subject"] = f"odotoken Guard {result.status}: {result.reason[:80]}"
        msg["From"] = from_addr
        msg["To"] = cfg.to
        msg.set_content(text)

        # ponytail: sync SMTP, small payload; async/email libs if volume grows
        with smtplib.SMTP(cfg.smtp_host, cfg.port, timeout=10) as s:
            try:
                s.ehlo()
                # try STARTTLS unless port 25
                if cfg.port != 25:
                    try:
                        s.starttls()
                        s.ehlo()
                    except Exception:
                        pass
            except Exception:
                pass
            if user and password:
                try:
                    s.login(user, password)
                except Exception:
                    logger.warning("email SMTP login failed", exc_info=True)
            s.send_message(msg)
    except Exception:
        logger.warning("email alert failed", exc_info=True)


def dispatch_alerts(limits: GuardLimits | None, result: GuardCheckResult) -> None:
    """Send alerts for warning/breached status. Never raises."""
    if limits is None or result.status not in ("warning", "breached"):
        return
    text = _build_alert_text(result)
    # legacy webhook_url — keep backwards compat
    if limits.webhook_url:
        _send_generic_webhook(limits.webhook_url, result, text)
    alerts = limits.alerts
    if not alerts:
        return
    if alerts.slack_url:
        _send_slack(alerts.slack_url, text)
    if alerts.discord_url:
        _send_discord(alerts.discord_url, text)
    if alerts.telegram:
        _send_telegram(alerts.telegram, text)
    if alerts.email:
        _send_email(alerts.email, text, result)


def send_alert(
    alert_cfg: str | dict[str, Any] | AlertsConfig | GuardLimits | None,
    result: GuardCheckResult,
) -> None:
    """Flexible entry-point: accepts webhook_url str, alerts dict, AlertsConfig or GuardLimits."""
    if alert_cfg is None:
        return
    if isinstance(alert_cfg, str):
        # bare webhook url
        _send_generic_webhook(alert_cfg, result, _build_alert_text(result))
        return
    if isinstance(alert_cfg, GuardLimits):
        dispatch_alerts(alert_cfg, result)
        return
    if isinstance(alert_cfg, AlertsConfig):
        # wrap in synthetic limits
        dispatch_alerts(GuardLimits(alerts=alert_cfg), result)
        return
    if isinstance(alert_cfg, dict):
        # could be full limits dict or alerts dict
        # heuristic: if it has daily/monthly keys treat as limits
        limit_keys = ("daily_cost_limit_usd", "monthly_cost_limit_usd", "webhook_url", "daily_budget_usd")
        if any(k in alert_cfg for k in limit_keys):
            limits = GuardLimits.from_dict(alert_cfg)
            dispatch_alerts(limits, result)
        else:
            alerts = _parse_alerts(alert_cfg)
            if alerts:
                dispatch_alerts(GuardLimits(alerts=alerts), result)
            elif "webhook_url" in alert_cfg:
                _send_generic_webhook(str(alert_cfg["webhook_url"]), result, _build_alert_text(result))
        return


class Guard:
    def __init__(self, db: Database, limits: GuardLimits | None = None):
        self.db = db
        self.limits = limits or GuardLimits()

    def update_limits(self, limits: GuardLimits) -> None:
        self.limits = limits

    @staticmethod
    def _apply_breach(res: GuardCheckResult, limits: GuardLimits, msg: str) -> None:
        if limits.action_on_breach == "block":
            res.allowed = False
            res.status = "breached"
            res.reason = msg
        else:
            res.status = "warning"
            res.reason = msg

    def _check_single(self, limits: GuardLimits, project: str | None = None) -> GuardCheckResult:
        res = GuardCheckResult(allowed=True, status="ok")
        from datetime import datetime

        now = datetime.now(UTC)
        today_start = f"{now.strftime('%Y-%m-%d')}T00:00:00"
        month_start = f"{now.strftime('%Y-%m')}-01T00:00:00"
        daily_stats = self.db.get_summary(since=today_start, project=project)
        monthly_stats = self.db.get_summary(since=month_start, project=project)
        res.current_daily_cost = float(daily_stats.get("total_cost_usd", 0.0))
        res.current_monthly_cost = float(monthly_stats.get("total_cost_usd", 0.0))
        res.current_daily_tokens = int(daily_stats.get("total_tokens", 0))
        res.current_monthly_tokens = int(monthly_stats.get("total_tokens", 0))
        prefix = f"[{project}] " if project else ""
        checks = (
            (
                "Daily budget limit breached: ${:.2f} >= ${:.2f}",
                res.current_daily_cost,
                limits.daily_cost_limit_usd,
            ),
            (
                "Monthly budget limit breached: ${:.2f} >= ${:.2f}",
                res.current_monthly_cost,
                limits.monthly_cost_limit_usd,
            ),
            ("Daily token limit breached: {} >= {}", res.current_daily_tokens, limits.daily_tokens_limit),
            (
                "Monthly token limit breached: {} >= {}",
                res.current_monthly_tokens,
                limits.monthly_tokens_limit,
            ),
        )
        for fmt, current_value, limit_value in checks:
            if limit_value and current_value >= limit_value:
                msg = f"{prefix}{fmt.format(current_value, limit_value)}"
                self._apply_breach(res, limits, msg)
                if not res.allowed:
                    break
        return res

    def check_allowance(self, project_id: str | None = None) -> GuardCheckResult:
        global_res = self._check_single(self.limits, project=None)
        if project_id is None:
            return global_res
        proj_limits = self.limits.projects.get(project_id)
        if not proj_limits:
            return global_res
        proj_res = self._check_single(proj_limits, project=project_id)
        if not proj_res.allowed:
            return proj_res
        if not global_res.allowed:
            return global_res
        if proj_res.status != "ok":
            return proj_res
        return global_res

    def check_project(self, project_id: str) -> GuardCheckResult:
        proj_limits = self.limits.projects.get(project_id)
        if not proj_limits:
            return GuardCheckResult(allowed=True, status="ok")
        return self._check_single(proj_limits, project=project_id)
