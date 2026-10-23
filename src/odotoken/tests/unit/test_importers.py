"""Opencode + pi importers map external usage schemas to RawUsage correctly."""

from __future__ import annotations

import sqlite3
import tempfile
from pathlib import Path

from odotoken.importers.opencode import OpencodeImporter
from odotoken.importers.pi import PiImporter

MSG = {
    "role": "assistant",
    "providerID": "verboo",
    "modelID": "glm-5.2",
    "cost": 1.25,
    "tokens": {"total": 10, "input": 6, "output": 4, "reasoning": 0,
               "cache": {"read": 2, "write": 1}},
    "path": {"cwd": "/home/zens/proj"},
    "time": {"created": 1792620373382},
}


def _make_opencode_db(tmp_path) -> Path:
    import os

    fd, path = tempfile.mkstemp(dir=tmp_path, suffix=".db")
    os.close(fd)
    conn = sqlite3.connect(path)
    conn.execute(
        "CREATE TABLE message (id TEXT, session_id TEXT, time_created INTEGER, data TEXT)"
    )
    conn.execute(
        "INSERT INTO message VALUES (?,?,?,?)",
        ("msg_1", "ses_1", 1792620373382, __import__("json").dumps(MSG)),
    )
    conn.commit()
    conn.close()
    return Path(path)


def test_opencode_importer_maps_fields(tmp_path):
    db_path = _make_opencode_db(tmp_path)
    imp = OpencodeImporter(db_path)
    usage = next(iter(imp.iter_usages()))
    assert usage.provider == "verboo"
    assert usage.model == "glm-5.2"
    assert usage.prompt_tokens == 6
    assert usage.completion_tokens == 4
    assert usage.cache_read_tokens == 2
    assert usage.cache_write_tokens == 1
    assert usage.raw_cost == 1.25
    assert usage.project == "/home/zens/proj"


def test_opencode_skips_non_assistant_and_no_tokens(tmp_path):
    db_path = _make_opencode_db(tmp_path)
    conn = sqlite3.connect(db_path)
    conn.execute(
        "INSERT INTO message VALUES (?,?,?,?)",
        (
            "msg_2",
            "ses_1",
            1792620373382,
            '{"role":"user","tokens":{"total":5}}',
        ),
    )
    conn.execute(
        "INSERT INTO message VALUES (?,?,?,?)",
        (
            "msg_3",
            "ses_1",
            1792620373382,
            '{"role":"assistant","tokens":{"total":0}}',
        ),
    )
    conn.commit()
    conn.close()
    usages = list(OpencodeImporter(db_path).iter_usages())
    assert [u.entry_id for u in usages] == ["msg_1"]


def test_pi_importer_maps_fields(tmp_path):
    session = tmp_path / "--home-zens-.od-projects-abc--" / "2026-01-01T00-00-00Z_ses1.jsonl"
    session.parent.mkdir()
    session.write_text(
        '\n'.join([
            '{"type":"session","id":"s1"}',
            '{"type":"message","timestamp":"2026-01-01T00:00:00.000Z",'
            '"message":{"role":"assistant","provider":"kodeo","model":"claude-opus-5",'
            '"usage":{"input":5,"output":3,"cacheRead":1,"cacheWrite":1,"totalTokens":10,'
            '"cost":{"total":0.2}},"responseId":"resp1"}}',
        ])
        + "\n"
    )
    imp = PiImporter(tmp_path)
    usage = next(iter(imp.iter_usages()))
    assert usage.provider == "kodeo"
    assert usage.model == "claude-opus-5"
    assert usage.prompt_tokens == 5
    assert usage.completion_tokens == 3
    assert usage.cache_read_tokens == 1
    assert usage.cache_write_tokens == 1
    assert usage.raw_cost == 0.2
    assert usage.entry_id == "resp1"
    assert usage.project == "home-zens-.od-projects-abc"


def test_pi_skips_non_assistant_and_no_usage(tmp_path):
    session = tmp_path / "a--" / "x.jsonl"
    session.parent.mkdir()
    session.write_text(
        '{"type":"message","timestamp":"2026-01-01T00:00:00.000Z",'
        '"message":{"role":"user","provider":"kodeo","model":"m","usage":{}}}\n'
        '{"type":"message","timestamp":"2026-01-01T00:00:00.000Z",'
        '"message":{"role":"assistant","provider":"kodeo","model":"m","usage":{"input":1,"output":1}}}\n'
    )
    usages = list(PiImporter(tmp_path).iter_usages())
    assert len(usages) == 1
