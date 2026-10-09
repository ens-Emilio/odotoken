"""Domain errors for token counting."""

from __future__ import annotations


class TokenCountError(Exception):
    """Base exception for token counting errors."""
    def __init__(self, message: str, code: str = "TOKEN_COUNT_ERROR"):
        super().__init__(message)
        self.code = code


class ValidationError(TokenCountError):
    """Invalid request payload."""
    def __init__(self, message: str):
        super().__init__(message, "VALIDATION_ERROR")


class ModelNotSupportedError(TokenCountError):
    """Model not in catalog or not supported."""
    def __init__(self, model: str):
        super().__init__(f"Model '{model}' is not supported", "MODEL_NOT_SUPPORTED")
        self.model = model


class ProviderNotConfiguredError(TokenCountError):
    """Provider not configured (no API key, no base URL)."""
    def __init__(self, provider: str):
        super().__init__(f"Provider '{provider}' is not configured", "PROVIDER_NOT_CONFIGURED")
        self.provider = provider


class TokenizerUnavailableError(TokenCountError):
    """Local tokenizer not available."""
    def __init__(self, tokenizer: str):
        super().__init__(f"Tokenizer '{tokenizer}' is not available", "TOKENIZER_UNAVAILABLE")
        self.tokenizer = tokenizer


class ProviderTimeoutError(TokenCountError):
    """Provider API timeout."""
    def __init__(self, provider: str):
        super().__init__(f"Provider '{provider}' request timed out", "PROVIDER_TIMEOUT")
        self.provider = provider


class ProviderError(TokenCountError):
    """Provider API returned error."""
    def __init__(self, provider: str, status_code: int, message: str):
        super().__init__(f"Provider '{provider}' error ({status_code}): {message}", "PROVIDER_ERROR")
        self.provider = provider
        self.status_code = status_code


class InputTooLargeError(TokenCountError):
    """Input exceeds maximum allowed size."""
    def __init__(self, max_size: int, actual_size: int):
        super().__init__(f"Input size {actual_size} exceeds maximum {max_size}", "INPUT_TOO_LARGE")
        self.max_size = max_size
        self.actual_size = actual_size


class BatchTooLargeError(TokenCountError):
    """Batch exceeds maximum allowed items."""
    def __init__(self, max_items: int, actual_items: int):
        super().__init__(f"Batch size {actual_items} exceeds maximum {max_items}", "BATCH_TOO_LARGE")
        self.max_items = max_items
        self.actual_items = actual_items


class CacheError(TokenCountError):
    """Cache operation failed."""
    def __init__(self, message: str):
        super().__init__(message, "CACHE_ERROR")