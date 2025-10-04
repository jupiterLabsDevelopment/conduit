package app

import (
	"encoding/csv"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

type auditLogItem struct {
	ID         int64     `json:"id"`
	Timestamp  time.Time `json:"timestamp"`
	UserID     *string   `json:"user_id,omitempty"`
	UserEmail  *string   `json:"user_email,omitempty"`
	Action     string    `json:"action"`
	ParamsHash string    `json:"params_sha256"`
	Result     string    `json:"result_status"`
	Error      *string   `json:"error_message,omitempty"`
}

func (a *App) handleListAuditLogs(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	if user == nil || !user.Role.Meets(RoleViewer) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	serverID := chi.URLParam(r, "id")
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			if parsed < 1 {
				parsed = 1
			}
			if parsed > 500 {
				parsed = 500
			}
			limit = parsed
		}
	}

	rows, err := a.DB.Query(r.Context(), `SELECT al.id, al.ts, al.user_id, u.email, al.action, al.params_sha256, al.result_status, al.error_message FROM audit_logs al LEFT JOIN users u ON u.id = al.user_id WHERE al.server_id = $1 ORDER BY al.ts DESC LIMIT $2`, serverID, limit)
	if err != nil {
		a.internalError(w, err)
		return
	}
	defer rows.Close()

	items := make([]auditLogItem, 0)
	for rows.Next() {
		var (
			item   auditLogItem
			userID *string
			email  *string
			errMsg *string
		)
		if err := rows.Scan(&item.ID, &item.Timestamp, &userID, &email, &item.Action, &item.ParamsHash, &item.Result, &errMsg); err != nil {
			a.internalError(w, err)
			return
		}
		item.UserID = userID
		item.UserEmail = email
		item.Error = errMsg
		items = append(items, item)
	}

	a.writeJSON(w, items)
}

func (a *App) handleExportAuditLogs(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	if user == nil || !user.Role.Meets(RoleViewer) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	serverID := chi.URLParam(r, "id")
	if serverID == "" {
		http.Error(w, "server id required", http.StatusBadRequest)
		return
	}

	limit := 1000
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			if parsed < 1 {
				parsed = 1
			}
			if parsed > 5000 {
				parsed = 5000
			}
			limit = parsed
		}
	}

	query := `SELECT al.ts, u.email, al.action, al.params_sha256, al.result_status, al.error_message FROM audit_logs al LEFT JOIN users u ON u.id = al.user_id WHERE al.server_id = $1`
	args := []any{serverID}
	param := 2

	if fromRaw := r.URL.Query().Get("from"); fromRaw != "" {
		from, err := time.Parse(time.RFC3339, fromRaw)
		if err != nil {
			http.Error(w, "invalid from timestamp", http.StatusBadRequest)
			return
		}
		query += fmt.Sprintf(" AND al.ts >= $%d", param)
		args = append(args, from)
		param++
	}

	if toRaw := r.URL.Query().Get("to"); toRaw != "" {
		to, err := time.Parse(time.RFC3339, toRaw)
		if err != nil {
			http.Error(w, "invalid to timestamp", http.StatusBadRequest)
			return
		}
		query += fmt.Sprintf(" AND al.ts <= $%d", param)
		args = append(args, to)
		param++
	}

	query += fmt.Sprintf(" ORDER BY al.ts ASC LIMIT $%d", param)
	args = append(args, limit)

	rows, err := a.DB.Query(r.Context(), query, args...)
	if err != nil {
		a.internalError(w, err)
		return
	}
	defer rows.Close()

	filename := fmt.Sprintf("server-%s-audit.csv", serverID)
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.WriteHeader(http.StatusOK)

	writer := csv.NewWriter(w)
	defer writer.Flush()

	if err := writer.Write([]string{"timestamp", "user_email", "action", "params_sha256", "result_status", "error_message"}); err != nil {
		a.Logger.Error("failed to write csv header", slog.Any("err", err))
		return
	}

	for rows.Next() {
		var (
			ts     time.Time
			email  *string
			action string
			params string
			result string
			errMsg *string
		)
		if err := rows.Scan(&ts, &email, &action, &params, &result, &errMsg); err != nil {
			a.internalError(w, err)
			return
		}

		emailVal := ""
		if email != nil {
			emailVal = *email
		}
		errVal := ""
		if errMsg != nil {
			errVal = *errMsg
		}

		record := []string{ts.UTC().Format(time.RFC3339), emailVal, action, params, result, errVal}
		if err := writer.Write(record); err != nil {
			a.Logger.Error("failed to write csv row", slog.Any("err", err))
			return
		}
	}

	if err := rows.Err(); err != nil {
		a.internalError(w, err)
		return
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		a.Logger.Error("csv writer error", slog.Any("err", err))
	}
}
