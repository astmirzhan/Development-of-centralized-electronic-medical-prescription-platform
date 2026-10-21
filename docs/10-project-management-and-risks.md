# 10. Project Management and Risks

| Item | Link |
|---|---|
| Repository | this repository (`main` = reviewed baseline) |
| YouTrack project | https://erx-se2414.youtrack.cloud/projects/DIP, project key **`DIP`** |
| Workflow | see [`../CONTRIBUTING.md`](../CONTRIBUTING.md) |

## 1. Responsibilities

| Member | Role | Owns (accountable) | Contributes to |
|---|---|---|---|
| **Mysyrgaliyev Mirzhan** | Team lead, backend, repository owner | Repository administration (access, branch protection), architecture, DB schema and migrations, `auth`, `prescriptions`, EDS integration (NCANode), CI/CD, VPS deployment, docs 01, 05, 07 | Code review of all backend PRs; load and security tests |
| **Muratbayuly Abylaykhan** (`@xJam-byte`) | Backend + mobile | `dispensing`, `qr`, `audit`, `admin` modules; OpenAPI spec; React Native patient app; docs 03, 04 | Integration tests, mobile test checklist |
| **Ibragim Nurkasym** | Frontend | Next.js portals (doctor, pharmacist, admin), NCALayer integration in browser, QR scanning, UI prototype; docs 02, 06, 08, 09 | E2E tests (Playwright), usability study |
| **All** | | Conducting interviews (doc 02), weekly sync, reviewing each other's PRs | |
| **Supervisor** Хаймульдин Асқар Ғазизұлы | | Reviews key deliverables at checkpoints (doc 11) | |

Rule: every PR is reviewed by **another** team member before merging into `main`.

**Repository roles.** The repository is owned and administered by the team lead (Mirzhan). The other members have the *Maintain* / *Write* role, and the supervisor has at least *Read* access (plus *Triage* if they leave review comments). Each member commits only from their own account.

## 2. Milestones

Dates of pre-defenses 2 and final are set by the university; update this table when they are announced.

| Milestone | Date | Deliverables | Git tag |
|---|---|---|---|
| **M1 — Pre-defense 1** | **24 Oct 2026** | README, docs 01–11, diagrams with sources, prototype, YouTrack with prioritised tasks | `pre-defense-1` |
| M2 — Core backend | Nov 2026 (TBD) | Auth (EDS + OTP), prescriptions with signing, DB migrations, CI; Figma prototype | — |
| M3 — Pre-defense 2 | Dec 2026 (TBD) | Dispensing with anti-reuse, patient app (list + QR), doctor and pharmacist portals: Must FRs end to end; AT-01…AT-08 | `pre-defense-2` |
| M4 — Hardening | Feb–Mar 2027 | Admin, audit hash chain, Should FRs, security scans, load tests | — |
| M5 — Final pre-defense | Apr 2027 (TBD) | Evaluation study (M1–M8), final report, deployed system | `final-pre-defense` |

## 3. Initial YouTrack backlog (for M1–M3)

Each task in YouTrack must have: **assignee, expected outcome, acceptance criteria, deadline, priority**. Branch and commit names use the task ID (e.g. `DIP-5-literature-review`, `DIP-5: Add literature review table`).

| ID | Task | Assignee | Priority | Deadline | Acceptance criteria |
|---|---|---|---|---|---|
| DIP-1 | Set up repository structure | Mirzhan | Critical | 19 Oct | Folders docs/diagrams/src/tests/config, .gitignore, .env.example; all members and supervisor have access |
| DIP-2 | Complete project README | Mirzhan | Critical | 23 Oct | All README sections required by the university; all links work |
| DIP-3 | Project proposal and scope (doc 01) | Mirzhan | Critical | 19 Oct | Doc 01 committed: problem, users, aim, objectives, outcomes, scope |
| DIP-4 | User interviews, 3–5 (doc 02) | Nurkasym (with all members) | Critical | 23 Oct | ≥ 3 interview notes in `docs/interviews/`; assumptions table updated |
| DIP-5 | Literature review (doc 03) | Abylaykhan | Critical | 20 Oct | ≥ 12 sources, ≥ 8 scientific, analysis table |
| DIP-6 | Existing systems comparison (doc 04) | Abylaykhan | Critical | 21 Oct | 5 systems × 9 criteria, with sources |
| DIP-7 | Technology comparison (doc 05) | Mirzhan | Critical | 20 Oct | ≥ 3 decisions × 3 alternatives, justified |
| DIP-8 | Requirements specification (doc 06) | Nurkasym | Critical | 21 Oct | FR/NFR with priorities; every Must has acceptance criteria |
| DIP-9 | Architecture and diagrams (doc 07) | Mirzhan | Critical | 21 Oct | System context, use case, component diagrams and ERD: PlantUML sources + PNG |
| DIP-10 | UX prototype (doc 08) | Nurkasym | High | 22 Oct | Prototype for 4 user scenarios; link and screenshots in doc 08 |
| DIP-11 | Verification and evaluation plan (doc 09) | Nurkasym | Critical | 22 Oct | Test plan, metrics, measurement conditions, baseline |
| DIP-12 | Project management and risks (doc 10) | Mirzhan | Critical | 20 Oct | Responsibilities, milestones, risk register, dependencies |
| DIP-13 | Supervisor review of deliverables (doc 11) | Mirzhan | High | 23 Oct | Supervisor feedback recorded in doc 11 with resulting changes |
| DIP-14 | Spike: NCALayer signing + NCANode verification with test certificates | Mirzhan | High | 7 Nov | Demo: sign JSON in browser, verify on server, get IIN |
| DIP-15 | Go project skeleton, Docker Compose, CI (lint, test, build) | Mirzhan | High | 31 Oct | `docker compose up` starts api + db; CI green |
| DIP-16 | DB schema v1 and migrations | Mirzhan | High | 7 Nov | Matches ERD; migration up/down tested |
| DIP-17 | OpenAPI spec v1 | Abylaykhan | High | 7 Nov | Spec validates; Swagger UI served |
| DIP-18 | Auth module: EDS login, OTP login, JWT | Mirzhan | High | 21 Nov | AT-01, AT-02 pass |
| DIP-19 | Prescriptions module: draft, canonicalise, sign, cancel | Mirzhan | High | 28 Nov | AT-03, AT-04, AT-05 pass |
| DIP-20 | QR token module | Abylaykhan | High | 28 Nov | AT-06 passes |
| DIP-21 | Dispensing module with row locking and pharmacist EDS signature | Abylaykhan | High | 5 Dec | AT-07, AT-08, AT-09, AT-15 pass |
| DIP-22 | Doctor portal | Nurkasym | High | 5 Dec | Scenario D works against API |
| DIP-23 | Pharmacist portal with QR scanning | Nurkasym | High | 5 Dec | Scenario PH works against API |
| DIP-24 | Prototype usability review with users | Nurkasym | Medium | 14 Nov | ≥3 users completed key scenarios; findings in doc 08 |
| DIP-25 | Patient mobile app: login, list, QR | Abylaykhan | High | 5 Dec | Scenario P works on Android and iOS |
| DIP-26 | Audit module (append-only) | Abylaykhan | Medium | 12 Dec | AT-11 passes |
| DIP-27 | Clinical safety checks: allergies and drug interactions (Should) | Mirzhan | Medium | 19 Dec | AT-16 passes |
| DIP-28 | Medicine catalogue and ICD-10 seed data (CSV import) | Abylaykhan | High | 14 Nov | Catalogue with INN, forms, brands and ICD-10 codes loaded; FR-A03 acceptance criteria met |

## 4. Risks

Probability (P) and impact (I): 1 = low … 3 = high. Score = P × I.

| ID | Risk | P | I | Score | Mitigation | Owner |
|---|---|---|---|---|---|---|
| R1 | **Relevance challenged**: committee says Kazakhstan already has e-prescriptions | 3 | 3 | 9 | Clear positioning on the paid segment (docs 01, 04); evidence from interviews (doc 02) | Mirzhan |
| R2 | **No real client / pilot partner** (clinic or pharmacy) | 2 | 2 | 4 | Interview several organisations; controlled simulation as evaluation method (doc 09) | All |
| R3 | **EDS integration harder than planned** (NCALayer protocol, NCANode version/licence, test certificates) | 2 | 3 | 6 | Early spike DIP-14; fallback: mock signer behind the same interface for demos | Mirzhan |
| R4 | **Team new to Go** → slower backend | 2 | 2 | 4 | Standard library + few dependencies; pair programming on first modules; code review | Mirzhan |
| R5 | **Uneven contributions** visible in Git history | 2 | 3 | 6 | Tasks assigned per person in YouTrack; each person commits their own work; weekly check | Mirzhan |
| R6 | **Legal constraints** on storing medical data (personal data law) | 2 | 2 | 4 | Synthetic data only; data inventory (doc 07 §7); no real patients in MVP | Mirzhan |
| R7 | **Scope creep** (integrations with state systems, MIS, POS; advanced clinical checks) | 2 | 2 | 4 | MoSCoW; all integrations marked as future work; clinical checks limited to allergy and interaction warnings (Should) | Nurkasym |
| R8 | **Low participant numbers** for usability and evaluation | 2 | 2 | 4 | Recruit medical and pharmacy students early; schedule by M4 | Nurkasym |
| R9 | **VPS / infrastructure failure** before demo | 1 | 3 | 3 | Daily backups; scripted deploy; local Docker demo as fallback | Mirzhan |
| R10 | **Mobile app store distribution delays** | 2 | 1 | 2 | Use Expo Go / internal builds; PWA fallback | Abylaykhan |

## 5. Dependencies

| Dependency | Needed for | Status |
|---|---|---|
| NCA RK test certificates and NCALayer | DIP-14, DIP-18, DIP-19 | ☐ to obtain |
| NCANode (Docker image) | EDS verification | ☐ confirm version and licence |
| Medicine catalogue and ICD-10 data (INN, forms, brands, diagnosis codes) | FR-A03, FR-D02, DIP-28 | ☐ source to be chosen (public state register of medicines as CSV, or manual seed) |
| VPS with domain + TLS | Deployment, evaluation | ☐ |
| Interview participants (pharmacists, doctors, patients) | Doc 02 | ☐ |
| Supervisor availability for reviews | Doc 11 | ☐ schedule |

## 6. Communication
- Weekly team sync (30 min). Notes go into the YouTrack task or a short `docs/meetings/` file.
- Supervisor checkpoint before each pre-defense; feedback recorded in doc 11.
