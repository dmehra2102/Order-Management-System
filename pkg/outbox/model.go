package outbox

import "time"

type Status string

const (
	StatusPending    Status = "pending"
	StatusInProgress Status = "in_progress"
	StatusSent       Status = "sent"
	StatusFailed     Status = "failed"
)

type Event struct {
	ID            int64
	AggregateType string
	AggregateID   string
	Type          string
	Payload       []byte
	Headers       map[string]string
	Traceparent   string
	CreatedAt     time.Time
	Status        Status
	ReplayID      string
	RetryCount    int
	LastError     *string
}
