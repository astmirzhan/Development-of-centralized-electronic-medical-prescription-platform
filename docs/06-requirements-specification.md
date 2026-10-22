# 06. Software Requirements Specification (SRS)

**Project Title:** Development of a Centralized Electronic Medical Prescription Platform  
**Document Version:** 1.3.0  
**Scope:** Client-Side Web/Mobile Portals (Doctor, Patient, Pharmacist, Administrator), Role-Based Access Control (RBAC), and RESTful API Integration Contracts.

---

## 1. Prioritization Methodology

Requirements in this specification are prioritized using the **MoSCoW** method to ensure clear engineering focus and verifiable delivery across project milestones:
* **Must Have (`Must`):** Critical requirements mandatory for the core prescription lifecycle (`CREATED` $\rightarrow$ `ACTIVE` $\rightarrow$ `DISPENSED`), security compliance, and system integrity.
* **Should Have (`Should`):** Important operational capabilities (such as prescription revocation and comprehensive audit filtering) that enhance clinical governance and usability.
* **Could Have (`Could`):** Desirable enhancements that do not block primary clinical workflows.

---

## 2. Functional Requirements & Verifiable Acceptance Criteria

### 2.1. Authentication and Role-Based Access Control (RBAC)

| ID | Priority | Requirement Description | Verifiable Acceptance Criteria (Given / When / Then) |
| :--- | :--- | :--- | :--- |
| **`FR-AUTH-01`** | **Must** | **Unified IIN Authentication & JWT Session Routing:** The system shall provide a single authentication gateway accepting a 12-digit Individual Identification Number (IIN) with input masking and a password, initializing a JWT session (Access + Refresh tokens) and routing users to their role-specific workspace (`DOCTOR`, `PHARMACIST`, `PATIENT`, `ADMIN`). | **1.** *Given* an unauthenticated user enters a valid 12-digit IIN and password, *when* the form is submitted, *then* a JWT session is established and the user is redirected to `#/<role>/dashboard` within `< 1.0s`.<br>**2.** *Given* an authenticated user with role `PATIENT`, *when* they attempt to access `#/doctor/dashboard` or any restricted route, *then* the client router intercepts the request, renders the `#/403` Forbidden view, and records a security event in the audit log. |

---

### 2.2. Doctor Portal Requirements

| ID | Priority | Requirement Description | Verifiable Acceptance Criteria (Given / When / Then) |
| :--- | :--- | :--- | :--- |
| **`FR-DOC-01`** | **Must** | **Patient Lookup & Prescription History:** Physicians shall search for patients by their 12-digit IIN and view their active and historical prescriptions along with GOBMP/OSMS attachment status. | *Given* a logged-in `DOCTOR`, *when* a valid 12-digit patient IIN is entered in the search bar, *then* the interface displays the patient's profile and complete chronological list of `ACTIVE`, `DISPENSED`, `EXPIRED`, and `CANCELLED` prescriptions. |
| **`FR-DOC-02`** | **Must** | **Electronic Prescription Issuance:** Physicians shall issue new prescriptions by selecting a medication from the centralized INN autocomplete catalog and specifying dosage, frequency, duration, and validity period. | **1.** *Given* the prescription issuance form is open, *when* the physician leaves the dosage field empty and attempts to submit, *then* client-side validation blocks the request and highlights the missing field.<br>**2.** *Given* all mandatory fields are valid, *when* submitted via `POST /api/v1/prescriptions`, *then* the prescription is created with status `ACTIVE`, assigned a unique ID (e.g., `RX-2024-09812`) and 8-character fallback code (e.g., `KZ-89F2A4`), and rendered in the registry in `< 1.0s`. |
| **`FR-DOC-03`** | **Should** | **Prescription Revocation:** Physicians shall be able to revoke (`CANCELLED`) a previously issued prescription provided it has not yet been dispensed at a pharmacy. | **1.** *Given* a prescription with status `ACTIVE`, *when* the physician triggers the revoke action, *then* its status updates to `CANCELLED` and its QR token is invalidated.<br>**2.** *Given* a prescription with status `DISPENSED`, *then* the revoke button in the Doctor Portal is strictly disabled (`disabled`). |

---

### 2.3. Citizen / Patient Portal Requirements

| ID | Priority | Requirement Description | Verifiable Acceptance Criteria (Given / When / Then) |
| :--- | :--- | :--- | :--- |
| **`FR-PAT-01`** | **Must** | **Segmented Prescription Registry:** Patients shall view their personal prescriptions categorized into "Active" (`ACTIVE`) and "History" (`DISPENSED`, `EXPIRED`, `CANCELLED`) tabs. | *Given* an authenticated `PATIENT`, *when* switching between the "Active" and "History" tabs on `#/patient/dashboard`, *then* the client filters and renders only prescriptions matching the respective lifecycle states without page reloads. |
| **`FR-PAT-02`** | **Must** | **Digital QR Token & Fallback Code Generation:** Patients shall open a detailed prescription card that dynamically renders a 2D QR verification token and an 8-character alphanumeric fallback code for active prescriptions. | **1.** *Given* a prescription has status `ACTIVE`, *when* the patient opens the detail view, *then* a scannable 2D QR code and an 8-character fallback code (e.g., `KZ-89F2A4`) are displayed.<br>**2.** *Given* a prescription has status `DISPENSED`, `EXPIRED`, or `CANCELLED`, *when* viewed by the patient, *then* the QR code is hidden and replaced with an immutable status badge. |

---

### 2.4. Pharmacist Portal Requirements

| ID | Priority | Requirement Description | Verifiable Acceptance Criteria (Given / When / Then) |
| :--- | :--- | :--- | :--- |
| **`FR-PHARM-01`** | **Must** | **Dual-Mode Prescription Verification:** Pharmacists shall retrieve and verify prescriptions via an optical 2D QR scanner viewport or by manually querying the 8-character fallback code, prescription ID, or patient IIN. | *Given* an authenticated `PHARMACIST`, *when* scanning a valid QR code or submitting fallback code `KZ-89F2A4`, *then* the system retrieves and displays the INN medication, dosage, regimen, prescribing doctor, medical facility, and current lifecycle status while omitting confidential clinical diagnoses. |
| **`FR-PHARM-02`** | **Must** | **Medication Dispensing & Duplicate Prevention:** Pharmacists shall fulfill active prescriptions via a single confirmation action, permanently transitioning the status to `DISPENSED` and blocking subsequent fulfillment attempts across all pharmacies. | **1.** *Given* a verified prescription has status `ACTIVE`, *when* the pharmacist clicks "Dispense Medication" (`PATCH /api/v1/prescriptions/{id}/dispense`), *then* the status transitions to `DISPENSED` and records the pharmacist's IIN, pharmacy chain, and timestamp.<br>**2.** *Given* a prescription already has status `DISPENSED`, *when* scanned or queried again, *then* the UI displays the warning *"Prescription already dispensed"* along with prior fulfillment details and disables the dispense button (`disabled`). |

---

### 2.5. Administrator Portal Requirements

| ID | Priority | Requirement Description | Verifiable Acceptance Criteria (Given / When / Then) |
| :--- | :--- | :--- | :--- |
| **`FR-ADM-01`** | **Must** | **User Provisioning, Organization Binding & RBAC Management:** Administrators shall create, modify, and suspend user accounts (`NewUserModal`), validate 12-digit IINs and medical licenses, assign system roles, and bind staff to clinics or pharmacies. | *Given* an authenticated `ADMIN`, *when* a user account is suspended or its role is updated in the user management table, *then* the change persists immediately and generates an `admin:user_block` or `admin:role_change` audit event. |
| **`FR-ADM-02`** | **Must** | **Centralized INN Medication Catalog Management:** Administrators shall add and edit medications in the national INN registry (`MnnModal`), including ATC codes, pharmacotherapeutic groups, dosage forms, and standard dosages. | *Given* an `ADMIN` adds or updates an INN entry via `MnnModal`, *when* a `DOCTOR` subsequently opens the prescription creation autocomplete (`FR-DOC-02`), *then* the updated medication and dosage parameters are immediately selectable. |
| **`FR-ADM-03`** | **Should** | **Immutable Security & Lifecycle Audit Trail:** Administrators shall inspect and filter system-wide audit logs covering prescription issuance, dispensing, cancellation, account management, and unauthorized access attempts. | *Given* any state-mutating action or `403 Forbidden` route interception occurs, *when* the `ADMIN` opens the Audit Trail view and filters by Actor, IIN, or Event Type (`prescription:create`, `prescription:dispense`, `prescription:cancel`), *then* the exact timestamped record is displayed. |

---

## 3. Client-to-API Integration Specification

The frontend client communicates with the centralized backend via RESTful JSON endpoints over HTTPS:

| Operation | HTTP Method & Endpoint | Authorized Roles | Success / Error Status Codes |
| :--- | :--- | :--- | :--- |
| **Authenticate User** | `POST /api/v1/auth/login` | Public | `200 OK` (JWT Pair), `401 Unauthorized` |
| **Lookup Patient & History** | `GET /api/v1/patients/{iin}/prescriptions` | `DOCTOR` | `200 OK`, `403 Forbidden`, `404 Not Found` |
| **Issue Prescription** | `POST /api/v1/prescriptions` | `DOCTOR` | `201 Created`, `400 Bad Request`, `422 Unprocessable` |
| **Revoke Prescription** | `PATCH /api/v1/prescriptions/{id}/cancel` | `DOCTOR` | `200 OK`, `409 Conflict` (if already `DISPENSED`) |
| **Fetch Patient Prescriptions** | `GET /api/v1/me/prescriptions` | `PATIENT` | `200 OK`, `401 Unauthorized` |
| **Verify Prescription by Token** | `GET /api/v1/prescriptions/verify/{code}` | `PHARMACIST` | `200 OK`, `404 Not Found` |
| **Dispense Medication** | `PATCH /api/v1/prescriptions/{id}/dispense` | `PHARMACIST` | `200 OK`, `409 Conflict` (Duplicate Dispense Blocked) |
| **Manage Users & INN Catalog** | `POST/PATCH /api/v1/admin/users`, `/medications` | `ADMIN` | `200 OK`, `201 Created`, `403 Forbidden` |

---

## 4. Non-Functional Requirements (NFR)

| ID | Category | Requirement Specification | Target Metric / Verification Condition |
| :--- | :--- | :--- | :--- |
| **`NFR-PERF-01`** | **Performance** | Prescription creation, QR token rendering, and pharmacist verification queries must execute with minimal latency to prevent clinical bottlenecks. | Client UI state update and API round-trip completion in **`< 1.0 second`** under normal network conditions; initial page load (FCP) **`<= 1.5 seconds`**. |
| **`NFR-RESP-01`** | **Responsiveness** | The UI must support both clinical desktop workstations and mobile citizen devices natively without layout overflow. | Verified across Desktop (`1440×900px`) for Doctor/Pharmacist/Admin views and Mobile (`390×844px`) for the Patient QR view. |
| **`NFR-SEC-01`** | **Security & Privacy** | Role-based route guards must prevent unauthorized view rendering; pharmacist views must enforce data minimization (hiding clinical diagnoses). | `100%` interception rate on unauthorized route navigation (`#/403` redirect + audit log entry); zero diagnostic fields exposed in Pharmacist payload. |
| **`NFR-REL-01`** | **Reliability & Idempotency** | Prescription dispensing operations must be strictly idempotent to prevent duplicate medication fulfillment across different pharmacy chains. | Repeated `PATCH /dispense` requests on a `DISPENSED` prescription return `409 Conflict` `100%` of the time, keeping the UI dispense button disabled. |
| **`NFR-USAB-01`** | **Clinical Usability** | Color contrast and interaction flows must minimize eye strain and operator error during high-volume shifts. | Low-glare *Warm Ivory* (`#F7F7F2`) surface palette; standard prescription issuance completable in **`<= 45 seconds`** and pharmacy dispensing in **`<= 15 seconds`**. |