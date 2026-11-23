package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type OutboxPayload struct {
	Host   string   `json:"host"`
	Paths  []string `json:"paths"`
	Reason string   `json:"reason,omitempty"`
}

type OutboxEvent struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	EventType   string
	Payload     OutboxPayload
	Status      string
	RetryCount  int
	LastError   *string
	CreatedAt   time.Time
	ProcessedAt *time.Time
}

type OutboxDBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row
}

// InsertCacheInvalidationEvent inserts an outbox event in the SAME transaction that modified the data (AP-11).
// This guarantees that if the transaction rolls back, no rogue purge event is emitted,
// and if it commits, the worker will reliably purge the affected URLs with retries.
func InsertCacheInvalidationEvent(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, eventType, host string, paths []string) error {
	if tenantID == uuid.Nil {
		return fmt.Errorf("tenant_id cannot be nil")
	}
	if host == "" {
		return fmt.Errorf("host cannot be empty")
	}
	if len(paths) == 0 {
		paths = []string{"/"}
	}

	payload := OutboxPayload{
		Host:  host,
		Paths: paths,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal outbox payload: %w", err)
	}

	query := `
INSERT INTO outbox_events (
    tenant_id,
    event_type,
    payload,
    status,
    retry_count,
    created_at
) VALUES ($1, $2, $3, 'pending', 0, now());`

	_, err = tx.Exec(ctx, query, tenantID, eventType, payloadBytes)
	if err != nil {
		return fmt.Errorf("failed to insert outbox event: %w", err)
	}

	return nil
}

// FetchPendingEvents retrieves up to limit pending outbox events for asynchronous processing.
// In production PostgreSQL, SELECT ... FOR UPDATE SKIP LOCKED is used to prevent concurrent worker collisions.
func FetchPendingEvents(ctx context.Context, db OutboxDBTX, limit int) ([]OutboxEvent, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
SELECT 
    id,
    tenant_id,
    event_type,
    payload,
    status,
    retry_count,
    last_error,
    created_at,
    processed_at
FROM outbox_events
WHERE status = 'pending'
ORDER BY created_at ASC
LIMIT $1
FOR UPDATE SKIP LOCKED;`

	rows, err := db.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending outbox events: %w", err)
	}
	defer rows.Close()

	var events []OutboxEvent
	for rows.Next() {
		var e OutboxEvent
		var rawPayload []byte
		err := rows.Scan(
			&e.ID,
			&e.TenantID,
			&e.EventType,
			&rawPayload,
			&e.Status,
			&e.RetryCount,
			&e.LastError,
			&e.CreatedAt,
			&e.ProcessedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan outbox event: %w", err)
		}

		if len(rawPayload) > 0 {
			if err := json.Unmarshal(rawPayload, &e.Payload); err != nil {
				return nil, fmt.Errorf("failed to unmarshal payload for event %s: %w", e.ID, err)
			}
		}

		events = append(events, e)
	}

	return events, rows.Err()
}

// MarkEventCompleted marks the outbox event as completed after successful CDN purge.
func MarkEventCompleted(ctx context.Context, db OutboxDBTX, eventID uuid.UUID) error {
	query := `
UPDATE outbox_events
SET 
    status = 'completed',
    processed_at = now()
WHERE id = $1;`

	tag, err := db.Exec(ctx, query, eventID)
	if err != nil {
		return fmt.Errorf("failed to mark outbox event %s completed: %w", eventID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("outbox event %s not found", eventID)
	}
	return nil
}

// MarkEventFailed updates the outbox event with error details and increments retry count.
// If retryCount exceeds maxRetries (3), status is set to 'failed'.
func MarkEventFailed(ctx context.Context, db OutboxDBTX, eventID uuid.UUID, lastErr string, retryCount int, maxRetries int) error {
	status := "pending"
	if retryCount >= maxRetries {
		status = "failed"
	}

	query := `
UPDATE outbox_events
SET 
    status = $2,
    retry_count = $3,
    last_error = $4
WHERE id = $1;`

	_, err := db.Exec(ctx, query, eventID, status, retryCount, lastErr)
	if err != nil {
		return fmt.Errorf("failed to update failed outbox event %s: %w", eventID, err)
	}
	return nil
}
