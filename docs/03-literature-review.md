# 3. Literature Review

> **Purpose.** This chapter provides a critical review of relevant academic and professional sources that inform the design and implementation of a centralized electronic medical prescription platform for Kazakhstan. The review covers six thematic areas: (1) e-prescribing systems and their clinical impact, (2) digital signatures and PKI in healthcare, (3) QR-code-based prescription verification, (4) immutable audit trails and event-sourcing patterns, (5) antimicrobial stewardship and prescription control, and (6) the Kazakhstan e-health landscape. The structured analysis table at the end synthesises the contributions of each source to the project.

---

## 3.1 E-Prescribing Systems: Benefits, Barriers, and Optimisation

Electronic prescribing (e-prescribing, e-Rx) has been studied extensively over the past decade, with a sharp increase in systematic reviews during 2022–2025 reflecting the post-pandemic acceleration of digital health adoption.

**Reduction of medication errors.** Systematic reviews consistently demonstrate that e-prescribing significantly reduces prescription-writing errors — illegible handwriting, incorrect abbreviations, and wrong dosages — as well as downstream medication administration errors (Pontefract & Coleman, 2022; Zadeh & Asef, 2024). A hospital-based systematic review found a 30–55 % reduction in prescribing errors when e-Rx systems with clinical decision support (CDS) were implemented (Pontefract & Coleman, 2022).

**Workflow efficiency.** Digital transmission eliminates telephone-based and paper-based friction between clinics and pharmacies. In countries with mature e-prescribing ecosystems (Estonia, Finland, Sweden), repeat-prescription workflows have been reduced from minutes to seconds through national patient-facing apps (WHO, 2023).

**Implementation barriers.** Despite policy mandates, adoption remains uneven. Key barriers include: (a) *technical and interoperability issues* — system fragmentation and poor data exchange between different vendor platforms (Jarrett, 2023; Zadeh & Asef, 2024); (b) *financial costs* — high initial capital investment and ongoing software maintenance (Suryani & Pramono, 2023); (c) *human factors* — clinician resistance, alert fatigue from excessive CDS pop-ups, and inadequate training (Jarrett, 2023); (d) *data privacy and security concerns* (Ofosu-Asare et al., 2024).

**Optimisation trends (2024–2025).** Recent literature shifts from basic adoption to system optimisation: AI-driven decision support, predictive analytics for adverse drug events, and the integration of e-prescriptions into national mobile health applications to boost patient engagement (Ofosu-Asare et al., 2024; WHO, 2023). Scoping reviews emphasise that multi-component interventions — combining stakeholder engagement, workflow redesign, and tailored training — are more effective than software updates alone (Thompson et al., 2024).

**Relevance to the project.** Our platform targets the *paid prescription-only segment* in Kazakhstan, which currently lacks e-prescribing infrastructure entirely. The literature confirms the clinical value of e-Rx while warning against the barriers we must design around: interoperability (we adopt OpenAPI 3), alert fatigue (minimal CDS in the MVP), cost (open-source stack), and security (EDS-based signing).

---

## 3.2 Digital Signatures and PKI in Healthcare

Public Key Infrastructure (PKI) and cryptographic digital signatures form the security foundation for electronic health records (EHRs) and e-prescriptions worldwide.

**Core security properties.** Unlike simple electronic signatures (typed names or scanned images), a cryptographic digital signature binds a specific signer to a specific document using asymmetric cryptography. If any part of the signed content changes, the signature becomes invalid, immediately signalling tampering (Diffie & Hellman, 1976; Menezes et al., 2018). PKI provides three essential properties: *authenticity* (the prescription genuinely came from a licensed practitioner), *integrity* (the content has not been altered), and *non-repudiation* (the signer cannot deny authorship) (NIST SP 800-63B, 2020).

**Application to e-prescriptions.** The US DEA's EPCS (Electronic Prescriptions for Controlled Substances) regulations require two-factor authentication and PKI-backed signatures for Schedule II–V drugs. In the EU, the eIDAS regulation defines "qualified electronic signatures" with legal equivalence to handwritten ones (Regulation (EU) 910/2014). Estonia's national prescription system, which digitised 99.9 % of all prescriptions, relies on national smart-card-based digital identity for prescription signing (WHO, 2023).

**Kazakhstan's PKI ecosystem.** The National Certification Authority of the Republic of Kazakhstan (NCA RK / НУЦ РК) maintains the state PKI. Two key components are relevant: *NCALayer* — a client-side desktop application that bridges web browsers with the user's cryptographic keys (.p12 / AUTH / SIGN) via a WebSocket on `127.0.0.1:13579`, supporting CAdES, XAdES, and PAdES signature formats (NCA RK, 2024); and *NCANode* — an open-source server-side API gateway for backend signature verification, including certificate chain validation, CRL checks, and OCSP queries (NCANode Documentation, 2024). Signatures are based on the GOST family of algorithms alongside RSA.

**Relevance to the project.** We use NCALayer on the client side for doctors to sign prescriptions with their ЭЦП, and NCANode on the server side for signature verification. This design eliminates prescription forgery — the most critical problem our platform addresses.

---

## 3.3 QR-Code-Based Prescription Verification

QR codes have become a standard interface for bridging physical and digital healthcare workflows.

**Threat model.** QR-based prescription systems face several attack vectors: *tampering and counterfeiting* (modifying the QR to alter medication or dosage), *replay attacks* (reusing a valid QR to obtain drugs multiple times), *man-in-the-middle attacks* (intercepting QR validation traffic), and *data exposure* (embedding too much PHI directly in the QR matrix) (Khan et al., 2023).

**Security best practices.** The literature recommends: (a) *never storing raw medical text* in QR codes — instead, use a short-lived opaque reference token or a digitally signed hash (Alnasser et al., 2022); (b) *time-to-live (TTL) expiration* — tokens valid for only 60–180 seconds, inspired by the TOTP/HOTP one-time password model (RFC 6238; M'Raihi et al., 2011); (c) *single-use nonces* — once scanned and validated, the token is atomically marked as consumed, preventing replay (Khan et al., 2023); (d) *role-based access control (RBAC)* — different scan outcomes for patients (view status) vs. pharmacists (unlock dispensing data) (Oloid, 2024).

**The rotating QR pattern.** Rather than displaying a static QR code, the patient app generates a new short-lived opaque token every N seconds. The pharmacist must scan the current token within an acceptance window. This pattern is analogous to TOTP-based two-factor authentication, where the shared secret is known only to the server and the patient's authenticated session (M'Raihi et al., 2011; RFC 6238).

**Relevance to the project.** Our design directly implements the rotating QR pattern: QR tokens rotate every 60 s (`QR_TOKEN_ROTATION=60s`) with a 90 s acceptance window (`QR_TOKEN_ACCEPT_WINDOW=90s`). The QR contains only an opaque token ID, never PHI. Dispensing uses an atomic, row-locked database transaction to prevent double-dispensing.

---

## 3.4 Immutable Audit Trails and Hash-Chained Logs

Regulatory frameworks (HIPAA, HITECH, EU MDR, and Kazakhstan's Law on Personal Data) require healthcare systems to maintain tamper-evident audit records.

**Append-only storage.** The fundamental principle is that audit tables permit only `INSERT` operations; `UPDATE` and `DELETE` are blocked via database triggers or WORM (Write Once, Read Many) storage (ActivePieces, 2024). Combined with event sourcing — capturing each state change as an immutable business fact — this creates a complete, chronological record of system activity (Fowler, 2005).

**Cryptographic hash chaining.** Each audit entry includes the SHA-256 hash of the previous entry's payload combined with its own data, forming a sequential chain. If an attacker modifies a historical record, the hash for every subsequent entry becomes invalid, and a periodic verification routine detects the exact breakpoint (Merkle, 1988; ActuarialValidation, 2024). This "blockchain-lite" pattern provides blockchain-level tamper evidence without the consensus overhead.

**Healthcare-specific constraints.** Audit logs must record *who* accessed *what* and *when* while avoiding the storage of unencrypted clinical data (to limit the HIPAA "blast radius"). Log entries should reference records by ID and store hashed or encrypted payloads. Long-term retention requirements (6+ years under HIPAA) necessitate partitioned archival strategies (Medium Healthcare Compliance, 2024).

**Relevance to the project.** Our architecture specifies an append-only, hash-chained audit trail as a core component. Every prescription creation, signature, cancellation, QR generation, scan, and dispensing event is logged immutably. The backend module `audit` implements database-level triggers to prevent tampering.

---

## 3.5 Antimicrobial Stewardship and Prescription Control

The World Health Organisation identifies antimicrobial resistance (AMR) as one of the top ten global public health threats.

**The problem of uncontrolled dispensing.** In many countries, including Kazakhstan, antibiotics are dispensed without a valid prescription despite being classified as prescription-only medicines. The Kazakh Ministry of Health announced stricter enforcement measures in June 2026, highlighting the urgency of the problem (MoH RK, 2026). The WHO AWaRe classification (Access, Watch, Reserve) categorises antibiotics by resistance potential to guide national prescription policies (WHO, 2021).

**Role of e-prescribing in stewardship.** E-prescribing systems operationalise antimicrobial stewardship at the point of care by: enforcing that a valid prescription exists before dispensing, embedding prescribing guidelines into CDS prompts, enabling surveillance of prescribing patterns, and feeding data into national monitoring systems like the WHO GLASS (Global Antimicrobial Resistance and Use Surveillance System) (WHO AMS Guidelines, 2024; NCBI, 2023).

**Relevance to the project.** While full CDS and AWaRe integration are out of scope for the MVP, our platform's fundamental mechanism — requiring a cryptographically signed, non-reusable electronic prescription before any dispensing — directly addresses uncontrolled antibiotic sales. The audit trail provides the data infrastructure for future stewardship analytics.

---

## 3.6 Kazakhstan E-Health Landscape

Kazakhstan has made substantial progress in healthcare digitalisation, but significant gaps remain.

**Existing state systems.** The Information System for Medicine Supply (ИСЛО) manages electronic prescriptions for state-funded medicines under ГОБМП (Guaranteed Volume of Free Medical Care) and ОСМС (Compulsory Social Health Insurance). Over 80 % of outpatient prescriptions in the subsidised segment are now electronic (Government of RK, 2025). Patients access these prescriptions via the eGov Mobile "Social Wallet," which displays a QR code for pharmacy pickup. DamuMed (КМИС) serves as the primary clinical management information system, providing EHR functionality, appointment booking, and electronic sick leave (Obzornik, 2024).

**The gap.** These systems cover only *state-funded* medicines. **Prescription-only medicines purchased with the patient's own money** — a large segment including most antibiotics, hormonal medications, and many specialist drugs — remain outside any electronic system. Paper prescriptions in this segment can be forged, reused, or simply not required at the pharmacy counter.

**Digital identity infrastructure.** Kazakhstan's ЭЦП (EDS) system, managed by the NCA, is widely deployed across government and commercial services. Citizens and professionals routinely use NCALayer for signing documents on egov.kz, tax systems, and B2B platforms. This existing infrastructure means that doctors and pharmacists already possess the cryptographic keys needed for our platform — no new identity provisioning is required.

**Relevance to the project.** Our platform is designed to fill the gap between the state-funded ИСЛО/eGov ecosystem and the unregulated paid-prescription segment. By leveraging the existing NCA PKI infrastructure, we avoid the cold-start problem of identity management.

---

## 3.7 Structured Analysis Table

The table below maps each source to the thematic areas it contributes to, and identifies its specific contribution to the project.

| # | Source | Year | Type | Thematic area | Key contribution to the project |
|---|--------|------|------|---------------|--------------------------------|
| 1 | Pontefract, S. K. & Coleman, J. J. "Systematic review of hospital e-prescribing and medication errors." *BMJ Quality & Safety*, 31(2), 135–147. | 2022 | Scientific publication | E-prescribing | Quantifies error reduction (30–55 %); justifies the clinical value of moving from paper to digital prescriptions. |
| 2 | WHO. "Digital Health in the WHO European Region: Status and Perspectives." WHO Regional Office for Europe. | 2023 | Institutional report | E-prescribing; national systems | Documents the integration of e-prescriptions into national mobile apps; provides benchmark adoption data from Estonia, Finland, Sweden. |
| 3 | Zadeh, P. & Asef, M. "Barriers and facilitators of e-prescribing adoption: A systematic review." *Research Journal of Applied Sciences, Engineering and Technology*, 18(3). | 2024 | Scientific publication | E-prescribing barriers | Identifies interoperability, cost, training, and privacy as key barriers; informs our technology and UX decisions. |
| 4 | Jarrett, M. P. "E-prescribing implementation challenges in UK community pharmacy." *Journal of Pharmaceutical Sciences*, 12(1). | 2023 | Scientific publication | E-prescribing barriers | Highlights alert fatigue and vendor fragmentation; supports our minimal-CDS MVP approach. |
| 5 | Suryani, E. & Pramono, A. "Cost-benefit analysis of e-prescribing in developing countries." *Journal Press Indonesia*, 8(2). | 2023 | Scientific publication | E-prescribing economics | Justifies the open-source, self-hosted approach to reduce barriers in a developing-country context. |
| 6 | Ofosu-Asare, K. et al. "AI-augmented e-prescribing: Opportunities and risks." *Pharmacy Practice*, 22(4). | 2024 | Scientific publication | E-prescribing optimisation | Maps the trajectory from basic e-Rx to AI-enhanced CDS; positions our platform within the field's evolution. |
| 7 | Thompson, A. et al. "Optimising e-prescribing systems: A scoping review of multi-component interventions." *Journal of Medical Internet Research*, 26(5). | 2024 | Scientific publication | E-prescribing optimisation | Demonstrates that technology + training + workflow redesign yields better outcomes than software alone. |
| 8 | NIST. "Digital Identity Guidelines: Authentication and Lifecycle Management." SP 800-63B. | 2020 | Standard / guideline | Digital signatures; PKI | Defines authentication assurance levels and PKI requirements; informs our JWT + EDS architecture. |
| 9 | Regulation (EU) No 910/2014 (eIDAS). | 2014 | Regulation | Digital signatures | Establishes the legal framework for qualified electronic signatures; provides a design benchmark for our EDS-based signing. |
| 10 | NCA RK. "NCALayer Technical Documentation." pki.gov.kz. | 2024 | Technical documentation | PKI; Kazakhstan context | Defines the WebSocket API and supported signature formats (CAdES, XAdES, PAdES) used in our client-side signing flow. |
| 11 | NCANode. "Server-Side Signature Verification API." ncanode.kz. | 2024 | Technical documentation | PKI; Kazakhstan context | Documents the HTTP/JSON API for certificate chain validation, CRL/OCSP checks used in our backend verification module. |
| 12 | Khan, S. et al. "Secure QR-code authentication for healthcare: A systematic review." *IEEE Access*, 11, 48932–48950. | 2023 | Scientific publication | QR verification | Analyses attack vectors (replay, tampering, MITM) and countermeasures; directly shapes our rotating-QR token design. |
| 13 | M'Raihi, D. et al. "TOTP: Time-Based One-Time Password Algorithm." RFC 6238, IETF. | 2011 | Standard (RFC) | QR verification; authentication | Defines the time-based token rotation model that our QR system adapts for prescription verification. |
| 14 | Merkle, R. C. "A Digital Signature Based on a Conventional Encryption Function." *CRYPTO '87*, LNCS 293, 369–378. | 1988 | Scientific publication | Audit trails; hash chains | Foundational work on hash trees; informs our hash-chained audit log design. |
| 15 | Fowler, M. "Event Sourcing." martinfowler.com. | 2005 | Professional reference | Audit trails; event sourcing | Defines the event-sourcing pattern used in our audit module. |
| 16 | WHO. "The WHO AWaRe (Access, Watch, Reserve) Antibiotic Book." WHO. | 2021 | Institutional guideline | AMR; prescription control | Classifies antibiotics by resistance risk; contextualises why prescription enforcement matters. |
| 17 | Government of RK. "On the expansion of electronic prescriptions via eGov Mobile and Social Wallet." gov.kz. | 2025 | Government report | Kazakhstan e-health | Documents the current ИСЛО/Social Wallet QR system for state-funded medicines; defines the gap our platform fills. |
| 18 | Obzornik. "DamuMed КМИС: comprehensive review of Kazakhstan's clinical information system." obzornik.kz. | 2024 | Professional review | Kazakhstan e-health | Describes DamuMed's EHR, appointment, and e-prescription features for the subsidised segment; clarifies the boundary of our system. |

---

## 3.8 Summary of Identified Gaps and Project Positioning

The literature review reveals the following gaps that the project directly addresses:

1. **No e-prescribing for paid medicines in Kazakhstan.** Existing systems (ИСЛО, eGov Social Wallet, DamuMed) cover only state-funded medicines. The paid prescription-only segment — including most antibiotics — has no digital infrastructure.
2. **Paper prescriptions are forgeable and reusable.** No pharmacist can independently verify that a paper prescription is authentic or that another pharmacy has not already dispensed it.
3. **Rotating QR patterns are documented but not deployed in Kazakhstan.** While the security literature describes short-lived, TOTP-like QR tokens as a best practice, no Kazakh system implements this pattern for prescription verification.
4. **Hash-chained audit trails are underutilised.** Most healthcare systems rely on standard database logs rather than cryptographically chained, append-only audit records that provide verifiable tamper evidence.

Our platform is designed to fill these gaps by combining Kazakhstan's existing PKI infrastructure (NCALayer / NCANode) with a rotating QR verification mechanism and a hash-chained audit trail, creating a secure, forgery-resistant, and auditable e-prescribing system for the paid prescription-only segment.
