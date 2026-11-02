#!/usr/bin/env bash
# Benchmark de carga para OdoToken - Fase 5 (Validação Externa)
# Uso: ./scripts/benchmark.sh [URL_BASE] [DURAÇÃO]
# Exemplo: ./scripts/benchmark.sh http://localhost:8080 30s

set -euo pipefail

URL_BASE="${1:-http://localhost:8080}"
DURATION="${2:-30s}"
API_KEY="${ODOTOKEN_API_KEY:-test-key}"

echo "=== OdoToken Benchmark ==="
echo "URL: $URL_BASE"
echo "Duração: $DURATION"
echo "API Key: ${API_KEY:0:8}..."
echo ""

# Verificar se o servidor está acessível
if ! curl -sf "$URL_BASE/health" > /dev/null 2>&1; then
    echo "ERRO: Servidor não acessível em $URL_BASE/health"
    echo "Inicie o servidor com: go run ./cmd/odotoken"
    exit 1
fi

echo "Servidor acessível. Iniciando benchmark..."
echo ""

# Benchmark do endpoint /health (leve)
echo "--- /health ---"
if command -v wrk &> /dev/null; then
    wrk -t4 -c50 -d"$DURATION" "$URL_BASE/health"
elif command -v ab &> /dev/null; then
    ab -n 10000 -c 50 "$URL_BASE/health"
else
    echo "wrk ou ab não encontrados. Usando curl simples..."
    START=$(date +%s%N)
    COUNT=0
    END_TIME=$((START + ${DURATION%s} * 1000000000))
    while [ "$(date +%s%N)" -lt "$END_TIME" ]; do
        curl -sf "$URL_BASE/health" > /dev/null
        COUNT=$((COUNT + 1))
    done
    ELAPSED=$(( ($(date +%s%N) - START) / 1000000 ))
    RPS=$((COUNT * 1000 / ELAPSED))
    echo "Requisições: $COUNT em ${ELAPSED}ms (~${RPS} req/s)"
fi

echo ""

# Benchmark do proxy /v1/chat/completions (se disponível)
echo "--- /v1/chat/completions ---"
PAYLOAD='{"model":"gpt-4","messages":[{"role":"user","content":"ping"}],"max_tokens":5}'
RESPONSE=$(curl -sf -X POST "$URL_BASE/v1/chat/completions" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    -d "$PAYLOAD" \
    -w "\n%{http_code}" 2>/dev/null || echo "000")

HTTP_CODE=$(echo "$RESPONSE" | tail -1)
if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "429" ]; then
    echo "Endpoint responsivo (HTTP $HTTP_CODE). Rodando benchmark..."
    if command -v wrk &> /dev/null; then
        wrk -t2 -c10 -d"$DURATION" \
            -H "Authorization: Bearer $API_KEY" \
            -H "Content-Type: application/json" \
            -s /dev/stdin "$URL_BASE/v1/chat/completions" <<EOF
wrk.method = "POST"
wrk.body = '$PAYLOAD'
EOF
    else
        echo "wrk não encontrado. Pulando benchmark pesado."
    fi
else
    echo "Endpoint retornou HTTP $HTTP_CODE. Pulando benchmark do proxy."
fi

echo ""
echo "=== Benchmark concluído ==="