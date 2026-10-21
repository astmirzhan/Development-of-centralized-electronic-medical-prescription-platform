# Development of a Centralized Electronic Medical Prescription Platform

A platform for issuing, verifying, and dispensing **EDS-signed electronic prescriptions** for prescription-only medicines in Kazakhstan. A doctor signs the prescription with their ЭЦП (EDS). The patient shows a rotating QR code from the mobile app. Any registered pharmacy verifies it in seconds and records the dispense. A prescription cannot be forged, edited after signing, or dispensed twice.

> Diploma project · Astana IT University · Software Engineering · Group SE-2414 · 2026–2027

---

## Problem statement

Kazakhstan already uses e-prescriptions for **state-funded** medicines (ГОБМП/ОСМС) through the Ministry of Health system and the eGov Social Wallet QR. **Prescription-only medicines bought with the patient's own money** are outside that system. In this paid segment:

1. **Paper prescriptions can be forged or reused.** The pharmacist cannot check authenticity or whether another pharmacy has already dispensed the prescription.
2. **Prescription-only drugs are sold without a prescription.** Antibiotics are the most visible case; the MoH announced stricter control in June 2026.
3. **Patients lose their prescription history.** Neither the patient nor the next doctor can see what was prescribed and dispensed.
4. **There is no overall picture** of how prescription-only medicines move from prescription to dispensing.

Details and evidence: [Proposal](docs/01-project-proposal.md) · [Evidence of relevance](docs/02-evidence-of-relevance.md)

## Target users

| User | What the platform gives them |
|---|---|
| **Doctors** | Issue structured prescriptions and sign them with EDS via NCALayer; see the patient's prescription history |
| **Patients** | All prescriptions in a mobile app; rotating QR / short code to show at any pharmacy; full dispense history |
| **Pharmacists** | Scan QR → instant result (valid / expired / cancelled / already dispensed / invalid signature) → full or partial dispense |
| **Administrators** | Verify clinics, pharmacies, and clinicians; manage the medicine catalogue; audit log and statistics |

## Expected outcomes

- A working MVP deployed on a VPS: web portals (doctor, pharmacist, admin) and a mobile app (patient).
- Server-side verification of NCA RK EDS signatures (certificate chain and revocation).
- Anti-reuse by design: short-lived opaque QR tokens + atomic, row-locked dispensing.
- Append-only, hash-chained audit trail.
- Evaluation against the paper process: forgery detection, repeat dispensing, verification time, SUS usability ([plan](docs/09-verification-and-evaluation-plan.md)).

## Team

| Member | GitHub | Role | Responsibilities |
|---|---|---|---|
| **Mysyrgaliyev Mirzhan** | [`@astmirzhan`](https://github.com/astmirzhan) | Team lead, backend developer, repository owner | Repository administration, project management, system architecture, database design, Go API core, authentication and EDS integration, prescription module, CI/CD and VPS deployment |
| **Muratbayuly Abylaykhan** | [`@xJam-byte`](https://github.com/xJam-byte) | Backend and mobile developer | Dispensing, QR token, audit, and admin modules; OpenAPI specification; React Native patient app |
| **Ibragim Nurkasym** | `@<github-username>` | Frontend developer | Next.js portals for doctors, pharmacists, and administrators; NCALayer integration in the browser; QR scanning; UI prototype |
| **Хаймульдин Асқар Ғазизұлы** | — | Supervisor | Reviews key deliverables at checkpoints |

Full allocation, milestones, and risks: [Project management](docs/10-project-management-and-risks.md)

## Scope

**In the MVP:** EDS login for doctors and pharmacists, OTP login for patients; prescription creation from a catalogue with structured dosage; EDS signing; cancellation; patient app with active prescriptions, history, and QR; pharmacist verification and full/partial dispensing; admin verification of organisations and users; medicine catalogue; audit log.

**Out of scope:** integration with state systems (ИСЛО, eGov, e-Densaulyk) — an adapter boundary only; free/subsidised medicines; narcotic and psychotropic substances; payments and pharmacy stock; drug interaction checks. The MVP uses synthetic data and NCA RK test certificates only.

See [Proposal §7](docs/01-project-proposal.md#7-scope) and [Requirements](docs/06-requirements-specification.md).

## Current status

| Stage | Status |
|---|---|
| Analysis: proposal, literature review, system and technology comparison | ✅ v0.1 done |
| User interviews | 🔄 in progress |
| Requirements v0.1 | ✅ done |
| Architecture and diagrams (draft) | ✅ done |
| UI prototype (Figma) | 🔄 in progress |
| Verification and evaluation plan | ✅ done |
| Implementation | ⏳ starting (milestone M2) |

Submitted versions are tagged: `pre-defense-1`, `pre-defense-2`, `final-pre-defense`.

## Selected technologies

| Layer | Technology |
|---|---|
| Backend | Go (chi, pgx/sqlc), modular monolith |
| Database | PostgreSQL 16 |
| Web | Next.js + TypeScript (doctor, pharmacist, admin portals) |
| Mobile | React Native + Expo (patient app) |
| EDS | NCALayer (signing on the client) + NCANode (server-side verification) |
| Auth | JWT access tokens + rotating refresh tokens, RBAC |
| API | REST, OpenAPI 3 |
| Infrastructure | Docker Compose on a VPS, Caddy (TLS), GitHub Actions |

Why these choices: [Technology comparison](docs/05-technology-comparison.md)

## Documentation

| # | Document |
|---|---|
| 01 | [Project proposal and scope](docs/01-project-proposal.md) |
| 02 | [Evidence of relevance (interviews)](docs/02-evidence-of-relevance.md) |
| 03 | [Literature review](docs/03-literature-review.md) |
| 04 | [Existing systems comparison](docs/04-existing-systems-comparison.md) |
| 05 | [Technology comparison](docs/05-technology-comparison.md) |
| 06 | [Requirements specification](docs/06-requirements-specification.md) |
| 07 | [Architecture and diagrams](docs/07-architecture.md) |
| 08 | [UX / prototype](docs/08-ux-prototype.md) |
| 09 | [Verification and evaluation plan](docs/09-verification-and-evaluation-plan.md) |
| 10 | [Project management and risks](docs/10-project-management-and-risks.md) |
| 11 | [Supervisor feedback](docs/11-supervisor-feedback.md) |

**YouTrack:** https://erx-se2414.youtrack.cloud/projects/DIP (project key `DIP`)

## Repository structure

```
.
├── docs/            Project documentation (01–11), interview notes
├── diagrams/
│   ├── src/         Editable PlantUML sources
│   └── export/      Rendered PNG / SVG
├── prototype/       UI prototype screenshots (Figma link in docs/08)
├── src/
│   ├── backend/     Go API
│   ├── web/         Next.js portals
│   └── mobile/      React Native patient app
├── tests/           E2E, load, security, evaluation materials
├── config/          Configuration examples
├── .env.example     Environment variables (placeholders only)
└── CONTRIBUTING.md  Branching, commits, pull requests
```

## Setup, run, and test

*Runnable code is not yet available. This section will be completed in milestone M2. Planned commands:*

```bash
cp .env.example .env          # fill in local values
docker compose up -d          # api, postgres, ncanode, caddy
cd src/backend && go test ./...
```

## Contributing

Branches, commit messages, and pull requests follow [CONTRIBUTING.md](CONTRIBUTING.md). Every change is linked to a YouTrack task, e.g. `DIP-21: Add row-locked dispense transaction`.