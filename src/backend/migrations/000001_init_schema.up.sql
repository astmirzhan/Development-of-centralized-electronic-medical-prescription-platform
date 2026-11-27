-- 000001_init_schema.up.sql
-- Source of truth: diagrams/src/04-erd.puml (6 ENUMs, 24 tables)

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- ==========================================
-- 0. ENUMS
-- ==========================================

CREATE TYPE org_type_enum AS ENUM (
    'clinic',
    'pharmacy'
);

CREATE TYPE prescription_status_enum AS ENUM (
    'draft',
    'active',
    'partially_dispensed',
    'fully_dispensed',
    'expired',
    'cancelled'
);

CREATE TYPE prescription_item_status_enum AS ENUM (
    'pending',
    'partially_dispensed',
    'dispensed',
    'cancelled'
);

CREATE TYPE ddi_severity_enum AS ENUM (
    'minor',
    'moderate',
    'critical'
);

CREATE TYPE signature_entity_enum AS ENUM (
    'prescription',
    'dispensing',
    'cancellation'
);

CREATE TYPE notification_type_enum AS ENUM (
    'sms',
    'push',
    'email'
);

-- ==========================================
-- 1. USERS & SECURITY
-- ==========================================

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    iin VARCHAR(12) NOT NULL UNIQUE,
    phone VARCHAR(20) NOT NULL UNIQUE,
    email VARCHAR(255) UNIQUE,
    password_hash VARCHAR(255),
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    middle_name VARCHAR(100),
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE roles (
    id SERIAL PRIMARY KEY,
    name VARCHAR(50) NOT NULL UNIQUE,
    description TEXT
);

CREATE TABLE user_roles (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id INT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    is_verified BOOLEAN NOT NULL DEFAULT false,
    verified_at TIMESTAMPTZ,
    verified_by UUID REFERENCES users(id),
    PRIMARY KEY (user_id, role_id)
);

CREATE TABLE refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL,
    device_info VARCHAR(255),
    ip_address VARCHAR(45),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ==========================================
-- 2. ORGANIZATIONS & PROFILES
-- ==========================================

CREATE TABLE organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bin VARCHAR(12) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    org_type org_type_enum NOT NULL,
    address TEXT NOT NULL,
    phone VARCHAR(20),
    license_number VARCHAR(100),
    is_verified BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE clinic_details (
    organization_id UUID PRIMARY KEY REFERENCES organizations(id) ON DELETE CASCADE,
    accreditation_category VARCHAR(100),
    accreditation_valid_until DATE
);

CREATE TABLE pharmacy_details (
    organization_id UUID PRIMARY KEY REFERENCES organizations(id) ON DELETE CASCADE,
    has_narcotics_license BOOLEAN NOT NULL DEFAULT false,
    narcotics_license_number VARCHAR(100),
    narcotics_license_valid_until DATE
);

CREATE TABLE doctor_profiles (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    license_number VARCHAR(100) NOT NULL UNIQUE,
    specialization VARCHAR(150) NOT NULL,
    eds_cert_dn VARCHAR(500),
    is_verified BOOLEAN NOT NULL DEFAULT false
);

CREATE TABLE doctor_clinics (
    doctor_id UUID NOT NULL REFERENCES doctor_profiles(user_id) ON DELETE CASCADE,
    clinic_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    is_primary BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (doctor_id, clinic_id)
);

CREATE TABLE pharmacist_profiles (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    pharmacy_id UUID NOT NULL REFERENCES organizations(id),
    specialist_cert_number VARCHAR(100) NOT NULL,
    is_verified BOOLEAN NOT NULL DEFAULT false
);

CREATE TABLE patient_profiles (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    date_of_birth DATE NOT NULL,
    gender VARCHAR(20) NOT NULL,
    blood_group VARCHAR(10),
    emergency_contact VARCHAR(100)
);

-- ==========================================
-- 3. MEDICINES & CLINICAL SAFETY
-- ==========================================

CREATE TABLE medicines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    inn_name_ru VARCHAR(255) NOT NULL,
    inn_name_kz VARCHAR(255) NOT NULL,
    inn_name_en VARCHAR(255) NOT NULL,
    atc_code VARCHAR(20),
    is_prescription_required BOOLEAN NOT NULL DEFAULT true,
    is_antibiotic BOOLEAN NOT NULL DEFAULT false,
    is_narcotic BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE medicine_forms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    medicine_id UUID NOT NULL REFERENCES medicines(id) ON DELETE CASCADE,
    form_type VARCHAR(100) NOT NULL,
    dosage_value DECIMAL(10,2) NOT NULL,
    dosage_unit VARCHAR(50) NOT NULL,
    package_quantity INT NOT NULL
);

CREATE TABLE medicine_brands (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    medicine_form_id UUID NOT NULL REFERENCES medicine_forms(id) ON DELETE CASCADE,
    trade_name VARCHAR(255) NOT NULL,
    manufacturer VARCHAR(255) NOT NULL,
    country VARCHAR(100) NOT NULL,
    reg_number VARCHAR(100) NOT NULL
);

CREATE TABLE icd10_codes (
    id SERIAL PRIMARY KEY,
    code VARCHAR(10) NOT NULL UNIQUE,
    name_ru VARCHAR(500) NOT NULL,
    name_kz VARCHAR(500) NOT NULL
);

CREATE TABLE patient_allergies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id UUID NOT NULL REFERENCES patient_profiles(user_id) ON DELETE CASCADE,
    medicine_id UUID NOT NULL REFERENCES medicines(id) ON DELETE CASCADE,
    reaction_description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_patient_allergies_patient_medicine UNIQUE (patient_id, medicine_id)
);

CREATE TABLE drug_interactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    medicine_a_id UUID NOT NULL REFERENCES medicines(id) ON DELETE CASCADE,
    medicine_b_id UUID NOT NULL REFERENCES medicines(id) ON DELETE CASCADE,
    severity_level ddi_severity_enum NOT NULL,
    description TEXT,
    CONSTRAINT chk_drug_interactions_diff_medicines CHECK (medicine_a_id <> medicine_b_id)
);

CREATE UNIQUE INDEX uq_drug_interactions_pair
    ON drug_interactions (LEAST(medicine_a_id, medicine_b_id), GREATEST(medicine_a_id, medicine_b_id));

-- ==========================================
-- 4. PRESCRIPTIONS & QR
-- ==========================================

CREATE TABLE prescriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    prescription_number VARCHAR(50) NOT NULL UNIQUE,
    doctor_id UUID NOT NULL REFERENCES doctor_profiles(user_id),
    clinic_id UUID NOT NULL REFERENCES organizations(id),
    patient_id UUID NOT NULL REFERENCES patient_profiles(user_id),
    diagnosis_id INT NOT NULL REFERENCES icd10_codes(id),
    status prescription_status_enum NOT NULL DEFAULT 'draft',
    valid_until TIMESTAMPTZ NOT NULL,
    notes TEXT,
    cancellation_reason TEXT,
    cancelled_by UUID REFERENCES users(id),
    cancelled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE prescription_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    prescription_id UUID NOT NULL REFERENCES prescriptions(id) ON DELETE CASCADE,
    medicine_form_id UUID NOT NULL REFERENCES medicine_forms(id),
    dosage_value DECIMAL(10,2) NOT NULL,
    dosage_unit VARCHAR(50) NOT NULL,
    frequency VARCHAR(100) NOT NULL,
    duration_days INT NOT NULL,
    quantity_prescribed INT NOT NULL,
    quantity_dispensed INT NOT NULL DEFAULT 0,
    status prescription_item_status_enum NOT NULL DEFAULT 'pending',
    instruction_text TEXT,
    CONSTRAINT chk_prescription_items_qty_prescribed CHECK (quantity_prescribed > 0),
    CONSTRAINT chk_prescription_items_qty_dispensed_nonneg CHECK (quantity_dispensed >= 0),
    CONSTRAINT chk_prescription_items_qty_dispensed_le_prescribed CHECK (quantity_dispensed <= quantity_prescribed)
);

CREATE TABLE qr_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    prescription_item_id UUID NOT NULL REFERENCES prescription_items(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL UNIQUE,
    nonce VARCHAR(64) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    is_consumed BOOLEAN NOT NULL DEFAULT false,
    consumed_by_user_id UUID REFERENCES users(id),
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ==========================================
-- 5. DISPENSING & SIGNATURES
-- ==========================================

CREATE TABLE signatures (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_type signature_entity_enum NOT NULL,
    entity_id UUID NOT NULL,
    signer_user_id UUID NOT NULL REFERENCES users(id),
    signer_iin VARCHAR(12) NOT NULL,
    cert_serial_number VARCHAR(100) NOT NULL,
    raw_signature TEXT NOT NULL,
    signature_hash VARCHAR(64) NOT NULL,
    signed_at TIMESTAMPTZ NOT NULL,
    ocsp_status VARCHAR(50) NOT NULL,
    verification_status VARCHAR(50) NOT NULL
);

CREATE TABLE dispensing_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    prescription_item_id UUID NOT NULL REFERENCES prescription_items(id),
    pharmacist_id UUID NOT NULL REFERENCES pharmacist_profiles(user_id),
    pharmacy_id UUID NOT NULL REFERENCES organizations(id),
    medicine_brand_id UUID NOT NULL REFERENCES medicine_brands(id),
    quantity_dispensed INT NOT NULL,
    dispensed_at TIMESTAMPTZ NOT NULL,
    notes TEXT,
    signature_id UUID REFERENCES signatures(id),
    CONSTRAINT chk_dispensing_events_qty_pos CHECK (quantity_dispensed > 0)
);

-- ==========================================
-- 6. AUDIT & NOTIFICATIONS
-- ==========================================

CREATE TABLE audit_logs (
    id BIGSERIAL PRIMARY KEY,
    prev_record_hash VARCHAR(64),
    record_hash VARCHAR(64) NOT NULL,
    actor_id UUID REFERENCES users(id),
    action VARCHAR(100) NOT NULL,
    entity_type VARCHAR(100) NOT NULL,
    entity_id UUID,
    payload JSONB,
    ip_address VARCHAR(45),
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type notification_type_enum NOT NULL,
    recipient VARCHAR(255) NOT NULL,
    message TEXT NOT NULL,
    status VARCHAR(50) NOT NULL,
    sent_at TIMESTAMPTZ
);

-- ==========================================
-- 7. PERFORMANCE INDEXES
-- ==========================================

CREATE INDEX idx_prescriptions_patient_status
    ON prescriptions (patient_id, status);

CREATE INDEX idx_prescriptions_doctor_created_at
    ON prescriptions (doctor_id, created_at DESC);

CREATE INDEX idx_prescription_items_prescription_id
    ON prescription_items (prescription_id);

CREATE INDEX idx_qr_tokens_active_item
    ON qr_tokens (prescription_item_id)
    WHERE is_consumed = false;

CREATE INDEX idx_drug_interactions_med_a
    ON drug_interactions (medicine_a_id);

CREATE INDEX idx_drug_interactions_med_b
    ON drug_interactions (medicine_b_id);

CREATE INDEX idx_patient_allergies_patient_id
    ON patient_allergies (patient_id);

CREATE INDEX idx_audit_logs_entity
    ON audit_logs (entity_type, entity_id);

CREATE INDEX idx_audit_logs_created_at
    ON audit_logs (created_at DESC);
