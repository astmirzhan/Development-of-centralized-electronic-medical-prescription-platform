package database_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xjam-byte/erx-platform/backend/internal/audit"
	"github.com/xjam-byte/erx-platform/backend/internal/database"
	"github.com/xjam-byte/erx-platform/backend/internal/seed"
	"github.com/xjam-byte/erx-platform/backend/internal/testutil"
)

func TestMigrationUpSeedDownUpCycle(t *testing.T) {
	pool := testutil.SetupSeededDB(t)
	ctx := context.Background()

	// Idempotent second seed run must succeed cleanly.
	if err := seed.Run(ctx, pool); err != nil {
		t.Fatalf("second seed.Run failed: %v", err)
	}

	var tableCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM information_schema.tables
		WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
	`).Scan(&tableCount); err != nil {
		t.Fatalf("count tables: %v", err)
	}
	if tableCount != 24 {
		t.Fatalf("expected exactly 24 tables from 04-erd.puml, got %d", tableCount)
	}

	// Migration down must cleanly drop all 24 tables and 6 enums.
	downSQL := testutil.ReadMigrationSQL(t, "000001_init_schema.down.sql")
	if _, err := pool.Exec(ctx, downSQL); err != nil {
		t.Fatalf("down migration failed: %v", err)
	}

	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM information_schema.tables
		WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
	`).Scan(&tableCount); err != nil {
		t.Fatalf("count tables after down: %v", err)
	}
	if tableCount != 0 {
		t.Fatalf("expected 0 tables after down migration, got %d", tableCount)
	}

	// Re-applying up migration + seed with a fresh connection pool (refreshes recreated ENUM OIDs).
	upSQL := testutil.ReadMigrationSQL(t, "000001_init_schema.up.sql")
	if _, err := pool.Exec(ctx, upSQL); err != nil {
		t.Fatalf("re-apply up migration failed: %v", err)
	}
	freshPool, err := pgxpool.New(ctx, pool.Config().ConnString())
	if err != nil {
		t.Fatalf("reconnect pool after migration cycle: %v", err)
	}
	defer freshPool.Close()
	if err := seed.Run(ctx, freshPool); err != nil {
		t.Fatalf("re-seed after up migration failed: %v", err)
	}
}

func TestSchemaCheckAndUniqueConstraints(t *testing.T) {
	pool := testutil.SetupSeededDB(t)
	ctx := context.Background()

	var diagID int
	if err := pool.QueryRow(ctx, `SELECT id FROM icd10_codes WHERE code = 'J18.9'`).Scan(&diagID); err != nil {
		t.Fatalf("lookup diagnosis J18.9: %v", err)
	}
	formID := uuid.MustParse("20000000-0000-0000-0000-000000000015") // Paracetamol 500mg
	brandID := uuid.MustParse("30000000-0000-0000-0000-000000000015")

	rxID := uuid.New()
	itemID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO prescriptions (id, prescription_number, doctor_id, clinic_id, patient_id, diagnosis_id, status, valid_until)
		VALUES ($1, 'RX-TEST-CHK-1', $2, $3, $4, $5, 'active', $6)
	`, rxID, seed.DoctorUserID, seed.ClinicOrgID, seed.PatientUserID, diagID, time.Now().UTC().Add(30*24*time.Hour))
	if err != nil {
		t.Fatalf("insert test prescription: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO prescription_items (id, prescription_id, medicine_form_id, dosage_value, dosage_unit, frequency, duration_days, quantity_prescribed, quantity_dispensed, status)
		VALUES ($1, $2, $3, 500.00, 'mg', '2x/day', 5, 10, 0, 'pending')
	`, itemID, rxID, formID)
	if err != nil {
		t.Fatalf("insert test prescription_item: %v", err)
	}

	// 1. Over-dispensing check constraint violation (23514).
	_, err = pool.Exec(ctx, `UPDATE prescription_items SET quantity_dispensed = 11 WHERE id = $1`, itemID)
	assertPGCode(t, err, "23514", "over-dispense CHECK constraint")

	// 2. Non-positive quantity_prescribed check constraint violation (23514).
	_, err = pool.Exec(ctx, `
		INSERT INTO prescription_items (id, prescription_id, medicine_form_id, dosage_value, dosage_unit, frequency, duration_days, quantity_prescribed)
		VALUES ($1, $2, $3, 500.00, 'mg', '1x/day', 5, 0)
	`, uuid.New(), rxID, formID)
	assertPGCode(t, err, "23514", "quantity_prescribed > 0 CHECK constraint")

	// 3. Non-positive dispensing_events.quantity_dispensed check constraint violation (23514).
	_, err = pool.Exec(ctx, `
		INSERT INTO dispensing_events (id, prescription_item_id, pharmacist_id, pharmacy_id, medicine_brand_id, quantity_dispensed, dispensed_at)
		VALUES ($1, $2, $3, $4, $5, 0, NOW())
	`, uuid.New(), itemID, seed.PharmacistUserID, seed.PharmacyOrgID, brandID)
	assertPGCode(t, err, "23514", "dispensing_events quantity_dispensed > 0 CHECK constraint")

	// 4. Symmetric drug_interactions unique index (LEAST, GREATEST) rejects reverse duplicate (23505).
	warfarinID := uuid.MustParse("10000000-0000-0000-0000-000000000004")
	aspirinID := uuid.MustParse("10000000-0000-0000-0000-000000000005")
	_, err = pool.Exec(ctx, `
		INSERT INTO drug_interactions (id, medicine_a_id, medicine_b_id, severity_level, description)
		VALUES ($1, $2, $3, 'critical', 'duplicate reverse pair')
	`, uuid.New(), aspirinID, warfarinID)
	assertPGCode(t, err, "23505", "drug_interactions symmetric pair unique index")
}

func assertPGCode(t *testing.T, err error, expectedCode, label string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: expected PostgreSQL error %s, got nil", label, expectedCode)
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("%s: expected *pgconn.PgError, got %T (%v)", label, err, err)
	}
	if pgErr.Code != expectedCode {
		t.Fatalf("%s: expected SQLSTATE %s, got %s (%s)", label, expectedCode, pgErr.Code, pgErr.Message)
	}
}

func TestAuditLogHashChainVerificationPG16(t *testing.T) {
	pool := testutil.SetupSeededDB(t)
	ctx := context.Background()
	logger := audit.NewPGLogger()

	actorID := seed.DoctorUserID
	e1 := uuid.New()
	e2 := uuid.New()

	err := database.WithTx(ctx, pool, func(tx pgx.Tx) error {
		if err := logger.AppendTx(ctx, tx, audit.Entry{
			ActorID:    &actorID,
			Action:     "prescription:create",
			EntityType: "prescription",
			EntityID:   &e1,
			Payload:    map[string]any{"step": 1},
		}); err != nil {
			return err
		}
		return logger.AppendTx(ctx, tx, audit.Entry{
			ActorID:    &actorID,
			Action:     "prescription:sign",
			EntityType: "prescription",
			EntityID:   &e2,
			Payload:    map[string]any{"step": 2},
		})
	})
	if err != nil {
		t.Fatalf("append audit chain: %v", err)
	}

	rows, err := pool.Query(ctx, `
		SELECT COALESCE(prev_record_hash, ''), record_hash, actor_id, action, entity_type, entity_id, payload, created_at
		FROM audit_logs
		ORDER BY id ASC
	`)
	if err != nil {
		t.Fatalf("query audit_logs: %v", err)
	}
	defer rows.Close()

	var expectedPrev string
	var count int
	for rows.Next() {
		var (
			prevHash, recordHash, action, entityType string
			rowActorID, rowEntityID                  *uuid.UUID
			payload                                  []byte
			createdAt                                time.Time
		)
		if err := rows.Scan(&prevHash, &recordHash, &rowActorID, &action, &entityType, &rowEntityID, &payload, &createdAt); err != nil {
			t.Fatalf("scan audit row: %v", err)
		}
		if prevHash != expectedPrev {
			t.Fatalf("broken prev_record_hash at row %d: got %q, want %q", count, prevHash, expectedPrev)
		}
		recomputed := audit.ComputeRecordHash(prevHash, audit.Entry{
			ActorID:    rowActorID,
			Action:     action,
			EntityType: entityType,
			EntityID:   rowEntityID,
		}, payload, createdAt)
		if recomputed != recordHash {
			t.Fatalf("hash mismatch at row %d: recomputed %s != stored %s", count, recomputed, recordHash)
		}
		expectedPrev = recordHash
		count++
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate audit rows: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 audit rows, got %d", count)
	}
}
