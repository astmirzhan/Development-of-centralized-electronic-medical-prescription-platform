# 02. Evidence of Relevance

> **Status: first round done (24 Oct 2026).** Section 1 is based on public data. Section 4 contains the results of an online survey with 4 respondents (1 doctor, 2 pharmacists, 1 patient). The raw answers are kept in the team's Google Forms account and can be shown on request. Follow-up interviews (section 3) continue until pre-defense 2.

The committee asks *"what evidence informs the requirements, and are verified findings distinguished from assumptions?"* This document separates the two explicitly.

---

## 1. Verified findings from public data

| # | Finding | Source | Requirement it informs |
|---|---|---|---|
| E1 | Kazakhstan already issues free and subsidised outpatient medicines through e-prescriptions in the MoH drug-provision system. The patient receives a QR voucher in the eGov "Social Wallet". 81.2% of these prescriptions were digital by Dec 2025, and 14.9% were still on paper. | [G2] | Scope: we target the **paid** segment and do not duplicate the state system |
| E2 | The QR flow is accepted in Kazakhstan: 1.7 million prescriptions were redeemed by QR in three months, across 700+ medical organisations. | [G3] | FR-P03, FR-PH01 (QR-based redemption is familiar to users) |
| E3 | Prescription-only drugs, especially antibiotics, are still sold without a prescription. In June 2026 the MoH announced tighter control and a planned ban on marketplace sales without a prescription. | [G7] | Problem 2, FR-PH02 (fast verification at the counter) |
| E4 | Before e-prescriptions, more than 4,000 citizens reported that the system showed subsidised drugs as "issued" to them although they never received them. | [G6] | FR-A04 audit trail, FR-P04 patient sees every dispense |
| E5 | Health data in Kazakhstan was fragmented. One vendor held >80% of the medical IS market, and there was no full data-exchange mechanism (2023). | [G4] | NFR-INT01, FR-SYS02 (open REST API), FR-SYS03 (adapter for future integration) |
| E6 | E-prescribing does **not** remove pharmacist interventions. Unclear directions, wrong quantity, and dropdown/design errors are common. | [L1], [L2] | FR-D03 structured dosage fields; NFR-USA |
| E7 | Physician satisfaction drops when the system is slow or poorly designed (Iran: 71.4% of doctors preferred paper after 2 years). | [L8], [L11] | NFR-PERF, NFR-USA, involving doctors in UI review |
| E8 | Security tokens and implementation cost are the main barriers in controlled-substance e-prescribing. | [L9] | Doctor signing must take one action through NCALayer, not several steps |

## 2. Assumptions to validate

| # | Assumption | How we validate it | Status after survey (section 4) |
|---|---|---|---|
| A1 | Pharmacists in retail pharmacies cannot verify a paper prescription's authenticity in practice. | Pharmacist interviews (Q-PH3, Q-PH4) | ✅ confirmed by both pharmacists: they check stamps, signature and dates only; S-PH2: "it is impossible to really check authenticity". Both have seen forged or altered prescriptions |
| A2 | Retail pharmacies would accept a QR scan step if it takes ≤10 seconds. | Pharmacist interviews (Q-PH6), usability test | ✅ confirmed: acceptable limit is 10–20 s (S-PH1) and 10–15 s, max 20 s (S-PH2). Our target (≤10 s median, M3 in doc 09) is inside this limit |
| A3 | Doctors in private clinics already have an EDS (NCA RK) and NCALayer installed. | Doctor interviews (Q-D2) | ⚠ partially: the one doctor (public polyclinic) uses EDS and NCALayer regularly, but reports NCALayer failures and expiring certificates. No private-clinic doctor answered yet |
| A4 | Patients lose paper prescriptions and want a digital history. | Patient interviews (Q-P2, Q-P3) | ✅ confirmed by S-P1 (lost paper prescriptions, wants history on the phone); the doctor also lacks history from other clinics (S-D1). Only 1 patient answered |
| A5 | Doctors spend more than 1 minute writing a paper prescription for a paid medicine. | Doctor interviews (Q-D4) and timing observation | ✅ confirmed: 2–3 min on paper, up to 5 min when duplicated in the MIS (S-D1). Still to be measured in the timing study (M4 in doc 09) |
| A6 | Patients are willing to use a separate app, or would prefer the prescription inside an existing app. | Patient interviews (Q-P5) | ⚠ partially, leaning to "existing channels": the patient prefers paper; the doctor asks for SMS/eGov delivery; a pharmacist asks for integration into the POS software instead of a separate site |

Statuses: ✅ confirmed / ❌ rejected / ⚠ partially. Requirements affected by these results are listed in section 5.

## 3. Interview plan

**Target:** at least 5 conversations: 2 pharmacists, 2 doctors (ideally one from a private clinic), and 1–2 patients with a chronic condition or frequent prescriptions. If a clinic or pharmacy agrees to act as a "client", record the agreement as a short letter or email and store it in `docs/interviews/`.

**Rules**
- Get oral consent. Do not record personal or medical data about patients. Use a role and an anonymised ID (e.g. `PH-1`, `D-2`).
- 15–20 minutes, semi-structured. One person asks, one takes notes.
- Store notes in `docs/interviews/<ID>-<date>.md`. Commit each interview separately; that also shows progressive contribution.

### Question guide

**Pharmacists**
- Q-PH1. How many prescription-only items do you dispense per day, roughly? What share comes with a paper prescription?
- Q-PH2. What do you do when a customer asks for a prescription-only drug (e.g. an antibiotic) without a prescription?
- Q-PH3. How do you check that a paper prescription is real? Have you seen forged or reused ones?
- Q-PH4. Is there any way to know if another pharmacy already dispensed a prescription?
- Q-PH5. How does the Social Wallet QR flow for free medicines work for you? What is good or bad about it?
- Q-PH6. How long could a verification step take before it slows the queue?
- Q-PH7. What equipment do you have at the counter (PC, scanner, phone)?

**Doctors**
- Q-D1. How do you currently write prescriptions for paid medicines? Paper, a clinic MIS, or something else?
- Q-D2. Do you have an NCA RK EDS and NCALayer? How often do you use them?
- Q-D3. What information do you lack about a patient's previous prescriptions?
- Q-D4. How long does writing one prescription take?
- Q-D5. What would stop you from using a new system?

**Patients**
- Q-P1. How often do you get prescriptions for medicines you pay for?
- Q-P2. Have you ever lost a prescription or needed an old one?
- Q-P3. Would you use a history of all your prescriptions? Where should it live (app, eGov, Telegram)?
- Q-P4. Have you used the Social Wallet QR for free medicines? Was it convenient?
- Q-P5. Would you install a separate app for this?

## 4. Survey results (round 1)

### 4.1 Method

- **Instrument:** anonymous Google Forms questionnaire in Russian, Kazakh and English. The respondent chooses a language and a role (doctor, pharmacist, patient) and gets only the questions for that role. The questions follow the question guide in section 3 (3 closed + 5–7 open questions per role).
- **Period:** 24 Oct 2026. **Responses:** 4 (1 doctor, 2 pharmacists, 1 patient).
- **Privacy:** no names, IINs or medical data were collected. Respondents are labelled S-D1, S-PH1, S-PH2, S-P1 (S = survey).
- **Raw data:** kept in Google Forms (team account) and can be shown on request. S-PH1 answered in Kazakh; the summaries below are our translation.

### 4.2 Respondents

| ID | Role | Organisation | Volume | Language |
|---|---|---|---|---|
| S-D1 | Doctor | Public polyclinic / hospital | 6–15 prescriptions for paid medicines per day; uses EDS and NCALayer regularly | Russian |
| S-PH1 | Pharmacist | Independent (non-chain) pharmacy | Up to 10 prescription-only items per day; PC + barcode/QR scanner | Kazakh |
| S-PH2 | Pharmacist | Pharmacy chain | 30–100 prescription-only items per day; PC + barcode/QR scanner + webcam | Russian |
| S-P1 | Patient | — | Paid prescription medicines about once a month; does not know the Social Wallet QR; keeps prescriptions on paper | Russian |

### 4.3 Findings

| # | Finding | Who said it | Assumption / requirement |
|---|---|---|---|
| F1 | Prescriptions for **paid** medicines are written on paper or in a consultation sheet; entering them in the clinic MIS is optional ("only if I have time"). Free medicines are already e-prescribed in the MIS with EDS. | S-D1 | Confirms the problem and the scope (paid segment) |
| F2 | Paper prescriptions are **reused**: the same prescription or a photocopy is presented for several months. Altered dosage or date written with another pen was seen. | S-D1, S-PH2; S-PH1 has seen forged prescriptions | A1; FR-PH02, FR-PH03, FR-PH04 |
| F3 | For paid prescriptions there is **no way to know** whether another pharmacy has already dispensed it. Marking "dispensed" on paper "protects from nothing". | S-PH2 | A1; FR-PH04, FR-P04 |
| F4 | Antibiotics are sold without a prescription. A pharmacy that refuses loses customers to competitors that sell. | S-PH2, S-P1, S-D1 | Problem 2; FR-PH02 |
| F5 | Verification must take **10–20 s at most**, otherwise the queue grows at peak time. The state QR system sometimes hangs. | S-PH1, S-PH2 | A2; NFR-PERF01, M3 |
| F6 | Writing a prescription takes 2–3 min on paper, up to 5 min with MIS duplication, out of a 15-minute appointment. No templates; dosages are calculated manually; pharmacies call because they cannot read the handwriting. | S-D1 | A5; FR-D03, FR-D07, NFR-PERF02 |
| F7 | The doctor does not see prescriptions from private clinics or specialists, does not know whether the patient bought the medicine, and allergies are often not recorded. | S-D1 | FR-D06, FR-P04, FR-D09 |
| F8 | Allergy and interaction warnings are wanted, especially for elderly patients on 5–8 medicines, **but too many warnings will be ignored**. | S-D1 | FR-D09 |
| F9 | Pharmacist EDS signature on each dispense is acceptable "if it is one click". Obstacles: pharmacists change at the counter between shifts; NCALayer is unstable on the POS computer; entering a password for every sale is too slow. S-PH1 is ready to sign. | S-PH2, S-PH1 | FR-PH07; risk R3 in doc 10 |
| F10 | Partial dispensing is hard in the current state system; the medicine is often out of stock. | S-PH2 | FR-PH03 |
| F11 | Elderly patients cannot open a QR code on the phone; a dead phone blocks the QR. Pharmacists want to find a prescription by IIN as a fallback. | S-PH2, S-D1 | FR-P05 (manual code); new candidate requirement (section 5) |
| F12 | Main adoption barriers: a separate system with its own login not connected to the MIS; slow internet in the office; training time for older colleagues. The pharmacy wants the system inside its POS software, not a separate website. | S-D1, S-PH2 | A6; FR-SYS02, FR-SYS03 (future integration) |
| F13 | The patient lost paper prescriptions, wants a history "to know what was prescribed and why", and worries that an electronic prescription comes without the doctor's explanation. | S-P1 | A4; FR-P02, FR-P04, FR-D03 (free-text instructions) |
| F14 | Wishes: prescription delivered to the patient by SMS or eGov; issuing a prescription from a phone during home visits. | S-D1 | FR-P07; out of MVP scope (see section 5) |

### 4.4 Limitations

- **Small sample:** 4 respondents, only 1 patient and 1 doctor; no doctor from a private clinic. The results show which problems exist, not how often they occur.
- **Self-selected and anonymous:** the respondents' roles cannot be verified. Answers are treated as qualitative evidence, not statistics.
- **Written answers:** no follow-up questions were possible. Follow-up interviews (section 3) target a private-clinic doctor and 1–2 more patients.

## 5. Summary and impact on requirements

**What the evidence confirms**
- The core problem exists in the paid segment: paper prescriptions are reused and sometimes altered, pharmacists cannot verify them, and there is no shared record of what was dispensed (F1–F3). This supports the Must requirements FR-PH02, FR-PH03, FR-PH04 and FR-P04.
- The time limit for verification is 10–20 s (F5). Our targets, ≤ 10 s median (M3) and ≤ 2 s API p95 (NFR-PERF01), fit inside it.

**What changes or is added**

| # | Change | Reason | Where |
|---|---|---|---|
| C1 | FR-D09 (allergy and interaction warnings) stays **Should**, with a design rule: show only clinically significant interactions and allergies, so that warnings are not ignored. | F7, F8 | doc 06, doc 08 |
| C2 | FR-PH07 (pharmacist EDS signature) stays **Must**, but the dispensing screen must need one signing action, and the usability test (doc 09, M6) must measure signing time at the counter. Pharmacist shift change and NCALayer instability are added to risk R3. | F9 | doc 06, doc 09, doc 10 |
| C3 | Prescription templates (FR-D07) are a candidate to move from **Could** to **Should**: the doctor names the lack of templates as a main time cost. | F6 | doc 06 |
| C4 | New candidate requirement (**Could**): a pharmacist can find a prescription by patient IIN **plus** the 8-character code when the QR cannot be shown. IIN alone is not enough, to keep NFR-PRIV01. | F11 | doc 06 |
| C5 | Integration with clinic MIS and pharmacy POS stays **future work**, but it is recorded as the main adoption barrier; the REST API (FR-SYS02) is the basis for it. SMS/eGov delivery and phone prescribing for home visits are recorded as future work. | F12, F14 | doc 01 (scope), doc 10 |
| C6 | The patient view must show the doctor's free-text instructions next to the structured dosage. | F13 | doc 06 (FR-D03, FR-P01), doc 08 |

Changes C1–C6 are proposals from this survey. They will be applied to doc 06 after the team review and recorded in its change log.
