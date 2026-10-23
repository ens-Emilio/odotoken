"""Dashboard auth — Bearer and/or Basic protection from config.json."""

from __future__ import annotations

import base64
import os
from typing import Any

from fastapi import Request
from fastapi.responses import JSONResponse


def _resolve_auth_config(config: dict[str, Any]) -> dict[str, str | None]:
    auth = config.get("auth") or {}
    if not isinstance(auth, dict):
        return {}
    bearer = auth.get("bearer_token")
    env_name = auth.get("bearer_token_env")
    if not bearer and env_name:
        bearer = os.environ.get(env_name)
    basic_cfg = auth.get("basic") or {}
    basic_user = basic_cfg.get("user") if isinstance(basic_cfg, dict) else None
    basic_pass = basic_cfg.get("pass") if isinstance(basic_cfg, dict) else None
    basic_pass_env = basic_cfg.get("pass_env") if isinstance(basic_cfg, dict) else None
    if not basic_pass and basic_pass_env:
        basic_pass = os.environ.get(basic_pass_env)
    out: dict[str, str | None] = {}
    if bearer:
        out["bearer"] = str(bearer)
    if basic_user and basic_pass:
        out["basic_user"] = str(basic_user)
        out["basic_pass"] = str(basic_pass)
    return out


def is_auth_configured(config: dict[str, Any]) -> bool:
    return bool(_resolve_auth_config(config))


def check_request_auth(request: Request, config: dict[str, Any]) -> bool:
    creds = _resolve_auth_config(config)
    if not creds:
        return True
    auth_header = request.headers.get("authorization", "")
    # Bearer check
    if "bearer" in creds and auth_header.startswith("Bearer "):
        if auth_header[7:].strip() == creds["bearer"]:
            return True
    # Basic check
    if "basic_user" in creds and auth_header.startswith("Basic "):
        try:
            decoded = base64.b64decode(auth_header[6:].strip()).decode()
            user, _, pwd = decoded.partition(":")
            if user == creds["basic_user"] and pwd == creds["basic_pass"]:
                return True
        except Exception:
            pass
    return False


def auth_dependency(config: dict[str, Any]):
    """Return a FastAPI dependency callable that returns 401 if not authed."""
    async def _dep(request: Request):
        if not check_request_auth(request, config):
            return JSONResponse({"detail": "Unauthorized"}, status_code=401)
        return None
    return _dep
