"""Tests for CLI commands."""

from __future__ import annotations

import json
import os
import sys
import subprocess
import pytest


# Get the odotoken executable from the same Python environment
ODOTOKEN_CMD = os.path.join(os.path.dirname(sys.executable), "odotoken")


def run_cli(args: list[str], input_text: str | None = None) -> subprocess.CompletedProcess:
    """Run CLI command and return result."""
    cmd = [ODOTOKEN_CMD] + args
    return subprocess.run(
        cmd,
        capture_output=True,
        text=True,
        timeout=30,
        input=input_text,
    )


class TestCLICommands:
    """Tests for CLI commands."""

    def test_help(self):
        result = run_cli(["--help"])
        assert result.returncode == 0
        assert "odotoken" in result.stdout

    def test_count_help(self):
        result = run_cli(["count", "--help"])
        assert result.returncode == 0
        assert "text" in result.stdout
        assert "chat" in result.stdout
        assert "batch" in result.stdout

    def test_models_help(self):
        result = run_cli(["models", "--help"])
        assert result.returncode == 0
        assert "list" in result.stdout
        assert "info" in result.stdout

    def test_count_text(self):
        result = run_cli(["count", "text", "Hello world"])
        assert result.returncode == 0
        assert "Token Count" in result.stdout
        assert "Total Tokens" in result.stdout

    def test_count_text_json(self):
        result = run_cli(["count", "text", "Hello world", "--output", "json"])
        assert result.returncode == 0
        data = json.loads(result.stdout.strip())
        assert data["total_tokens"] > 0
        assert data["method"] == "heuristic"

    def test_count_chat(self):
        messages = '[{"role": "user", "content": "Hello"}]'
        result = run_cli(["count", "chat", "--model", "gpt-4o-mini", "--file", "-"], input_text=messages)
        assert result.returncode == 0
        assert "Token Count" in result.stdout

    def test_count_chat_json(self):
        messages = '[{"role": "user", "content": "Hello"}]'
        result = run_cli(["count", "chat", "--model", "gpt-4o-mini", "--file", "-", "--output", "json"], input_text=messages)
        assert result.returncode == 0
        data = json.loads(result.stdout.strip())
        assert data["total_tokens"] > 0

    def test_models_list(self):
        result = run_cli(["models", "list"])
        assert result.returncode == 0
        assert "Supported Models" in result.stdout

    def test_models_list_json(self):
        result = run_cli(["models", "list", "--output", "json"])
        assert result.returncode == 0
        data = json.loads(result.stdout.strip())
        assert isinstance(data, list)
        assert len(data) == 8

    def test_models_list_filter(self):
        result = run_cli(["models", "list", "--provider", "openai"])
        assert result.returncode == 0
        assert "openai" in result.stdout.lower()

    def test_models_info(self):
        result = run_cli(["models", "info", "gpt-4o-mini"])
        assert result.returncode == 0
        assert "Model Info" in result.stdout
        assert "gpt-4o-mini" in result.stdout

    def test_models_info_unknown(self):
        result = run_cli(["models", "info", "unknown-model"])
        assert result.returncode != 0

    def test_count_batch(self, tmp_path):
        batch_file = tmp_path / "batch.jsonl"
        batch_file.write_text('{"model": "gpt-4o-mini", "text": "Test 1"}\n{"model": "gpt-4o-mini", "text": "Test 2"}')

        result = run_cli(["count", "batch", str(batch_file)])
        assert result.returncode == 0
        assert "Batch Token Count Results" in result.stdout
        assert "Total Items" in result.stdout
        assert "2" in result.stdout