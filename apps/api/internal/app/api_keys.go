package app

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type apiKey struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type apiKeyWithSecret struct {
	apiKey
	Secret string `json:"secret"`
}

type createAPIKeyRequest struct {
	Name string `json:"name"`
}

func (a *App) handleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	if user == nil || !user.Role.Meets(RoleOwner) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	rows, err := a.DB.Query(r.Context(), `SELECT id, name, created_at FROM api_keys WHERE user_id = $1 ORDER BY created_at DESC`, user.ID)
	if err != nil {
		a.internalError(w, err)
		return
	}
	defer rows.Close()

	var keys []apiKey
	for rows.Next() {
		var item apiKey
		if err := rows.Scan(&item.ID, &item.Name, &item.CreatedAt); err != nil {
			a.internalError(w, err)
			return
		}
		keys = append(keys, item)
	}

	a.writeJSON(w, keys)
}

func (a *App) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	if user == nil || !user.Role.Meets(RoleOwner) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var req createAPIKeyRequest
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}

	secretPlain, secretHash, err := generateAPIKeySecret()
	if err != nil {
		a.internalError(w, err)
		return
	}

	id := uuid.NewString()
	now := time.Now()
	if _, err := a.DB.Exec(r.Context(), `INSERT INTO api_keys (id, user_id, name, secret, created_at) VALUES ($1, $2, $3, $4, $5)`, id, user.ID, name, secretHash, now); err != nil {
		a.internalError(w, err)
		return
	}

	a.writeJSONStatus(w, http.StatusCreated, apiKeyWithSecret{
		apiKey: apiKey{
			ID:        id,
			Name:      name,
			CreatedAt: now,
		},
		Secret: secretPlain,
	})
}

func (a *App) handleDeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	if user == nil || !user.Role.Meets(RoleOwner) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	keyID := chi.URLParam(r, "id")
	if keyID == "" {
		http.Error(w, "invalid key", http.StatusBadRequest)
		return
	}

	tag, err := a.DB.Exec(r.Context(), `DELETE FROM api_keys WHERE id = $1 AND user_id = $2`, keyID, user.ID)
	if err != nil {
		a.internalError(w, err)
		return
	}

	if tag.RowsAffected() == 0 {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func generateAPIKeySecret() (string, string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}

	plain := base64.RawURLEncoding.EncodeToString(buf)
	hash := sha256.Sum256([]byte(plain))
	return plain, hex.EncodeToString(hash[:]), nil
}
