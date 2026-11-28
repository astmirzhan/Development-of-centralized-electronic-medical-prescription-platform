package audit

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestComputeRecordHashMicrosecondPrecision(t *testing.T) {
	actorID := uuid.New()
	entityID := uuid.New()
	entry := Entry{
		ActorID:    &actorID,
		Action:     "prescription:sign",
		EntityType: "prescription",
		EntityID:   &entityID,
	}
	payload := []byte(`{"status":"active"}`)

	withNanos := time.Date(2026, 11, 28, 12, 30, 45, 123456789, time.UTC)
	pgMicros := withNanos.Truncate(time.Microsecond)

	h1 := ComputeRecordHash("prevhash", entry, payload, withNanos)
	h2 := ComputeRecordHash("prevhash", entry, payload, pgMicros)
	if h1 != h2 {
		t.Fatalf("expected identical hash across nanosecond and PostgreSQL microsecond truncation: %s != %s", h1, h2)
	}
}
