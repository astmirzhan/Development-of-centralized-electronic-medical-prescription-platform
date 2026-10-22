# 08. UX/UI Prototype and Design System Architecture

**Project Title:** Development of a Centralized Electronic Medical Prescription Platform  
**Interactive Figma Prototype:** [View Prototype in Figma](https://www.figma.com/design/pVOkeJhL6n5K9SAMfbCUEt/Untitled?node-id=0-1&t=dJn1LLhpU05KWzPJ-1)  
**Document Version:** 1.3.0  
**Implementation Status:** Complete interactive mockups and responsive client views for Authentication & RBAC (`FR-AUTH-01`), Doctor Portal (`FR-DOC-01`–`03`), Patient Portal (`FR-PAT-01`–`02`), Pharmacist Portal (`FR-PHARM-01`–`02`), and Admin & Security Audit Portal (`FR-ADM-01`–`03`).

---

## 1. UX Design Philosophy and Design System Architecture

The client-side interface is engineered around core clinical UX principles: **minimizing cognitive load, preventing human input errors, and reducing time-on-task during patient consultations and pharmacy dispensing**. Because healthcare professionals operate under strict time constraints, the visual hierarchy eliminates unnecessary decorative clutter and relies on unambiguous semantic status indicators across the prescription lifecycle (`ACTIVE`, `DISPENSED`, `EXPIRED`, `CANCELLED`).

### 1.1. Color Palette (Sage & Warm Ivory)
To prevent eye strain during extended clinical shifts, traditional high-glare white backgrounds are replaced with a low-fatigue *Warm Ivory* (`#F7F7F2`) workspace, paired with high-contrast *Charcoal* typography and *Primary Sage* (`#537765`) interactive accents.

| Design Token | HEX Code | Functional Role & UI Application |
| :--- | :--- | :--- |
| **Primary Sage** | `#537765` | Primary action buttons (CTAs), active navigation states, input focus rings |
| **Deep Sage** | `#405F4D` | Pressed/hover button states (`:active`, `:hover`), high-emphasis interactive accents |
| **Soft Mint** | `#E5EDE5` | Subtle backgrounds, role badges, active tab indicators, filter pill-chips |
| **Warm Ivory** | `#F7F7F2` | Primary application background surface (reduces visual fatigue for clinical staff) |
| **Pure White** | `#FFFFFF` | Elevated data cards, modal containers (`NewUserModal`, `MnnModal`), form inputs |
| **Charcoal** | `#26332C` | Primary headings, critical clinical data (INN medication names, dosages, IIN) |
| **Muted Gray** | `#858D86` | Secondary labels, audit timestamps, input placeholders, helper text |
| **Border Gray** | `#E5E8E2` | Structural dividers, card borders, table grid lines, inactive input outlines |

### 1.2. Responsive Layout Strategy
* **Desktop-First Clinical Workstations (`1280px–1440px+`):** The Doctor, Pharmacist, and Administrator portals are optimized for desktop and tablet viewports used in clinics and pharmacy point-of-sale (POS) terminals, featuring persistent sidebar navigation, multi-column data tables, and instant modal workflows.
* **Mobile-First Citizen Interface (`390×844px`):** The Patient portal is designed specifically for mobile viewports, enabling effortless one-handed navigation and rapid QR code presentation at pharmacy counters.

---

## 2. Key User Scenarios and Interface Mockups

### Scenario 1: Unified Authentication and Role-Based Access Control (RBAC)
* **Prototype Exports:** `../prototype/01-login.png`, `../prototype/06-403-forbidden.png`
* **Client Routes:** `#/login`, `#/403`
* **Interaction Flow:**
  1. **Unified Sign-In (`FR-AUTH-01`):** The user enters their 12-digit Individual Identification Number (IIN, enforced with an input mask) and password.
  2. **Role Routing:** Upon successful authentication and JWT session initialization (Access + Refresh tokens), the client router directs the user to their designated workspace (`#/doctor/dashboard`, `#/pharmacist/dashboard`, `#/patient/dashboard`, or `#/admin/dashboard`).
  3. **Unauthorized Access Interception:** If an authenticated user attempts to navigate to a restricted route outside their assigned role (e.g., a `PATIENT` attempting to access `#/doctor/dashboard`), the client-side route guard intercepts the request, renders the **403 Forbidden** security screen, and dispatches an unauthorized access event to the centralized audit log.

---

### Scenario 2: Electronic Prescription Issuance and Management (Doctor Portal)
* **Prototype Export:** `../prototype/02-doctor-dashboard.png`
* **Client Route:** `#/doctor/dashboard`
* **Interaction Flow:**
  1. **Patient Lookup (`FR-DOC-01`):** The physician searches for a patient using their 12-digit IIN. The interface retrieves the patient's profile, insurance attachment status (GOBMP/OSMS), and a chronological history of active and past prescriptions to prevent adverse drug interactions.
  2. **Prescription Creation (`FR-DOC-02`):** The physician opens the prescription issuance form and selects a medication using an autocomplete search bound to the centralized International Nonproprietary Name (INN) catalog. The form requires structured inputs for dosage, administration frequency, treatment duration, and expiration date. Client-side validation strictly blocks submission if the dosage field is empty, and valid submissions complete in `< 1.0s`.
  3. **Prescription Revocation (`FR-DOC-03`):** From the prescription registry, the physician can revoke an erroneously issued prescription, transitioning its state to `CANCELLED`. To preserve data integrity, the revocation action is disabled for any prescription already marked as `DISPENSED`.

---

### Scenario 3: Prescription Tracking and QR Token Presentation (Patient Portal)
* **Prototype Export:** `../prototype/03-patient-dashboard.png`
* **Client Route:** `#/patient/dashboard`
* **Interaction Flow:**
  1. **Filtered Prescription Views (`FR-PAT-01`):** The patient dashboard segregates prescriptions into two distinct tabs: **Active** (`ACTIVE`) and **History** (`DISPENSED`, `EXPIRED`, `CANCELLED`), ensuring immediate access to valid prescriptions.
  2. **Digital Verification Token (`FR-PAT-02`):** Selecting an active prescription opens a detailed card displaying the prescribed INN medication, dosage instructions, prescribing physician, and a dynamically generated **2D QR Code** alongside an 8-character alphanumeric fallback code (e.g., `KZ-89F2A4`) and unique prescription ID (`RX-2024-09812`).
  3. **State-Driven Token Protection:** Once a prescription transitions to `DISPENSED`, `EXPIRED`, or `CANCELLED`, the QR code and fallback token are automatically hidden and replaced by a prominent status badge to prevent reuse at pharmacy counters.

---

### Scenario 4: Prescription Verification and Medication Dispensing (Pharmacist Portal)
* **Prototype Export:** `../prototype/04-pharmacist-dashboard.png`
* **Client Route:** `#/pharmacist/dashboard`
* **Interaction Flow:**
  1. **Dual-Mode Prescription Lookup (`FR-PHARM-01`):** The pharmacist verifies a prescription either by scanning the patient's QR code via the optical 2D scanner viewport (featuring an active laser alignment overlay) or by manually entering the 8-character fallback code (`KZ-89F2A4`), prescription number (`RX-2024-09812`), or patient IIN.
  2. **Privacy-Preserving Verification:** The interface displays essential dispensing parameters—INN medication, dosage, administration regimen, prescribing physician, medical organization, and GOBMP/OSMS status—while omitting confidential diagnostic details in accordance with medical privacy findings.
  3. **Single-Click Dispensing & Duplicate Prevention (`FR-PHARM-02`):**
     * For an `ACTIVE` prescription, the **"Dispense Medication"** button is enabled and triggers `PATCH /api/v1/prescriptions/{id}/dispense`.
     * Upon confirmation, the prescription status transitions to `DISPENSED`, recording the pharmacist's IIN (`currentUser.iin`), pharmacy organization, and exact timestamp.
     * **Duplicate Scan Protection:** If the same QR code or fallback code is scanned again at any pharmacy, the interface renders a high-visibility alert (**"Prescription already dispensed"**), displays the timestamp and organization of the prior fulfillment, and disables the dispense action (`disabled`).

---

### Scenario 5: System Administration, INN Catalog, and Security Audit (Admin Portal)
* **Prototype Export:** `../prototype/05-admin-dashboard.png`
* **Client Route:** `#/admin/dashboard`
* **Interaction Flow:**
  1. **User & RBAC Administration (`FR-ADM-01`):** Using the `NewUserModal` interface, the administrator provisions user accounts with 12-digit IIN validation, assigns system roles (`DOCTOR`, `PHARMACIST`, `PATIENT`, `ADMIN`), verifies medical licenses, and links clinicians and pharmacists to their respective healthcare facilities or pharmacy chains. Administrators can also instantaneously suspend/reactivate accounts or modify roles directly from the user table.
  2. **Centralized INN Medication Catalog (`FR-ADM-02`):** Through the `MnnModal` controller, the administrator maintains the national medication registry, including ATC codes, pharmacotherapeutic groups, dosage forms, and standard regimens. Updates propagate immediately to the physician's autocomplete selector (`FR-DOC-02`).
  3. **Immutable Audit Trail (`FR-ADM-03`):** The security audit view records all state-mutating operations across the platform (`prescription:create`, `prescription:dispense`, `prescription:cancel`, `admin:user_block`, `admin:role_change`, and `403 Forbidden` access interceptions) with real-time filtering by actor, IIN, or event type.

---

## 3. Prototype Traceability Matrix and Verification Accounts

To support interactive evaluation during pre-defense demonstrations, the prototype environment is pre-configured with four role-specific test accounts covering all mandatory functional requirements:

| System Role | Full Name (Demo Actor) | IIN (Login ID) | Password | Default Entry Route | Mapped Requirements & Mockup File |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **DOCTOR** | Alibek B. Kassymov | `840315301294` | `password123` | `#/doctor/dashboard` | `FR-AUTH-01`, `FR-DOC-01`–`03` (`02-doctor-dashboard.png`) |
| **PHARMACIST** | Dana K. Akhmetova | `920721402851` | `password123` | `#/pharmacist/dashboard` | `FR-AUTH-01`, `FR-PHARM-01`–`02` (`04-pharmacist-dashboard.png`) |
| **PATIENT** | Yernar M. Sultanov | `981105303419` | `password123` | `#/patient/dashboard` | `FR-AUTH-01`, `FR-PAT-01`–`02` (`03-patient-dashboard.png`) |
| **ADMIN** | Sergey A. Kovalev | `790510300182` | `password123` | `#/admin/dashboard` | `FR-AUTH-01`, `FR-ADM-01`–`03` (`05-admin-dashboard.png`) |