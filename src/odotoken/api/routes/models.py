"""Models API routes."""

from __future__ import annotations

from fastapi import APIRouter, Depends, HTTPException, Request
from pydantic import BaseModel

from odotoken.services.token_counter import TokenCounter


router = APIRouter(prefix="/v1", tags=["models"])


class ModelInfoResponse(BaseModel):
    id: str
    aliases: list[str] = []
    provider: str
    family: str = ""
    context_window: int = 0
    max_output: int = 0
    supports_messages: bool = False
    supports_tools: bool = False
    supports_vision: bool = False
    counting_method: str = "heuristic"
    tokenizer: str | None = None
    chat_overhead: int = 0
    message_overhead: int = 0
    tool_overhead: int = 0
    notes: str = ""


class ModelsListResponse(BaseModel):
    models: list[ModelInfoResponse]
    total: int


def get_token_counter(request: Request) -> TokenCounter:
    return request.app.state.token_counter


@router.get("/models", response_model=ModelsListResponse)
async def list_models(
    provider: str | None = None,
    token_counter: TokenCounter = Depends(get_token_counter),
) -> ModelsListResponse:
    """List all supported models, optionally filtered by provider."""
    models = token_counter.list_models()

    if provider:
        models = [m for m in models if m.provider.lower() == provider.lower()]

    return ModelsListResponse(
        models=[
            ModelInfoResponse(
                id=m.id,
                aliases=m.aliases,
                provider=m.provider,
                family=m.family,
                context_window=m.context_window,
                max_output=m.max_output,
                supports_messages=m.supports_messages,
                supports_tools=m.supports_tools,
                supports_vision=m.supports_vision,
                counting_method=m.counting_method,
                tokenizer=m.tokenizer,
                chat_overhead=m.chat_overhead,
                message_overhead=m.message_overhead,
                tool_overhead=m.tool_overhead,
                notes=m.notes,
            )
            for m in models
        ],
        total=len(models),
    )


@router.get("/models/{model_id}", response_model=ModelInfoResponse)
async def get_model(
    model_id: str,
    token_counter: TokenCounter = Depends(get_token_counter),
) -> ModelInfoResponse:
    """Get detailed information about a specific model."""
    try:
        m = token_counter.get_model_info(model_id)
    except Exception:
        raise HTTPException(status_code=404, detail=f"Model not found: {model_id}")

    return ModelInfoResponse(
        id=m.id,
        aliases=m.aliases,
        provider=m.provider,
        family=m.family,
        context_window=m.context_window,
        max_output=m.max_output,
        supports_messages=m.supports_messages,
        supports_tools=m.supports_tools,
        supports_vision=m.supports_vision,
        counting_method=m.counting_method,
        tokenizer=m.tokenizer,
        chat_overhead=m.chat_overhead,
        message_overhead=m.message_overhead,
        tool_overhead=m.tool_overhead,
        notes=m.notes,
    )