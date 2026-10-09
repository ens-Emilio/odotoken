"""Database layer for odotoken using SQLite in WAL mode.
Stores token consumption events and provides aggregation queries.
"""

from __future__ import annotations

import json
import sqlite3
from contextlib import contextmanager
from dataclasses import asdict, dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Generator, Optional


@dataclass
class TokenEvent:
    id: Optional[int] = None
    timestamp: str = ""  # ISO-8601 UTC
    provider: str = ""   # "anthropic", "openai", "gemini", etc.
    model: str = ""
    client_app: str = "unknown"  # "pi-agent", "omp", "opencode", etc.
    prompt_tokens: int = 0
    completion_tokens: int = 0
    total_tokens: int = 0
    cache_read_tokens: int = 0
    cache_write_tokens: int = 0
    cost_usd: float = 0.0
    latency_ms: float = 0.0
    status_code: int = 200
    metadata: str = "{}"  # JSON string for extra fields

    def to_dict(self) -> dict[str, Any]:
        return asdict(self)


class Database:
    def __init__(self, db_path: str | Path = "odotoken.db"):
        self.db_path = Path(db_path)
        self._init_db()

    @contextmanager
    def connection(self) -> Generator[sqlite3.Connection, None, None]:
        conn = sqlite3.connect(self.db_path, timeout=10.0)
        conn.row_factory = sqlite3.Row
        try:
            yield conn
        finally:
            conn.close()

    def _init_db(self) -> None:
        self.db_path.parent.mkdir(parents=True, exist_ok=True)
        with self.connection() as conn:
            conn.execute("PRAGMA journal_mode = WAL;")
            conn.execute("PRAGMA synchronous = NORMAL;")
            conn.execute("PRAGMA busy_timeout = 5000;")
            conn.execute(
                """
                CREATE TABLE IF NOT EXISTS token_events (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    timestamp TEXT NOT NULL,
                    provider TEXT NOT NULL,
                    model TEXT NOT NULL,
                    client_app TEXT NOT NULL DEFAULT 'unknown',
                    prompt_tokens INTEGER NOT NULL DEFAULT 0,
                    completion_tokens INTEGER NOT NULL DEFAULT 0,
                    total_tokens INTEGER NOT NULL DEFAULT 0,
                    cache_read_tokens INTEGER NOT NULL DEFAULT 0,
                    cache_write_tokens INTEGER NOT NULL DEFAULT 0,
                    cost_usd REAL NOT NULL DEFAULT 0.0,
                    latency_ms REAL NOT NULL DEFAULT 0.0,
                    status_code INTEGER NOT NULL DEFAULT 200,
                    metadata TEXT NOT NULL DEFAULT '{}'
                );
                """
            )
            conn.execute(
                "CREATE INDEX IF NOT EXISTS idx_token_events_timestamp ON token_events(timestamp);"
            )
            conn.execute(
                "CREATE INDEX IF NOT EXISTS idx_token_events_model ON token_events(model);"
            )
            conn.execute(
                "CREATE INDEX IF NOT EXISTS idx_token_events_client ON token_events(client_app);"
            )
            conn.commit()

    def record_event(self, event: TokenEvent) -> int:
        if not event.timestamp:
            event.timestamp = datetime.now(timezone.utc).isoformat()
        if not event.total_tokens:
            event.total_tokens = event.prompt_tokens + event.completion_tokens

        with self.connection() as conn:
            cur = conn.cursor()
            cur.execute(
                """
                INSERT INTO token_events (
                    timestamp, provider, model, client_app,
                    prompt_tokens, completion_tokens, total_tokens,
                    cache_read_tokens, cache_write_tokens,
                    cost_usd, latency_ms, status_code, metadata
                ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    event.timestamp,
                    event.provider,
                    event.model,
                    event.client_app,
                    event.prompt_tokens,
                    event.completion_tokens,
                    event.total_tokens,
                    event.cache_read_tokens,
                    event.cache_write_tokens,
                    event.cost_usd,
                    event.latency_ms,
                    event.status_code,
                    event.metadata,
                ),
            )
            conn.commit()
            return cur.lastrowid or 0

    def get_summary(self, since: Optional[str] = None) -> dict[str, Any]:
        """Aggregate totals across the board, optionally filtered by ISO date/timestamp 'since'."""
        query = """
            SELECT
                COUNT(*) as total_requests,
                COALESCE(SUM(prompt_tokens), 0) as total_prompt_tokens,
                COALESCE(SUM(completion_tokens), 0) as total_completion_tokens,
                COALESCE(SUM(total_tokens), 0) as total_tokens,
                COALESCE(SUM(cache_read_tokens), 0) as total_cache_read_tokens,
                COALESCE(SUM(cache_write_tokens), 0) as total_cache_write_tokens,
                COALESCE(SUM(cost_usd), 0.0) as total_cost_usd,
                COALESCE(AVG(latency_ms), 0.0) as avg_latency_ms
            FROM token_events
        """
        params: list[Any] = []
        if since:
            query += " WHERE timestamp >= ?"
            params.append(since)

        with self.connection() as conn:
            row = conn.execute(query, params).fetchone()
            if not row:
                return {}
            return dict(row)

    def get_summary_by_model(self, since: Optional[str] = None) -> list[dict[str, Any]]:
        query = """
            SELECT
                model,
                provider,
                COUNT(*) as requests,
                SUM(prompt_tokens) as prompt_tokens,
                SUM(completion_tokens) as completion_tokens,
                SUM(total_tokens) as total_tokens,
                SUM(cache_read_tokens) as cache_read_tokens,
                SUM(cache_write_tokens) as cache_write_tokens,
                SUM(cost_usd) as cost_usd
            FROM token_events
        """
        params: list[Any] = []
        if since:
            query += " WHERE timestamp >= ?"
            params.append(since)
        query += " GROUP BY model ORDER BY cost_usd DESC"

        with self.connection() as conn:
            rows = conn.execute(query, params).fetchall()
            return [dict(r) for r in rows]

    def get_summary_by_client(self, since: Optional[str] = None) -> list[dict[str, Any]]:
        query = """
            SELECT
                client_app,
                COUNT(*) as requests,
                SUM(total_tokens) as total_tokens,
                SUM(cost_usd) as cost_usd
            FROM token_events
        """
        params: list[Any] = []
        if since:
            query += " WHERE timestamp >= ?"
            params.append(since)
        query += " GROUP BY client_app ORDER BY cost_usd DESC"

        with self.connection() as conn:
            rows = conn.execute(query, params).fetchall()
            return [dict(r) for r in rows]

    def get_recent_events(self, limit: int = 50) -> list[dict[str, Any]]:
        query = """
            SELECT *
            FROM token_events
            ORDER BY id DESC
            LIMIT ?
        """
        with self.connection() as conn:
            rows = conn.execute(query, (limit,)).fetchall()
            return [dict(r) for r in rows]

    def get_daily_usage(self, days: int = 30) -> list[dict[str, Any]]:
        query = """
            SELECT
                substr(timestamp, 1, 10) as day,
                COUNT(*) as requests,
                SUM(total_tokens) as total_tokens,
                SUM(cost_usd) as cost_usd
            FROM token_events
            WHERE timestamp >= datetime('now', '-' || ? || ' days')
            GROUP BY day
            ORDER BY day ASC
        """
        with self.connection() as conn:
            rows = conn.execute(query, (days,)).fetchall()
            return [dict(r) for r in rows]
