#!/bin/bash
# API examples using curl

BASE_URL="http://127.0.0.1:8080"

# Count text
curl -X POST "$BASE_URL/v1/count" \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4o-mini", "text": "Hello world"}'

echo -e "\n\n---\n"

# Count chat messages
curl -X POST "$BASE_URL/v1/count" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [
      {"role": "user", "content": "Hello"},
      {"role": "assistant", "content": "Hi there!"}
    ]
  }'

echo -e "\n\n---\n"

# Batch count
curl -X POST "$BASE_URL/v1/count/batch" \
  -H "Content-Type: application/json" \
  -d '{
    "items": [
      {"model": "gpt-4o-mini", "text": "Test 1"},
      {"model": "gpt-4o-mini", "text": "Test 2"}
    ]
  }'

echo -e "\n\n---\n"

# List models
curl "$BASE_URL/v1/models"

echo -e "\n\n---\n"

# Health check
curl "$BASE_URL/health"