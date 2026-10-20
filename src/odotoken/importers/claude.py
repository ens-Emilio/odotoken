"""Claude Code importer — ~/.claude/projects/<hash>/<session>.jsonl"""

from __future__ import annotations

import json
from pathlib import Path

from odotoken.importers.base import RawUsage, ScanResult, expand_path


class ClaudeImporter:
    name = "claude"

    def __init__(self, projects_dir: str | Path = "~/.claude/projects"):
        self.projects_dir = expand_path(projects_dir)

    def detect(self) -> bool:
        return self.projects_dir.exists() and any(self.projects_dir.iterdir())

    def _iter_files(self):
        if not self.projects_dir.exists():
            return
        for project_dir in self.projects_dir.iterdir():
            if not project_dir.is_dir():
                continue
            for f in project_dir.glob("*.jsonl"):
                yield f, project_dir.name

    def scan(self, since: str | None = None) -> ScanResult:
        if not self.detect():
            return ScanResult(cli=self.name, available=False, path=str(self.projects_dir))
        files = list(self._iter_files())
        # count entries without full parse (rough: lines with '"type":"assistant"')
        return ScanResult(
            cli=self.name, available=True, session_files=len(files), path=str(self.projects_dir)
        )

    def iter_usages(self, since: str | None = None):
        for filepath, project_hash in self._iter_files():
            session_id = filepath.stem
            # project hash like "-home-zens-allkodex-..." -> reverse to path
            project = project_hash.replace("-", "/") if project_hash.startswith("-") else project_hash
            try:
                with open(filepath, encoding="utf-8") as f:
                    for idx, line in enumerate(f):
                        line = line.strip()
                        if not line:
                            continue
                        try:
                            obj = json.loads(line)
                        except json.JSONDecodeError:
                            continue
                        if obj.get("type") != "assistant":
                            continue
                        msg = obj.get("message") or {}
                        usage = msg.get("usage") or {}
                        if not usage:
                            continue
                        model = msg.get("model") or obj.get("model") or "unknown"
                        ts = obj.get("timestamp") or ""
                        # filter by since
                        if since and ts < since:
                            continue
                        prompt = int(usage.get("input_tokens", 0) or 0)
                        completion = int(usage.get("output_tokens", 0) or 0)
                        cache_read = int(usage.get("cache_read_input_tokens", 0) or 0)
                        # cache_creation: ephemeral_5m + ephemeral_1h
                        cache_write = int(usage.get("cache_creation_input_tokens", 0) or 0)
                        if not cache_write:
                            cc = usage.get("cache_creation") or {}
                            if isinstance(cc, dict):
                                cache_write = int(cc.get("ephemeral_5m_input_tokens", 0) or 0) + int(
                                    cc.get("ephemeral_1h_input_tokens", 0) or 0
                                )
                        # provider is anthropic for claude
                        yield RawUsage(
                            cli=self.name,
                            session_id=session_id,
                            session_file=str(filepath),
                            entry_id=obj.get("uuid") or f"{session_id}:{idx}",
                            timestamp=ts,
                            provider="anthropic",
                            model=model,
                            prompt_tokens=prompt,
                            completion_tokens=completion,
                            cache_read_tokens=cache_read,
                            cache_write_tokens=cache_write,
                            project=project,
                        )
            except OSError:
                continue
