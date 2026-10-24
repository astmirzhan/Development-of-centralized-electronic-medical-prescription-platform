# 11. Supervisor Feedback

| Field | Value |
|---|---|
| **Supervisor** | Хаймульдин Асқар Ғазизұлы |
| **Related task** | DIP-6 (YouTrack) |
| **Document version** | v0.1, October 2026 |

This document records the supervisor's comments on key deliverables, the team's decision on each comment, and the commit that applies the change. It is updated after every review checkpoint.

---

## 1. Review process

- **Checkpoints:** before each pre-defense (prob-period-1, pre-defense 2, final pre-defense) and after major design decisions.
- **What is reviewed:** proposal and scope (doc 01), requirements (doc 06), architecture and diagrams (doc 07), evaluation plan (doc 09), and, from the next stage, selected pull requests.
- **Format:** meeting or written comments. Each comment gets an ID (`SF-<checkpoint>-<n>`), a decision (accepted / partially accepted / rejected with reason), and a link to the commit or pull request that resolves it.
- **Owner:** Mysyrgaliyev Mirzhan (team lead) records the feedback; the author of the affected document applies the change.

## 2. Checkpoint 1: probationary period (prob-period-1)

| Date | Materials sent | Status |
|---|---|---|
| 24 Oct 2026 | Repository snapshot at tag `prob-period-1`: docs 01–10, diagrams, prototype, and YouTrack project | Materials prepared for review; supervisor feedback has not yet been received |

### Supervisor comments

Supervisor feedback has not been received yet. This section will be completed after the review.

| ID | Deliverable | Comment | Decision | Change (commit / PR) |
|---|---|---|---|---|
| — | — | Feedback pending | Pending supervisor review | To be recorded after review |

### Checkpoint 1 summary

All deliverables required for the probationary period were submitted on time at tag `prob-period-1`: project proposal and scope (01), evidence of relevance with a user survey (02), literature review (03), existing systems comparison (04), technology comparison (05), requirements specification (06), architecture and diagrams with editable sources (07), UX prototype (08), verification and evaluation plan (09), and project management plan with risk register (10). Work is tracked in YouTrack (project DIP) and every commit is linked to a task ID.

Before submission the team carried out its own review of all documents. The findings and planned actions are listed in section 3, so the supervisor can confirm priorities at the review meeting.

## 3. Internal team review (before checkpoint 1)

The team reviewed the submitted documents and identified the following issues for discussion at the checkpoint. These are team findings, not supervisor comments.

| ID | Deliverable | Issue found | Planned action | Task |
|---|---|---|---|---|
| TR-1 | Docs 06, 09 | Requirement and test identifiers are inconsistent across the documents (for example, `FR-DOC-01` / `TC-E2E-01` versus `FR-D04` / `AT-01`). Some proposal and architecture features, including EDS signing, partial dispensing, and per-item QR, are not fully reflected in doc 06. | Reconcile requirements and evaluation cases with docs 01 and 07; use one identifier scheme and record any scope decisions. | Next stage |
| TR-2 | Docs 02, 06 | Survey findings identify potential changes C1–C6, including reducing alert fatigue, simplifying the pharmacist signing flow, and supporting IIN plus code fallback. These changes are not yet consistently reflected in the requirements. | Review each finding with the team, then incorporate accepted changes into doc 06 and its change log. | Next stage |
| TR-3 | Repository process | Some checkpoint-stage changes were committed directly to `main`, while CONTRIBUTING.md specifies task branches and pull requests with peer review. | Use task branches and reviewed pull requests for subsequent work. | Next stage |
| TR-4 | YouTrack | The Priority field is being used for sprint names, and the backlog does not consistently expose due dates, although doc 10 requires priority and deadline fields. | Restore priority values and add/use a Due Date field; check the backlog against doc 10. | Next stage |
| TR-5 | Docs 01, 06, 07 | The proposal and architecture describe clinical allergy and drug-interaction warnings as planned functionality, while the requirements should make their priority and acceptance criteria explicit. | Confirm the MoSCoW priority and add traceable acceptance criteria, or document a scope change. | Next stage |
| TR-6 | Docs 01, 07, 09 | Security and evaluation claims include EDS verification, QR anti-reuse, and atomic dispensing. These need traceable requirements and matching verification cases before implementation. | Add cross-references from each security requirement to its test case and evaluation method. | Next stage |

## 4. Change log

| Date | Change |
|---|---|
| 24 Oct 2026 | Checkpoint 1 materials recorded; supervisor feedback marked pending. Internal team review findings documented separately from supervisor comments. |
