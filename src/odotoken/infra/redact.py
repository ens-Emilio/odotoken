"""PII redaction for logs and stored previews.

The proxy never stores request/response bodies — this module guards the two
places where sensitive strings can still surface: structured log messages
and (opt-in) prompt previews in event metadata.

Patterns are pre-compiled at import: redaction must add < 2ms per message
(benchmarked in test_privacy.py).
"""

from __future__ import annotations

import re

# Pre-compiled, ordered: first match wins per span.
_DEFAULT_PATTERNS: list[tuple[str, re.Pattern[str]]] = [
    # Cloud / API keys
    ("REDACTED_AWS_KEY", re.compile(r"AKIA[0-9A-Z]{16}")),
    ("REDACTED_SECRET_KEY", re.compile(r"(?:sk|rk|ek)-[A-Za-z0-9_-]{20,}")),
    ("REDACTED_GITHUB_TOKEN", re.compile(r"gh[pousr]_[A-Za-z0-9]{30,}")),
    ("REDACTED_BEARER", re.compile(r"Bearer\s+[A-Za-z0-9._-]{20,}")),
    ("REDACTED_JWT", re.compile(r"eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{5,}")),
    ("REDACTED_GOOGLE_KEY", re.compile(r"AIza[A-Za-z0-9_-]{30,}")),
    # Personal data
    ("REDACTED_EMAIL", re.compile(r"[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}")),
    ("REDACTED_CPF", re.compile(r"\d{3}\.?\d{3}\.?\d{3}-?\d{2}")),
    ("REDACTED_CNPJ", re.compile(r"\d{2}\.?\d{3}\.?\d{3}/?\d{4}-?\d{2}")),
    # Local paths (home dirs leak usernames)
    ("REDACTED_PATH", re.compile(r"(?:/home|/Users)/[A-Za-z0-9_.-]+(?:/[^\s\"']*)?")),
    # Inline authorization headers
    ("REDACTED_AUTH_HEADER", re.compile(r"(?:x-api-key|authorization)\s*[:=]\s*\S+", re.IGNORECASE)),
]


class PIIRedactor:
    """Replaces known sensitive patterns with placeholder tokens."""

    def __init__(self, custom_patterns: list[dict[str, str]] | None = None, enabled: bool = True):
        self.enabled = enabled
        self._patterns = list(_DEFAULT_PATTERNS)
        for spec in custom_patterns or []:
            token = spec.get("replace", "REDACTED_CUSTOM")
            self._patterns.append((token, re.compile(spec["pattern"])))

    def redact(self, text: str) -> str:
        if not self.enabled or not text:
            return text
        for token, pattern in self._patterns:
            text = pattern.sub(token, text)
        return text


def build_redactor(config: dict) -> PIIRedactor:
    """Build a redactor from the config.json `privacy` section."""
    privacy = config.get("privacy", {})
    return PIIRedactor(
        custom_patterns=privacy.get("custom_patterns", []),
        enabled=privacy.get("redact_logs", True),
    )
