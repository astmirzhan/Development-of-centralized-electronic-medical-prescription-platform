# 4. Existing Systems Comparison

> **Purpose.** This chapter provides a structured comparison of existing electronic prescription systems against a consistent set of criteria. Five systems are evaluated across ten comparison dimensions. The analysis demonstrates why the proposed solution is needed and why no existing system adequately covers the specific gap our platform targets: prescription-only medicines purchased with the patient's own money in Kazakhstan.

---

## 4.1 Systems Selected for Comparison

The following five systems were selected based on their relevance to the project's domain — electronic prescription issuance, verification, and dispensing:

| # | System | Country / Region | Operator | Rationale for inclusion |
|---|--------|-----------------|----------|------------------------|
| S1 | **ИСЛО + eGov Social Wallet** | Kazakhstan | Ministry of Health RK / НИТЭЦ | The primary domestic e-prescription system; direct predecessor to the gap our platform fills |
| S2 | **DamuMed (КМИС)** | Kazakhstan | DamuMed LLP | The most widely deployed clinical MIS in Kazakhstan; integrates with ИСЛО for subsidised prescriptions |
| S3 | **Surescripts Network** | United States | Surescripts (private alliance) | The world's largest e-prescribing network; represents the decentralised, market-driven model |
| S4 | **Kanta e-Prescription (eResepti)** | Finland | Kela (Social Insurance Institution) | A fully centralised national system with 100 % digital adoption; benchmark for small-country implementation |
| S5 | **Estonia Digital Prescription** | Estonia | Estonian Health Insurance Fund / TEHIK | A pioneer of e-government and digital health; 99.9 % of prescriptions are electronic; cross-border EU interoperability |

**Justification.** S1 and S2 represent the current Kazakhstan landscape and are necessary for a precise gap analysis. S3 provides the dominant private-sector model. S4 and S5 are Nordic-Baltic systems whose centralised, government-mandated architectures are closest to our proposed design. No comparable system for *paid-segment prescription-only medicines* exists in Kazakhstan — which is precisely the gap that motivates this project.

---

## 4.2 Comparison Criteria

Ten criteria were selected to cover architecture, security, user experience, and regulatory compliance:

| # | Criterion | What it measures |
|---|-----------|-----------------|
| C1 | **System architecture** | Centralised repository vs. decentralised/intermediary network |
| C2 | **Prescription coverage scope** | Which medicines and funding models are covered (state-funded, paid, controlled substances) |
| C3 | **Digital signature support** | Whether prescriptions are cryptographically signed with PKI-backed EDS |
| C4 | **Prescription verification mechanism** | How a pharmacy verifies that a prescription is valid, authentic, and not yet dispensed |
| C5 | **Anti-forgery and anti-reuse** | Technical controls against forged, modified, or double-dispensed prescriptions |
| C6 | **Patient-facing access** | Mobile app, portal, or QR-based access for the patient to view and present prescriptions |
| C7 | **Audit trail** | Whether the system maintains a tamper-evident, immutable record of all events |
| C8 | **Interoperability** | Integration with other national/international systems; data exchange standards |
| C9 | **Authentication and identity** | How doctors, pharmacists, and patients are identified and authenticated |
| C10 | **Openness and extensibility** | Open API, open-source components, ability for third-party integration |

---

## 4.3 Detailed Comparison

### C1. System Architecture

| System | Architecture | Details |
|--------|-------------|---------|
| **ИСЛО + eGov** | Centralised | Single national repository operated by НИТЭЦ; all subsidised prescriptions flow through the state database |
| **DamuMed** | Centralised (regional) | Cloud-based КМИС with regional sub-instances (e.g., `ast.dmed.kz`); interfaces with ИСЛО for prescription data |
| **Surescripts** | Decentralised / intermediary | Peer-to-peer network routing data between EHR vendors, pharmacies, and PBMs; does not store a permanent national repository |
| **Kanta** | Centralised | Single national prescription database managed by Kela; all pharmacies nationwide can access any prescription |
| **Estonia** | Centralised | National health information system (terviseportaal.ee) stores all prescriptions; accessible from any pharmacy |

**Observation.** Centralised architectures (ИСЛО, Kanta, Estonia) provide the strongest guarantees for de-duplication and anti-reuse, because a single authoritative source determines prescription status. Our platform follows the centralised model.

### C2. Prescription Coverage Scope

| System | State-funded | Paid (patient's own money) | Controlled substances |
|--------|:---:|:---:|:---:|
| **ИСЛО + eGov** | ✅ ГОБМП / ОСМС | ❌ | ❌ Separate regulation |
| **DamuMed** | ✅ (via ИСЛО) | ❌ | ❌ |
| **Surescripts** | ✅ Insurance-covered | ✅ Cash-pay | ✅ EPCS (DEA-certified) |
| **Kanta** | ✅ All categories | ✅ All categories | ✅ Integrated |
| **Estonia** | ✅ All categories | ✅ All categories | ✅ Integrated |

**Key finding.** This is the most critical comparison dimension. **Both Kazakh systems (ИСЛО and DamuMed) cover only state-funded medicines.** Paid prescription-only medicines — the segment with the highest forgery and uncontrolled dispensing risk — have no electronic prescription infrastructure. Our platform directly addresses this gap.

### C3. Digital Signature Support

| System | EDS signing | Signature standard | Server-side verification |
|--------|:---:|-----|:---:|
| **ИСЛО + eGov** | ✅ Doctors sign via ЭЦП | NCA RK (GOST/RSA) | ✅ |
| **DamuMed** | ✅ NCALayer integration | NCA RK (GOST/RSA) | ✅ |
| **Surescripts** | ⚠️ EPCS requires two-factor; PKI varies by vendor | DEA-compliant PKI | Vendor-dependent |
| **Kanta** | ✅ Smart card / mobile ID | Finnish Trust Network (eIDAS) | ✅ |
| **Estonia** | ✅ ID-card / Mobile-ID / Smart-ID | Estonian CA (eIDAS qualified) | ✅ |

**Observation.** All mature systems use PKI-backed digital signatures. Our platform leverages Kazakhstan's existing NCA infrastructure (NCALayer + NCANode), meaning no new PKI provisioning is needed — doctors and pharmacists already have their ЭЦП keys.

### C4. Prescription Verification Mechanism

| System | Verification method | Offline capability |
|--------|--------------------|--------------------|
| **ИСЛО + eGov** | QR code in eGov Social Wallet → pharmacist scans → online lookup in ИСЛО | ❌ Online only |
| **DamuMed** | Lookup via КМИС interface; QR verification on medical documents | ❌ Online only |
| **Surescripts** | Electronic message routed to pharmacy's PMS; no patient-facing QR | ❌ Online only |
| **Kanta** | Patient shows ID → pharmacist looks up by SSN in Kanta database | ❌ Online only |
| **Estonia** | Patient shows ID-card or prescription code → pharmacist queries national DB | ❌ Online only |

**Observation.** Most systems use either a patient ID lookup or an electronic routing mechanism. Only ИСЛО uses a QR code, but it is static. **No system implements rotating, time-limited QR tokens** — the pattern we adopt from the TOTP model to prevent screenshot sharing and replay attacks.

### C5. Anti-Forgery and Anti-Reuse Controls

| System | Anti-forgery | Anti-reuse (double-dispensing) | Technical mechanism |
|--------|:---:|:---:|-----|
| **ИСЛО + eGov** | ✅ EDS-signed | ✅ Central DB status check | Status flag in ИСЛО database |
| **DamuMed** | ✅ (via ИСЛО) | ✅ (via ИСЛО) | Delegates to ИСЛО for dispensing control |
| **Surescripts** | ⚠️ Vendor-dependent | ⚠️ PBM-level checks | Varies by pharmacy management system |
| **Kanta** | ✅ EDS-signed | ✅ Atomic database update | National DB marks prescription as fully/partially dispensed |
| **Estonia** | ✅ EDS-signed | ✅ Atomic database update | National DB with row-level locking |
| **Our platform** | ✅ NCA EDS-signed | ✅ Row-locked atomic dispense | Short-lived opaque QR token + `SELECT ... FOR UPDATE` + hash-chained audit |

**Key finding.** Our platform combines all three layers of protection: cryptographic signing (forgery prevention), rotating QR tokens (replay prevention), and row-locked atomic dispensing (double-dispense prevention). This layered approach goes beyond what most existing systems offer individually.

### C6. Patient-Facing Access

| System | Mobile app | QR presentation | Prescription history | Notifications |
|--------|:---:|:---:|:---:|:---:|
| **ИСЛО + eGov** | ✅ eGov Mobile | ✅ Static QR in Social Wallet | ⚠️ Limited to subsidised | ❌ |
| **DamuMed** | ✅ DamuMed app | ⚠️ QR on medical documents only | ✅ Subsidised segment | ✅ Push notifications |
| **Surescripts** | ❌ No patient app | ❌ | ⚠️ Via insurance portals | ❌ |
| **Kanta** | ✅ My Kanta Pages | ❌ ID-based lookup | ✅ All prescriptions | ✅ |
| **Estonia** | ✅ Patient portal | ❌ ID-based lookup | ✅ All prescriptions | ✅ |
| **Our platform** | ✅ React Native app | ✅ Rotating QR (60 s TTL) | ✅ All paid prescriptions | ✅ Planned |

**Observation.** Our platform provides the most user-friendly verification experience: the patient simply opens the app and shows a self-refreshing QR code — no national ID card needed, no static code that could be photographed and reused.

### C7. Audit Trail

| System | Audit type | Tamper evidence | Hash chaining |
|--------|-----------|:---:|:---:|
| **ИСЛО + eGov** | Database logs | ⚠️ Standard DB security | ❌ |
| **DamuMed** | Application-level logs | ⚠️ Standard DB security | ❌ |
| **Surescripts** | Network transaction logs | ⚠️ Proprietary | ❌ |
| **Kanta** | Regulatory audit log | ✅ Compliant with Finnish/EU law | ❌ |
| **Estonia** | X-Road audit log | ✅ Digitally signed logs | ⚠️ Partial (X-Road message signatures) |
| **Our platform** | Append-only, hash-chained | ✅ DB triggers block UPDATE/DELETE | ✅ SHA-256 hash chain |

**Key finding.** Our hash-chained audit trail provides *mathematically verifiable* tamper evidence — a stronger guarantee than standard database security or application-level logging. A broken hash chain pinpoints the exact record that was modified.

### C8. Interoperability

| System | National integration | International / cross-border | Standards |
|--------|:---:|:---:|-----|
| **ИСЛО + eGov** | ✅ eGov ecosystem | ❌ | Proprietary |
| **DamuMed** | ✅ ИСЛО, ОСМС | ❌ | HL7 (partial) |
| **Surescripts** | ✅ 2M+ providers, all US pharmacies | ❌ US only | NCPDP SCRIPT, HL7 FHIR |
| **Kanta** | ✅ Full national coverage | ✅ MyHealth@EU (cross-border) | HL7 CDA, FHIR, eHDSI/OpenNCP |
| **Estonia** | ✅ Full national coverage | ✅ MyHealth@EU (cross-border) | X-Road, HL7, eHDSI/OpenNCP |
| **Our platform** | ⚠️ Adapter boundary to ИСЛО (future) | ❌ (MVP scope) | REST / OpenAPI 3.0 |

**Observation.** Interoperability with state systems (ИСЛО, e-Densaulyk) is outside the MVP scope. However, the REST/OpenAPI design and the adapter boundary documented in the architecture ensure that future integration is technically feasible without redesigning the core system.

### C9. Authentication and Identity

| System | Doctors | Pharmacists | Patients |
|--------|---------|-------------|----------|
| **ИСЛО + eGov** | ЭЦП (NCALayer) | ЭЦП (NCALayer) | IIN + eGov Mobile login |
| **DamuMed** | ЭЦП + institutional login | ЭЦП + institutional login | Phone + SMS OTP |
| **Surescripts** | EHR vendor credentials | PMS vendor credentials | Insurance portal login |
| **Kanta** | Smart card / mobile ID | Smart card / mobile ID | Finnish Trust Network (bank ID / mobile ID) |
| **Estonia** | ID-card / Mobile-ID / Smart-ID | ID-card / Mobile-ID / Smart-ID | Same methods |
| **Our platform** | ЭЦП (NCALayer) | ЭЦП (NCALayer) | Phone + SMS OTP |

**Observation.** Our authentication model mirrors the existing Kazakh practice: ЭЦП for professionals, OTP for patients. This minimises friction — no new credentials or hardware are required.

### C10. Openness and Extensibility

| System | Open API | Open source | Third-party extensibility |
|--------|:---:|:---:|-----|
| **ИСЛО + eGov** | ❌ Closed | ❌ | Banking-app integration announced (2025) |
| **DamuMed** | ❌ Closed | ❌ | API available to verified partners only |
| **Surescripts** | ⚠️ Partner API (certified vendors only) | ❌ | Vendor certification required |
| **Kanta** | ⚠️ Regulated API for healthcare providers | ❌ | Finnish healthcare org registration required |
| **Estonia** | ✅ X-Road services | ⚠️ X-Road is open-source | Open to registered e-services |
| **Our platform** | ✅ OpenAPI 3.0 | ✅ Fully open-source (MIT) | REST API; Docker-based deployment |

**Key finding.** Our platform is the only fully open-source solution in the comparison. The OpenAPI specification and Docker-based deployment lower the barrier for integration, auditing, and community contribution.

---

## 4.4 Summary Comparison Matrix

| Criterion | ИСЛО + eGov | DamuMed | Surescripts | Kanta (Finland) | Estonia | **Our platform** |
|-----------|:---:|:---:|:---:|:---:|:---:|:---:|
| C1. Architecture | Centralised | Centralised | Decentralised | Centralised | Centralised | **Centralised** |
| C2. Paid medicines | ❌ | ❌ | ✅ | ✅ | ✅ | **✅** |
| C3. EDS signing | ✅ | ✅ | ⚠️ | ✅ | ✅ | **✅** |
| C4. QR verification | Static QR | ⚠️ | ❌ | ❌ | ❌ | **Rotating QR** |
| C5. Anti-reuse | ✅ | ✅ | ⚠️ | ✅ | ✅ | **✅ (3-layer)** |
| C6. Patient app | ✅ | ✅ | ❌ | ✅ | ✅ | **✅** |
| C7. Hash-chained audit | ❌ | ❌ | ❌ | ❌ | ⚠️ | **✅** |
| C8. Interoperability | National | National | US national | EU cross-border | EU cross-border | **REST/OpenAPI** |
| C9. Kazakh ЭЦП | ✅ | ✅ | N/A | N/A | N/A | **✅** |
| C10. Open source | ❌ | ❌ | ❌ | ❌ | ⚠️ | **✅** |

---

## 4.5 Why the Proposed Solution Is Needed

The comparison reveals four structural reasons why no existing system satisfies the project's requirements:

1. **Coverage gap.** The two Kazakh systems (ИСЛО and DamuMed) do not cover paid prescription-only medicines. International systems (Surescripts, Kanta, Estonia) cover all categories but are not available in Kazakhstan and cannot be deployed domestically due to infrastructure and regulatory differences.

2. **No rotating QR verification.** No existing system uses short-lived, TOTP-inspired rotating QR tokens. ИСЛО's Social Wallet uses a static QR that can be screenshotted and shared. Kanta and Estonia use ID-based lookups that require the patient to carry a national smart card or government-issued ID.

3. **No cryptographic audit trail.** None of the compared systems implement a hash-chained, append-only audit log with SHA-256 integrity verification. Standard database logging is vulnerable to insider tampering and does not provide mathematically verifiable proof of data integrity.

4. **No open-source, Kazakh-PKI-compatible platform.** International open-source e-prescribing frameworks (e.g., OpenMRS, GNU Health) do not integrate with Kazakhstan's NCA PKI (NCALayer / NCANode). Building on top of them would require more adaptation effort than building a focused, purpose-built system.

**Conclusion.** The proposed platform fills a well-defined gap: a centralised, EDS-signed, rotating-QR-verified, hash-chain-audited e-prescribing system for paid prescription-only medicines in Kazakhstan, built on open-source technologies and integrated with the existing national PKI infrastructure.
