package retention

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const DefaultRetentionDays = 90

type RetentionDBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error)
}

type Cleaner struct {
	db RetentionDBTX
}

func NewCleaner(db RetentionDBTX) *Cleaner {
	return &Cleaner{db: db}
}

// AnonymizeExpiredGuestData scrubs personal contact details (guest_name, guest_phone, guest_lookup_hash)
// for registrations whose workshop ended more than 90 days ago or were cancelled more than 90 days ago.
// For paid workshops, personal data is only scrubbed if any associated purchase is finalized (completed, refunded, or failed)
// so that active financial disputes (refund_required) are never scrubbed prematurely.
// Financial snapshots, transaction amounts, and reference numbers in `purchases` and `refunds` remain completely intact.
func (c *Cleaner) AnonymizeExpiredGuestData(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	if limit <= 0 {
		limit = 500
	}

	query := `
UPDATE workshop_registrations wr
SET 
    guest_name = 'مهمان',
    guest_phone = '',
    guest_lookup_hash = '',
    updated_at = now()
WHERE wr.id IN (
    SELECT r.id
    FROM workshop_registrations r
    JOIN workshops w ON w.id = r.workshop_id AND w.tenant_id = r.tenant_id
    LEFT JOIN purchases p ON p.id = r.purchase_id AND p.tenant_id = r.tenant_id
    WHERE (
        -- Workshop ended before cutoff OR registration was cancelled before cutoff
        (w.ends_at < $1) OR (r.status = 'cancelled' AND r.updated_at < $1)
    )
    -- Has PII to scrub
    AND (r.guest_phone != '' OR r.guest_name != 'مهمان' OR r.guest_lookup_hash != '')
    -- Financial disputes/pending transactions must be resolved
    AND (
        r.purchase_id IS NULL OR 
        p.status IN ('completed', 'refunded', 'failed')
    )
    LIMIT $2
);`

	tag, err := c.db.Exec(ctx, query, cutoff, limit)
	if err != nil {
		return 0, fmt.Errorf("failed to anonymize expired registrations: %w", err)
	}

	return tag.RowsAffected(), nil
}

// AnonymizeExpiredAppointmentData scrubs personal contact details (guest_name, guest_phone, guest_lookup_hash, notes)
// for appointments whose scheduled slot started more than 90 days ago, or were cancelled more than 90 days ago.
func (c *Cleaner) AnonymizeExpiredAppointmentData(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	if limit <= 0 {
		limit = 500
	}

	query := `
UPDATE appointments a
SET 
    guest_name = 'مراجع',
    guest_phone = '',
    guest_lookup_hash = '',
    notes = '',
    updated_at = now()
WHERE a.id IN (
    SELECT app.id
    FROM appointments app
    JOIN appointment_slots s ON s.id = app.slot_id AND s.tenant_id = app.tenant_id
    WHERE (
        (s.starts_at < $1) OR (app.status = 'cancelled' AND app.cancelled_at < $1)
    )
    AND (app.guest_phone != '' OR app.guest_name != 'مراجع' OR app.guest_lookup_hash != '' OR app.notes != '')
    LIMIT $2
);`

	tag, err := c.db.Exec(ctx, query, cutoff, limit)
	if err != nil {
		return 0, fmt.Errorf("failed to anonymize expired appointments: %w", err)
	}

	return tag.RowsAffected(), nil
}

// PurgeExpiredOTPs permanently deletes buyer OTP records that either expired before cutoff or were consumed before cutoff.
func (c *Cleaner) PurgeExpiredOTPs(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	if limit <= 0 {
		limit = 1000
	}

	query := `
DELETE FROM buyer_otp_codes
WHERE id IN (
    SELECT id
    FROM buyer_otp_codes
    WHERE expires_at < $1 OR (consumed_at IS NOT NULL AND consumed_at < $1)
    LIMIT $2
);`

	tag, err := c.db.Exec(ctx, query, cutoff, limit)
	if err != nil {
		return 0, fmt.Errorf("failed to purge expired buyer otps: %w", err)
	}

	return tag.RowsAffected(), nil
}

// PurgeExpiredBuyerSessions permanently deletes buyer session records that expired before cutoff.
func (c *Cleaner) PurgeExpiredBuyerSessions(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	if limit <= 0 {
		limit = 1000
	}

	query := `
DELETE FROM buyer_sessions
WHERE id IN (
    SELECT id
    FROM buyer_sessions
    WHERE expires_at < $1
    LIMIT $2
);`

	tag, err := c.db.Exec(ctx, query, cutoff, limit)
	if err != nil {
		return 0, fmt.Errorf("failed to purge expired buyer sessions: %w", err)
	}

	return tag.RowsAffected(), nil
}
