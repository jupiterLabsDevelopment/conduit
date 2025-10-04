package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
	"nhooyr.io/websocket"
)

type App struct {
	DB        *pgxpool.Pool
	Hub       *Hub
	Logger    *slog.Logger
	jwtSecret []byte
	Router    http.Handler
}

type Config struct {
	JWTSecret string
}

func NewApp(db *pgxpool.Pool, cfg Config, logger *slog.Logger) *App {
	hub := NewHub(db, logger)
	app := &App{
		DB:        db,
		Hub:       hub,
		Logger:    logger,
		jwtSecret: []byte(cfg.JWTSecret),
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173", "http://127.0.0.1:5173"},
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Requested-With"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Post("/v1/users/bootstrap", app.handleBootstrap)
	r.Post("/v1/auth/login", app.handleLogin)

	r.Route("/v1", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(app.authMiddleware)
			r.Post("/auth/logout", app.handleLogout)
			r.Get("/servers", app.handleListServers)
			r.Post("/servers", app.requireRole(RoleOwner, app.handleCreateServer))
			r.Route("/servers/{id}", func(r chi.Router) {
				r.Get("/", app.handleGetServer)
				r.Get("/schema", app.handleServerSchema)
				r.Post("/rpc", app.handleServerRPC)
				r.Get("/audit", app.handleListAuditLogs)
				r.Get("/audit/export", app.handleExportAuditLogs)
				r.Post("/gamerules/apply-preset", app.requireRole(RoleModerator, app.handleApplyGameRulePreset))
			})
			r.Get("/game-rule-presets", app.requireRole(RoleViewer, app.handleListGameRulePresets))
			r.Get("/api-keys", app.requireRole(RoleOwner, app.handleListAPIKeys))
			r.Post("/api-keys", app.requireRole(RoleOwner, app.handleCreateAPIKey))
			r.Delete("/api-keys/{id}", app.requireRole(RoleOwner, app.handleDeleteAPIKey))
		})
	})

	r.Group(func(r chi.Router) {
		r.Use(app.authMiddleware)
		r.Get("/ws/servers/{id}/events", app.handleServerEvents)
	})

	r.Get("/agent/connect", app.handleAgentConnect)

	app.Router = r
	return app
}

type bootstrapRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authLoginResponse struct {
	Token string    `json:"token"`
	User  *AuthUser `json:"user"`
}

func (a *App) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	var req bootstrapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || req.Password == "" {
		http.Error(w, "email and password required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	var userCount int
	if err := a.DB.QueryRow(ctx, "SELECT COUNT(1) FROM users").Scan(&userCount); err != nil {
		a.internalError(w, err)
		return
	}
	if userCount > 0 {
		http.Error(w, "bootstrap already completed", http.StatusForbidden)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		a.internalError(w, err)
		return
	}

	id := uuid.NewString()
	if _, err := a.DB.Exec(ctx, `INSERT INTO users (id, email, password_hash, role) VALUES ($1, $2, $3, 'owner')`, id, req.Email, string(hash)); err != nil {
		a.internalError(w, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req bootstrapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || req.Password == "" {
		http.Error(w, "email and password required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	var (
		id     string
		stored string
		role   Role
	)
	if err := a.DB.QueryRow(ctx, `SELECT id, password_hash, role FROM users WHERE email=$1`, req.Email).Scan(&id, &stored, &role); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		a.internalError(w, err)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(stored), []byte(req.Password)); err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	expiresAt := time.Now().Add(24 * time.Hour).UTC()
	claims := jwt.MapClaims{
		"sub":   id,
		"email": req.Email,
		"role":  string(role),
		"exp":   expiresAt.Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(a.jwtSecret)
	if err != nil {
		a.internalError(w, err)
		return
	}

	tokenHash := hashToken(signed)
	if _, err := a.DB.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1 AND expires_at < now()`, id); err != nil {
		a.Logger.Warn("failed to prune expired sessions", slog.Any("err", err))
	}
	if _, err := a.DB.Exec(ctx, `INSERT INTO sessions (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`, id, tokenHash, expiresAt); err != nil {
		a.internalError(w, err)
		return
	}

	a.writeJSON(w, authLoginResponse{
		Token: signed,
		User:  &AuthUser{ID: id, Email: req.Email, Role: role},
	})
}

type serverRow struct {
	ID          string
	Name        string
	Description *string
	ConnectedAt *time.Time
	CreatedAt   time.Time
}

type serverListItem struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description *string    `json:"description,omitempty"`
	Connected   bool       `json:"connected"`
	ConnectedAt *time.Time `json:"connected_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

func (a *App) handleListServers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := a.DB.Query(ctx, `SELECT id, name, description, connected_at, created_at FROM servers ORDER BY created_at DESC`)
	if err != nil {
		a.internalError(w, err)
		return
	}
	defer rows.Close()

	var list []serverListItem
	for rows.Next() {
		var row serverRow
		if err := rows.Scan(&row.ID, &row.Name, &row.Description, &row.ConnectedAt, &row.CreatedAt); err != nil {
			a.internalError(w, err)
			return
		}
		item := serverListItem{
			ID:          row.ID,
			Name:        row.Name,
			Description: row.Description,
			Connected:   row.ConnectedAt != nil,
			ConnectedAt: row.ConnectedAt,
			CreatedAt:   row.CreatedAt,
		}
		list = append(list, item)
	}

	a.writeJSON(w, list)
}

type createServerRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

type createServerResponse struct {
	ID          string    `json:"id"`
	AgentToken  string    `json:"agent_token"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

func (a *App) handleCreateServer(w http.ResponseWriter, r *http.Request) {
	var req createServerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}

	agentToken, err := generateAgentToken()
	if err != nil {
		a.internalError(w, err)
		return
	}

	id := uuid.NewString()
	now := time.Now()
	if _, err := a.DB.Exec(r.Context(), `INSERT INTO servers (id, name, description, agent_token, created_at) VALUES ($1, $2, $3, $4, $5)`, id, req.Name, req.Description, agentToken, now); err != nil {
		a.internalError(w, err)
		return
	}

	a.writeJSONStatus(w, http.StatusCreated, createServerResponse{
		ID:          id,
		AgentToken:  agentToken,
		Name:        req.Name,
		Description: req.Description,
		CreatedAt:   now,
	})
}

func (a *App) handleGetServer(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	var row serverRow
	if err := a.DB.QueryRow(r.Context(), `SELECT id, name, description, connected_at, created_at FROM servers WHERE id=$1`, serverID).Scan(&row.ID, &row.Name, &row.Description, &row.ConnectedAt, &row.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		a.internalError(w, err)
		return
	}

	a.writeJSON(w, serverListItem{
		ID:          row.ID,
		Name:        row.Name,
		Description: row.Description,
		Connected:   row.ConnectedAt != nil,
		ConnectedAt: row.ConnectedAt,
		CreatedAt:   row.CreatedAt,
	})
}

func (a *App) handleServerSchema(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	var schema json.RawMessage
	if err := a.DB.QueryRow(r.Context(), `SELECT schema_json FROM servers WHERE id=$1`, serverID).Scan(&schema); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		a.internalError(w, err)
		return
	}
	if schema == nil {
		schema = json.RawMessage("null")
	}
	a.writeJSONRaw(w, schema)
}

func (a *App) handleServerRPC(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	user := userFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req JSONRPC
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	minRole := roleForMethod(req.Method)
	if !user.Role.Meets(minRole) {
		http.Error(w, "forbidden", http.StatusForbidden)
		a.recordAudit(r.Context(), user.ID, serverID, req.Method, req.Params, "error", errors.New("rbac denied"))
		return
	}

	agent := a.Hub.AgentFor(serverID)
	if agent == nil {
		http.Error(w, "agent not connected", http.StatusServiceUnavailable)
		a.recordAudit(r.Context(), user.ID, serverID, req.Method, req.Params, "error", errors.New("agent disconnected"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	resp, err := agent.Call(ctx, req)
	status := "ok"
	if err != nil {
		status = "error"
		http.Error(w, err.Error(), http.StatusBadGateway)
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.Write(resp)
	}

	a.recordAudit(r.Context(), user.ID, serverID, req.Method, req.Params, status, err)
}

func (a *App) handleServerEvents(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	user := userFromContext(r.Context())
	if user == nil || !user.Role.Meets(RoleViewer) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		CompressionMode: websocket.CompressionContextTakeover,
		Subprotocols:    []string{"jwt"},
	})
	if err != nil {
		a.Logger.Error("ws accept failed", slog.Any("err", err))
		return
	}
	defer conn.Close(websocket.StatusInternalError, "closed")

	client := a.Hub.RegisterClient(serverID, conn)
	defer a.Hub.removeClient(serverID, client)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	for {
		if _, _, err := conn.Read(ctx); err != nil {
			return
		}
	}
}

func (a *App) handleAgentConnect(w http.ResponseWriter, r *http.Request) {
	token := extractBearerToken(r.Header.Get("Authorization"))
	if token == "" {
		http.Error(w, "authorization required", http.StatusUnauthorized)
		return
	}

	var serverID string
	if err := a.DB.QueryRow(r.Context(), `SELECT id FROM servers WHERE agent_token=$1`, token).Scan(&serverID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		a.internalError(w, err)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		CompressionMode: websocket.CompressionDisabled,
	})
	if err != nil {
		a.Logger.Error("agent ws accept failed", slog.Any("err", err))
		return
	}

	agent := a.Hub.RegisterAgent(r.Context(), serverID, conn)

	select {
	case <-agent.Closed():
		return
	case <-r.Context().Done():
		agent.Close(websocket.StatusNormalClosure, "context canceled")
		return
	}
}

func (a *App) recordAudit(ctx context.Context, userID, serverID, action string, params json.RawMessage, status string, rpcErr error) {
	hash := sha256.Sum256(params)
	paramsHash := hex.EncodeToString(hash[:])

	var errMsg *string
	if rpcErr != nil {
		s := rpcErr.Error()
		errMsg = &s
	}

	_, err := a.DB.Exec(ctx, `INSERT INTO audit_logs (user_id, server_id, action, params_sha256, result_status, error_message) VALUES ($1, $2, $3, $4, $5, $6)`, userID, serverID, action, paramsHash, status, errMsg)
	if err != nil {
		a.Logger.Error("failed to write audit log", slog.Any("err", err))
	}
}

func (a *App) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractTokenFromRequest(r)
		if token == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		claims := jwt.MapClaims{}
		parsed, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("invalid signing method")
			}
			return a.jwtSecret, nil
		})
		if err != nil || !parsed.Valid {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		sub, _ := claims["sub"].(string)

		user, sessionHash, err := a.lookupSession(r.Context(), token)
		if err != nil {
			switch {
			case errors.Is(err, pgx.ErrNoRows), errors.Is(err, errSessionRevoked), errors.Is(err, errSessionExpired):
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			default:
				a.internalError(w, err)
				return
			}
		}

		if sub != "" && sub != user.ID {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), contextKeyUser, user)
		ctx = context.WithValue(ctx, contextKeySessionHash, sessionHash)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *App) requireRole(min Role, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := userFromContext(r.Context())
		if user == nil || !user.Role.Meets(min) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		handler(w, r)
	}
}

func (a *App) internalError(w http.ResponseWriter, err error) {
	if err != nil {
		a.Logger.Error("internal error", slog.Any("err", err))
	}
	http.Error(w, "internal server error", http.StatusInternalServerError)
}

func (a *App) writeJSON(w http.ResponseWriter, payload any) {
	a.writeJSONStatus(w, http.StatusOK, payload)
}

func (a *App) writeJSONStatus(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		a.Logger.Error("failed to encode json", slog.Any("err", err))
	}
}

func (a *App) writeJSONRaw(w http.ResponseWriter, payload json.RawMessage) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(payload)
}

func extractTokenFromRequest(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	token := extractBearerToken(authHeader)
	if token != "" {
		return token
	}

	for _, proto := range r.Header.Values("Sec-Websocket-Protocol") {
		parts := strings.Split(proto, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		if len(parts) >= 2 && strings.EqualFold(parts[0], "jwt") {
			return parts[1]
		}
	}
	return ""
}

func extractBearerToken(header string) string {
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 {
		return ""
	}
	if !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func generateAgentToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
