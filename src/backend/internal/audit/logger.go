package audit

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Entry describes an append-only audit log event.
type Entry struct {
	ActorID    *uuid.UUID
	Action     string
	EntityType string
	EntityID   *uuid.UUID
	Payload    any
	IPAddress  string
	UserAgent  string
}

// Logger writes hash-chained audit records inside an active database transaction.
type Logger interface {
	AppendTx(ctx context.Context, tx pgx.Tx, entry Entry) error
}

// PGLogger stores tamper-evident hash-chained records in audit_logs.
type PGLogger struct{}

// NewPGLogger creates a new PostgreSQL audit logger.
func NewPGLogger() *PGLogger {
	return &PGLogger{}
}

// AppendTx appends an entry to audit_logs chained to the previous record's SHA-256 hash.
func (l *PGLogger) AppendTx(ctx context.Context, tx pgx.Tx, entry Entry) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('audit_logs_chain'))`); err != nil {
		return fmt.Errorf("acquire audit chain lock: %w", err)
	}

	var prevHash string
	err := tx.QueryRow(ctx, `SELECT COALESCE((SELECT record_hash FROM audit_logs ORDER BY id DESC LIMIT 1), '')`).Scan(&prevHash)
	if err != nil {
		return fmt.Errorf("query previous audit hash: %w", err)
	}

	var payloadBytes []byte
	if entry.Payload != nil {
		payloadBytes, err = json.Marshal(entry.Payload)
		if err != nil {
			return fmt.Errorf("marshal audit payload: %w", err)
		}
	}

	createdAt := time.Now().UTC().Truncate(time.Microsecond)
	recordHash := ComputeRecordHash(prevHash, entry, payloadBytes, createdAt)

	var prevPtr *string
	if prevHash != "" {
		prevPtr = &prevHash
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO audit_logs (
			prev_record_hash, record_hash, actor_id, action, entity_type, entity_id,
			payload, ip_address, user_agent, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`,
		prevPtr,
		recordHash,
		entry.ActorID,
		entry.Action,
		entry.EntityType,
		entry.EntityID,
		payloadBytes,
		nullIfEmpty(entry.IPAddress),
		nullIfEmpty(entry.UserAgent),
		createdAt,
	)
	if err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}
	return nil
}

// ComputeRecordHash deterministically hashes an audit entry chained to prevHash.
// Timestamps are normalized to UTC microsecond precision to match PostgreSQL TIMESTAMPTZ storage.
func ComputeRecordHash(prevHash string, entry Entry, payload []byte, createdAt time.Time) string {
	h := sha256.New()
	h.Write([]byte(prevHash))
	h.Write([]byte("|"))
	if entry.ActorID != nil {
		h.Write([]byte(entry.ActorID.String()))
	}
	h.Write([]byte("|"))
	h.Write([]byte(entry.Action))
	h.Write([]byte("|"))
	h.Write([]byte(entry.EntityType))
	h.Write([]byte("|"))
	if entry.EntityID != nil {
		h.Write([]byte(entry.EntityID.String()))
	}
	h.Write([]byte("|"))
	h.Write(canonicalizePayload(payload))
	h.Write([]byte("|"))
	h.Write([]byte(createdAt.UTC().Truncate(time.Microsecond).Format(time.RFC3339Nano)))
	return hex.EncodeToString(h.Sum(nil))
}

// canonicalizePayload normalizes JSON bytes (e.g., PostgreSQL JSONB pretty-spacing) into compact
// deterministic JSON so hashes match across Go marshaling and PostgreSQL round-trips.
func canonicalizePayload(payload []byte) []byte {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.UseNumber()
	var parsed any
	if err := dec.Decode(&parsed); err != nil {
		return trimmed
	}
	compact, err := json.Marshal(parsed)
	if err != nil {
		return trimmed
	}
	return compact
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
