package app

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"nhooyr.io/websocket"
)

type Hub struct {
	db      *pgxpool.Pool
	logger  *slog.Logger
	mu      sync.RWMutex
	agents  map[string]*AgentConn
	clients map[string]map[*ClientConn]struct{}
}

func NewHub(db *pgxpool.Pool, logger *slog.Logger) *Hub {
	return &Hub{
		db:      db,
		logger:  logger,
		agents:  make(map[string]*AgentConn),
		clients: make(map[string]map[*ClientConn]struct{}),
	}
}

func (h *Hub) RegisterAgent(ctx context.Context, serverID string, conn *websocket.Conn) *AgentConn {
	agent := newAgentConn(h, serverID, conn)

	h.mu.Lock()
	if existing, ok := h.agents[serverID]; ok {
		existing.Close(websocket.StatusPolicyViolation, "replaced")
	}
	h.agents[serverID] = agent
	h.mu.Unlock()

	if _, err := h.db.Exec(ctx, "UPDATE servers SET connected_at = now() WHERE id = $1", serverID); err != nil {
		h.logger.Error("failed to update server connected_at", slog.String("server_id", serverID), slog.Any("err", err))
	}

	go agent.readLoop()
	return agent
}

func (h *Hub) AgentFor(serverID string) *AgentConn {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.agents[serverID]
}

func (h *Hub) RegisterClient(serverID string, conn *websocket.Conn) *ClientConn {
	client := &ClientConn{conn: conn}

	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[serverID]; !ok {
		h.clients[serverID] = make(map[*ClientConn]struct{})
	}
	h.clients[serverID][client] = struct{}{}
	return client
}

func (h *Hub) removeClient(serverID string, client *ClientConn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if clients, ok := h.clients[serverID]; ok {
		delete(clients, client)
		if len(clients) == 0 {
			delete(h.clients, serverID)
		}
	}
}

func (h *Hub) broadcast(serverID string, payload []byte) {
	h.mu.RLock()
	clientsMap := h.clients[serverID]
	clients := make([]*ClientConn, 0, len(clientsMap))
	for client := range clientsMap {
		clients = append(clients, client)
	}
	h.mu.RUnlock()

	for _, client := range clients {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := client.Send(ctx, payload); err != nil {
			cancel()
			h.logger.Warn("failed to send to client", slog.String("server_id", serverID), slog.Any("err", err))
			client.Close(websocket.StatusInternalError, "send error")
			h.removeClient(serverID, client)
			continue
		}
		cancel()
	}
}

func (h *Hub) agentClosed(serverID string) {
	h.mu.Lock()
	delete(h.agents, serverID)
	h.mu.Unlock()

	if _, err := h.db.Exec(context.Background(), "UPDATE servers SET connected_at = NULL WHERE id = $1", serverID); err != nil {
		h.logger.Error("failed to clear connected_at", slog.String("server_id", serverID), slog.Any("err", err))
	}
}

type AgentConn struct {
	hub      *Hub
	serverID string
	conn     *websocket.Conn
	writeMu  sync.Mutex
	pending  map[string]chan []byte
	pendMu   sync.Mutex
	closed   chan struct{}
}

func newAgentConn(hub *Hub, serverID string, conn *websocket.Conn) *AgentConn {
	return &AgentConn{
		hub:      hub,
		serverID: serverID,
		conn:     conn,
		pending:  make(map[string]chan []byte),
		closed:   make(chan struct{}),
	}
}

func (a *AgentConn) Close(status websocket.StatusCode, reason string) {
	a.writeMu.Lock()
	a.conn.Close(status, reason)
	a.writeMu.Unlock()
	select {
	case <-a.closed:
	default:
		close(a.closed)
		a.failPending()
	}
}

func (a *AgentConn) Closed() <-chan struct{} {
	return a.closed
}

func (a *AgentConn) Call(ctx context.Context, frame JSONRPC) ([]byte, error) {
	if frame.JSONRPC == "" {
		frame.JSONRPC = "2.0"
	}
	if frame.ID == nil {
		idVal := uuid.NewString()
		raw, err := json.Marshal(idVal)
		if err != nil {
			return nil, err
		}
		rawMsg := json.RawMessage(raw)
		frame.ID = &rawMsg
	}
	idKey := string(*frame.ID)

	respCh := make(chan []byte, 1)
	a.pendMu.Lock()
	a.pending[idKey] = respCh
	a.pendMu.Unlock()

	payload, err := json.Marshal(frame)
	if err != nil {
		if ch := a.removePending(idKey); ch != nil {
			close(ch)
		}
		return nil, err
	}

	if err := a.write(ctx, payload); err != nil {
		if ch := a.removePending(idKey); ch != nil {
			close(ch)
		}
		return nil, err
	}

	select {
	case <-ctx.Done():
		if ch := a.removePending(idKey); ch != nil {
			close(ch)
		}
		return nil, ctx.Err()
	case <-a.closed:
		if ch := a.removePending(idKey); ch != nil {
			close(ch)
		}
		return nil, errors.New("agent disconnected")
	case resp := <-respCh:
		if resp == nil {
			return nil, errors.New("agent disconnected")
		}
		return resp, nil
	}
}

func (a *AgentConn) write(ctx context.Context, data []byte) error {
	a.writeMu.Lock()
	defer a.writeMu.Unlock()
	return a.conn.Write(ctx, websocket.MessageText, data)
}

func (a *AgentConn) removePending(idKey string) chan []byte {
	a.pendMu.Lock()
	ch := a.pending[idKey]
	if ch != nil {
		delete(a.pending, idKey)
	}
	a.pendMu.Unlock()
	return ch
}

func (a *AgentConn) readLoop() {
	ctx := context.Background()
	for {
		_, data, err := a.conn.Read(ctx)
		if err != nil {
			a.hub.logger.Info("agent connection closing", slog.String("server_id", a.serverID), slog.Any("err", err))
			a.Close(websocket.StatusNormalClosure, "read error")
			a.hub.agentClosed(a.serverID)
			return
		}

		var env map[string]json.RawMessage
		if err := json.Unmarshal(data, &env); err != nil {
			a.hub.logger.Warn("invalid agent payload", slog.String("server_id", a.serverID), slog.Any("err", err))
			continue
		}

		if ctrl, ok := env["_control"]; ok {
			var controlType string
			if err := json.Unmarshal(ctrl, &controlType); err != nil {
				continue
			}
			a.handleControl(ctx, controlType, env)
			continue
		}

		if idRaw, ok := env["id"]; ok && len(idRaw) > 0 {
			idKey := string(idRaw)
			if ch := a.removePending(idKey); ch != nil {
				select {
				case ch <- data:
				default:
				}
				close(ch)
			}
			continue
		}

		if _, ok := env["method"]; ok {
			// Notification - fan out to clients
			a.hub.broadcast(a.serverID, data)
			continue
		}
	}
}

func (a *AgentConn) handleControl(ctx context.Context, controlType string, env map[string]json.RawMessage) {
	switch controlType {
	case "discover":
		schema, ok := env["schema"]
		if !ok {
			return
		}
		if _, err := a.hub.db.Exec(ctx, "UPDATE servers SET schema_json = $1 WHERE id = $2", schema, a.serverID); err != nil {
			a.hub.logger.Error("failed to persist schema", slog.String("server_id", a.serverID), slog.Any("err", err))
		}
	default:
		a.hub.logger.Info("unknown control message", slog.String("server_id", a.serverID), slog.String("type", controlType))
	}
}

func (a *AgentConn) failPending() {
	a.pendMu.Lock()
	for id, ch := range a.pending {
		delete(a.pending, id)
		select {
		case ch <- nil:
		default:
		}
		close(ch)
	}
	a.pendMu.Unlock()
}

type ClientConn struct {
	conn    *websocket.Conn
	writeMu sync.Mutex
}

func (c *ClientConn) Send(ctx context.Context, payload []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.conn.Write(ctx, websocket.MessageText, payload)
}

func (c *ClientConn) Close(status websocket.StatusCode, reason string) {
	c.writeMu.Lock()
	c.conn.Close(status, reason)
	c.writeMu.Unlock()
}
