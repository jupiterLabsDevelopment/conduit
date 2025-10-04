package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

type GameRulePreset struct {
	Key         string         `json:"key"`
	Label       string         `json:"label"`
	Description string         `json:"description"`
	GameRules   map[string]any `json:"game_rules,omitempty"`
	Settings    map[string]any `json:"settings,omitempty"`
}

type presetApplicationResult struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Value   any    `json:"value"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type applyPresetRequest struct {
	Preset string `json:"preset"`
}

type applyPresetResponse struct {
	Preset   GameRulePreset            `json:"preset"`
	Results  []presetApplicationResult `json:"results"`
	Duration int64                     `json:"duration_ms"`
}

var defaultPresets = []GameRulePreset{
	{
		Key:         "builder-friendly",
		Label:       "Builder Friendly",
		Description: "Keeps inventory on death, disables mob griefing, and pauses day/weather cycles for creative building sessions.",
		GameRules: map[string]any{
			"keepInventory":   true,
			"doDaylightCycle": false,
			"doWeatherCycle":  false,
			"mobGriefing":     false,
			"doFireTick":      false,
		},
		Settings: map[string]any{
			"difficulty": "peaceful",
			"pvp":        false,
		},
	},
	{
		Key:         "survival-challenge",
		Label:       "Survival Challenge",
		Description: "Tightens survival difficulty by disabling natural regeneration and enabling hostile mechanics.",
		GameRules: map[string]any{
			"keepInventory":       false,
			"naturalRegeneration": false,
			"doImmediateRespawn":  false,
			"fallDamage":          true,
		},
		Settings: map[string]any{
			"difficulty": "hard",
			"pvp":        true,
		},
	},
}

func (a *App) handleListGameRulePresets(w http.ResponseWriter, r *http.Request) {
	a.writeJSON(w, defaultPresets)
}

func (a *App) handleApplyGameRulePreset(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	user := userFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req applyPresetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	key := strings.TrimSpace(strings.ToLower(req.Preset))
	if key == "" {
		http.Error(w, "preset required", http.StatusBadRequest)
		return
	}

	preset, err := findPreset(key)
	if err != nil {
		http.Error(w, "preset not found", http.StatusNotFound)
		return
	}

	agent := a.Hub.AgentFor(serverID)
	if agent == nil {
		http.Error(w, "agent not connected", http.StatusServiceUnavailable)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	results := make([]presetApplicationResult, 0, len(preset.GameRules)+len(preset.Settings))
	start := time.Now()

	for name, value := range preset.GameRules {
		res := a.applyMinecraftSetting(ctx, agent, serverID, user, "minecraft:gamerule/set", []any{name, value})
		res.Type = "gamerule"
		res.Name = name
		res.Value = value
		results = append(results, res)
	}

	for name, value := range preset.Settings {
		res := a.applyMinecraftSetting(ctx, agent, serverID, user, "minecraft:settings/set", []any{name, value})
		res.Type = "setting"
		res.Name = name
		res.Value = value
		results = append(results, res)
	}

	response := applyPresetResponse{
		Preset:   *preset,
		Results:  results,
		Duration: time.Since(start).Milliseconds(),
	}

	a.writeJSON(w, response)
}

func (a *App) applyMinecraftSetting(ctx context.Context, agent *AgentConn, serverID string, user *AuthUser, method string, params []any) presetApplicationResult {
	payload, err := json.Marshal(params)
	if err != nil {
		return presetApplicationResult{Status: "error", Message: fmt.Sprintf("marshal params: %v", err)}
	}

	frame := JSONRPC{Method: method, Params: json.RawMessage(payload)}

	resp, callErr := agent.Call(ctx, frame)
	status := "ok"
	message := ""
	if callErr != nil {
		status = "error"
		message = callErr.Error()
	} else if err := decodeJSONRPCError(resp); err != nil {
		status = "error"
		message = err.Error()
	}

	var auditErr error
	if status != "ok" && message != "" {
		auditErr = errors.New(message)
	}
	a.recordAudit(ctx, user.ID, serverID, method, json.RawMessage(payload), status, auditErr)

	if status == "ok" {
		return presetApplicationResult{Status: status}
	}
	return presetApplicationResult{Status: status, Message: message}
}

func decodeJSONRPCError(data []byte) error {
	var env struct {
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if env.Error != nil {
		if env.Error.Message != "" {
			return errors.New(env.Error.Message)
		}
		return fmt.Errorf("rpc error %d", env.Error.Code)
	}
	return nil
}

func findPreset(key string) (*GameRulePreset, error) {
	for _, preset := range defaultPresets {
		if strings.EqualFold(preset.Key, key) {
			copy := preset
			return &copy, nil
		}
	}
	return nil, fmt.Errorf("preset %q not found", key)
}
