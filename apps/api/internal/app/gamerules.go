package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
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

type serverSettingRPC struct {
	Method string
	Param  string
	Coerce func(any) (any, error)
}

var serverSettingCommands = map[string]serverSettingRPC{
	"difficulty":                     {Method: "minecraft:serversettings/difficulty/set", Param: "difficulty", Coerce: coerceEnumValue("peaceful", "easy", "normal", "hard")},
	"allow_flight":                   {Method: "minecraft:serversettings/allow_flight/set", Param: "allow", Coerce: coerceBoolValue},
	"enforce_allowlist":              {Method: "minecraft:serversettings/enforce_allowlist/set", Param: "enforce", Coerce: coerceBoolValue},
	"use_allowlist":                  {Method: "minecraft:serversettings/use_allowlist/set", Param: "use", Coerce: coerceBoolValue},
	"max_players":                    {Method: "minecraft:serversettings/max_players/set", Param: "max", Coerce: coerceIntValue},
	"pause_when_empty_seconds":       {Method: "minecraft:serversettings/pause_when_empty_seconds/set", Param: "seconds", Coerce: coerceIntValue},
	"player_idle_timeout":            {Method: "minecraft:serversettings/player_idle_timeout/set", Param: "seconds", Coerce: coerceIntValue},
	"motd":                           {Method: "minecraft:serversettings/motd/set", Param: "message", Coerce: coerceStringValue},
	"spawn_protection_radius":        {Method: "minecraft:serversettings/spawn_protection_radius/set", Param: "radius", Coerce: coerceIntValue},
	"force_game_mode":                {Method: "minecraft:serversettings/force_game_mode/set", Param: "force", Coerce: coerceBoolValue},
	"game_mode":                      {Method: "minecraft:serversettings/game_mode/set", Param: "mode", Coerce: coerceEnumValue("survival", "creative", "adventure", "spectator")},
	"view_distance":                  {Method: "minecraft:serversettings/view_distance/set", Param: "distance", Coerce: coerceIntValue},
	"simulation_distance":            {Method: "minecraft:serversettings/simulation_distance/set", Param: "distance", Coerce: coerceIntValue},
	"accept_transfers":               {Method: "minecraft:serversettings/accept_transfers/set", Param: "accept", Coerce: coerceBoolValue},
	"status_heartbeat_interval":      {Method: "minecraft:serversettings/status_heartbeat_interval/set", Param: "seconds", Coerce: coerceIntValue},
	"operator_user_permission_level": {Method: "minecraft:serversettings/operator_user_permission_level/set", Param: "level", Coerce: coerceIntValue},
	"hide_online_players":            {Method: "minecraft:serversettings/hide_online_players/set", Param: "hide", Coerce: coerceBoolValue},
	"status_replies":                 {Method: "minecraft:serversettings/status_replies/set", Param: "enable", Coerce: coerceBoolValue},
	"entity_broadcast_range":         {Method: "minecraft:serversettings/entity_broadcast_range/set", Param: "percentage_points", Coerce: coerceIntValue},
	"autosave":                       {Method: "minecraft:serversettings/autosave/set", Param: "enable", Coerce: coerceBoolValue},
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
			"difficulty":        "peaceful",
			"allow_flight":      true,
			"enforce_allowlist": true,
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
			"difficulty":        "hard",
			"enforce_allowlist": true,
			"allow_flight":      false,
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
		res := a.applyMinecraftGameRule(ctx, agent, serverID, user, name, value)
		res.Type = "gamerule"
		res.Name = name
		res.Value = value
		results = append(results, res)
	}

	for name, value := range preset.Settings {
		res := a.applyMinecraftServerSetting(ctx, agent, serverID, user, name, value)
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

func (a *App) applyMinecraftGameRule(ctx context.Context, agent *AgentConn, serverID string, user *AuthUser, name string, value any) presetApplicationResult {
	params := map[string]any{
		"gamerule": map[string]any{
			"key":   name,
			"value": stringifyGameRuleValue(value),
		},
	}

	payload, err := json.Marshal(params)
	if err != nil {
		return presetApplicationResult{Status: "error", Message: fmt.Sprintf("marshal params: %v", err)}
	}

	frame := JSONRPC{Method: "minecraft:gamerules/update", Params: json.RawMessage(payload)}

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
	a.recordAudit(ctx, user.ID, serverID, frame.Method, json.RawMessage(payload), status, auditErr)

	if status == "ok" {
		return presetApplicationResult{Status: status}
	}
	return presetApplicationResult{Status: status, Message: message}
}

func (a *App) applyMinecraftServerSetting(ctx context.Context, agent *AgentConn, serverID string, user *AuthUser, name string, value any) presetApplicationResult {
	cmd, ok := serverSettingCommands[name]
	if !ok {
		return presetApplicationResult{Status: "error", Message: fmt.Sprintf("unsupported setting %q", name)}
	}

	coerced := value
	if cmd.Coerce != nil {
		var err error
		coerced, err = cmd.Coerce(value)
		if err != nil {
			return presetApplicationResult{Status: "error", Message: err.Error()}
		}
	}

	params := map[string]any{cmd.Param: coerced}
	payload, err := json.Marshal(params)
	if err != nil {
		return presetApplicationResult{Status: "error", Message: fmt.Sprintf("marshal params: %v", err)}
	}

	frame := JSONRPC{Method: cmd.Method, Params: json.RawMessage(payload)}

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
	a.recordAudit(ctx, user.ID, serverID, frame.Method, json.RawMessage(payload), status, auditErr)

	if status == "ok" {
		return presetApplicationResult{Status: status}
	}
	return presetApplicationResult{Status: status, Message: message}
}

func stringifyGameRuleValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case bool:
		if v {
			return "true"
		}
		return "false"
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprintf("%v", value)
	}
}

func coerceBoolValue(value any) (any, error) {
	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		trimmed := strings.TrimSpace(strings.ToLower(v))
		switch trimmed {
		case "true", "1", "yes", "on":
			return true, nil
		case "false", "0", "no", "off":
			return false, nil
		default:
			return nil, fmt.Errorf("invalid boolean value %q", v)
		}
	case int:
		return v != 0, nil
	case int32:
		return v != 0, nil
	case int64:
		return v != 0, nil
	case float64:
		return v != 0, nil
	default:
		return nil, fmt.Errorf("invalid boolean value type %T", value)
	}
}

func coerceIntValue(value any) (any, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case int32:
		return int(v), nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return nil, errors.New("empty integer value")
		}
		i, err := strconv.Atoi(trimmed)
		if err != nil {
			return nil, fmt.Errorf("invalid integer value %q", v)
		}
		return i, nil
	case json.Number:
		i, err := v.Int64()
		if err != nil {
			return nil, err
		}
		return int(i), nil
	default:
		return nil, fmt.Errorf("invalid integer value type %T", value)
	}
}

func coerceStringValue(value any) (any, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case fmt.Stringer:
		return v.String(), nil
	case []byte:
		return string(v), nil
	default:
		return fmt.Sprintf("%v", value), nil
	}
}

func coerceEnumValue(valid ...string) func(any) (any, error) {
	allowed := make(map[string]struct{}, len(valid))
	for _, v := range valid {
		allowed[strings.ToLower(v)] = struct{}{}
	}
	return func(value any) (any, error) {
		strAny, err := coerceStringValue(value)
		if err != nil {
			return nil, err
		}
		str := strings.ToLower(strings.TrimSpace(strAny.(string)))
		if str == "" {
			return nil, fmt.Errorf("invalid value %q", value)
		}
		if _, ok := allowed[str]; !ok {
			return nil, fmt.Errorf("invalid value %q; expected one of %v", value, valid)
		}
		return str, nil
	}
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
