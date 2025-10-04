package main

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"nhooyr.io/websocket"
)

type Config struct {
	APIURL            string
	AgentToken        string
	MCURL             string
	MCToken           string
	MCInsecure        bool
	MCTLSServerName   string
	MCTLSRootCAs      *x509.CertPool
	MCDialTimeout     time.Duration
	BackoffInitial    time.Duration
	BackoffMax        time.Duration
	BackoffMultiplier float64
	BackoffJitter     time.Duration
	TelemetryInterval time.Duration
}

type JSONRPC struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method,omitempty"`
	Params  json.RawMessage  `json:"params,omitempty"`
	Result  json.RawMessage  `json:"result,omitempty"`
	Error   json.RawMessage  `json:"error,omitempty"`
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := loadConfig()
	if err != nil {
		logger.Error("invalid configuration", slog.Any("err", err))
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	metrics := newTelemetry(logger, cfg.TelemetryInterval)
	defer metrics.stop()

	backoff := cfg.BackoffInitial
	if backoff <= 0 {
		backoff = time.Second
	}
	attempt := 1
	for {
		if ctx.Err() != nil {
			return
		}

		metrics.recordSessionStart()
		started := time.Now()
		err := runOnce(ctx, cfg, logger, metrics)
		duration := time.Since(started)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, context.Canceled) {
				metrics.recordSessionFailure(duration, err)
				return
			}
			metrics.recordSessionFailure(duration, err)
			wait := applyJitter(backoff, cfg.BackoffJitter)
			logger.Warn("agent session ended; scheduling reconnect", slog.Int("attempt", attempt), slog.Duration("backoff", wait), slog.Any("err", err))
			attempt++
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return
			}
			backoff = nextBackoff(backoff, cfg.BackoffMultiplier, cfg.BackoffMax)
			continue
		}

		metrics.recordSessionSuccess(duration)
		backoff = cfg.BackoffInitial
		if backoff <= 0 {
			backoff = time.Second
		}
		attempt = 1
	}
}

func loadConfig() (Config, error) {
	insecureRaw := strings.TrimSpace(strings.ToLower(os.Getenv("MC_TLS_INSECURE")))
	modeRaw := strings.TrimSpace(strings.ToLower(os.Getenv("MC_TLS_MODE")))
	initialBackoff, err := durationFromEnv("AGENT_BACKOFF_INITIAL", time.Second)
	if err != nil {
		return Config{}, err
	}
	maxBackoff, err := durationFromEnv("AGENT_BACKOFF_MAX", 30*time.Second)
	if err != nil {
		return Config{}, err
	}
	multiplier, err := floatFromEnv("AGENT_BACKOFF_MULTIPLIER", 2.0)
	if err != nil {
		return Config{}, err
	}
	jitter, err := durationFromEnv("AGENT_BACKOFF_JITTER", 500*time.Millisecond)
	if err != nil {
		return Config{}, err
	}
	telemetryInterval, err := durationFromEnv("AGENT_TELEMETRY_INTERVAL", time.Minute)
	if err != nil {
		return Config{}, err
	}
	dialTimeout, err := durationFromEnv("MC_TLS_HANDSHAKE_TIMEOUT", 15*time.Second)
	if err != nil {
		return Config{}, err
	}

	caPath := strings.TrimSpace(os.Getenv("MC_TLS_ROOT_CA"))
	var caPool *x509.CertPool
	if caPath != "" {
		pemBytes, err := os.ReadFile(caPath)
		if err != nil {
			return Config{}, fmt.Errorf("failed to read MC_TLS_ROOT_CA %q: %w", caPath, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pemBytes) {
			return Config{}, fmt.Errorf("invalid PEM data in MC_TLS_ROOT_CA %q", caPath)
		}
		caPool = pool
	}

	serverName := strings.TrimSpace(os.Getenv("MC_TLS_SERVER_NAME"))
	mcInsecure := insecureRaw == "true" || insecureRaw == "1" || insecureRaw == "yes"
	if modeRaw != "" {
		switch modeRaw {
		case "skip", "insecure", "disabled", "off":
			mcInsecure = true
		case "strict", "verify", "on", "default":
			mcInsecure = false
		default:
			return Config{}, fmt.Errorf("invalid MC_TLS_MODE %q", modeRaw)
		}
	}

	cfg := Config{
		APIURL:            strings.TrimSpace(os.Getenv("CONDUIT_API_WS")),
		AgentToken:        strings.TrimSpace(os.Getenv("CONDUIT_AGENT_TOKEN")),
		MCURL:             strings.TrimSpace(os.Getenv("MC_MGMT_WS")),
		MCToken:           strings.TrimSpace(os.Getenv("MC_MGMT_TOKEN")),
		MCInsecure:        mcInsecure,
		MCTLSServerName:   serverName,
		MCTLSRootCAs:      caPool,
		MCDialTimeout:     dialTimeout,
		BackoffInitial:    initialBackoff,
		BackoffMax:        maxBackoff,
		BackoffMultiplier: multiplier,
		BackoffJitter:     jitter,
		TelemetryInterval: telemetryInterval,
	}

	if cfg.APIURL == "" || cfg.AgentToken == "" || cfg.MCURL == "" || cfg.MCToken == "" {
		return Config{}, errors.New("missing required environment variables")
	}
	if cfg.BackoffInitial <= 0 {
		cfg.BackoffInitial = time.Second
	}
	if cfg.BackoffMax < cfg.BackoffInitial {
		cfg.BackoffMax = cfg.BackoffInitial
	}
	if cfg.BackoffMultiplier < 1.1 {
		cfg.BackoffMultiplier = 2.0
	}
	if cfg.BackoffJitter < 0 {
		cfg.BackoffJitter = 0
	}

	return cfg, nil
}

func (cfg Config) buildMCTLSConfig() *tls.Config {
	if !strings.HasPrefix(strings.ToLower(cfg.MCURL), "wss://") {
		return nil
	}
	tlsCfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}
	if cfg.MCTLSServerName != "" {
		tlsCfg.ServerName = cfg.MCTLSServerName
	}
	if cfg.MCTLSRootCAs != nil {
		tlsCfg.RootCAs = cfg.MCTLSRootCAs
	}
	if cfg.MCInsecure {
		tlsCfg.InsecureSkipVerify = true
	}
	return tlsCfg
}

func runOnce(ctx context.Context, cfg Config, logger *slog.Logger, metrics *telemetry) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	apiHeader := http.Header{}
	apiHeader.Set("Authorization", "Bearer "+cfg.AgentToken)
	apiDialStart := time.Now()
	apiConn, _, err := websocket.Dial(ctx, cfg.APIURL, &websocket.DialOptions{HTTPHeader: apiHeader})
	if err != nil {
		metrics.recordDialFailure("api", err)
		return err
	}
	metrics.recordDialSuccess("api", time.Since(apiDialStart))

	mcHeader := http.Header{}
	mcHeader.Set("Authorization", "Bearer "+cfg.MCToken)
	mcDialOpts := &websocket.DialOptions{HTTPHeader: mcHeader}
	if strings.HasPrefix(strings.ToLower(cfg.MCURL), "wss://") {
		transport := &http.Transport{
			Proxy: http.ProxyFromEnvironment,
		}
		tlsCfg := cfg.buildMCTLSConfig()
		if tlsCfg != nil {
			transport.TLSClientConfig = tlsCfg
			if tlsCfg.InsecureSkipVerify {
				logger.Warn("minecraft TLS verification disabled", slog.String("mc_url", cfg.MCURL))
			}
		}
		timeout := cfg.MCDialTimeout
		if timeout <= 0 {
			timeout = 15 * time.Second
		}
		mcDialOpts.HTTPClient = &http.Client{Transport: transport, Timeout: timeout}
	}

	mcDialStart := time.Now()
	mcConn, _, err := websocket.Dial(ctx, cfg.MCURL, mcDialOpts)
	if err != nil {
		metrics.recordDialFailure("minecraft", err)
		apiConn.Close(websocket.StatusInternalError, "mc dial failed")
		return err
	}
	metrics.recordDialSuccess("minecraft", time.Since(mcDialStart))

	session := newSession(cfg, logger, metrics, apiConn, mcConn)
	return session.run(ctx)
}

type session struct {
	cfg     Config
	logger  *slog.Logger
	metrics *telemetry
	apiConn *websocket.Conn
	mcConn  *websocket.Conn
	pendMu  sync.Mutex
	pending map[string]chan []byte
}

func newSession(cfg Config, logger *slog.Logger, metrics *telemetry, apiConn, mcConn *websocket.Conn) *session {
	return &session{
		cfg:     cfg,
		logger:  logger,
		metrics: metrics,
		apiConn: apiConn,
		mcConn:  mcConn,
		pending: make(map[string]chan []byte),
	}
}

func (s *session) run(ctx context.Context) error {
	s.logger.Info("bridge established", slog.String("api", s.cfg.APIURL), slog.String("minecraft", s.cfg.MCURL))
	s.metrics.recordBridgeEstablished()

	go s.discoverLoop(ctx)

	errCh := make(chan error, 2)
	go func() { errCh <- s.pipeAPIToMC(ctx) }()
	go func() { errCh <- s.pipeMCToAPI(ctx) }()

	select {
	case <-ctx.Done():
		s.close()
		return ctx.Err()
	case err := <-errCh:
		s.close()
		return err
	}
}

func (s *session) close() {
	s.pendMu.Lock()
	for id, ch := range s.pending {
		close(ch)
		delete(s.pending, id)
	}
	s.pendMu.Unlock()

	s.apiConn.Close(websocket.StatusNormalClosure, "session closed")
	s.mcConn.Close(websocket.StatusNormalClosure, "session closed")
}

func (s *session) discoverLoop(ctx context.Context) {
	backoff := 5 * time.Second
	attempt := 0

	for {
		attempt++

		select {
		case <-ctx.Done():
			return
		default:
		}

		err := s.sendDiscover(ctx)
		if err == nil {
			if attempt > 1 {
				s.logger.Info("rpc.discover succeeded", slog.Int("attempt", attempt))
			}
			s.metrics.recordDiscover(true, nil)
			return
		}

		if errors.Is(err, context.Canceled) || websocket.CloseStatus(err) != -1 {
			return
		}

		s.logger.Warn("rpc.discover attempt failed", slog.Int("attempt", attempt), slog.Any("err", err))
		s.metrics.recordDiscover(false, err)

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

		if backoff < time.Minute {
			backoff *= 2
			if backoff > time.Minute {
				backoff = time.Minute
			}
		}
	}
}

func (s *session) removePending(idKey string) chan []byte {
	s.pendMu.Lock()
	defer s.pendMu.Unlock()
	ch := s.pending[idKey]
	if ch != nil {
		delete(s.pending, idKey)
	}
	return ch
}

func (s *session) registerPending(idKey string) chan []byte {
	s.pendMu.Lock()
	defer s.pendMu.Unlock()
	ch := make(chan []byte, 1)
	s.pending[idKey] = ch
	return ch
}

func (s *session) pipeAPIToMC(ctx context.Context) error {
	for {
		_, data, err := s.apiConn.Read(ctx)
		if err != nil {
			return err
		}
		if err := s.mcConn.Write(ctx, websocket.MessageText, data); err != nil {
			return err
		}
		s.metrics.recordForwardAPIToMC()
	}
}

func (s *session) pipeMCToAPI(ctx context.Context) error {
	for {
		_, data, err := s.mcConn.Read(ctx)
		if err != nil {
			return err
		}
		handled, err := s.handleMCMessage(ctx, data)
		if err != nil {
			return err
		}
		if handled {
			continue
		}
		if err := s.apiConn.Write(ctx, websocket.MessageText, data); err != nil {
			return err
		}
		s.metrics.recordForwardMCToAPI()
	}
}

func (s *session) handleMCMessage(ctx context.Context, data []byte) (bool, error) {
	var frame map[string]json.RawMessage
	if err := json.Unmarshal(data, &frame); err != nil {
		s.logger.Warn("invalid minecraft payload", slog.Any("err", err))
		return false, nil
	}

	if idRaw, ok := frame["id"]; ok && len(idRaw) > 0 {
		idKey := string(idRaw)
		if ch := s.removePending(idKey); ch != nil {
			ch <- data
			close(ch)
			return true, nil
		}
	}

	return false, nil
}

func (s *session) sendDiscover(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	result, err := s.callMinecraft(ctx, "rpc.discover", json.RawMessage("[]"))
	if err != nil {
		return err
	}

	control := map[string]json.RawMessage{
		"_control": json.RawMessage(`"discover"`),
		"schema":   result,
	}
	payload, err := json.Marshal(control)
	if err != nil {
		return err
	}

	return s.apiConn.Write(ctx, websocket.MessageText, payload)
}

func (s *session) callMinecraft(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
	if params == nil {
		params = json.RawMessage("[]")
	}
	id := "agent:" + uuid.NewString()
	idRaw, err := json.Marshal(id)
	if err != nil {
		return nil, err
	}
	idKey := string(idRaw)

	req := JSONRPC{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	idMsg := json.RawMessage(idRaw)
	req.ID = &idMsg

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	respCh := s.registerPending(idKey)
	if err := s.mcConn.Write(ctx, websocket.MessageText, payload); err != nil {
		s.removePending(idKey)
		return nil, err
	}

	select {
	case <-ctx.Done():
		s.removePending(idKey)
		return nil, ctx.Err()
	case data := <-respCh:
		if data == nil {
			return nil, errors.New("minecraft call canceled")
		}
		var resp struct {
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, err
		}
		if resp.Error != nil {
			return nil, errors.New(resp.Error.Message)
		}
		return resp.Result, nil
	}
}

type telemetry struct {
	logger              *slog.Logger
	interval            time.Duration
	mu                  sync.Mutex
	sessions            uint64
	failures            uint64
	bridges             uint64
	lastError           string
	lastSessionDuration time.Duration
	dialSuccess         map[string]uint64
	dialFailures        map[string]uint64
	dialLatency         map[string]time.Duration
	discoverSuccess     uint64
	discoverFailures    uint64
	apiToMCTotal        uint64
	mcToAPITotal        uint64
	stopCh              chan struct{}
	doneCh              chan struct{}
}

func newTelemetry(logger *slog.Logger, interval time.Duration) *telemetry {
	if interval <= 0 {
		interval = time.Minute
	}
	t := &telemetry{
		logger:       logger.With(slog.String("component", "telemetry")),
		interval:     interval,
		dialSuccess:  make(map[string]uint64),
		dialFailures: make(map[string]uint64),
		dialLatency:  make(map[string]time.Duration),
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
	}
	go t.loop()
	return t
}

func (t *telemetry) loop() {
	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()
	defer close(t.doneCh)
	for {
		select {
		case <-ticker.C:
			t.snapshot()
		case <-t.stopCh:
			t.snapshot()
			return
		}
	}
}

func (t *telemetry) stop() {
	if t == nil {
		return
	}
	close(t.stopCh)
	<-t.doneCh
}

func (t *telemetry) snapshot() {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	successCopy := make(map[string]uint64, len(t.dialSuccess))
	for k, v := range t.dialSuccess {
		successCopy[k] = v
	}
	failureCopy := make(map[string]uint64, len(t.dialFailures))
	for k, v := range t.dialFailures {
		failureCopy[k] = v
	}
	latencyCopy := make(map[string]time.Duration, len(t.dialLatency))
	for k, v := range t.dialLatency {
		latencyCopy[k] = v
	}

	attrs := []any{
		slog.Uint64("sessions_total", t.sessions),
		slog.Uint64("session_failures_total", t.failures),
		slog.Uint64("bridges_established_total", t.bridges),
		slog.Duration("last_session_duration", t.lastSessionDuration),
		slog.Uint64("discover_success_total", t.discoverSuccess),
		slog.Uint64("discover_failures_total", t.discoverFailures),
		slog.Uint64("messages_forwarded_api_to_mc", t.apiToMCTotal),
		slog.Uint64("messages_forwarded_mc_to_api", t.mcToAPITotal),
		slog.Any("dial_success_total", successCopy),
		slog.Any("dial_failures_total", failureCopy),
		slog.Any("dial_last_latency", latencyCopy),
	}
	if t.lastError != "" {
		attrs = append(attrs, slog.String("last_error", t.lastError))
	}
	t.logger.Info("agent telemetry snapshot", attrs...)
}

func (t *telemetry) recordSessionStart() {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.sessions++
	t.mu.Unlock()
}

func (t *telemetry) recordSessionSuccess(duration time.Duration) {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.lastSessionDuration = duration
	t.lastError = ""
	t.mu.Unlock()
}

func (t *telemetry) recordSessionFailure(duration time.Duration, err error) {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.failures++
	t.lastSessionDuration = duration
	if err != nil {
		t.lastError = err.Error()
	}
	t.mu.Unlock()
}

func (t *telemetry) recordDialSuccess(target string, latency time.Duration) {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.dialSuccess[target]++
	t.dialLatency[target] = latency
	t.mu.Unlock()
}

func (t *telemetry) recordDialFailure(target string, err error) {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.dialFailures[target]++
	if err != nil {
		t.lastError = err.Error()
	}
	t.mu.Unlock()
}

func (t *telemetry) recordDiscover(success bool, err error) {
	if t == nil {
		return
	}
	t.mu.Lock()
	if success {
		t.discoverSuccess++
	} else {
		t.discoverFailures++
		if err != nil {
			t.lastError = err.Error()
		}
	}
	t.mu.Unlock()
}

func (t *telemetry) recordBridgeEstablished() {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.bridges++
	t.mu.Unlock()
}

func (t *telemetry) recordForwardAPIToMC() {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.apiToMCTotal++
	t.mu.Unlock()
}

func (t *telemetry) recordForwardMCToAPI() {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.mcToAPITotal++
	t.mu.Unlock()
}

func durationFromEnv(key string, def time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid duration for %s: %w", key, err)
	}
	return d, nil
}

func floatFromEnv(key string, def float64) (float64, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def, nil
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid float for %s: %w", key, err)
	}
	return v, nil
}

func applyJitter(base, jitter time.Duration) time.Duration {
	if jitter <= 0 {
		return base
	}
	max := big.NewInt(int64(jitter))
	if max.Sign() <= 0 {
		return base
	}
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return base
	}
	return base + time.Duration(n.Int64())
}

func nextBackoff(current time.Duration, multiplier float64, max time.Duration) time.Duration {
	if current <= 0 {
		current = time.Second
	}
	if multiplier < 1.1 {
		multiplier = 2.0
	}
	next := time.Duration(float64(current) * multiplier)
	if max > 0 && next > max {
		next = max
	}
	if next <= 0 {
		next = current
	}
	return next
}
