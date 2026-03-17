package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ── Config ────────────────────────────────────────────────────────────────────

type Config struct {
	PhoneNumberID string
	AccessToken   string
	VerifyToken   string
	Port          string
}

var cfg Config

// ── Models ────────────────────────────────────────────────────────────────────

type WAWebhookPayload struct {
	Object string  `json:"object"`
	Entry  []Entry `json:"entry"`
}

type Entry struct {
	ID      string   `json:"id"`
	Changes []Change `json:"changes"`
}

type Change struct {
	Value ChangeValue `json:"value"`
	Field string      `json:"field"`
}

type ChangeValue struct {
	MessagingProduct string    `json:"messaging_product"`
	Metadata         Metadata  `json:"metadata"`
	Contacts         []Contact `json:"contacts,omitempty"`
	Messages         []Message `json:"messages,omitempty"`
	Statuses         []Status  `json:"statuses,omitempty"`
}

type Metadata struct {
	DisplayPhoneNumber string `json:"display_phone_number"`
	PhoneNumberID      string `json:"phone_number_id"`
}

type Contact struct {
	Profile ContactProfile `json:"profile"`
	WaID    string         `json:"wa_id"`
}

type ContactProfile struct {
	Name string `json:"name"`
}

type Message struct {
	From      string      `json:"from"`
	ID        string      `json:"id"`
	Timestamp string      `json:"timestamp"`
	Type      string      `json:"type"`
	Text      *TextBody   `json:"text,omitempty"`
	Image     *MediaBody  `json:"image,omitempty"`
	Audio     *MediaBody  `json:"audio,omitempty"`
	Document  *MediaBody  `json:"document,omitempty"`
}

type TextBody struct {
	Body string `json:"body"`
}

type MediaBody struct {
	ID       string `json:"id"`
	MimeType string `json:"mime_type"`
	Caption  string `json:"caption,omitempty"`
}

type Status struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	Timestamp    string `json:"timestamp"`
	RecipientID  string `json:"recipient_id"`
}

type SendMessageRequest struct {
	To   string `json:"to"`
	Text string `json:"text"`
}

// ── WebSocket hub ─────────────────────────────────────────────────────────────

type Hub struct {
	mu      sync.RWMutex
	clients map[*websocket.Conn]bool
}

func NewHub() *Hub { return &Hub{clients: make(map[*websocket.Conn]bool)} }

func (h *Hub) Register(c *websocket.Conn) {
	h.mu.Lock()
	h.clients[c] = true
	h.mu.Unlock()
}

func (h *Hub) Unregister(c *websocket.Conn) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
}

func (h *Hub) Broadcast(msg interface{}) {
	data, _ := json.Marshal(msg)
	h.mu.RLock()
	for c := range h.clients {
		if err := c.WriteMessage(websocket.TextMessage, data); err != nil {
			go h.Unregister(c)
		}
	}
	h.mu.RUnlock()
}

var hub = NewHub()

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func webhookHandler(w http.ResponseWriter, r *http.Request) {
	// Verification handshake
	if r.Method == http.MethodGet {
		mode  := r.URL.Query().Get("hub.mode")
		token := r.URL.Query().Get("hub.verify_token")
		challenge := r.URL.Query().Get("hub.challenge")
		if mode == "subscribe" && token == cfg.VerifyToken {
			log.Println("[webhook] verificado com sucesso")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, challenge)
			return
		}
		log.Println("[webhook] token inválido:", token)
		w.WriteHeader(http.StatusForbidden)
		return
	}

	// Receive events
	if r.Method == http.MethodPost {
		// ACK imediato — regra crítica da Meta (< 5s)
		w.WriteHeader(http.StatusOK)

		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Println("[webhook] erro ao ler body:", err)
			return
		}

		var payload WAWebhookPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			log.Println("[webhook] erro ao parsear JSON:", err)
			return
		}

		// Processar de forma assíncrona
		go processWebhook(payload)
	}
}

func processWebhook(payload WAWebhookPayload) {
	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			if change.Field != "messages" {
				continue
			}
			val := change.Value

			// Mensagens recebidas
			for i, msg := range val.Messages {
				contactName := "Desconhecido"
				if i < len(val.Contacts) {
					contactName = val.Contacts[i].Profile.Name
				}

				event := map[string]interface{}{
					"type":        "message_received",
					"id":          msg.ID,
					"from":        msg.From,
					"from_name":   contactName,
					"timestamp":   msg.Timestamp,
					"msg_type":    msg.Type,
					"received_at": time.Now().Format(time.RFC3339),
				}

				switch msg.Type {
				case "text":
					if msg.Text != nil {
						event["text"] = msg.Text.Body
					}
				case "image":
					if msg.Image != nil {
						event["media_id"] = msg.Image.ID
						event["caption"] = msg.Image.Caption
					}
				case "audio":
					if msg.Audio != nil {
						event["media_id"] = msg.Audio.ID
					}
				}

				log.Printf("[msg recebida] de=%s (%s) tipo=%s\n", msg.From, contactName, msg.Type)
				hub.Broadcast(event)

				// Marca como lida
				go markAsRead(msg.ID)
			}

			// Status updates
			for _, s := range val.Statuses {
				event := map[string]interface{}{
					"type":      "status_update",
					"msg_id":    s.ID,
					"status":    s.Status,
					"to":        s.RecipientID,
					"timestamp": s.Timestamp,
				}
				log.Printf("[status] msgId=%s status=%s\n", s.ID, s.Status)
				hub.Broadcast(event)
			}
		}
	}
}

func sendMessageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if req.To == "" || req.Text == "" {
		http.Error(w, "to e text são obrigatórios", http.StatusBadRequest)
		return
	}

	msgID, err := sendWhatsAppMessage(req.To, req.Text)
	if err != nil {
		log.Println("[send] erro:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "sent",
		"msg_id": msgID,
	})
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("[ws] upgrade error:", err)
		return
	}
	hub.Register(conn)
	log.Println("[ws] cliente conectado")

	// Keep-alive ping
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				hub.Unregister(conn)
				conn.Close()
				return
			}
		}
	}()

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			hub.Unregister(conn)
			conn.Close()
			log.Println("[ws] cliente desconectado")
			return
		}
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":          "ok",
		"phone_number_id": cfg.PhoneNumberID,
		"ws_clients":      len(hub.clients),
		"time":            time.Now().Format(time.RFC3339),
	})
}

// ── Meta API calls ────────────────────────────────────────────────────────────

func sendWhatsAppMessage(to, text string) (string, error) {
	url := fmt.Sprintf("https://graph.facebook.com/v21.0/%s/messages", cfg.PhoneNumberID)

	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                to,
		"type":              "text",
		"text":              map[string]string{"body": text},
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+cfg.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	respBody, _ := io.ReadAll(resp.Body)
	json.Unmarshal(respBody, &result)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("meta api error %d: %s", resp.StatusCode, string(respBody))
	}

	// Extrai o message ID da resposta
	if msgs, ok := result["messages"].([]interface{}); ok && len(msgs) > 0 {
		if msg, ok := msgs[0].(map[string]interface{}); ok {
			if id, ok := msg["id"].(string); ok {
				log.Printf("[send] mensagem enviada para %s msgId=%s\n", to, id)
				return id, nil
			}
		}
	}

	return "", nil
}

func markAsRead(messageID string) {
	url := fmt.Sprintf("https://graph.facebook.com/v21.0/%s/messages", cfg.PhoneNumberID)
	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"status":            "read",
		"message_id":        messageID,
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+cfg.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println("[read] erro:", err)
		return
	}
	defer resp.Body.Close()
	log.Printf("[read] msgId=%s marcada como lida\n", messageID)
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	cfg = Config{
		PhoneNumberID: getEnv("WA_PHONE_NUMBER_ID", ""),
		AccessToken:   getEnv("WA_ACCESS_TOKEN", ""),
		VerifyToken:   getEnv("WA_VERIFY_TOKEN", "meu_verify_token_secreto"),
		Port:          getEnv("PORT", "8080"),
	}

	if cfg.PhoneNumberID == "" || cfg.AccessToken == "" {
		log.Fatal("ERRO: defina WA_PHONE_NUMBER_ID e WA_ACCESS_TOKEN como variáveis de ambiente")
	}

	mux := http.NewServeMux()

	// CORS middleware
	handler := corsMiddleware(mux)

	mux.HandleFunc("/webhook",      webhookHandler)
	mux.HandleFunc("/api/send",     sendMessageHandler)
	mux.HandleFunc("/ws",           wsHandler)
	mux.HandleFunc("/health",       healthHandler)

	// Frontend estático
	mux.Handle("/", http.FileServer(http.Dir("./static")))

	log.Printf("🚀 Servidor rodando em http://localhost:%s\n", cfg.Port)
	log.Printf("📡 Webhook URL: http://localhost:%s/webhook\n", cfg.Port)
	log.Printf("🔑 Verify token: %s\n", cfg.VerifyToken)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, handler))
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
