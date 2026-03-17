# wa-agent

Painel de atendimento ao cliente via WhatsApp, integrado diretamente à **Meta Cloud API oficial**. Backend em Go com WebSocket para atualizações em tempo real e frontend leve servido pelo próprio servidor.

## Arquitetura

```
WhatsApp (usuário)
       │
       ▼  webhook POST (Meta → seu servidor)
┌─────────────────────────────────────────┐
│           Go Backend (main.go)          │
│                                         │
│  POST /webhook  ── processa eventos     │
│  POST /api/send ── envia mensagens      │
│  GET  /ws       ── WebSocket hub        │
│  GET  /          ── UI estática         │
└──────────────┬──────────────────────────┘
               │ WebSocket
               ▼
      Browser (static/index.html)
      UI do agente em tempo real
```

**Fluxo de mensagem recebida:**
1. Usuário envia mensagem no WhatsApp
2. Meta faz `POST /webhook` no seu servidor
3. Go responde `200 OK` imediatamente (< 5s, obrigatório)
4. Processa assincronamente e publica via WebSocket
5. UI do agente recebe e exibe em tempo real

**Fluxo de mensagem enviada:**
1. Agente digita e envia pela UI
2. UI faz `POST /api/send` no Go
3. Go chama `graph.facebook.com/v21.0/{phone_id}/messages`
4. Meta retorna `wamid` (message ID)
5. Status `delivered`/`read` chegam via webhook → atualizam os ticks na UI

## Pré-requisitos

- [Go 1.21+](https://go.dev/dl/) — ou Docker
- Conta no [Meta for Developers](https://developers.facebook.com)
- [ngrok](https://ngrok.com/download) — para expor localhost via HTTPS

## Configuração rápida

### 1. Clone e configure as variáveis

```bash
git clone https://github.com/SEU_USUARIO/wa-agent.git
cd wa-agent

cp .env.example .env
# Edite .env com seus valores reais
```

### 2. Configure o app na Meta

1. Acesse [developers.facebook.com](https://developers.facebook.com) → **Create App** → tipo **Business**
2. Adicione o produto **WhatsApp** → aceite os termos
3. A Meta cria automaticamente um WABA de teste e um número de teste
4. Em **WhatsApp → API Setup**, copie o **Phone Number ID** e o **WABA ID**

### 3. Gere um token permanente

1. Acesse [business.facebook.com](https://business.facebook.com) → Settings → **Users → System Users**
2. Crie um usuário **Admin** → **Assign Assets** → selecione seu app (Full control) e seu WhatsApp Account (Manage)
3. **Generate Token** → escopos: `whatsapp_business_messaging`, `whatsapp_business_management`
4. Copie o token para `WA_ACCESS_TOKEN` no `.env`

### 4. Exponha o localhost com ngrok

```bash
# Instale e autentique o ngrok
ngrok config add-authtoken SEU_AUTHTOKEN

# Reserve seu domínio estático gratuito em: dashboard.ngrok.com → Domains → + New Domain
# Depois inicie o túnel:
ngrok http --domain=SEU_SUBDOMINIO.ngrok-free.app 8080
```

### 5. Configure o webhook na Meta

No painel **WhatsApp → Configuration**:
- **Callback URL:** `https://SEU_SUBDOMINIO.ngrok-free.app/webhook`
- **Verify Token:** valor de `WA_VERIFY_TOKEN` no seu `.env`
- Clique **Verify and Save**
- Em **Webhook fields**, assine: `messages`

### 6. Rode o servidor

```bash
# Com Go instalado
source .env  # ou: export $(cat .env | xargs)
./start.sh

# Com Docker
docker compose up --build
```

Abra **http://localhost:8080** no browser.

## Testando

No painel **API Setup** da Meta, adicione seu número pessoal como destinatário de teste. Envie uma mensagem do seu WhatsApp para o número de teste — ela aparece na UI em tempo real.

Para responder: selecione a conversa na sidebar, digite e pressione Enter (ou clique Enviar).

## Variáveis de ambiente

| Variável | Obrigatória | Descrição |
|---|---|---|
| `WA_PHONE_NUMBER_ID` | ✅ | ID do número de telefone (Meta API Setup) |
| `WA_ACCESS_TOKEN` | ✅ | Token permanente do System User |
| `WA_VERIFY_TOKEN` | ✅ | Token de verificação do webhook (você define) |
| `PORT` | — | Porta do servidor (padrão: `8080`) |

## Endpoints

| Método | Path | Descrição |
|---|---|---|
| `GET` | `/webhook` | Verificação do webhook pela Meta |
| `POST` | `/webhook` | Recebe eventos da Meta (mensagens, status) |
| `POST` | `/api/send` | Envia mensagem via Cloud API |
| `GET` | `/ws` | WebSocket para o frontend |
| `GET` | `/health` | Status do servidor |
| `GET` | `/` | UI do agente |

## Estrutura do projeto

```
wa-agent/
├── main.go              # Backend Go completo
├── go.mod               # Dependências
├── start.sh             # Script de inicialização
├── Dockerfile           # Build da imagem Docker
├── docker-compose.yml   # Orquestração local
├── .env.example         # Modelo de variáveis de ambiente
├── .gitignore
├── .github/
│   └── workflows/
│       └── build.yml    # CI via GitHub Actions
└── static/
    └── index.html       # UI do agente (WebSocket + REST)
```

## Próximos passos

- [ ] Persistência de conversas (PostgreSQL)
- [ ] Autenticação de agentes (JWT)
- [ ] Multi-tenant (múltiplos WABAs)
- [ ] Suporte a mídia (imagens, áudio, documentos)
- [ ] Roteamento de conversas entre agentes
- [ ] App mobile com Flutter + `flutter_chat_ui`

## Licença

MIT
