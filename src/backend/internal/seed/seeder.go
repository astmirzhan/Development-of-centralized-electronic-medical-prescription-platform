// Package seed implements an idempotent database seeder for synthetic demo data.
package seed

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xjam-byte/erx-platform/backend/internal/database"
	seeddata "github.com/xjam-byte/erx-platform/backend/seed"
)

// Deterministic UUIDs for synthetic demo entities (safe for repeated runs and tests).
var (
	ClinicOrgID   = uuid.MustParse("00000000-0000-0000-0000-000000000101")
	PharmacyOrgID = uuid.MustParse("00000000-0000-0000-0000-000000000102")

	AdminUserID      = uuid.MustParse("00000000-0000-0000-0000-000000000201")
	DoctorUserID     = uuid.MustParse("00000000-0000-0000-0000-000000000202")
	PharmacistUserID = uuid.MustParse("00000000-0000-0000-0000-000000000203")
	PatientUserID    = uuid.MustParse("00000000-0000-0000-0000-000000000204")
	ChildUserID      = uuid.MustParse("00000000-0000-0000-0000-000000000205")

	AmoxicillinID = uuid.MustParse("10000000-0000-0000-0000-000000000001")
	CeftriaxoneID = uuid.MustParse("10000000-0000-0000-0000-000000000010")
)

type icd10Seed struct {
	Code   string `json:"code"`
	NameRU string `json:"name_ru"`
	NameKZ string `json:"name_kz"`
}

type brandSeed struct {
	ID           string `json:"id"`
	TradeName    string `json:"trade_name"`
	Manufacturer string `json:"manufacturer"`
	Country      string `json:"country"`
	RegNumber    string `json:"reg_number"`
}

type formSeed struct {
	ID              string      `json:"id"`
	FormType        string      `json:"form_type"`
	DosageValue     float64     `json:"dosage_value"`
	DosageUnit      string      `json:"dosage_unit"`
	PackageQuantity int         `json:"package_quantity"`
	Brands          []brandSeed `json:"brands"`
}

type medicineSeed struct {
	ID                     string     `json:"id"`
	INNNameRU              string     `json:"inn_name_ru"`
	INNNameKZ              string     `json:"inn_name_kz"`
	INNNameEN              string     `json:"inn_name_en"`
	ATCCode                string     `json:"atc_code"`
	IsPrescriptionRequired bool       `json:"is_prescription_required"`
	IsAntibiotic           bool       `json:"is_antibiotic"`
	IsNarcotic             bool       `json:"is_narcotic"`
	Forms                  []formSeed `json:"forms"`
}

type ddiSeed struct {
	ID            string `json:"id"`
	MedicineAID   string `json:"medicine_a_id"`
	MedicineBID   string `json:"medicine_b_id"`
	SeverityLevel string `json:"severity_level"`
	Description   string `json:"description"`
}

// Run populates roles, organizations, demo users, ICD-10 codes, medicines, DDI, and allergies idempotently.
func Run(ctx context.Context, pool *pgxpool.Pool) error {
	icdCodes, err := loadJSON[[]icd10Seed]("data/icd10_codes.json")
	if err != nil {
		return fmt.Errorf("load icd10_codes.json: %w", err)
	}
	medicines, err := loadJSON[[]medicineSeed]("data/medicines.json")
	if err != nil {
		return fmt.Errorf("load medicines.json: %w", err)
	}
	ddis, err := loadJSON[[]ddiSeed]("data/drug_interactions.json")
	if err != nil {
		return fmt.Errorf("load drug_interactions.json: %w", err)
	}

	return database.WithTx(ctx, pool, func(tx pgx.Tx) error {
		if err := seedRoles(ctx, tx); err != nil {
			return err
		}
		if err := seedOrganizations(ctx, tx); err != nil {
			return err
		}
		if err := seedDemoUsers(ctx, tx); err != nil {
			return err
		}
		if err := seedICD10(ctx, tx, icdCodes); err != nil {
			return err
		}
		if err := seedMedicines(ctx, tx, medicines); err != nil {
			return err
		}
		if err := seedDDIs(ctx, tx, ddis); err != nil {
			return err
		}
		if err := seedAllergies(ctx, tx); err != nil {
			return err
		}
		return nil
	})
}

func loadJSON[T any](path string) (T, error) {
	var out T
	raw, err := seeddata.DataFS.ReadFile(path)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, err
	}
	return out, nil
}

func seedRoles(ctx context.Context, tx pgx.Tx) error {
	roles := []struct {
		name string
		desc string
	}{
		{"doctor", "Licensed physician authorized to prescribe medications"},
		{"pharmacist", "Licensed pharmacist authorized to verify and dispense prescriptions"},
		{"patient", "Citizen viewing active prescriptions and generating QR tokens"},
		{"admin", "Platform administrator managing organizations, catalogs, and audit logs"},
	}
	for _, r := range roles {
		if _, err := tx.Exec(ctx, `
			INSERT INTO roles (name, description)
			VALUES ($1, $2)
			ON CONFLICT (name) DO UPDATE SET description = EXCLUDED.description
		`, r.name, r.desc); err != nil {
			return fmt.Errorf("seed role %s: %w", r.name, err)
		}
	}
	return nil
}

func seedOrganizations(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO organizations (id, bin, name, org_type, address, phone, license_number, is_verified)
		VALUES
			($1, '240140000101', 'Astana Central Outpatient Clinic (Demo)', 'clinic', 'Astana, Mangilik El Ave 10', '+77172000101', 'CL-KZ-000101', true),
			($2, '240240000202', 'Europharma Demo Pharmacy #1', 'pharmacy', 'Astana, Dostyk St 18', '+77172000202', 'PH-KZ-000202', true)
		ON CONFLICT (bin) DO UPDATE SET
			name = EXCLUDED.name,
			is_verified = EXCLUDED.is_verified
	`, ClinicOrgID, PharmacyOrgID); err != nil {
		return fmt.Errorf("seed organizations: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO clinic_details (organization_id, accreditation_category, accreditation_valid_until)
		VALUES ($1, 'Highest Category', '2028-12-31')
		ON CONFLICT (organization_id) DO NOTHING
	`, ClinicOrgID); err != nil {
		return fmt.Errorf("seed clinic_details: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO pharmacy_details (organization_id, has_narcotics_license, narcotics_license_number, narcotics_license_valid_until)
		VALUES ($1, false, NULL, NULL)
		ON CONFLICT (organization_id) DO NOTHING
	`, PharmacyOrgID); err != nil {
		return fmt.Errorf("seed pharmacy_details: %w", err)
	}
	return nil
}

func seedDemoUsers(ctx context.Context, tx pgx.Tx) error {
	// Demo password hash placeholder for synthetic admin account only (bcrypt-formatted test placeholder).
	const demoAdminHash = "$2a$10$7EqJtq98hPqEX7fNZaFWoOhi5E0kD9W3zS9w9r2g8r7F3L6y1z2aK"

	users := []struct {
		id         uuid.UUID
		iin        string
		phone      string
		email      string
		pwHash     *string
		firstName  string
		lastName   string
		middleName string
		role       string
	}{
		{AdminUserID, "790510300182", "+77010000001", "admin@demo.erx.kz", strPtr(demoAdminHash), "Marat", "Nurlanov", "Serikovich", "admin"},
		{DoctorUserID, "840315301294", "+77010000002", "doctor@demo.erx.kz", nil, "Aibek", "Kassymov", "Tolegenovich", "doctor"},
		{PharmacistUserID, "920721402851", "+77010000003", "pharmacist@demo.erx.kz", nil, "Dana", "Akhmetova", "Bolatovna", "pharmacist"},
		{PatientUserID, "981105303419", "+77010000004", "patient@demo.erx.kz", nil, "Yerkebulan", "Sultanov", "Maratovich", "patient"},
		{ChildUserID, "150510500112", "+77010000005", "child.patient@demo.erx.kz", nil, "Alikhan", "Serikov", "Daurenovich", "patient"},
	}

	for _, u := range users {
		if _, err := tx.Exec(ctx, `
			INSERT INTO users (id, iin, phone, email, password_hash, first_name, last_name, middle_name, is_active)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, true)
			ON CONFLICT (iin) DO UPDATE SET
				first_name = EXCLUDED.first_name,
				last_name = EXCLUDED.last_name,
				is_active = true
		`, u.id, u.iin, u.phone, u.email, u.pwHash, u.firstName, u.lastName, u.middleName); err != nil {
			return fmt.Errorf("seed user %s: %w", u.iin, err)
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO user_roles (user_id, role_id, is_verified, verified_at, verified_by)
			SELECT u.id, r.id, true, NOW(), $2
			FROM users u, roles r
			WHERE u.iin = $1 AND r.name = $3
			ON CONFLICT (user_id, role_id) DO UPDATE SET is_verified = true
		`, u.iin, AdminUserID, u.role); err != nil {
			return fmt.Errorf("seed user_role %s/%s: %w", u.iin, u.role, err)
		}
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO doctor_profiles (user_id, license_number, specialization, eds_cert_dn, is_verified)
		SELECT id, 'DOC-LIC-840315', 'General Practitioner', 'CN=KASSYMOV AIBEK,SERIALNUMBER=IIN840315301294', true
		FROM users WHERE iin = '840315301294'
		ON CONFLICT (user_id) DO UPDATE SET is_verified = true
	`); err != nil {
		return fmt.Errorf("seed doctor_profile: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO doctor_clinics (doctor_id, clinic_id, is_primary)
		SELECT id, $1, true FROM users WHERE iin = '840315301294'
		ON CONFLICT (doctor_id, clinic_id) DO NOTHING
	`, ClinicOrgID); err != nil {
		return fmt.Errorf("seed doctor_clinics: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO pharmacist_profiles (user_id, pharmacy_id, specialist_cert_number, is_verified)
		SELECT id, $1, 'PHARM-CERT-920721', true
		FROM users WHERE iin = '920721402851'
		ON CONFLICT (user_id) DO UPDATE SET is_verified = true
	`, PharmacyOrgID); err != nil {
		return fmt.Errorf("seed pharmacist_profile: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO patient_profiles (user_id, date_of_birth, gender, blood_group, emergency_contact)
		SELECT id, '1998-11-05', 'male', 'O(I) Rh+', '+77019990001'
		FROM users WHERE iin = '981105303419'
		ON CONFLICT (user_id) DO NOTHING
	`); err != nil {
		return fmt.Errorf("seed adult patient_profile: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO patient_profiles (user_id, date_of_birth, gender, blood_group, emergency_contact)
		SELECT id, '2015-05-10', 'male', 'A(II) Rh+', '+77019990002'
		FROM users WHERE iin = '150510500112'
		ON CONFLICT (user_id) DO NOTHING
	`); err != nil {
		return fmt.Errorf("seed pediatric patient_profile: %w", err)
	}

	return nil
}

func seedICD10(ctx context.Context, tx pgx.Tx, codes []icd10Seed) error {
	for _, c := range codes {
		if _, err := tx.Exec(ctx, `
			INSERT INTO icd10_codes (code, name_ru, name_kz)
			VALUES ($1, $2, $3)
			ON CONFLICT (code) DO UPDATE SET
				name_ru = EXCLUDED.name_ru,
				name_kz = EXCLUDED.name_kz
		`, c.Code, c.NameRU, c.NameKZ); err != nil {
			return fmt.Errorf("seed icd10 %s: %w", c.Code, err)
		}
	}
	return nil
}

func seedMedicines(ctx context.Context, tx pgx.Tx, medicines []medicineSeed) error {
	for _, m := range medicines {
		medID, err := uuid.Parse(m.ID)
		if err != nil {
			return fmt.Errorf("invalid medicine id %s: %w", m.ID, err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO medicines (id, inn_name_ru, inn_name_kz, inn_name_en, atc_code, is_prescription_required, is_antibiotic, is_narcotic)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (id) DO UPDATE SET
				inn_name_ru = EXCLUDED.inn_name_ru,
				inn_name_kz = EXCLUDED.inn_name_kz,
				inn_name_en = EXCLUDED.inn_name_en,
				atc_code = EXCLUDED.atc_code,
				is_antibiotic = EXCLUDED.is_antibiotic
		`, medID, m.INNNameRU, m.INNNameKZ, m.INNNameEN, m.ATCCode, m.IsPrescriptionRequired, m.IsAntibiotic, m.IsNarcotic); err != nil {
			return fmt.Errorf("seed medicine %s: %w", m.INNNameEN, err)
		}

		for _, f := range m.Forms {
			formID, err := uuid.Parse(f.ID)
			if err != nil {
				return fmt.Errorf("invalid form id %s: %w", f.ID, err)
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO medicine_forms (id, medicine_id, form_type, dosage_value, dosage_unit, package_quantity)
				VALUES ($1, $2, $3, $4, $5, $6)
				ON CONFLICT (id) DO UPDATE SET
					form_type = EXCLUDED.form_type,
					dosage_value = EXCLUDED.dosage_value,
					dosage_unit = EXCLUDED.dosage_unit,
					package_quantity = EXCLUDED.package_quantity
			`, formID, medID, f.FormType, f.DosageValue, f.DosageUnit, f.PackageQuantity); err != nil {
				return fmt.Errorf("seed medicine_form %s: %w", f.ID, err)
			}

			for _, b := range f.Brands {
				brandID, err := uuid.Parse(b.ID)
				if err != nil {
					return fmt.Errorf("invalid brand id %s: %w", b.ID, err)
				}
				if _, err := tx.Exec(ctx, `
					INSERT INTO medicine_brands (id, medicine_form_id, trade_name, manufacturer, country, reg_number)
					VALUES ($1, $2, $3, $4, $5, $6)
					ON CONFLICT (id) DO UPDATE SET
						trade_name = EXCLUDED.trade_name,
						manufacturer = EXCLUDED.manufacturer
				`, brandID, formID, b.TradeName, b.Manufacturer, b.Country, b.RegNumber); err != nil {
					return fmt.Errorf("seed medicine_brand %s: %w", b.TradeName, err)
				}
			}
		}
	}
	return nil
}

func seedDDIs(ctx context.Context, tx pgx.Tx, ddis []ddiSeed) error {
	for _, d := range ddis {
		id, err := uuid.Parse(d.ID)
		if err != nil {
			return fmt.Errorf("invalid ddi id %s: %w", d.ID, err)
		}
		medA, err := uuid.Parse(d.MedicineAID)
		if err != nil {
			return fmt.Errorf("invalid medicine_a_id %s: %w", d.MedicineAID, err)
		}
		medB, err := uuid.Parse(d.MedicineBID)
		if err != nil {
			return fmt.Errorf("invalid medicine_b_id %s: %w", d.MedicineBID, err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO drug_interactions (id, medicine_a_id, medicine_b_id, severity_level, description)
			VALUES ($1, $2, $3, $4::ddi_severity_enum, $5)
			ON CONFLICT (LEAST(medicine_a_id, medicine_b_id), GREATEST(medicine_a_id, medicine_b_id))
			DO UPDATE SET
				severity_level = EXCLUDED.severity_level,
				description = EXCLUDED.description
		`, id, medA, medB, d.SeverityLevel, d.Description); err != nil {
			return fmt.Errorf("seed ddi %s: %w", d.ID, err)
		}
	}
	return nil
}

func seedAllergies(ctx context.Context, tx pgx.Tx) error {
	allergies := []struct {
		medID    uuid.UUID
		reaction string
	}{
		{AmoxicillinID, "Acute urticaria and angioedema to beta-lactam penicillins"},
		{CeftriaxoneID, "Cross-reactive rash with beta-lactam cephalosporins"},
	}

	for _, a := range allergies {
		if _, err := tx.Exec(ctx, `
			INSERT INTO patient_allergies (patient_id, medicine_id, reaction_description)
			SELECT u.id, $1, $2
			FROM users u
			WHERE u.iin = '981105303419'
			ON CONFLICT (patient_id, medicine_id) DO UPDATE SET
				reaction_description = EXCLUDED.reaction_description
		`, a.medID, a.reaction); err != nil {
			return fmt.Errorf("seed patient_allergy: %w", err)
		}
	}
	return nil
}

func strPtr(s string) *string {
	return &s
}
