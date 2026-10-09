"""Token counting API routes."""

from __future__ import annotations

from fastapi import APIRouter, Depends, HTTPException, Request
from pydantic import BaseModel, Field

from odotoken.domain.entities import CountRequest
from odotoken.domain.errors import TokenCountError
from odotoken.services.token_counter import TokenCounter


router = APIRouter(prefix="/v1", tags=["token-counting"])


# Request/Response models
class CountTextRequest(BaseModel):
    model: str = Field(..., description="Model identifier (e.g., gpt-4o-mini)")
    text: str = Field(..., description="Text to count tokens for")
    provider: str | None = Field(None, description="Override provider detection")


class CountMessagesRequest(BaseModel):
    model: str = Field(..., description="Model identifier")
    messages: list[dict] = Field(..., description="Chat messages")
    provider: str | None = Field(None, description="Override provider detection")
    tools: list[dict] | None = Field(None, description="Tools/functions definitions")


class BatchCountRequest(BaseModel):
    items: list[CountTextRequest | CountMessagesRequest] = Field(..., min_length=1, max_length=100)


class CountResponse(BaseModel):
    model: str
    resolved_model: str
    provider: str
    tokenizer_used: str
    method: str
    input_tokens: int
    output_tokens: int
    total_tokens: int
    tokens_per_message: list[int] = []
    tokens_per_part: list[int] = []
    tool_tokens: int = 0
    overhead_tokens: int = 0
    context_window: int | None = None
    context_utilization: float | None = None
    warnings: list[str] = []
    errors: list[str] = []
    cache_hit: bool = False
    duration_ms: float = 0.0


class BatchCountResponse(BaseModel):
    total_items: int
    successful_items: int
    failed_items: int
    total_tokens: int
    duration_ms: float
    results: list[CountResponse] = []
    errors: list[dict] = []


def get_token_counter(request: Request) -> TokenCounter:
    """Dependency to get TokenCounter from app state."""
    return request.app.state.token_counter


@router.post("/count", response_model=CountResponse)
async def count_tokens(
    request: CountTextRequest | CountMessagesRequest,
    token_counter: TokenCounter = Depends(get_token_counter),
) -> CountResponse:
    """Count tokens for text or chat messages."""
    try:
        # Convert to domain entity
        if isinstance(request, CountTextRequest):
            domain_req = CountRequest(
                model=request.model,
                text=request.text,
                provider=request.provider,
            )
        else:
            domain_req = CountRequest(
                model=request.model,
                messages=request.messages,
                provider=request.provider,
                tools=request.tools,
            )

        result = await token_counter.count(domain_req)

        return CountResponse(
            model=result.model,
            resolved_model=result.resolved_model,
            provider=result.provider,
            tokenizer_used=result.tokenizer_used,
            method=result.method,
            input_tokens=result.input_tokens,
            output_tokens=result.output_tokens,
            total_tokens=result.total_tokens,
            tokens_per_message=result.tokens_per_message,
            tokens_per_part=result.tokens_per_part,
            tool_tokens=result.tool_tokens,
            overhead_tokens=result.overhead_tokens,
            context_window=result.context_window,
            context_utilization=result.context_utilization,
            warnings=result.warnings,
            errors=result.errors,
            cache_hit=result.cache_hit,
            duration_ms=result.duration_ms,
        )

    except TokenCountError as e:
        raise HTTPException(status_code=400, detail=str(e))
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Internal error: {e}")


@router.post("/count/batch", response_model=BatchCountResponse)
async def count_tokens_batch(
    request: BatchCountRequest,
    token_counter: TokenCounter = Depends(get_token_counter),
) -> BatchCountResponse:
    """Count tokens for multiple requests in batch."""
    try:
        domain_requests = []
        for item in request.items:
            if isinstance(item, CountTextRequest):
                domain_requests.append(CountRequest(
                    model=item.model,
                    text=item.text,
                    provider=item.provider,
                ))
            else:
                domain_requests.append(CountRequest(
                    model=item.model,
                    messages=item.messages,
                    provider=item.provider,
                    tools=item.tools,
                ))

        batch_result = await token_counter.count_batch(domain_requests)

        return BatchCountResponse(
            total_items=batch_result.total_items,
            successful_items=batch_result.successful_items,
            failed_items=batch_result.failed_items,
            total_tokens=batch_result.total_tokens,
            duration_ms=batch_result.duration_ms,
            results=[
                CountResponse(
                    model=r.model,
                    resolved_model=r.resolved_model,
                    provider=r.provider,
                    tokenizer_used=r.tokenizer_used,
                    method=r.method,
                    input_tokens=r.input_tokens,
                    output_tokens=r.output_tokens,
                    total_tokens=r.total_tokens,
                    tokens_per_message=r.tokens_per_message,
                    tokens_per_part=r.tokens_per_part,
                    tool_tokens=r.tool_tokens,
                    overhead_tokens=r.overhead_tokens,
                    context_window=r.context_window,
                    context_utilization=r.context_utilization,
                    warnings=r.warnings,
                    errors=r.errors,
                    cache_hit=r.cache_hit,
                    duration_ms=r.duration_ms,
                )
                for r in batch_result.results
            ],
            errors=batch_result.errors,
        )

    except TokenCountError as e:
        raise HTTPException(status_code=400, detail=str(e))
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Internal error: {e}")