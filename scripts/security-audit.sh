#!/usr/bin/env bash
# Auditoria de Segurança para OdoToken - Fase 5 (Validação Externa)
# Uso: ./scripts/security-audit.sh [URL_BASE]
# Verifica headers de segurança, endpoints expostos e configurações básicas.

set -euo pipefail

URL_BASE="${1:-http://localhost:8080}"
API_KEY="${ODOTOKEN_API_KEY:-test-key}"
ISSUES=0

echo "=== OdoToken Security Audit ==="
echo "URL: $URL_BASE"
echo "Data: $(date -u +"%Y-%m-%dT%H:%M:%SZ")"
echo ""

# Verificar se o servidor está acessível
if ! curl -sf "$URL_BASE/health" > /dev/null 2>&1; then
 echo "ERRO: Servidor não acessível em $URL_BASE/health"
 exit 1
fi

check_header() {
 local url="$1"
 local header="$2"
 local expected="$3"
 local desc="$4"
 
 value=$(curl -sI "$url" | grep -i "^$header:" | head -1 | sed "s/^$header: //i" | tr -d '\r')
 if [ -z "$value" ]; then
 echo "[WARN] $desc: header '$header' ausente"
 ISSUES=$((ISSUES + 1))
 elif [ -n "$expected" ] && ! echo "$value" | grep -qi "$expected"; then
 echo "[WARN] $desc: '$header' = '$value' (esperado: '$expected')"
 ISSUES=$((ISSUES + 1))
 else
 echo "[OK] $desc: '$header' presente"
 fi
}

check_endpoint_auth() {
 local endpoint="$1"
 local desc="$2"
 
 status=$(curl -s -o /dev/null -w "%{http_code}" "$URL_BASE$endpoint")
 if [ "$status" = "200" ]; then
 echo "[CRIT] $desc ($endpoint): acessível sem autenticação (HTTP 200)"
 ISSUES=$((ISSUES + 1))
 elif [ "$status" = "401" ] || [ "$status" = "403" ]; then
 echo "[OK] $desc ($endpoint): requer autenticação (HTTP $status)"
 else
 echo "[INFO] $desc ($endpoint): HTTP $status"
 fi
}

echo "--- Headers de Segurança ---"
check_header "$URL_BASE/health" "X-Content-Type-Options" "nosniff" "X-Content-Type-Options"
check_header "$URL_BASE/health" "X-Frame-Options" "" "X-Frame-Options"
check_header "$URL_BASE/health" "Strict-Transport-Security" "" "HSTS"
check_header "$URL_BASE/health" "Content-Security-Policy" "" "CSP"
echo ""

echo "--- Endpoints Sensíveis (sem auth) ---"
check_endpoint_auth "/api/client-keys" "Lista de chaves"
check_endpoint_auth "/api/prices" "Regras de preço"
check_endpoint_auth "/api/guard" "Políticas de guard"
check_endpoint_auth "/metrics" "Métricas Prometheus"
echo ""

echo "--- Teste de Autenticação ---"
status_no_key=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$URL_BASE/v1/chat/completions" \
 -H "Content-Type: application/json" \
 -d '{"model":"test","messages":[{"role":"user","content":"hi"}]}')
 
status_bad_key=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$URL_BASE/v1/chat/completions" \
 -H "Authorization: Bearer invalid-key-12345" \
 -H "Content-Type: application/json" \
 -d '{"model":"test","messages":[{"role":"user","content":"hi"}]}')

if [ "$status_no_key" = "401" ] || [ "$status_no_key" = "403" ]; then
 echo "[OK] Proxy rejeita requisição sem API Key (HTTP $status_no_key)"
else
 echo "[CRIT] Proxy aceita requisição sem API Key (HTTP $status_no_key)"
 ISSUES=$((ISSUES + 1))
fi

if [ "$status_bad_key" = "401" ] || [ "$status_bad_key" = "403" ]; then
 echo "[OK] Proxy rejeita API Key inválida (HTTP $status_bad_key)"
else
 echo "[CRIT] Proxy aceita API Key inválida (HTTP $status_bad_key)"
 ISSUES=$((ISSUES + 1))
fi
echo ""

echo "--- Verificação de Dependências (Go) ---"
if command -v go &> /dev/null && [ -f "go.mod" ]; then
 echo "Executando 'go list -m all' para verificar dependências..."
 if go list -m all 2>/dev/null | grep -q "golang.org/x/net"; then
 echo "[INFO] golang.org/x/net presente (verifique CVEs conhecidos)"
 fi
 echo "[OK] Dependências listadas com sucesso"
else
 echo "[SKIP] Go não disponível ou go.mod ausente"
fi
echo ""

echo "=== Resultado ==="
if [ "$ISSUES" -eq 0 ]; then
 echo "Nenhum problema crítico encontrado."
 exit 0
else
 echo "$ISSUES problema(s) encontrado(s). Revise os itens acima."
 exit 1
fi