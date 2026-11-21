package cache

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type MockOutboxDB struct {
	mu     sync.Mutex
	events map[uuid.UUID]OutboxEvent
}

func NewMockOutboxDB() *MockOutboxDB {
	return &MockOutboxDB{
		events: make(map[uuid.UUID]OutboxEvent),
	}
}

func (m *MockOutboxDB) AddEvent(e OutboxEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events[e.ID] = e
}

func (m *MockOutboxDB) GetEvent(id uuid.UUID) (OutboxEvent, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.events[id]
	return e, ok
}

func (m *MockOutboxDB) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Update status completed
	if len(arguments) == 1 {
		id := arguments[0].(uuid.UUID)
		e, ok := m.events[id]
		if !ok {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		}
		e.Status = "completed"
		now := time.Now()
		e.ProcessedAt = &now
		m.events[id] = e
		return pgconn.NewCommandTag("UPDATE 1"), nil
	}

	// Update status failed
	if len(arguments) >= 4 {
		id := arguments[0].(uuid.UUID)
		status := arguments[1].(string)
		retry := arguments[2].(int)
		lastErr := arguments[3].(string)

		e, ok := m.events[id]
		if !ok {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		}
		e.Status = status
		e.RetryCount = retry
		e.LastError = &lastErr
		m.events[id] = e
		return pgconn.NewCommandTag("UPDATE 1"), nil
	}

	return pgconn.NewCommandTag("1"), nil
}

func (m *MockOutboxDB) Query(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var pending []OutboxEvent
	for _, e := range m.events {
		if e.Status == "pending" {
			pending = append(pending, e)
		}
	}

	return &MockOutboxRows{events: pending, index: -1}, nil
}

func (m *MockOutboxDB) QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row {
	return nil
}

type MockOutboxRows struct {
	events []OutboxEvent
	index  int
}

func (r *MockOutboxRows) Close() {}
func (r *MockOutboxRows) Err() error {
	return nil
}
func (r *MockOutboxRows) CommandTag() pgconn.CommandTag {
	return pgconn.NewCommandTag("")
}
func (r *MockOutboxRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (r *MockOutboxRows) Next() bool {
	r.index++
	return r.index < len(r.events)
}
func (r *MockOutboxRows) Values() ([]any, error) {
	return nil, nil
}
func (r *MockOutboxRows) RawValues() [][]byte {
	return nil
}
func (r *MockOutboxRows) TypeMap() *pgtype.Map {
	return pgtype.NewMap()
}
func (r *MockOutboxRows) Conn() *pgx.Conn {
	return nil
}
func (r *MockOutboxRows) Scan(dest ...any) error {
	e := r.events[r.index]
	*dest[0].(*uuid.UUID) = e.ID
	*dest[1].(*uuid.UUID) = e.TenantID
	*dest[2].(*string) = e.EventType

	payloadBytes, _ := json.Marshal(e.Payload)
	*dest[3].(*[]byte) = payloadBytes

	*dest[4].(*string) = e.Status
	*dest[5].(*int) = e.RetryCount
	*dest[6].(**string) = e.LastError
	*dest[7].(*time.Time) = e.CreatedAt
	*dest[8].(**time.Time) = e.ProcessedAt
	return nil
}
