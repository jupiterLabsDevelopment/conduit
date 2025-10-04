package app

import (
	"context"
	"encoding/json"
)

type contextKey string

const (
	contextKeyUser        contextKey = "user"
	contextKeySessionHash contextKey = "session-hash"
)

type Role string

const (
	RoleViewer    Role = "viewer"
	RoleModerator Role = "moderator"
	RoleOwner     Role = "owner"
)

var roleOrder = map[Role]int{
	RoleViewer:    1,
	RoleModerator: 2,
	RoleOwner:     3,
}

func (r Role) Meets(min Role) bool {
	return roleOrder[r] >= roleOrder[min]
}

type AuthUser struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Role  Role   `json:"role"`
}

type JSONRPC struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method,omitempty"`
	Params  json.RawMessage  `json:"params,omitempty"`
	Result  json.RawMessage  `json:"result,omitempty"`
	Error   json.RawMessage  `json:"error,omitempty"`
}

func userFromContext(ctx context.Context) *AuthUser {
	v := ctx.Value(contextKeyUser)
	if v == nil {
		return nil
	}
	if user, ok := v.(*AuthUser); ok {
		return user
	}
	return nil
}

func sessionHashFromContext(ctx context.Context) string {
	v := ctx.Value(contextKeySessionHash)
	if hash, ok := v.(string); ok {
		return hash
	}
	return ""
}
