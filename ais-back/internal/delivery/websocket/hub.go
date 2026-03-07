package websocket

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/bakdaulet/ais/ais-back/internal/application/chat"
	domainai "github.com/bakdaulet/ais/ais-back/internal/domain/ai"
	"github.com/bakdaulet/ais/ais-back/internal/domain/analysis"
	"github.com/bakdaulet/ais/ais-back/pkg/logger"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true // CORS is handled at the HTTP layer
	},
	HandshakeTimeout: 10 * time.Second,
}

// MessageType enumerates WebSocket message types.
type MessageType string

const (
	MsgTypeSubscribe   MessageType = "subscribe"
	MsgTypeUnsubscribe MessageType = "unsubscribe"
	MsgTypeChat        MessageType = "chat"
	MsgTypeProgress    MessageType = "progress"
	MsgTypeChatToken   MessageType = "chat_token"
	MsgTypeError       MessageType = "error"
	MsgTypePing        MessageType = "ping"
	MsgTypePong        MessageType = "pong"
)

// IncomingMessage represents a message from the frontend client.
type IncomingMessage struct {
	Type    MessageType  `json:"type"`
	RepoID  string       `json:"repoId,omitempty"`
	NodeID  string       `json:"nodeId,omitempty"`
	Message string       `json:"message,omitempty"`
	History []ChatHistoryItem `json:"history,omitempty"`
}

// ChatHistoryItem is a single message in the conversation history.
type ChatHistoryItem struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OutgoingMessage represents a message sent from server to frontend.
type OutgoingMessage struct {
	Type       MessageType  `json:"type"`
	RepoID     string       `json:"repoId,omitempty"`
	Step       string       `json:"step,omitempty"`
	Progress   int          `json:"progress,omitempty"`
	Message    string       `json:"message,omitempty"`
	Error      string       `json:"error,omitempty"`
	Token      string       `json:"token,omitempty"`
	Done       bool         `json:"done,omitempty"`
	References interface{}  `json:"references,omitempty"`
	At         time.Time    `json:"at,omitempty"`
}

// Hub manages all active WebSocket connections and message routing.
type Hub struct {
	clients     map[*Client]bool
	repoSubs    map[string]map[*Client]bool // repoID → set of clients
	progressCh  chan *analysis.ProgressEvent
	register    chan *Client
	unregister  chan *Client
	broadcast   chan *OutgoingMessage
	chatUseCase *chat.UseCase
	mu          sync.RWMutex
	log         *logger.Logger
}

// Client represents a single WebSocket connection.
type Client struct {
	hub        *Hub
	conn       *websocket.Conn
	send       chan []byte
	repoSubs   map[string]bool
	log        *logger.Logger
}

// NewHub creates and returns a new WebSocket Hub.
func NewHub(chatUseCase *chat.UseCase, log *logger.Logger) *Hub {
	return &Hub{
		clients:     make(map[*Client]bool),
		repoSubs:    make(map[string]map[*Client]bool),
		progressCh:  make(chan *analysis.ProgressEvent, 512),
		register:    make(chan *Client, 32),
		unregister:  make(chan *Client, 32),
		broadcast:   make(chan *OutgoingMessage, 512),
		chatUseCase: chatUseCase,
		log:         log.WithComponent("ws_hub"),
	}
}

// Run starts the hub's event loop. Must be called in a goroutine.
func (h *Hub) Run(ctx context.Context) {
	h.log.Info("WebSocket hub started")
	for {
		select {
		case <-ctx.Done():
			h.log.Info("WebSocket hub shutting down")
			return

		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			h.log.Debug("client connected", zap.Int("total_clients", len(h.clients)))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				// Remove from all repo subscriptions
				for repoID := range client.repoSubs {
					if subs, ok := h.repoSubs[repoID]; ok {
						delete(subs, client)
						if len(subs) == 0 {
							delete(h.repoSubs, repoID)
						}
					}
				}
				close(client.send)
			}
			h.mu.Unlock()
			h.log.Debug("client disconnected", zap.Int("total_clients", len(h.clients)))

		case event := <-h.progressCh:
			h.broadcastToRepo(event.RepoID, &OutgoingMessage{
				Type:     MsgTypeProgress,
				RepoID:   event.RepoID,
				Step:     string(event.Step),
				Progress: event.Progress,
				Message:  event.Message,
				Error:    event.Error,
				At:       event.At,
			})

		case msg := <-h.broadcast:
			if msg.RepoID != "" {
				h.broadcastToRepo(msg.RepoID, msg)
			} else {
				h.broadcastAll(msg)
			}
		}
	}
}

// EmitProgress sends a progress event to all subscribers of a repository.
// Implements analysis.ProgressEmitter.
func (h *Hub) Emit(event *analysis.ProgressEvent) {
	select {
	case h.progressCh <- event:
	default:
		// Drop if channel is full (non-blocking)
		h.log.Warn("progress channel full, dropping event",
			zap.String("repo_id", event.RepoID),
			zap.String("step", string(event.Step)))
	}
}

// ServeHTTP upgrades an HTTP connection to WebSocket.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Error("WebSocket upgrade failed", zap.Error(err))
		return
	}

	client := &Client{
		hub:      h,
		conn:     conn,
		send:     make(chan []byte, 256),
		repoSubs: make(map[string]bool),
		log:      h.log,
	}

	h.register <- client

	go client.writePump()
	go client.readPump()
}

// broadcastToRepo sends a message to all clients subscribed to a repo.
func (h *Hub) broadcastToRepo(repoID string, msg *OutgoingMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		h.log.Error("failed to marshal WebSocket message", zap.Error(err))
		return
	}

	h.mu.RLock()
	subs := h.repoSubs[repoID]
	h.mu.RUnlock()

	for client := range subs {
		select {
		case client.send <- data:
		default:
			// Client send buffer full
			h.log.Warn("client send buffer full, dropping message",
				zap.String("repo_id", repoID))
		}
	}
}

// broadcastAll sends a message to all connected clients.
func (h *Hub) broadcastAll(msg *OutgoingMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		select {
		case client.send <- data:
		default:
		}
	}
}

// subscribe adds a client to a repo's subscriber list.
func (h *Hub) subscribe(client *Client, repoID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.repoSubs[repoID]; !ok {
		h.repoSubs[repoID] = make(map[*Client]bool)
	}
	h.repoSubs[repoID][client] = true
	client.repoSubs[repoID] = true

	h.log.Debug("client subscribed to repo",
		zap.String("repo_id", repoID),
		zap.Int("subscribers", len(h.repoSubs[repoID])))
}

// ---------------------------------------------------------------------------
// Client read/write pumps
// ---------------------------------------------------------------------------

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 65536
)

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure,
			) {
				c.log.Warn("WebSocket read error", zap.Error(err))
			}
			return
		}

		var msg IncomingMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			c.log.Warn("invalid WebSocket message", zap.Error(err))
			c.sendError("invalid message format")
			continue
		}

		c.handleMessage(msg)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case data, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(data)

			// Flush any pending messages in the buffer
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte("\n"))
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleMessage(msg IncomingMessage) {
	switch msg.Type {
	case MsgTypeSubscribe:
		if msg.RepoID != "" {
			c.hub.subscribe(c, msg.RepoID)
			c.sendJSON(&OutgoingMessage{
				Type:    MsgTypePong,
				RepoID:  msg.RepoID,
				Message: "subscribed",
			})
		}

	case MsgTypeUnsubscribe:
		if msg.RepoID != "" {
			c.hub.mu.Lock()
			if subs, ok := c.hub.repoSubs[msg.RepoID]; ok {
				delete(subs, c)
			}
			delete(c.repoSubs, msg.RepoID)
			c.hub.mu.Unlock()
		}

	case MsgTypeChat:
		if msg.RepoID == "" || msg.Message == "" {
			c.sendError("repoId and message are required for chat")
			return
		}
		go c.handleChat(msg)

	case MsgTypePing:
		c.sendJSON(&OutgoingMessage{Type: MsgTypePong})

	default:
		c.sendError("unknown message type: " + string(msg.Type))
	}
}

func (c *Client) handleChat(msg IncomingMessage) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Convert history
	history := make([]*domainai.ChatMessage, 0, len(msg.History))
	for _, h := range msg.History {
		history = append(history, &domainai.ChatMessage{
			Role:    h.Role,
			Content: h.Content,
		})
	}

	tokenCh := make(chan *chat.TokenEvent, 128)

	// Start streaming in goroutine
	go func() {
		err := c.hub.chatUseCase.StreamChat(ctx, &chat.ChatRequest{
			RepoID:  msg.RepoID,
			Message: msg.Message,
			NodeID:  msg.NodeID,
			History: history,
		}, tokenCh)
		if err != nil {
			c.log.Error("chat stream error", zap.Error(err))
			c.sendJSON(&OutgoingMessage{
				Type:   MsgTypeError,
				RepoID: msg.RepoID,
				Error:  "Chat service error: " + err.Error(),
			})
		}
	}()

	// Forward tokens to client
	for token := range tokenCh {
		c.sendJSON(&OutgoingMessage{
			Type:       MsgTypeChatToken,
			RepoID:     msg.RepoID,
			Token:      token.Token,
			Done:       token.Done,
			References: token.References,
		})
	}
}

func (c *Client) sendJSON(msg *OutgoingMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	select {
	case c.send <- data:
	default:
	}
}

func (c *Client) sendError(errMsg string) {
	c.sendJSON(&OutgoingMessage{
		Type:  MsgTypeError,
		Error: errMsg,
	})
}
