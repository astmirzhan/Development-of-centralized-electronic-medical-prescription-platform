# 05. Technology Comparison

We made seven key technical decisions. For each one we compare 3 reasonable alternatives against criteria derived from our requirements (see `06-requirements-specification.md`). Scores are 1 (poor) to 5 (excellent). Weights reflect project priorities. The weighted total is a decision aid; the reasoning below each table is the actual justification.

Team context: all three members have production experience with **TypeScript** (React Native, Next.js, Nest.js). The diploma requires every member to write backend or frontend code. The team chose **Go** for the backend deliberately, as a learning goal and because of the requirements below.

---

## D1. Backend platform

| Criterion (weight) | **Go** (net/http + chi) | Node.js (Nest.js) | Java (Spring Boot) |
|---|---|---|---|
| Concurrency and performance under parallel QR scans (0.25) | 5: goroutines, low latency | 3: single event loop, fine for I/O | 4: mature, heavier |
| Strong typing and explicit error handling for a safety-critical domain (0.20) | 5 | 4 (TypeScript) | 5 |
| Deployment simplicity on one VPS (0.15) | 5: single static binary, ~20 MB image | 3: node_modules, runtime | 2: JVM, higher RAM |
| Team experience (0.20) | 2: learning | 5 | 2 |
| Ecosystem for crypto / EDS (0.20) | 3: standard crypto; GOST via external service | 3: same | 5: Kalkan SDK and NCANode are Java |
| **Weighted total** | **4.00** | 3.60 | 3.70 |

**Decision: Go.** Go wins on concurrency and deployment, which match NFR-PERF and the VPS constraint. The team's lack of Go experience is the main cost; it is mitigated by keeping the service a modular monolith and using the standard library where possible (risk R4). The Java ecosystem advantage for GOST signatures is neutralised by D4: verification runs in a separate service, so the main backend does not have to be Java.

## D2. Data storage

| Criterion (weight) | **PostgreSQL** | MongoDB | MySQL |
|---|---|---|---|
| ACID transactions and row locking for atomic dispensing (0.35) | 5: `SELECT … FOR UPDATE`, serializable isolation | 3: multi-document transactions exist but are not the natural model | 4 |
| Relational integrity: doctors, patients, prescriptions, items, dispenses (0.25) | 5: FK, CHECK constraints, enums | 2 | 4 |
| Flexible fields (signed document payload, audit details) (0.15) | 4: JSONB | 5 | 3: JSON type, weaker indexing |
| Audit features: append-only tables, triggers, row-level security (0.15) | 5 | 3 | 3 |
| Team experience (0.10) | 4 | 3 | 3 |
| **Weighted total** | **4.75** | 3.05 | 3.60 |

**Decision: PostgreSQL 16.** The central guarantee of the system ("a prescription item cannot be dispensed beyond the prescribed quantity") is a transactional guarantee, so we need a relational database with strong locking. JSONB covers the semi-structured parts (audit payloads). The data model has 24 tables in six groups; see the ERD in `07-architecture.md`.

## D3. Component interaction (clients ↔ backend)

| Criterion (weight) | **REST + JSON (OpenAPI 3)** | gRPC | GraphQL |
|---|---|---|---|
| Support in Next.js and React Native without extra layers (0.30) | 5 | 2: needs gRPC-Web proxy | 4 |
| Simplicity of authorization per endpoint / role (0.25) | 5 | 4 | 3: field-level auth is complex |
| Documentation and testability for the committee (0.20) | 5: Swagger UI | 3 | 4 |
| Performance (0.10) | 4 | 5 | 3 |
| Openness for future integration with clinic MIS and pharmacy POS software (0.15) | 5 | 3 | 3 |
| **Weighted total** | **4.90** | 3.15 | 3.50 |

**Decision: REST/JSON with an OpenAPI 3 contract** maintained in `src/backend/api/openapi.yaml`. Clients generate typed API clients from it. The same contract is the basis for future integration with clinic MIS and pharmacy POS systems, which are out of the MVP scope. gRPC would only make sense for internal service-to-service calls, and we have only one (to the EDS verifier), so it does not justify the extra tooling.

## D4. EDS (ЭЦП) signing and verification

| Criterion (weight) | **NCALayer (client) + NCANode (server, self-hosted)** | NCALayer + KalkanCrypt SDK called from Go via cgo | Own GOST 34.310 / CMS implementation in Go |
|---|---|---|---|
| Correctness of GOST signature, chain, and OCSP/CRL checks (0.35) | 5: purpose-built for NCA RK | 5: official SDK | 2: high risk of subtle errors |
| Effort to integrate (0.25) | 4: REST call to a container | 2: cgo, native libs, licensing | 1 |
| Isolation (crypto in a separate process) (0.15) | 5 | 2 | 3 |
| Licence / availability (0.15) | 4: open source | 3: SDK obtained from NCA | 5 |
| Doctor UX: uses tool doctors already have (0.10) | 5: NCALayer | 5 | 5 |
| **Weighted total** | **4.60** | 3.50 | 2.65 |

**Decision:** signing happens in the browser through **NCALayer** (the standard NCA RK client app) in two places: the **doctor** signs the canonical prescription JSON, and the **pharmacist** signs each dispensing record. The Go backend sends the CMS signature to a self-hosted **NCANode** container, which verifies the signature, certificate chain, validity, and revocation status (OCSP), and returns the signer's IIN and certificate serial. The backend checks that the IIN matches the logged-in user and stores the result in the `signatures` table. In development we use the NCA RK **test** certificates. *Before implementation, the team must confirm the current NCANode version and its licence (task DIP-14).*

## D5. Mobile client (patient app)

| Criterion (weight) | **React Native (Expo)** | Flutter | PWA (Next.js) |
|---|---|---|---|
| Team experience (0.30) | 5 | 1 | 5 |
| Code sharing with Next.js web (types, API client) (0.20) | 5: TypeScript end to end | 1 | 5 |
| Native features: secure storage for tokens, screen brightness for QR, biometrics (0.25) | 5: expo-secure-store, expo-local-authentication | 5 | 2: limited on iOS |
| Distribution for testing (0.15) | 4: Expo Go / EAS | 4 | 5: just a URL |
| Performance (0.10) | 4 | 5 | 3 |
| **Weighted total** | **4.75** | 2.85 | 4.05 |

**Decision: React Native with Expo.** A PWA is a valid fallback if app-store distribution becomes a problem. It is also the planned "no install" option for patients (FR-P06, Could).

## D6. Architecture style

| Criterion (weight) | **Modular monolith** | Microservices | Serverless functions |
|---|---|---|---|
| Transactional consistency across prescription and dispense (0.30) | 5: one DB transaction | 2: sagas needed | 2 |
| Operational effort for a 3-person team on one VPS (0.30) | 5 | 1 | 3: vendor lock-in |
| Clear module boundaries for parallel work (0.20) | 4: Go packages per module | 5 | 4 |
| Ability to extract services later (0.20) | 4 | 5 | 3 |
| **Weighted total** | **4.60** | 2.90 | 2.90 |

**Decision: modular monolith** with modules `iam`, `organizations`, `catalog`, `cds`, `prescriptions`, `pki`, `qr`, `dispensing`, `audit`, `notifications`. The only separate process is the EDS verifier (D4), which isolates crypto-specific code.

## D7. Caching and asynchronous processing

| Criterion (weight) | **PostgreSQL only** | PostgreSQL + Redis | PostgreSQL + Redis + message broker |
|---|---|---|---|
| Operational simplicity on one VPS for a 3-person team (0.35) | 5: one stateful service | 3: second stateful service | 1: three stateful services |
| Consistency: one source of truth for QR token consumption and dispensing (0.30) | 5: token check and dispense in the same transaction | 3: Redis and PostgreSQL must be kept in sync | 3 |
| Performance for MVP load (≈ 50 concurrent pharmacists, NFR-PERF01) (0.15) | 4: indexed lookups are sufficient | 5 | 5 |
| Scalability beyond the MVP (0.20) | 3 | 4 | 5 |
| **Weighted total** | **4.45** | 3.50 | 3.00 |

**Decision: PostgreSQL only for the MVP.** QR token nonces and expiry live in the `qr_tokens` table, so checking and consuming a token happens in the same transaction as the dispense. Notifications are written to the `notifications` table and sent by a background worker inside the Go service. Redis and a message broker are listed as a future scaling option if load tests (AT-09, NFR-PERF01) show a bottleneck.

---

## Summary of the selected stack

| Layer | Technology |
|---|---|
| Web (doctor, pharmacist, admin) | Next.js 15 (App Router), TypeScript, Tailwind CSS |
| Mobile (patient) | React Native + Expo, TypeScript |
| Backend | Go 1.23, chi router, sqlc/pgx, golang-migrate |
| Database | PostgreSQL 16 |
| EDS | NCALayer (client) + NCANode (server, Docker) |
| Auth | Short-lived JWT access token + rotating refresh token (httpOnly cookie on web, SecureStore on mobile) |
| API contract | OpenAPI 3.1, Swagger UI |
| Caching / async | None in MVP: PostgreSQL tables + background worker in the Go service |
| Infra | Docker Compose on a VPS, Caddy (reverse proxy, TLS, rate limiting), GitHub Actions CI |
| Testing | Go `testing` + testcontainers, Playwright (web E2E), k6 (load) |

*Versions will be pinned in `config/` and dependency files once the code skeleton is committed.*
