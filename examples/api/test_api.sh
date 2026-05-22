#!/bin/bash
# Script para probar el loan process via REST API
# Uso: bash test_api.sh

set -e

BASE="http://localhost:8080/api/v1"
BPMN_FILE="../BPMN-diagram-op1.bpmn"

echo "============================================"
echo "  BPMN Engine - Loan Process via API"
echo "============================================"

# ─── 1. Crear proceso ───────────────────────────
echo ""
echo "1. Creando proceso..."
BPMN_XML=$(python -c "import json; print(json.dumps(open('$BPMN_FILE').read()))")

RESP=$(curl -s -X POST "$BASE/processes" \
  -H "Content-Type: application/json" \
  -d "{\"name\": \"Loan Processing\", \"bpmn_xml\": $BPMN_XML}")
echo "   Respuesta: $RESP"

# ─── 2. Iniciar caso (approved) ─────────────────
echo ""
echo "2. Iniciando caso (documentsComplete=true, creditScoreApproved=true)..."
RESP=$(curl -s -X POST "$BASE/processes/Process_LoanProcessing/start" \
  -H "Content-Type: application/json" \
  -d '{"title": "Loan #42", "variables": {"documentsComplete": true, "creditScoreApproved": true}}')
echo "   $RESP"
CASE_ID=$(echo "$RESP" | python -c "import sys,json; print(json.load(sys.stdin)['id'])" 2>/dev/null || echo "")
echo "   Case ID: $CASE_ID"

# ─── 3. Ver caso ────────────────────────────────
echo ""
echo "3. Detalle del caso..."
curl -s "$BASE/cases/$CASE_ID" | python -m json.tool

# ─── 4. Listar tareas activas ───────────────────
echo ""
echo "4. Tareas activas..."
curl -s "$BASE/cases/$CASE_ID/tasks" | python -m json.tool

# ─── 5. Completar primera tarea ─────────────────
TASK_ID=$(curl -s "$BASE/cases/$CASE_ID/tasks" | python -c "import sys,json; tasks=json.load(sys.stdin); print(tasks[0]['flow_id'] if tasks else '')" 2>/dev/null)
if [ -n "$TASK_ID" ]; then
  echo ""
  echo "5. Completando tarea $TASK_ID..."
  curl -s -X POST "$BASE/tasks/$TASK_ID/complete" \
    -H "Content-Type: application/json" \
    -d '{"variables": {"documentsComplete": true}}' | python -m json.tool
fi

# ─── 6. History ─────────────────────────────────
echo ""
echo "6. History de ejecucion..."
curl -s "$BASE/cases/$CASE_ID/history" | python -m json.tool

# ─── 7. Audit logs ──────────────────────────────
echo ""
echo "7. Audit logs..."
curl -s "$BASE/audit" | python -m json.tool 2>/dev/null || curl -s "$BASE/audit"

echo ""
echo "============================================"
echo "  Prueba completada"
echo "============================================"
