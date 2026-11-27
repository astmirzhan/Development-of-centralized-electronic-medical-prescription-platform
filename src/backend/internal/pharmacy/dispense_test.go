package pharmacy_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xjam-byte/erx-platform/backend/internal/audit"
	"github.com/xjam-byte/erx-platform/backend/internal/auth"
	"github.com/xjam-byte/erx-platform/backend/internal/httpserver"
	"github.com/xjam-byte/erx-platform/backend/internal/pharmacy"
	"github.com/xjam-byte/erx-platform/backend/internal/pki"
	"github.com/xjam-byte/erx-platform/backend/internal/qr"
	"github.com/xjam-byte/erx-platform/backend/internal/seed"
	"github.com/xjam-byte/erx-platform/backend/internal/testutil"
)

func setupActivePrescriptionItem(t *testing.T, pool *pgxpool.Pool, prescribedQty int) (rxID, itemID, brandID uuid.UUID) {
	t.Helper()
	ctx := context.Background()

	var diagID int
	if err := pool.QueryRow(ctx, `SELECT id FROM icd10_codes WHERE code = 'J18.9'`).Scan(&diagID); err != nil {
		t.Fatalf("lookup J18.9: %v", err)
	}

	formID := uuid.MustParse("20000000-0000-0000-0000-000000000015") // Paracetamol 500mg
	brandID = uuid.MustParse("30000000-0000-0000-0000-000000000015")
	rxID = uuid.New()
	itemID = uuid.New()

	if _, err := pool.Exec(ctx, `
		INSERT INTO prescriptions (id, prescription_number, doctor_id, clinic_id, patient_id, diagnosis_id, status, valid_until)
		VALUES ($1, $2, $3, $4, $5, $6, 'active', $7)
	`, rxID, "RX-PHARM-"+rxID.String()[:8], seed.DoctorUserID, seed.ClinicOrgID, seed.PatientUserID, diagID, time.Now().UTC().Add(30*24*time.Hour)); err != nil {
		t.Fatalf("insert active prescription: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO prescription_items (id, prescription_id, medicine_form_id, dosage_value, dosage_unit, frequency, duration_days, quantity_prescribed)
		VALUES ($1, $2, $3, 500.00, 'mg', '2x/day', 5, $4)
	`, itemID, rxID, formID, prescribedQty); err != nil {
		t.Fatalf("insert prescription item: %v", err)
	}
	return rxID, itemID, brandID
}

func newTestPharmacyServer(t *testing.T, pool *pgxpool.Pool, actor auth.ActorContext) (*httptest.Server, *qr.Service, *pharmacy.Service) {
	t.Helper()
	qrRepo := qr.NewPGRepository(pool)
	qrSvc := qr.NewService(qrRepo, 60*time.Second, 90*time.Second)
	sigRepo := pki.NewPGRepository()
	verifier, err := pki.NewMockVerifier("development", true, "dispense-test-secret")
	if err != nil {
		t.Fatalf("NewMockVerifier: %v", err)
	}
	pharmSvc := pharmacy.NewService(pool, qrRepo, sigRepo, verifier, audit.NewPGLogger(), nil)
	h := pharmacy.NewHandler(pharmSvc, nil)

	router := httpserver.NewRouter(httpserver.RouterDeps{
		RegisterRoutes: func(r chi.Router) {
			r.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					next.ServeHTTP(w, r.WithContext(auth.WithActor(r.Context(), actor)))
				})
			})
			h.RegisterRoutes(r)
		},
	})

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return srv, qrSvc, pharmSvc
}

func signDispenseReq(t *testing.T, qrToken, pharmacistIIN string, brandID uuid.UUID, qty int) pharmacy.DispenseRequest {
	t.Helper()
	canonical, err := pharmacy.BuildDispensingCanonical(qrToken, pharmacistIIN, brandID, qty)
	if err != nil {
		t.Fatalf("BuildDispensingCanonical: %v", err)
	}
	cms, err := pki.CreateTestEnvelope("dispense-test-secret", pharmacistIIN, "PHARM-CERT-1", canonical)
	if err != nil {
		t.Fatalf("CreateTestEnvelope: %v", err)
	}
	return pharmacy.DispenseRequest{
		QRToken:         qrToken,
		MedicineBrandID: brandID,
		Quantity:        qty,
		CMSSignature:    cms,
	}
}

func TestConcurrentReplayAttackPG16(t *testing.T) {
	pool := testutil.SetupSeededDB(t)
	ctx := context.Background()

	_, itemID, brandID := setupActivePrescriptionItem(t, pool, 10)
	pharmacist := auth.ActorContext{
		UserID: seed.PharmacistUserID,
		IIN:    "920721402851",
		Role:   auth.RolePharmacist,
	}
	srv, qrSvc, _ := newTestPharmacyServer(t, pool, pharmacist)

	issued, err := qrSvc.IssueToken(ctx, itemID, seed.PatientUserID)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	reqBody := signDispenseReq(t, issued.Token, pharmacist.IIN, brandID, 5)
	payload, _ := json.Marshal(reqBody)

	const goroutines = 10
	startCh := make(chan struct{})
	statuses := make([]int, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		idx := i
		go func() {
			defer wg.Done()
			<-startCh
			resp, err := http.Post(srv.URL+"/api/v1/dispensing", "application/json", bytes.NewReader(payload))
			if err != nil {
				t.Errorf("goroutine %d HTTP error: %v", idx, err)
				return
			}
			_ = resp.Body.Close()
			statuses[idx] = resp.StatusCode
		}()
	}

	close(startCh)
	wg.Wait()

	var okCount, conflictOrGoneCount int
	for _, code := range statuses {
		switch code {
		case http.StatusOK:
			okCount++
		case http.StatusConflict, http.StatusGone:
			conflictOrGoneCount++
		default:
			t.Fatalf("unexpected HTTP status %d in concurrent replay attack (statuses=%v)", code, statuses)
		}
	}

	if okCount != 1 {
		t.Fatalf("expected exactly 1 success (200 OK), got %d (statuses=%v)", okCount, statuses)
	}
	if conflictOrGoneCount != goroutines-1 {
		t.Fatalf("expected %d failures (409/410), got %d", goroutines-1, conflictOrGoneCount)
	}

	var qtyDispensed, eventCount int
	var isConsumed bool
	_ = pool.QueryRow(ctx, `SELECT quantity_dispensed FROM prescription_items WHERE id = $1`, itemID).Scan(&qtyDispensed)
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM dispensing_events WHERE prescription_item_id = $1`, itemID).Scan(&eventCount)
	_ = pool.QueryRow(ctx, `SELECT is_consumed FROM qr_tokens WHERE token_hash = $1`, qr.HashToken(issued.Token)).Scan(&isConsumed)

	if qtyDispensed != 5 || eventCount != 1 || !isConsumed {
		t.Fatalf("post-race DB state mismatch: qty=%d events=%d consumed=%v", qtyDispensed, eventCount, isConsumed)
	}
}

func TestPartialThenFullDispensePG16(t *testing.T) {
	pool := testutil.SetupSeededDB(t)
	ctx := context.Background()

	rxID, itemID, brandID := setupActivePrescriptionItem(t, pool, 10)
	pharmacist := auth.ActorContext{
		UserID: seed.PharmacistUserID,
		IIN:    "920721402851",
		Role:   auth.RolePharmacist,
	}
	_, qrSvc, pharmSvc := newTestPharmacyServer(t, pool, pharmacist)

	// Step 1: Partial dispense (4 of 10) with QR #1.
	qr1, err := qrSvc.IssueToken(ctx, itemID, seed.PatientUserID)
	if err != nil {
		t.Fatalf("IssueToken #1: %v", err)
	}
	res1, err := pharmSvc.Dispense(ctx, pharmacist, signDispenseReq(t, qr1.Token, pharmacist.IIN, brandID, 4))
	if err != nil {
		t.Fatalf("partial dispense failed: %v", err)
	}
	if res1.ItemStatus != "partially_dispensed" || res1.PrescriptionStatus != "partially_dispensed" || res1.RemainingQuantity != 6 {
		t.Fatalf("unexpected partial dispense result: %#v", res1)
	}

	// Step 2: Issue fresh QR #2 and dispense remaining 6 -> transitions to fully_dispensed.
	qr2, err := qrSvc.IssueToken(ctx, itemID, seed.PatientUserID)
	if err != nil {
		t.Fatalf("IssueToken #2: %v", err)
	}

	// Attempting to over-dispense (7 > 6) must fail with ErrInsufficientQuantity.
	if _, err := pharmSvc.Dispense(ctx, pharmacist, signDispenseReq(t, qr2.Token, pharmacist.IIN, brandID, 7)); !errors.Is(err, pharmacy.ErrInsufficientQuantity) {
		t.Fatalf("expected ErrInsufficientQuantity, got %v", err)
	}

	res2, err := pharmSvc.Dispense(ctx, pharmacist, signDispenseReq(t, qr2.Token, pharmacist.IIN, brandID, 6))
	if err != nil {
		t.Fatalf("final dispense failed: %v", err)
	}
	if res2.ItemStatus != "dispensed" || res2.PrescriptionStatus != "fully_dispensed" || res2.RemainingQuantity != 0 {
		t.Fatalf("unexpected final dispense result: %#v", res2)
	}

	var dbRxStatus string
	_ = pool.QueryRow(ctx, `SELECT status::text FROM prescriptions WHERE id = $1`, rxID).Scan(&dbRxStatus)
	if dbRxStatus != "fully_dispensed" {
		t.Fatalf("expected DB prescription status fully_dispensed, got %s", dbRxStatus)
	}
}

func TestDispenseValidationAndErrorMappingsPG16(t *testing.T) {
	pool := testutil.SetupSeededDB(t)
	ctx := context.Background()

	_, itemID, brandID := setupActivePrescriptionItem(t, pool, 10)
	pharmacist := auth.ActorContext{
		UserID: seed.PharmacistUserID,
		IIN:    "920721402851",
		Role:   auth.RolePharmacist,
	}
	srv, qrSvc, _ := newTestPharmacyServer(t, pool, pharmacist)
	qrRepo := qr.NewPGRepository(pool)

	// 1. Unauthenticated -> 401
	unauthH := pharmacy.NewHandler(pharmacy.NewService(pool, qrRepo, pki.NewPGRepository(), nil, nil, nil), nil)
	unauthRouter := httpserver.NewRouter(httpserver.RouterDeps{RegisterRoutes: unauthH.RegisterRoutes})
	rec401 := httptest.NewRecorder()
	unauthRouter.ServeHTTP(rec401, httptest.NewRequest(http.MethodPost, "/api/v1/dispensing", bytes.NewBufferString(`{}`)))
	if rec401.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec401.Code)
	}

	postDispense := func(body any) int {
		b, _ := json.Marshal(body)
		resp, err := http.Post(srv.URL+"/api/v1/dispensing", "application/json", bytes.NewReader(b))
		if err != nil {
			t.Fatalf("HTTP POST error: %v", err)
		}
		_ = resp.Body.Close()
		return resp.StatusCode
	}

	// 2. Invalid request -> 400
	if code := postDispense(pharmacy.DispenseRequest{}); code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", code)
	}

	// 3. Unknown QR token -> 404
	if code := postDispense(signDispenseReq(t, "erx_nonexistent", pharmacist.IIN, brandID, 1)); code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", code)
	}

	// 4. Expired QR token -> 410 Gone
	expiredRaw := "erx_expired_dispense_token"
	if _, err := qrRepo.InsertToken(ctx, itemID, qr.HashToken(expiredRaw), "nonce123", time.Now().UTC().Add(-2*time.Minute)); err != nil {
		t.Fatalf("insert expired token: %v", err)
	}
	if code := postDispense(signDispenseReq(t, expiredRaw, pharmacist.IIN, brandID, 1)); code != http.StatusGone {
		t.Fatalf("expected 410 Gone, got %d", code)
	}

	// 5. Wrong brand for prescribed form -> 400
	issued, err := qrSvc.IssueToken(ctx, itemID, seed.PatientUserID)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	wrongBrandID := uuid.MustParse("30000000-0000-0000-0000-000000000001") // Amoxicillin brand
	if code := postDispense(signDispenseReq(t, issued.Token, pharmacist.IIN, wrongBrandID, 1)); code != http.StatusBadRequest {
		t.Fatalf("expected 400 for brand mismatch, got %d", code)
	}

	// 6. Tampered CMS signature -> 422
	badSig := signDispenseReq(t, issued.Token, pharmacist.IIN, brandID, 1)
	badSig.CMSSignature = "invalid-signature"
	if code := postDispense(badSig); code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for invalid EDS, got %d", code)
	}

	// 7. Forbidden non-pharmacist role -> 403
	docActor := auth.ActorContext{UserID: seed.DoctorUserID, IIN: "840315301294", Role: auth.RoleDoctor}
	docSrv, _, _ := newTestPharmacyServer(t, pool, docActor)
	b, _ := json.Marshal(signDispenseReq(t, issued.Token, docActor.IIN, brandID, 1))
	resp403, err := http.Post(docSrv.URL+"/api/v1/dispensing", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("HTTP POST docSrv error: %v", err)
	}
	_ = resp403.Body.Close()
	if resp403.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp403.StatusCode)
	}
}

func TestDispenseAdditionalBranchesPG16(t *testing.T) {
	pool := testutil.SetupSeededDB(t)
	ctx := context.Background()

	rxID, itemID, brandID := setupActivePrescriptionItem(t, pool, 10)
	pharmacist := auth.ActorContext{
		UserID: seed.PharmacistUserID,
		IIN:    "920721402851",
		Role:   auth.RolePharmacist,
	}
	srv, qrSvc, pharmSvc := newTestPharmacyServer(t, pool, pharmacist)
	qrRepo := qr.NewPGRepository(pool)

	// 1. Missing CMSSignature -> ErrInvalidDispenseRequest (400)
	issued, err := qrSvc.IssueToken(ctx, itemID, seed.PatientUserID)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	if _, err := pharmSvc.Dispense(ctx, pharmacist, pharmacy.DispenseRequest{
		QRToken:         issued.Token,
		MedicineBrandID: brandID,
		Quantity:        1,
	}); !errors.Is(err, pharmacy.ErrInvalidDispenseRequest) {
		t.Fatalf("expected ErrInvalidDispenseRequest for empty signature, got %v", err)
	}

	// 2. Malformed JSON body -> 400
	respBadJSON, err := http.Post(srv.URL+"/api/v1/dispensing", "application/json", bytes.NewBufferString(`{invalid`))
	if err != nil {
		t.Fatalf("HTTP POST bad JSON: %v", err)
	}
	_ = respBadJSON.Body.Close()
	if respBadJSON.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for malformed JSON, got %d", respBadJSON.StatusCode)
	}

	// 3. Unknown pharmacist profile -> ErrForbiddenPharmacist
	unknownPharm := auth.ActorContext{UserID: uuid.New(), IIN: "999999999999", Role: auth.RolePharmacist}
	if _, err := pharmSvc.Dispense(ctx, unknownPharm, signDispenseReq(t, issued.Token, unknownPharm.IIN, brandID, 1)); !errors.Is(err, pharmacy.ErrForbiddenPharmacist) {
		t.Fatalf("expected ErrForbiddenPharmacist for unknown profile, got %v", err)
	}

	// 4. Non-existent medicine brand UUID -> ErrBrandMismatch
	if _, err := pharmSvc.Dispense(ctx, pharmacist, signDispenseReq(t, issued.Token, pharmacist.IIN, uuid.New(), 1)); !errors.Is(err, pharmacy.ErrBrandMismatch) {
		t.Fatalf("expected ErrBrandMismatch for unknown brand, got %v", err)
	}

	// 5. Dispense with non-empty Notes succeeds cleanly
	reqWithNotes := signDispenseReq(t, issued.Token, pharmacist.IIN, brandID, 2)
	reqWithNotes.Notes = "Patient counseled on dosage"
	res, err := pharmSvc.Dispense(ctx, pharmacist, reqWithNotes)
	if err != nil {
		t.Fatalf("dispense with notes failed: %v", err)
	}
	if res.RemainingQuantity != 8 {
		t.Fatalf("expected remaining 8, got %d", res.RemainingQuantity)
	}

	// 6. Cancelled prescription -> ErrPrescriptionNotActive
	issued2, err := qrSvc.IssueToken(ctx, itemID, seed.PatientUserID)
	if err != nil {
		t.Fatalf("IssueToken #2: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE prescriptions SET status = 'cancelled' WHERE id = $1`, rxID); err != nil {
		t.Fatalf("cancel prescription: %v", err)
	}
	if _, err := pharmSvc.Dispense(ctx, pharmacist, signDispenseReq(t, issued2.Token, pharmacist.IIN, brandID, 1)); !errors.Is(err, pharmacy.ErrPrescriptionNotActive) {
		t.Fatalf("expected ErrPrescriptionNotActive for cancelled prescription, got %v", err)
	}

	// 7. Already dispensed item -> ErrItemAlreadyDispensed
	raw3 := "erx_already_dispensed_item_token"
	if _, err := qrRepo.InsertToken(ctx, itemID, qr.HashToken(raw3), "nonce3", time.Now().UTC().Add(time.Minute)); err != nil {
		t.Fatalf("insert token #3: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE prescriptions SET status = 'active' WHERE id = $1`, rxID); err != nil {
		t.Fatalf("reactivate prescription: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE prescription_items SET status = 'dispensed' WHERE id = $1`, itemID); err != nil {
		t.Fatalf("mark item dispensed: %v", err)
	}
	if _, err := pharmSvc.Dispense(ctx, pharmacist, signDispenseReq(t, raw3, pharmacist.IIN, brandID, 1)); !errors.Is(err, pharmacy.ErrItemAlreadyDispensed) {
		t.Fatalf("expected ErrItemAlreadyDispensed, got %v", err)
	}
}
