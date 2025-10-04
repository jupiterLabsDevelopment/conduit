package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

var (
	errSessionRevoked = errors.New("session revoked")
	errSessionExpired = errors.New("session expired")
)

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (a *App) lookupSession(ctx context.Context, token string) (*AuthUser, string, error) {
	tokenHash := hashToken(token)

	var (
		userID    string
		email     string
		role      Role
		expiresAt time.Time
		revokedAt *time.Time
	)

	err := a.DB.QueryRow(ctx, `SELECT s.user_id, u.email, u.role, s.expires_at, s.revoked_at FROM sessions s JOIN users u ON u.id = s.user_id WHERE s.token_hash = $1`, tokenHash).Scan(&userID, &email, &role, &expiresAt, &revokedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, "", err
		}
		return nil, "", err
	}

	if revokedAt != nil {
		return nil, tokenHash, errSessionRevoked
	}

	now := time.Now()
	if now.After(expiresAt) {
		if _, execErr := a.DB.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash); execErr != nil {
			a.Logger.Warn("failed to purge expired session", slog.Any("err", execErr))
		}
		return nil, tokenHash, errSessionExpired
	}

	return &AuthUser{ID: userID, Email: email, Role: role}, tokenHash, nil
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	hash := sessionHashFromContext(r.Context())
	if hash == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if _, err := a.DB.Exec(r.Context(), `UPDATE sessions SET revoked_at = now() WHERE token_hash = $1 AND revoked_at IS NULL`, hash); err != nil {
		a.internalError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
