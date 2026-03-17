#!/bin/bash
# ─────────────────────────────────────────────────────────────
# wa-agent — script de inicialização
# Uso: ./start.sh
# ─────────────────────────────────────────────────────────────
set -e

# Verifica variáveis obrigatórias
if [ -z "$WA_PHONE_NUMBER_ID" ]; then
  echo "❌ ERRO: variável WA_PHONE_NUMBER_ID não definida"
  echo "   export WA_PHONE_NUMBER_ID=seu_phone_number_id"
  exit 1
fi

if [ -z "$WA_ACCESS_TOKEN" ]; then
  echo "❌ ERRO: variável WA_ACCESS_TOKEN não definida"
  echo "   export WA_ACCESS_TOKEN=seu_token_permanente"
  exit 1
fi

WA_VERIFY_TOKEN=${WA_VERIFY_TOKEN:-"meu_verify_token_secreto"}
PORT=${PORT:-"8080"}

echo ""
echo "🚀 Iniciando wa-agent..."
echo "   Phone Number ID : $WA_PHONE_NUMBER_ID"
echo "   Verify Token    : $WA_VERIFY_TOKEN"
echo "   Porta           : $PORT"
echo ""
echo "📋 Próximos passos:"
echo "   1. Em outro terminal: ngrok http --domain=SEU_DOMINIO.ngrok-free.app $PORT"
echo "   2. No painel Meta: configure o webhook para https://SEU_DOMINIO.ngrok-free.app/webhook"
echo "   3. Verify Token: $WA_VERIFY_TOKEN"
echo ""

# Download de dependências e build
go mod tidy
go run main.go
