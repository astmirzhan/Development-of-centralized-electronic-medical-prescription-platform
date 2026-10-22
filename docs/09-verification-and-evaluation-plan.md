# 09. Verification and Evaluation Plan

**Project Title:** Development of a Centralized Electronic Medical Prescription Platform  
**Document Version:** 1.3.0  
**Scope:** Verification of functional/non-functional requirements, end-to-end (E2E) clinical workflow testing, outcome metrics, measurement conditions, and comparative evaluation against the baseline process.

---

## 1. Baseline Solution for Comparison

To objectively evaluate the engineering and clinical value of the proposed platform, system performance and workflow metrics are benchmarked against the **prevailing baseline process in outpatient and retail pharmacy settings**:
1. **Baseline Workflow (Paper-Based & Fragmented Clinic MIS):**
   * **Issuance:** A physician manually writes or prints a paper prescription form (or enters it into a clinic-locked MIS that is inaccessible to external private retail pharmacies) and stamps it physically.
   * **Presentation:** The patient carries the physical paper slip to a retail pharmacy.
   * **Verification & Fulfillment:** The pharmacist visually inspects the handwriting, dosage, physician stamp, and expiration date. Because private pharmacy chains lack a shared, real-time prescription fulfillment registry, the paper slip is frequently returned to the patient without technical invalidation, creating a critical vulnerability for **duplicate dispensing** of prescription-only medications (e.g., antibiotics) across multiple pharmacies.

---

## 2. Multi-Tier Testing Plan

Verification is structured across three testing tiers to ensure complete traceability to the Software Requirements Specification (`docs/06-requirements-specification.md`).

### 2.1. Unit and Component Testing (Frontend Layer)
* **Form Validation & Input Constraints:**
  * Verify that the 12-digit IIN input mask rejects non-numeric characters and incomplete strings (`< 12` digits) across Login, Doctor Search, and `NewUserModal`.
  * Verify that the prescription issuance form (`FR-DOC-02`) blocks submission when the dosage field or INN selection is empty.
* **Conditional State Rendering:**
  * Verify that the Patient Portal (`FR-PAT-02`) renders the 2D QR code and 8-character fallback code (`KZ-89F2A4`) **exclusively** when `status === 'ACTIVE'`, and hides the QR component when `status` transitions to `'DISPENSED'`, `'EXPIRED'`, or `'CANCELLED'`.
  * Verify that the Doctor Portal disables (`disabled`) the prescription revoke action (`FR-DOC-03`) and the Pharmacist Portal disables the dispense button (`FR-PHARM-02`) whenever `status === 'DISPENSED'`.

### 2.2. API Integration and Security Guard Testing
* **RBAC Route Interception (`FR-AUTH-01`):** Automated route tests verifying that unauthorized role navigation (e.g., `PATIENT` session accessing `#/doctor/dashboard` or `#/admin/dashboard`) triggers an immediate redirect to `#/403` and appends a security violation entry to the audit log.
* **Idempotent Dispensing Contract (`FR-PHARM-02`):** Verify that sending a duplicate `PATCH /api/v1/prescriptions/{id}/dispense` request for an already fulfilled prescription returns `HTTP 409 Conflict` and triggers the UI warning banner *"Prescription already dispensed"*.

### 2.3. End-to-End (E2E) Acceptance Test Suite

The following E2E test cases validate the complete prescription lifecycle using the pre-configured verification accounts:

| Test ID | Mapped Req. | Actor (Demo Profile) | Test Scenario & Execution Steps | Expected Verifiable Outcome |
| :--- | :--- | :--- | :--- | :--- |
| **`TC-E2E-01`** | `FR-ADM-01`, `FR-ADM-02` | **ADMIN** (`790510300182`) | 1. Log in to `#/admin/dashboard`.<br>2. Add a new INN entry via `MnnModal`.<br>3. Verify active license status for Doctor (`840315301294`) and Pharmacist (`920721402851`). | New INN medication is immediately persisted in the registry; audit log records administrative action with actor IIN and timestamp. |
| **`TC-E2E-02`** | `FR-DOC-01`, `FR-DOC-02` | **DOCTOR** (`840315301294`) | 1. Log in to `#/doctor/dashboard`.<br>2. Query patient IIN `981105303419` (Sultanov Y.M.).<br>3. Select INN from autocomplete, enter dosage/regimen, and submit. | Prescription is created in `< 1.0s` with status `ACTIVE`, unique ID (`RX-2024-09812`), and 8-char fallback code (`KZ-89F2A4`). Audit log records `prescription:create`. |
| **`TC-E2E-03`** | `FR-PAT-01`, `FR-PAT-02` | **PATIENT** (`981105303419`) | 1. Log in to `#/patient/dashboard` on Mobile viewport (`390×844`).<br>2. Open the newly issued prescription under the "Active" tab. | Prescription card displays complete dosage instructions, a scannable 2D QR code, and fallback code `KZ-89F2A4`. |
| **`TC-E2E-04`** | `FR-PHARM-01`, `FR-PHARM-02` | **PHARMACIST** (`920721402851`) | 1. Log in to `#/pharmacist/dashboard`.<br>2. Scan the QR code or enter `KZ-89F2A4`.<br>3. Verify details and click "Dispense Medication". | Prescription status transitions to `DISPENSED`. Audit log records `prescription:dispense` with pharmacist IIN. Patient view moves item to "History" and hides QR code. |
| **`TC-E2E-05`** | `FR-PHARM-02`, `FR-AUTH-01` | **PHARMACIST** & **PATIENT** | 1. Pharmacist re-scans `KZ-89F2A4` after fulfillment.<br>2. Patient attempts manual navigation to `#/doctor/dashboard`. | **1.** Pharmacist UI displays *"Prescription already dispensed"* and disables the dispense button.<br>**2.** Patient is blocked by `#/403` Forbidden screen and logged in the Audit Trail. |

---

## 3. Outcome Metrics and Baseline Comparison

The achievement of the project aim will be demonstrated quantitatively using the following outcome metrics:

| Evaluation Metric | Baseline Solution (Paper / Isolated MIS) | Target Platform Metric | Verification Method |
| :--- | :--- | :--- | :--- |
| **1. Prescription Issuance Time** (Doctor Time-on-Task) | `120 – 180 sec` (manual handwriting/printing & stamping) | **`<= 45 sec`** (INN autocomplete & structured form) | Timed execution across 5 standardized clinical prescription scenarios (`TC-E2E-02`). |
| **2. Pharmacy Verification & Dispensing Time** | `60 – 90 sec` (deciphering handwriting & manual stamp check) | **`<= 15 sec`** (2D QR scan / 8-char code lookup + 1-click dispense) | Timed measurement from patient QR presentation to `DISPENSED` status confirmation (`TC-E2E-04`). |
| **3. Duplicate Dispensing Prevention Rate** | `0%` technical cross-pharmacy enforcement | **`100%`** technical block (`409 Conflict` & disabled UI) | Automated negative testing of repeated fulfillment attempts (`TC-E2E-05`). |
| **4. Unreadable / Incomplete Prescription Error Rate** | `12% – 18%` (missing dosage or illegible handwriting) | **`0%`** (enforced client-side & schema validation) | Attempted submission of incomplete forms during component/integration testing (`FR-DOC-02`). |
| **5. System Usability Scale (SUS) Score** | N/A (Reference industry threshold: `68.0`) | **`>= 80.0 / 100`** (Grade A usability) | Post-task SUS questionnaire completed by test evaluators across all role portals. |

---

## 4. Measurement Conditions and Evaluation Environment

To ensure reproducibility during pre-defense and final defense demonstrations, all measurements and verification procedures are conducted under standardized conditions:
1. **Hardware & Viewport Profiles:**
   * **Clinical & Pharmacy Workstations:** Desktop browser environment (Chrome / Safari on macOS/Windows) at `1440×900px` resolution for Doctor, Pharmacist, and Admin portals.
   * **Citizen Device:** Mobile viewport emulation and mobile browser testing at `390×844px` (iPhone profile) for the Patient portal and QR rendering.
2. **Dataset & Network Conditions:**
   * Pre-seeded database containing standardized INN medication records (ATC codes, dosage forms), 4 role-specific test accounts, and historical prescription records across all 4 lifecycle states (`ACTIVE`, `DISPENSED`, `EXPIRED`, `CANCELLED`).
   * Standard local/cloud network environment with API response latency measured via browser DevTools Network profiling (`< 1000ms` threshold per transaction).
3. **Evaluation Protocol:**
   * Each evaluator executes the 5 sequential end-to-end tasks (`TC-E2E-01` through `TC-E2E-05`) without external assistance, recording task completion time, error incidence, and audit trail consistency.