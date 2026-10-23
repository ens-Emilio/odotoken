"""Database layer for odotoken using SQLite in WAL mode.
Stores token consumption events and provides aggregation queries.
"""

from __future__ import annotations

import sqlite3
from collections.abc import Generator
from contextlib import contextmanager
from dataclasses import asdict, dataclass
from datetime import UTC, datetime
from pathlib import Path
from typing import Any


@dataclass
class TokenEvent:
    id: int | None = None
    timestamp: str = ""  # ISO-8601 UTC
    provider: str = ""  # "anthropic", "openai", "gemini", etc.
    model: str = ""
    client_app: str = "unknown"  # "pi-agent", "omp", "opencode", etc.
    prompt_tokens: int = 0
    completion_tokens: int = 0
    total_tokens: int = 0
    cache_read_tokens: int = 0
    cache_write_tokens: int = 0
    cost_usd: float = 0.0
    shadow_cost: float = 0.0  # theoretical cloud-equivalent cost (USD)
    latency_ms: float = 0.0
    ttfb_ms: float = 0.0
    streaming_ms: float = 0.0
    status_code: int = 200
    project_id: str = "default"
    tags: str = ""  # from X-Odo-Project header
    metadata: str = "{}"  # JSON string for extra fields
    source: str = "proxy"  # proxy | import:claude | import:codex | import:omp | import:crush
    session_id: str = ""  # original session id from CLI
    session_file: str = ""  # source file path
    entry_id: str = ""  # unique entry id inside session

    def to_dict(self) -> dict[str, Any]:
        return asdict(self)


class Database:
    def __init__(self, db_path: str | Path = "odotoken.db"):
        self.db_path = Path(db_path)
        self._init_db()

    @contextmanager
    def connection(self) -> Generator[sqlite3.Connection]:
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
            conn.execute("CREATE INDEX IF NOT EXISTS idx_token_events_timestamp ON token_events(timestamp);")
            conn.execute("CREATE INDEX IF NOT EXISTS idx_token_events_model ON token_events(model);")
            conn.execute("CREATE INDEX IF NOT EXISTS idx_token_events_client ON token_events(client_app);")

            # Idempotent column migrations (SQLite has no ADD COLUMN IF NOT EXISTS)
            cols = {row[1] for row in conn.execute("PRAGMA table_info(token_events);")}
            if "project_id" not in cols:
                conn.execute(
                    "ALTER TABLE token_events ADD COLUMN project_id TEXT NOT NULL DEFAULT 'default';"
                )
            if "shadow_cost" not in cols:
                conn.execute("ALTER TABLE token_events ADD COLUMN shadow_cost REAL NOT NULL DEFAULT 0.0;")
            if "source" not in cols:
                conn.execute("ALTER TABLE token_events ADD COLUMN source TEXT NOT NULL DEFAULT 'proxy';")
            if "session_id" not in cols:
                conn.execute("ALTER TABLE token_events ADD COLUMN session_id TEXT NOT NULL DEFAULT '';")
            if "session_file" not in cols:
                conn.execute("ALTER TABLE token_events ADD COLUMN session_file TEXT NOT NULL DEFAULT '';")
            if "entry_id" not in cols:
                conn.execute("ALTER TABLE token_events ADD COLUMN entry_id TEXT NOT NULL DEFAULT '';")
            if "ttfb_ms" not in cols:
                conn.execute("ALTER TABLE token_events ADD COLUMN ttfb_ms REAL NOT NULL DEFAULT 0.0;")
            if "streaming_ms" not in cols:
                conn.execute("ALTER TABLE token_events ADD COLUMN streaming_ms REAL NOT NULL DEFAULT 0.0;")
            if "tags" not in cols:
                conn.execute("ALTER TABLE token_events ADD COLUMN tags TEXT NOT NULL DEFAULT '';")
            conn.execute("CREATE INDEX IF NOT EXISTS idx_token_events_tags ON token_events(tags);")
            conn.execute("CREATE INDEX IF NOT EXISTS idx_token_events_project ON token_events(project_id);")
            conn.execute("CREATE INDEX IF NOT EXISTS idx_token_events_source ON token_events(source);")
            conn.execute(
                "CREATE INDEX IF NOT EXISTS idx_token_events_source_entry ON "
                "token_events(source, session_file, entry_id);"
            )
            # Dedup: unique where entry_id != '' (imports); proxy rows keep '' and can duplicate
            # Use a unique index with WHERE entry_id != '' (partial index)
            conn.execute(
                "CREATE UNIQUE INDEX IF NOT EXISTS uq_token_events_import ON "
                "token_events(source, session_file, entry_id) WHERE entry_id != '';"
            )
            # Import offsets for incremental JSONL scans
            conn.execute(
                """
                CREATE TABLE IF NOT EXISTS import_offsets (
                    session_file TEXT PRIMARY KEY,
                    offset INTEGER NOT NULL DEFAULT 0,
                    mtime INTEGER NOT NULL DEFAULT 0
                );
                """
            )
            # Seen entries for OMP-style SQLite imports (alternative dedup tracking)
            conn.execute(
                """
                CREATE TABLE IF NOT EXISTS import_seen (
                    source TEXT NOT NULL,
                    session_file TEXT NOT NULL,
                    entry_id TEXT NOT NULL,
                    PRIMARY KEY (source, session_file, entry_id)
                );
                """
            )
            conn.execute(
                """
                CREATE TABLE IF NOT EXISTS semantic_cache (
                    key TEXT PRIMARY KEY,
                    response TEXT NOT NULL,
                    created_at TEXT NOT NULL
                );
                """
            )
            conn.execute("CREATE INDEX IF NOT EXISTS idx_semantic_cache_created ON semantic_cache(created_at);")
            conn.commit()

    def record_event(self, event: TokenEvent) -> int:
        if not event.timestamp:
            event.timestamp = datetime.now(UTC).isoformat()
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
                    cost_usd, shadow_cost, latency_ms, ttfb_ms, streaming_ms, status_code,
                    project_id, tags, metadata, source, session_id, session_file, entry_id
                ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
                    event.shadow_cost,
                    event.latency_ms,
                    event.ttfb_ms,
                    event.streaming_ms,
                    event.status_code,
                    event.project_id,
                    event.tags,
                    event.metadata,
                    event.source,
                    event.session_id,
                    event.session_file,
                    event.entry_id,
                ),
            )
            conn.commit()
            return cur.lastrowid or 0

    def record_events_ignore_dup(self, events: list[TokenEvent]) -> tuple[int, int]:
        """Bulk insert, dedup via UNIQUE index. Returns (inserted, skipped). Single transaction."""
        if not events:
            return 0, 0
        for e in events:
            if not e.timestamp:
                e.timestamp = datetime.now(UTC).isoformat()
            if not e.total_tokens:
                e.total_tokens = e.prompt_tokens + e.completion_tokens
        rows = [
            (
                e.timestamp, e.provider, e.model, e.client_app,
                e.prompt_tokens, e.completion_tokens, e.total_tokens,
                e.cache_read_tokens, e.cache_write_tokens,
                e.cost_usd, e.shadow_cost, e.latency_ms, e.ttfb_ms, e.streaming_ms, e.status_code,
                e.project_id, e.tags, e.metadata, e.source, e.session_id, e.session_file, e.entry_id,
            )
            for e in events
        ]
        with self.connection() as conn:
            cur = conn.cursor()
            before = conn.total_changes
            cur.executemany(
                """
                INSERT OR IGNORE INTO token_events (
                    timestamp, provider, model, client_app,
                    prompt_tokens, completion_tokens, total_tokens,
                    cache_read_tokens, cache_write_tokens,
                    cost_usd, shadow_cost, latency_ms, ttfb_ms, streaming_ms, status_code,
                    project_id, tags, metadata, source, session_id, session_file, entry_id
                ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                """,
                rows,
            )
            inserted = conn.total_changes - before
            seen_rows = [(e.source, e.session_file, e.entry_id) for e in events if e.entry_id]
            if seen_rows:
                cur.executemany(
                    "INSERT OR IGNORE INTO import_seen (source, session_file, entry_id) VALUES (?, ?, ?)",
                    seen_rows,
                )
            conn.commit()
            skipped = len(events) - inserted
            return inserted, skipped


    def _tag_filter(self, tag: str | None) -> tuple[str, list[Any]]:
        if not tag:
            return "", []
        return "(',' || tags || ',') LIKE ?", [f"%,{tag},%"]

    def get_summary_by_tag(self, since: str | None = None, project: str | None = None, source: str | None = None) -> list[dict[str, Any]]:
        query = "SELECT tags, COUNT(*) as requests, SUM(total_tokens) as total_tokens, SUM(cost_usd) as cost_usd FROM token_events WHERE tags != ''"
        params: list[Any] = []
        if since:
            query += " AND strftime('%s', timestamp) >= strftime('%s', ?)"
            params.append(since)
        if project:
            query += " AND project_id = ?"
            params.append(project)
        if source and source != "all":
            sf, sp = self._source_filter(source)
            if sf:
                query += f" AND {sf}"
                params.extend(sp)
        query += " GROUP BY tags"
        with self.connection() as conn:
            tag_rows = [dict(r) for r in conn.execute(query, params).fetchall()]
        exploded: dict[str, dict] = {}
        for r in tag_rows:
            for tag in [x.strip() for x in (r["tags"] or "").split(",") if x.strip()]:
                if tag not in exploded:
                    exploded[tag] = {"tag": tag, "requests": 0, "total_tokens": 0, "cost_usd": 0.0}
                exploded[tag]["requests"] += r["requests"]
                exploded[tag]["total_tokens"] += r["total_tokens"] or 0
                exploded[tag]["cost_usd"] += r["cost_usd"] or 0
        return sorted(exploded.values(), key=lambda x: x["cost_usd"], reverse=True)

    def _source_filter(self, source: str | None) -> tuple[str, list[Any]]:
        if not source or source == "all":
            return "", []
        if source.startswith("import:"):
            return "source = ?", [source]
        if source == "proxy":
            return "source = ?", ["proxy"]
        if source == "import":
            return "source LIKE ?", ["import:%"]
        return "source = ?", [source]

    def get_summary(
        self, since: str | None = None, project: str | None = None, source: str | None = None, tag: str | None = None
    ) -> dict[str, Any]:
        """Aggregate totals, optionally filtered by ISO 'since' and/or project tag and/or source."""
        query = """
            SELECT
                COUNT(*) as total_requests,
                COALESCE(SUM(prompt_tokens), 0) as total_prompt_tokens,
                COALESCE(SUM(completion_tokens), 0) as total_completion_tokens,
                COALESCE(SUM(total_tokens), 0) as total_tokens,
                COALESCE(SUM(cache_read_tokens), 0) as total_cache_read_tokens,
                COALESCE(SUM(cache_write_tokens), 0) as total_cache_write_tokens,
                COALESCE(SUM(cost_usd), 0.0) as total_cost_usd,
                COALESCE(SUM(shadow_cost), 0.0) as total_shadow_cost_usd,
                COALESCE(AVG(latency_ms), 0.0) as avg_latency_ms
            FROM token_events
        """
        params: list[Any] = []
        conds = []
        if since:
            conds.append("strftime('%s', timestamp) >= strftime('%s', ?)")
            params.append(since)
        if project:
            conds.append("project_id = ?")
            params.append(project)
        if source and source != "all":
            sf, sp = self._source_filter(source)
            if sf:
                conds.append(sf)
                params.extend(sp)
        if tag:
            tf, tp = self._tag_filter(tag)
            conds.append(tf)
            params.extend(tp)
        if conds:
            query += " WHERE " + " AND ".join(conds)

        with self.connection() as conn:
            row = conn.execute(query, params).fetchone()
            if not row:
                return {}
            return dict(row)

    def get_summary_by_model(
        self, since: str | None = None, project: str | None = None, source: str | None = None
    ) -> list[dict[str, Any]]:
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
        conds = []
        if since:
            conds.append("strftime('%s', timestamp) >= strftime('%s', ?)")
            params.append(since)
        if project:
            conds.append("project_id = ?")
            params.append(project)
        if source and source != "all":
            sf, sp = self._source_filter(source)
            if sf:
                conds.append(sf)
                params.extend(sp)
        if conds:
            query += " WHERE " + " AND ".join(conds)
        query += " GROUP BY model ORDER BY cost_usd DESC"

        with self.connection() as conn:
            rows = conn.execute(query, params).fetchall()
            return [dict(r) for r in rows]

    def get_summary_by_client(
        self, since: str | None = None, project: str | None = None, source: str | None = None
    ) -> list[dict[str, Any]]:
        query = """
            SELECT
                client_app,
                COUNT(*) as requests,
                SUM(total_tokens) as total_tokens,
                SUM(cost_usd) as cost_usd
            FROM token_events
        """
        params: list[Any] = []
        conds = []
        if since:
            conds.append("strftime('%s', timestamp) >= strftime('%s', ?)")
            params.append(since)
        if project:
            conds.append("project_id = ?")
            params.append(project)
        if source and source != "all":
            sf, sp = self._source_filter(source)
            if sf:
                conds.append(sf)
                params.extend(sp)
        if conds:
            query += " WHERE " + " AND ".join(conds)
        query += " GROUP BY client_app ORDER BY cost_usd DESC"

        with self.connection() as conn:
            rows = conn.execute(query, params).fetchall()
            return [dict(r) for r in rows]

    def get_summary_by_source(
        self, since: str | None = None, project: str | None = None
    ) -> list[dict[str, Any]]:
        query = """
            SELECT
                source,
                COUNT(*) as requests,
                SUM(prompt_tokens) as prompt_tokens,
                SUM(completion_tokens) as completion_tokens,
                SUM(total_tokens) as total_tokens,
                SUM(cost_usd) as cost_usd
            FROM token_events
        """
        params: list[Any] = []
        conds = []
        if since:
            conds.append("strftime('%s', timestamp) >= strftime('%s', ?)")
            params.append(since)
        if project:
            conds.append("project_id = ?")
            params.append(project)
        if conds:
            query += " WHERE " + " AND ".join(conds)
        query += " GROUP BY source ORDER BY cost_usd DESC"
        with self.connection() as conn:
            rows = conn.execute(query, params).fetchall()
            return [dict(r) for r in rows]

    def get_summary_by_project(
        self, since: str | None = None, source: str | None = None
    ) -> list[dict[str, Any]]:
        query = """
            SELECT
                project_id,
                COUNT(*) as requests,
                SUM(total_tokens) as total_tokens,
                SUM(cost_usd) as cost_usd,
                SUM(shadow_cost) as shadow_cost_usd
            FROM token_events
        """
        params: list[Any] = []
        conds = []
        if since:
            conds.append("strftime('%s', timestamp) >= strftime('%s', ?)")
            params.append(since)
        if source and source != "all":
            sf, sp = self._source_filter(source)
            if sf:
                conds.append(sf)
                params.extend(sp)
        if conds:
            query += " WHERE " + " AND ".join(conds)
        query += " GROUP BY project_id ORDER BY cost_usd DESC"

        with self.connection() as conn:
            rows = conn.execute(query, params).fetchall()
            return [dict(r) for r in rows]

    def get_recent_events(
        self, limit: int = 50, since: str | None = None, project: str | None = None, source: str | None = None, tag: str | None = None
    ) -> list[dict[str, Any]]:
        query = "SELECT * FROM token_events"
        params: list[Any] = []
        conds = []
        if since:
            conds.append("strftime('%s', timestamp) >= strftime('%s', ?)")
            params.append(since)
        if project:
            conds.append("project_id = ?")
            params.append(project)
        if source and source != "all":
            sf, sp = self._source_filter(source)
            if sf:
                conds.append(sf)
                params.extend(sp)
        if tag:
            tf, tp = self._tag_filter(tag)
            conds.append(tf)
            params.extend(tp)
        if conds:
            query += " WHERE " + " AND ".join(conds)
        query += " ORDER BY id DESC LIMIT ?"
        params.append(limit)
        with self.connection() as conn:
            rows = conn.execute(query, params).fetchall()
            return [dict(r) for r in rows]

    def get_daily_usage(
        self, days: int = 30, since: str | None = None, source: str | None = None
    ) -> list[dict[str, Any]]:
        query = """
            SELECT
                substr(timestamp, 1, 10) as day,
                COUNT(*) as requests,
                SUM(total_tokens) as total_tokens,
                SUM(cost_usd) as cost_usd
            FROM token_events
            WHERE 1=1
        """
        params: list[Any] = []
        if since:
            query += " AND strftime('%s', timestamp) >= strftime('%s', ?)"
            params.append(since)
        # Always cap at N days from the lower bound (since or now)
        if since:
            query += " AND strftime('%s', timestamp) < strftime('%s', datetime(?, '+' || ? || ' days'))"
            params.extend([since, days])
        else:
            query += " AND timestamp >= datetime('now', '-' || ? || ' days')"
            params.append(days)
        if source and source != "all":
            sf, sp = self._source_filter(source)
            if sf:
                query += f" AND {sf}"
                params.extend(sp)
        query += " GROUP BY day ORDER BY day ASC"
        with self.connection() as conn:
            rows = conn.execute(query, params).fetchall()
            return [dict(r) for r in rows]
    def get_daily_usage_grouped(
        self, days: int = 30, since: str | None = None
    ) -> list[dict[str, Any]]:
        """Daily usage pivoted by source (proxy vs import:*). Returns {day, source, ...} rows."""
        query = """
            SELECT substr(timestamp,1,10) as day, source, COUNT(*) as requests,
                SUM(total_tokens) as total_tokens, SUM(cost_usd) as cost_usd
            FROM token_events WHERE 1=1
        """
        params: list[Any] = []
        if since:
            query += " AND strftime('%s', timestamp) >= strftime('%s', ?)"
            params.append(since)
        # Always cap at N days from the lower bound (since or now)
        if since:
            query += " AND strftime('%s', timestamp) < strftime('%s', datetime(?, '+' || ? || ' days'))"
            params.extend([since, days])
        else:
            query += " AND timestamp >= datetime('now','-' || ? || ' days')"
            params.append(days)
        query += " GROUP BY day, source ORDER BY day ASC, source ASC"
        with self.connection() as conn:
            return [dict(r) for r in conn.execute(query, params).fetchall()]

    def get_tps_series(self, since: str | None = None, interval: str = "1h", project: str | None = None, source: str | None = None) -> list[dict[str, Any]]:
        fmt_map = {"1m": "%Y-%m-%dT%H:%M:00Z", "1h": "%Y-%m-%dT%H:00:00Z", "1d": "%Y-%m-%d"}
        fmt = fmt_map.get(interval, "%Y-%m-%dT%H:00:00Z")

        query = f"""
            SELECT strftime('{fmt}', timestamp) as bucket,
                   COALESCE(SUM(completion_tokens),0) as completion_tokens,
                   COALESCE(SUM(latency_ms),0) as total_latency_ms,
                   COALESCE(AVG(latency_ms),0) as avg_latency_ms,
                   COALESCE(SUM(completion_tokens)*1000.0/NULLIF(SUM(latency_ms),0),0) as avg_tps,
                   COALESCE(MAX(CASE WHEN latency_ms>0 THEN completion_tokens*1000.0/latency_ms ELSE 0 END),0) as peak_tps
            FROM token_events WHERE 1=1
        """
        params: list[Any] = []
        if since:
            query += " AND strftime('%s', timestamp) >= strftime('%s', ?)"
            params.append(since)
        if project:
            query += " AND project_id = ?"
            params.append(project)
        if source and source != "all":
            sf, sp = self._source_filter(source)
            if sf:
                query += f" AND {sf}"
                params.extend(sp)
        query += " GROUP BY bucket ORDER BY bucket ASC"
        with self.connection() as conn:
            return [dict(r) for r in conn.execute(query, params).fetchall()]

    def get_tps_by_model(self, since: str | None = None, project: str | None = None, source: str | None = None) -> list[dict[str, Any]]:
        query = """SELECT model, provider, COUNT(*) as requests, SUM(completion_tokens) as completion_tokens, SUM(total_tokens) as total_tokens, SUM(cost_usd) as cost_usd, AVG(latency_ms) as avg_latency_ms, COALESCE(SUM(completion_tokens)*1000.0/NULLIF(SUM(latency_ms),0),0) as tps FROM token_events WHERE 1=1"""
        params: list[Any] = []
        if since:
            query += " AND strftime('%s', timestamp) >= strftime('%s', ?)"
            params.append(since)
        if project:
            query += " AND project_id = ?"
            params.append(project)
        if source and source != "all":
            sf, sp = self._source_filter(source)
            if sf:
                query += f" AND {sf}"
                params.extend(sp)
        query += " GROUP BY model, provider ORDER BY tps DESC"
        with self.connection() as conn:
            return [dict(r) for r in conn.execute(query, params).fetchall()]

    def get_tps_by_project(self, since: str | None = None, source: str | None = None) -> list[dict[str, Any]]:
        query = """SELECT project_id, COUNT(*) as requests, SUM(completion_tokens) as completion_tokens, SUM(total_tokens) as total_tokens, SUM(cost_usd) as cost_usd, AVG(latency_ms) as avg_latency_ms, COALESCE(SUM(completion_tokens)*1000.0/NULLIF(SUM(latency_ms),0),0) as tps FROM token_events WHERE 1=1"""
        params: list[Any] = []
        if since:
            query += " AND strftime('%s', timestamp) >= strftime('%s', ?)"
            params.append(since)
        if source and source != "all":
            sf, sp = self._source_filter(source)
            if sf:
                query += f" AND {sf}"
                params.extend(sp)
        query += " GROUP BY project_id ORDER BY tps DESC"
        with self.connection() as conn:
            return [dict(r) for r in conn.execute(query, params).fetchall()]

    def get_tps_summary(self, since: str | None = None, project: str | None = None, source: str | None = None) -> dict[str, Any]:
        query = """SELECT COUNT(*) as requests, COALESCE(SUM(completion_tokens),0) as completion_tokens, COALESCE(SUM(total_tokens),0) as total_tokens, COALESCE(SUM(latency_ms),0) as total_latency_ms, COALESCE(AVG(latency_ms),0) as avg_latency_ms, COALESCE(SUM(completion_tokens)*1000.0/NULLIF(SUM(latency_ms),0),0) as avg_tps, COALESCE(MAX(CASE WHEN latency_ms>0 THEN completion_tokens*1000.0/latency_ms ELSE 0 END),0) as peak_tps FROM token_events WHERE 1=1"""
        params: list[Any] = []
        if since:
            query += " AND strftime('%s', timestamp) >= strftime('%s', ?)"
            params.append(since)
        if project:
            query += " AND project_id = ?"
            params.append(project)
        if source and source != "all":
            sf, sp = self._source_filter(source)
            if sf:
                query += f" AND {sf}"
                params.extend(sp)
        with self.connection() as conn:
            row = conn.execute(query, params).fetchone()
            return dict(row) if row else {"requests": 0, "completion_tokens": 0, "total_tokens": 0, "total_latency_ms": 0, "avg_latency_ms": 0, "avg_tps": 0, "peak_tps": 0}

    def semantic_cache_get(self, key: str, ttl_seconds: int) -> dict | None:
        import json as _json
        from datetime import datetime as _dt
        with self.connection() as conn:
            row = conn.execute("SELECT response, created_at FROM semantic_cache WHERE key=?", (key,)).fetchone()
            if row is None:
                return None
            try:
                created = _dt.fromisoformat(row["created_at"])
                if created.tzinfo is None:
                    created = created.replace(tzinfo=UTC)
            except Exception:
                conn.execute("DELETE FROM semantic_cache WHERE key=?", (key,))
                conn.commit()
                return None
            if (_dt.now(UTC) - created).total_seconds() > ttl_seconds:
                conn.execute("DELETE FROM semantic_cache WHERE key=?", (key,))
                conn.commit()
                return None
            try:
                return _json.loads(row["response"])
            except Exception:
                return None

    def semantic_cache_set(self, key: str, response: dict) -> None:
        import json as _json
        from datetime import datetime as _dt
        now = _dt.now(UTC).isoformat()
        with self.connection() as conn:
            conn.execute(
                "INSERT OR REPLACE INTO semantic_cache (key, response, created_at) VALUES (?, ?, ?)",
                (key, _json.dumps(response), now),
            )
            conn.commit()

    # import offsets helpers
    def get_import_offset(self, session_file: str) -> tuple[int, int]:
        with self.connection() as conn:
            row = conn.execute(
                "SELECT offset, mtime FROM import_offsets WHERE session_file=?", (session_file,)
            ).fetchone()
            if row:
                return int(row["offset"]), int(row["mtime"])
            return 0, 0

    def set_import_offset(self, session_file: str, offset: int, mtime: int) -> None:
        with self.connection() as conn:
            conn.execute(
                "INSERT OR REPLACE INTO import_offsets (session_file, offset, mtime) VALUES (?, ?, ?)",
                (session_file, offset, mtime),
            )
            conn.commit()
