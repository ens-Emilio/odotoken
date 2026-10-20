"""OMP importer — ~/.omp/stats.db (SQLite messages)."""

from __future__ import annotations

import sqlite3
from pathlib import Path

from odotoken.importers.base import RawUsage, ScanResult, expand_path


class OmpImporter:
    name = "omp"

    def __init__(self, db_path: str | Path = "~/.omp/stats.db"):
        self.db_path = expand_path(db_path)

    def detect(self) -> bool:
        return self.db_path.exists()

    def scan(self, since: str | None = None) -> ScanResult:
        if not self.detect():
            return ScanResult(cli=self.name, available=False, path=str(self.db_path))
        try:
            conn = sqlite3.connect(str(self.db_path), timeout=5.0)
            cur = conn.execute("SELECT COUNT(*) FROM messages")
            total = cur.fetchone()[0]
            conn.close()
            return ScanResult(cli=self.name, available=True, session_files=1, total_entries=total, path=str(self.db_path))
        except Exception:
            return ScanResult(cli=self.name, available=True, path=str(self.db_path))

    def iter_usages(self, since: str | None = None):
        if not self.db_path.exists():
            return
        # since is ISO8601; OMP timestamps are millis since epoch (int)
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
        query = (
            "SELECT session_file, entry_id, provider, model, folder, timestamp,"
            " input_tokens, output_tokens, cache_read_tokens, cache_write_tokens,"
            " total_tokens, cost_total FROM messages"
        )
        params: list = []
        if since_ms is not None:
            query += " WHERE timestamp >= ?"
            params.append(since_ms)
        query += " ORDER BY timestamp ASC"
        try:
            for row in conn.execute(query, params):
                ts_ms = row["timestamp"]
                # to ISO8601
                try:
                    from datetime import UTC, datetime

                    ts = datetime.fromtimestamp(ts_ms / 1000, tz=UTC).isoformat()
                except Exception:
                    ts = ""
                if since and ts and ts < since:
                    continue
                # session_id from session_file basename
                sf = row["session_file"] or ""
                session_id = Path(sf).stem if sf else "unknown"
                yield RawUsage(
                    cli=self.name,
                    session_id=session_id,
                    session_file=sf,
                    entry_id=row["entry_id"] or f"{sf}:{ts_ms}",
                    timestamp=ts,
                    provider=row["provider"] or "unknown",
                    model=row["model"] or "unknown",
                    prompt_tokens=int(row["input_tokens"] or 0),
                    completion_tokens=int(row["output_tokens"] or 0),
                    cache_read_tokens=int(row["cache_read_tokens"] or 0),
                    cache_write_tokens=int(row["cache_write_tokens"] or 0),
                    project=row["folder"] or "default",
                    raw_cost=float(row["cost_total"] or 0.0),
                )
        finally:
            conn.close()
