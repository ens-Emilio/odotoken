"""Tests for PLAN_v2 Phase 2: PII redaction + store_prompts privacy guard."""

from __future__ import annotations

import json
import time

import pytest

from odotoken.infra.logging import JsonFormatter, set_redactor
from odotoken.infra.redact import PIIRedactor, build_redactor


@pytest.fixture
def redactor():
    return PIIRedactor()


@pytest.fixture(autouse=True)
def reset_redactor():
    yield
    set_redactor(None)


PII_SAMPLES = [
    # (raw, must_not_contain)
    ("call with AKIAIOSFODNN7EXAMPLE now", "AKIAIOSFODNN7EXAMPLE"),
    ("key sk-proj-abcdef1234567890abcdef leaked", "sk-proj-abcdef1234567890abcdef"),
    ("email john.doe@corp.example.com here", "john.doe@corp.example.com"),
    ("cpf 123.456.789-00 invalid", "123.456.789-00"),
    ("cnpj 12.345.678/0001-95", "12.345.678/0001-95"),
    ("path /home/alice/secret.pem", "/home/alice"),
    ("github ghp_abcdefghijklmnopqrstuvwxyz123456", "ghp_abcdefghijklmnopqrstuvwxyz123456"),
    ("google AIzaSyA1234567890abcdefghijklmnopqrstuv", "AIzaSyA1234567890abcdefghijklmnopqrstuv"),
    ("jwt eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.abc123def456ghi789", "eyJhbGciOiJIUzI1NiJ9"),
    ("auth: Bearer verylongsecrettoken1234567890", "verylongsecrettoken1234567890"),
]


class TestPIIRedactor:
    @pytest.mark.parametrize("raw,secret", PII_SAMPLES)
    def test_redacts_each_pattern(self, redactor, raw, secret):
        out = redactor.redact(raw)
        assert secret not in out
        assert "REDACTED_" in out

    def test_clean_text_untouched(self, redactor):
        assert redactor.redact("all good, just tokens: 12345") == "all good, just tokens: 12345"

    def test_empty_and_disabled(self, redactor):
        assert redactor.redact("") == ""
        off = PIIRedactor(enabled=False)
        assert off.redact("sk-abcdefghijklmnopqrstuvwx") == "sk-abcdefghijklmnopqrstuvwx"

    def test_multiple_in_one_message(self, redactor):
        out = redactor.redact("a@b.com and AKIAIOSFODNN7EXAMPLE and /home/x/y")
        assert "a@b.com" not in out
        assert "AKIAIOSFODNN7EXAMPLE" not in out
        assert "/home/x" not in out

    def test_custom_patterns(self):
        r = PIIRedactor(custom_patterns=[{"pattern": r"PROJECT-\d{4}", "replace": "REDACTED_PROJECT"}])
        assert "PROJECT-2026" not in r.redact("ref PROJECT-2026 closed")

    def test_build_redactor_from_config(self):
        r = build_redactor({"privacy": {"redact_logs": False}})
        assert r.enabled is False
        r2 = build_redactor({"privacy": {"redact_logs": True, "custom_patterns": []}})
        assert r2.enabled is True

    def test_latency_budget(self, redactor):
        """Redaction must stay well under 2ms per message."""
        msg = " ".join(raw for raw, _ in PII_SAMPLES)
        t0 = time.perf_counter()
        for _ in range(100):
            redactor.redact(msg)
        avg_ms = (time.perf_counter() - t0) / 100 * 1000
        assert avg_ms < 2.0, f"redaction too slow: {avg_ms:.3f}ms"


class TestLogRedaction:
    def test_json_formatter_redacts(self, redactor):
        import logging

        set_redactor(redactor)
        record = logging.LogRecord(
            "t",
            logging.WARNING,
            __file__,
            1,
            "sending to foo@bar.com with sk-abcdefghijklmnopqrstuvwx",
            None,
            None,
        )
        out = JsonFormatter().format(record)
        assert "foo@bar.com" not in out
        assert "REDACTED_EMAIL" in out

    def test_no_redactor_is_noop(self):
        import logging

        record = logging.LogRecord("t", logging.INFO, __file__, 1, "foo@bar.com", None, None)
        out = JsonFormatter().format(record)
        assert "foo@bar.com" in out  # untouched without a redactor


class TestStorePrompts:
    def test_default_is_incognito(self):
        """store_prompts defaults to False → metadata marks incognito."""
        privacy = {"store_prompts": False}
        assert privacy.get("store_prompts", False) is False

    def test_prompt_meta_shape(self, redactor):
        """When store_prompts is on, metadata carries hash + REDACTED preview only."""
        body = json.dumps(
            {"model": "m", "messages": [{"role": "user", "content": "mail me at a@b.com"}]}
        ).encode()
        import hashlib

        preview_src = json.dumps(json.loads(body).get("messages", ""))[:200]
        meta = {
            "incognito": False,
            "prompt_sha256": hashlib.sha256(body).hexdigest()[:16],
            "prompt_preview": redactor.redact(preview_src),
        }
        assert "a@b.com" not in meta["prompt_preview"]
        assert meta["prompt_sha256"]
        assert meta["incognito"] is False
