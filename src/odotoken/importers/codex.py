"""Codex importer — ~/.codex/sessions/**/*.jsonl (token_usage_record)."""

from __future__ import annotations

import json
from pathlib import Path

from odotoken.importers.base import RawUsage, ScanResult, expand_path


class CodexImporter:
    name = "codex"

    def __init__(self, sessions_dir: str | Path = "~/.codex/sessions"):
        self.sessions_dir = expand_path(sessions_dir)

    def detect(self) -> bool:
        return self.sessions_dir.exists()

    def _iter_files(self):
        if not self.sessions_dir.exists():
            return
        yield from self.sessions_dir.rglob("*.jsonl")

    def scan(self, since: str | None = None) -> ScanResult:
        if not self.detect():
            return ScanResult(cli=self.name, available=False, path=str(self.sessions_dir))
        files = list(self._iter_files())
        return ScanResult(
            cli=self.name, available=True, session_files=len(files), path=str(self.sessions_dir)
        )

    def iter_usages(self, since: str | None = None):
        for filepath in self._iter_files():
            session_id = filepath.stem
            provider_hint = ""
            model_hint = ""
            cwd_hint = ""
            try:
                with open(filepath, encoding="utf-8") as f:
                    for line in f:
                        line = line.strip()
                        if not line:
                            continue
                        try:
                            obj = json.loads(line)
                        except json.JSONDecodeError:
                            continue
                        t = obj.get("type")
                        if t == "session_meta":
                            payload = obj.get("payload") or {}
                            provider_hint = payload.get("model_provider") or provider_hint
                            cwd_hint = payload.get("cwd") or cwd_hint
                        if t == "event_msg":
                            payload = obj.get("payload") or {}
                            ts_settings = payload.get("thread_settings") or {}
                            if ts_settings.get("model"):
                                model_hint = ts_settings["model"]
                            if ts_settings.get("model_provider_id"):
                                provider_hint = ts_settings["model_provider_id"]
                        if t == "token_usage_record":
                            payload = obj.get("payload") or {}
                            usage = payload.get("usage") or {}
                            if not usage:
                                continue
                            ts = obj.get("timestamp") or ""
                            if since and ts < since:
                                continue
                            prompt = int(usage.get("input_tokens", 0) or 0)
                            completion = int(usage.get("output_tokens", 0) or 0)
                            cache_read = int(usage.get("cached_input_tokens", 0) or 0)
                            cache_write = int(usage.get("cache_write_input_tokens", 0) or 0)
                            model = payload.get("model") or model_hint or provider_hint or "unknown"
                            provider = provider_hint or "openai"
                            project = cwd_hint or payload.get("cwd") or "default"
                            yield RawUsage(
                                cli=self.name,
                                session_id=payload.get("session_id") or session_id,
                                session_file=str(filepath),
                                entry_id=(
                                    payload.get("response_id")
                                    or obj.get("timestamp")
                                    or f"{session_id}:{prompt}"
                                ),
                                timestamp=ts,
                                provider=provider,
                                model=model,
                                prompt_tokens=prompt,
                                completion_tokens=completion,
                                cache_read_tokens=cache_read,
                                cache_write_tokens=cache_write,
                                project=project,
                            )
            except OSError:
                continue
