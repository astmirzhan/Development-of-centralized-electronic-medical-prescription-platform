# 01. Project Proposal

| Field | Value |
|---|---|
| **Project title** | Development of a Centralized Electronic Medical Prescription Platform |
| **Short name** | eRx Platform |
| **Team** | Mysyrgaliyev Mirzhan, Muratbayuly Abylaykhan, Ibragim Nurkasym (group SE-2414, Astana IT University) |
| **Supervisor** | Хаймульдин Асқар Ғазизұлы |
| **Project type** | Team diploma project, 3 members |
| **Document version** | v0.2, October 2026 (pre-defense 1) |

---

## 1. Background

Electronic prescribing (e-prescribing) replaces handwritten prescriptions with structured records that the prescriber signs digitally and sends to a central service, where a pharmacy can verify and dispense them. Research links e-prescribing to fewer illegibility-related errors and to better traceability of dispensed medicines (Radley et al., 2013; Annisa et al., 2023). It also reports that new types of errors appear and that system design and usability decide whether clinicians accept it (Farghali & Borycki, 2024; Cassidy et al., 2023; Bouraghi et al., 2024).

Kazakhstan already digitised prescriptions for the **state-funded segment**. Free and subsidised outpatient medicines (ГОБМП/ОСМС) are prescribed in the Ministry of Health's drug-provision information system (ИСЛО) and reach the patient as a QR voucher in the eGov "Social Wallet". By December 2025, 81.2% of these prescriptions were digital (Inbusiness.kz, 2025). That system covers only medicines the state pays for. When a patient buys a **prescription-only medicine with their own money** in a retail pharmacy, there is no common way to issue, verify, or redeem the prescription digitally. Paper prescriptions, or no prescription at all, are still the norm there. In June 2026 the Ministry of Health announced stricter control of antibiotic sales without a prescription, including on online marketplaces (Zakon.kz, 2026). That announcement confirms the gap exists.

## 2. Problem statement

In the **paid (retail) segment** of prescription-only medicines in Kazakhstan:

1. **Paper prescriptions can be forged or reused.** A pharmacist cannot check whether a paper prescription is authentic or whether another pharmacy has already dispensed it.
2. **Prescription-only medicines are dispensed without a prescription.** The pharmacy has no fast way to check one, so the rule is hard to enforce. Antibiotics are the most visible case.
3. **Patients lose their prescription history.** Paper prescriptions stay at the pharmacy or get lost, so the patient and the next doctor cannot see what was prescribed and dispensed.
4. **No one has a complete picture.** Clinics and regulators cannot see in one place how prescription-only medicines move from prescription to dispensing in the paid segment.

## 3. Target users

| User | Need |
|---|---|
| **Doctor** (clinic or private practice) | Issue a prescription by INN in under a minute, get warnings about allergies and drug interactions, sign it with their EDS (ЭЦП), and see the patient's prescription history |
| **Patient** | See all prescriptions on their phone, show a QR code for each item at any pharmacy, and see the full dispensing history |
| **Pharmacist** | Scan a QR code, confirm within seconds that the item is authentic and still valid, choose the trade name from stock, and sign the dispense with their EDS |
| **Administrator** (platform operator or regulator role) | Verify clinics and pharmacies by BIN, verify doctor and pharmacist accounts, manage the medicine catalogue, and review the tamper-evident audit log |

## 4. Aim

Design, implement, and evaluate a centralized platform that lets doctors issue **EDS-signed electronic prescriptions** for prescription-only medicines in the paid segment, and lets pharmacies **verify and dispense** them by QR code with an **EDS-signed dispensing record**. The platform must make forgery and dispensing beyond the prescribed quantity technically impossible and give patients a complete digital prescription history.

## 5. Objectives

| # | Objective | Measurable result |
|---|---|---|
| O1 | Analyse the domain: users, existing systems, regulation, literature | ≥ 3 user interviews, ≥ 12 literature sources, comparison of 5 systems |
| O2 | Specify requirements | Prioritised FR/NFR list, each Must requirement with acceptance criteria |
| O3 | Design the architecture and data model | System context, use case, and component diagrams; ERD with 24 tables |
| O4 | Implement the backend: Go REST API, PostgreSQL, EDS verification, prescription and dispensing lifecycle | API covering all Must requirements, with automated tests |
| O5 | Implement the clients: Next.js web for doctors, pharmacists, and admins; React Native app for patients | All Must user scenarios work end to end |
| O6 | Prevent forgery and over-dispensing | 0 successful dispenses of forged, modified, or already-dispensed items in the security test suite |
| O7 | Evaluate the system against the paper-based baseline | Time to verify, time to issue, error rate, SUS usability score (see `09-verification-and-evaluation-plan.md`) |

## 6. Expected outcomes

- A working MVP deployed on a VPS: web portals (doctor, pharmacist, admin) and a mobile app (patient).
- Prescriptions signed by doctors and dispenses signed by pharmacists with Kazakhstan NCA RK EDS keys, verified on the server.
- Per-item QR redemption with short-lived, single-use tokens and atomic dispensing, so an item cannot be dispensed beyond the prescribed quantity.
- Allergy and drug-interaction warnings when a doctor prescribes (Should priority).
- A hash-chained, append-only audit log of every action.
- An evaluation report that compares the platform with the paper process on time, error, and fraud-resistance metrics.
- Full project documentation and source code in this repository.

## 7. Scope

### In scope (MVP)
- **Accounts and access:** EDS-based login for doctors and pharmacists; phone + one-time code for patients (SMS simulated); role-based access for four roles.
- **Organizations:** registration and verification of clinics and pharmacies by BIN; linking doctors to clinics and pharmacists to pharmacies.
- **Medicine catalogue:** INN (ru/kz/en), ATC code, dosage forms, trade names (brands), prescription-only / antibiotic flags; ICD-10 diagnosis codes. Loaded from a CSV seed.
- **Doctor:** patient search by IIN, prescription creation by INN with a diagnosis (ICD-10) and structured dosage per item, EDS signing, cancellation with a reason, patient history.
- **Clinical safety checks (Should):** warnings for known patient allergies and for drug–drug interactions between items in the same prescription.
- **Patient:** active and past prescriptions, a QR code per prescription item, dispensing history.
- **Pharmacist:** QR scan or manual code entry, verification result, choice of trade name, full or partial dispense, EDS signature of the dispense.
- **Admin:** organization and user verification, catalogue management, audit log viewer with integrity check, basic statistics.
- **Security:** RBAC, TLS, signature verification, single-use QR tokens, append-only hash-chained audit log.

### Out of scope (future work)
- **Integrations:** state systems (ИСЛО, eGov Social Wallet, eGov SMS 1414, e-Densaulyk), the State Register of Medicines / KNF (live sync), clinic MIS (Damumed and others), and pharmacy POS / 1C. The REST API is designed so these can be added later, but none is implemented in the MVP.
- Free or subsidised medicines (already covered by the state system).
- Narcotic and psychotropic substances, which have special legal forms and procedures.
- Payments and pharmacy stock management.
- Caching and message-broker infrastructure (Redis, queues); the MVP uses PostgreSQL only.
- Real patient data. The MVP uses synthetic data and NCA RK **test** certificates.

## 8. Engineering complexity

The project is more than a CRUD application:
- **Cryptography and trust.** The server verifies CMS/PKCS#7 signatures issued by NCA RK (GOST and RSA) and checks the certificate chain and revocation status (OCSP). Two parties sign: the doctor signs the prescription, and the pharmacist signs the dispense.
- **Concurrency and consistency.** Two pharmacies scanning QR codes for the same item at the same moment must not both succeed. This requires transactional state changes with row locking and is tested under load.
- **Security design.** QR tokens are short-lived, single-use, and contain no personal data; four roles with different permissions; an audit log where each record contains the hash of the previous one, so tampering can be detected.
- **Clinical safety logic.** Allergy and drug-interaction checks against reference tables before signing.
- **Multi-client system.** One Go API serves three web portals and one mobile app.
- **Evaluation** against a measurable baseline instead of a demo only.

## 9. Team and responsibilities (summary)

| Member | Role | Main areas |
|---|---|---|
| Mysyrgaliyev Mirzhan | Team lead, backend | Architecture, database design, Go API core, authentication, EDS integration, prescription module, clinical safety checks, deployment |
| Muratbayuly Abylaykhan | Backend and mobile | Dispensing module, QR tokens, audit log, admin API, React Native patient app |
| Ibragim Nurkasym | Frontend | Next.js portals for doctors, pharmacists, and admins; NCALayer integration in the browser; QR scanning; UI prototype |

The full allocation, milestones, and risks are in [`10-project-management-and-risks.md`](10-project-management-and-risks.md).

## References
All sources cited here are listed with full details in [`03-literature-review.md`](03-literature-review.md).
