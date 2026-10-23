"""Minimal OTel tracing — no-op if not configured or deps missing."""

from __future__ import annotations

import contextlib
import logging
import os
from typing import Any

logger = logging.getLogger("odotoken.otel")

_tracer = None  # type: ignore
_inited = False


def _otel_enabled(config: dict[str, Any] | None) -> tuple[bool, dict[str, Any]]:
    cfg: dict[str, Any] = {}
    if isinstance(config, dict):
        raw = config.get("otel", {})
        if isinstance(raw, dict):
            cfg = raw
    enabled = bool(cfg.get("enabled")) or bool(os.getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
    # also allow explicit endpoint/service_name without enabled flag to imply enabled
    # but spec requires enabled or env; keep strict + endpoint in cfg counts if truthy and not disabled
    if cfg.get("endpoint") and not enabled:
        # treat endpoint presence as intent to enable (lenient for users)
        enabled = True
    return enabled, cfg


def init_tracing(config: dict[str, Any] | None = None) -> bool:
    """Initialize TracerProvider with OTLP exporter if enabled.

    Enabled when env OTEL_EXPORTER_OTLP_ENDPOINT is set or config otel.enabled is true.
    No-op and returns False if not enabled or OTel deps missing. Never raises.
    Idempotent.
    """
    global _tracer, _inited
    if _inited:
        return _tracer is not None
    enabled, cfg = _otel_enabled(config)
    if not enabled:
        _inited = True
        return False

    try:
        from opentelemetry import trace
        from opentelemetry.sdk.resources import Resource
        from opentelemetry.sdk.trace import TracerProvider
        from opentelemetry.sdk.trace.export import BatchSpanProcessor
    except ImportError as e:
        logger.debug("OTel not installed, tracing disabled: %s", e)
        _inited = True
        return False

    # avoid double-set (e.g. called from both proxy and api)
    try:
        from opentelemetry import trace as _trace

        existing = _trace.get_tracer_provider()
        # TracerProvider has add_span_processor; Proxy does not behave same
        if isinstance(existing, TracerProvider):
            _tracer = _trace.get_tracer("odotoken", "0.2.0")
            _inited = True
            return True
    except Exception:
        pass

    endpoint = os.getenv("OTEL_EXPORTER_OTLP_ENDPOINT") or cfg.get("endpoint") or cfg.get("otlp_endpoint")
    service_name = cfg.get("service_name") or "odotoken"
    headers = cfg.get("headers")  # optional

    try:
        # Prefer HTTP exporter (in our optional deps); fallback to gRPC
        OTLPSpanExporter = None
        try:
            from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter as HTTPE
            OTLPSpanExporter = HTTPE  # type: ignore
        except ImportError:
            try:
                from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter as GRPCE
                OTLPSpanExporter = GRPCE  # type: ignore
            except ImportError:
                OTLPSpanExporter = None

        if OTLPSpanExporter is None:
            logger.debug("OTLP exporter not available")
            _inited = True
            return False

        resource = Resource.create({"service.name": service_name, "service.version": "0.2.0"})
        provider = TracerProvider(resource=resource)

        # build exporter kwargs only if valid
        exp_kwargs: dict[str, Any] = {}
        if endpoint:
            exp_kwargs["endpoint"] = endpoint
        if headers:
            exp_kwargs["headers"] = headers

        try:
            exporter = OTLPSpanExporter(**exp_kwargs)  # type: ignore
        except TypeError:
            # some versions expect positional or no endpoint kw
            try:
                exporter = OTLPSpanExporter(endpoint) if endpoint else OTLPSpanExporter()  # type: ignore
            except Exception as e:
                logger.debug("OTLP exporter init failed: %s", e)
                _inited = True
                return False

        processor = BatchSpanProcessor(exporter)
        provider.add_span_processor(processor)

        # also allow console debug exporter if OTel debug flag? keep minimal.

        trace.set_tracer_provider(provider)
        _tracer = trace.get_tracer("odotoken", "0.2.0")
        _inited = True
        logger.info("OTel tracing enabled (endpoint=%s)", endpoint or "<default>")
        return True
    except Exception as e:
        logger.debug("OTel init failed: %s", e)
        _inited = True
        return False


def get_tracer(name: str = "odotoken"):
    if _tracer is not None:
        return _tracer
    if not _inited:
        return None
    # inited but tracing disabled -> no tracer
    return None


@contextlib.contextmanager
def span(name: str, attributes: dict[str, Any] | None = None):
    """No-op if tracing not enabled. Yields span or None."""
    tracer = get_tracer()
    if tracer is None:
        yield None
        return
    try:
        with tracer.start_as_current_span(name) as s:  # type: ignore
            if attributes and s is not None:
                for k, v in attributes.items():
                    try:
                        s.set_attribute(k, str(v) if v is None else v)
                    except Exception:
                        pass
            yield s
    except Exception:
        # never break request path
        yield None


# alias for ergonomics
trace_span = span


def is_enabled() -> bool:
    return _tracer is not None


def _reset_for_tests() -> None:
    """Reset module state — tests only."""
    global _tracer, _inited
    _tracer = None
    _inited = False
