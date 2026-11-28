-- 000001_init_schema.down.sql
-- Clean reverse drop order for all 24 tables and 6 ENUMs.

DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS dispensing_events;
DROP TABLE IF EXISTS signatures;
DROP TABLE IF EXISTS qr_tokens;
DROP TABLE IF EXISTS prescription_items;
DROP TABLE IF EXISTS prescriptions;
DROP TABLE IF EXISTS drug_interactions;
DROP TABLE IF EXISTS patient_allergies;
DROP TABLE IF EXISTS icd10_codes;
DROP TABLE IF EXISTS medicine_brands;
DROP TABLE IF EXISTS medicine_forms;
DROP TABLE IF EXISTS medicines;
DROP TABLE IF EXISTS patient_profiles;
DROP TABLE IF EXISTS pharmacist_profiles;
DROP TABLE IF EXISTS doctor_clinics;
DROP TABLE IF EXISTS doctor_profiles;
DROP TABLE IF EXISTS pharmacy_details;
DROP TABLE IF EXISTS clinic_details;
DROP TABLE IF EXISTS organizations;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS roles;
DROP TABLE IF EXISTS users;

DROP TYPE IF EXISTS notification_type_enum;
DROP TYPE IF EXISTS signature_entity_enum;
DROP TYPE IF EXISTS ddi_severity_enum;
DROP TYPE IF EXISTS prescription_item_status_enum;
DROP TYPE IF EXISTS prescription_status_enum;
DROP TYPE IF EXISTS org_type_enum;
