"""OpenCode importer — ~/.local/share/opencode/opencode.db (per-request rows).

Each row: message.id, session_id, time_created (ms), data JSON with
providerID, modelID, cost, tokens {total,input,output,reasoning,cache{read,write}}.
"""

from __future__ import annotations

import json
import sqlite3
from pathlib import Path

from odotoken.importers.base import RawUsage, ScanResult, expand_path


class OpencodeImporter:
    name = "opencode"

    def __init__(self, db_path: str | Path = "~/.local/share/opencode/opencode.db"):
        self.db_path = expand_path(db_path)

    def detect(self) -> bool:
        return self.db_path.exists()

    def scan(self, since: str | None = None) -> ScanResult:
        if not self.detect():
            return ScanResult(cli=self.name, available=False, path=str(self.db_path))
        try:
            conn = sqlite3.connect(str(self.db_path), timeout=5.0)
            total = conn.execute("SELECT COUNT(*) FROM message").fetchone()[0]
            conn.close()
            return ScanResult(
                cli=self.name,
                available=True,
                session_files=1,
                total_entries=total,
                path=str(self.db_path),
            )
        except Exception:
            return ScanResult(cli=self.name, available=True, path=str(self.db_path))

    def iter_usages(self, since: str | None = None):
        if not self.detect():
            return
        since_ms = None
        if since:
            try:
                from datetime import datetime

                dt = datetime.fromisoformat(since.replace("Z", "+00:00"))
                since_ms = int(dt.timestamp() * 1000)
            except Exception:
                since_ms = None

        conn = sqlite3.connect(str(self.db_path), timeout=5.0)
        conn.row_factory = sqlite3.Row
        query = "SELECT id, session_id, time_created, data FROM message"
        params: list = []
        if since_ms is not None:
            query += " WHERE time_created >= ?"
            params.append(since_ms)
        query += " ORDER BY time_created ASC"
        try:
            for row in conn.execute(query, params):
                try:
                    data = json.loads(row["data"])
                except (json.JSONDecodeError, TypeError):
                    continue
                if data.get("role") != "assistant":
                    continue
                tokens = data.get("tokens") or {}
                if not tokens.get("total"):
                    continue
                ts_ms = row["time_created"]
                try:
                    from datetime import UTC, datetime

                    ts = datetime.fromtimestamp(ts_ms / 1000, tz=UTC).isoformat()
                except Exception:
                    ts = ""
                if since and ts and ts < since:
                    continue
                yield RawUsage(
                    cli=self.name,
                    session_id=row["session_id"] or "unknown",
                    session_file="",  # opencode stores no per-row session file
                    entry_id=row["id"] or f"{row['session_id']}:{ts_ms}",
                    timestamp=ts,
                    provider=data.get("providerID") or "unknown",
                    model=data.get("modelID") or "unknown",
                    prompt_tokens=int(tokens.get("input", 0) or 0),
                    completion_tokens=int(tokens.get("output", 0) or 0),
                    cache_read_tokens=int((tokens.get("cache") or {}).get("read", 0) or 0),
                    cache_write_tokens=int((tokens.get("cache") or {}).get("write", 0) or 0),
                    project=(data.get("path") or {}).get("cwd") or "default",
                    raw_cost=float(data.get("cost") or 0.0),
                )
        finally:
            conn.close()
    def live_sessions(self):
        """Yield currently-open sessions from session_v2 (not archived/suspended)."""
        if not self.detect():
            return
        from datetime import UTC, datetime

        conn = sqlite3.connect(str(self.db_path), timeout=5.0)
        conn.row_factory = sqlite3.Row
        query = (
            "SELECT id, directory, title, time_created, time_updated, time_archived,"
            " time_suspended, cost, tokens_input, tokens_output, tokens_reasoning,"
            " tokens_cache_read, tokens_cache_write, json_extract(model, '$.providerID') AS provider,"
            " json_extract(model, '$.id') AS model"
            " FROM session_v2"
            " WHERE time_archived IS NULL AND time_suspended IS NULL"
            " ORDER BY time_updated DESC"
        )
        try:
            for row in conn.execute(query):
                ts = row["time_updated"] or row["time_created"] or 0
                try:
                    updated = datetime.fromtimestamp(ts / 1000, tz=UTC).isoformat()
                except Exception:
                    updated = ""
                yield {
                    "cli": self.name,
                    "session_id": row["id"],
                    "provider": row["provider"] or "unknown",
                    "model": row["model"] or "unknown",
                    "tokens_input": int(row["tokens_input"] or 0),
                    "tokens_output": int(row["tokens_output"] or 0),
                    "cost_usd": float(row["cost"] or 0.0),
                    "project": row["directory"] or "default",
                    "title": row["title"] or "",
                    "last_updated": updated,
                }
        finally:
            conn.close()
