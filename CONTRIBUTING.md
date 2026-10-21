# Contributing

## Accounts
- Each team member commits **from their own GitHub account**. Shared accounts are not used.
- Before the first commit, set your identity:
  ```bash
  git config user.name  "Firstname Lastname"
  git config user.email "your-github-email@example.com"
  ```

## Branches
- `main` is the reviewed baseline. Direct pushes are disabled (branch protection).
- One branch per YouTrack task: `<TASK-ID>-<short-description>`, e.g. `DIP-21-dispense-locking`.

## Commits
- Format: `<TASK-ID>: <what changed, imperative mood>`, e.g. `DIP-5: Add literature analysis table`.
- One coherent change per commit. Documentation and diagram changes are committed the same way as code.
- Never commit secrets, real personal data, or `.env` files.

## Pull requests
- Title: `DIP-21: Dispensing with row-level locking`.
- Use the PR template: what changed, linked task, how it was checked.
- At least **one review by another team member** before merge.
- After merge, add the PR link to the YouTrack task. Mark the task done only when its acceptance criteria are met.

## Releases for pre-defenses
- Tags: `pre-defense-1`, `pre-defense-2`, `final-pre-defense`, on `main`.
- Tags are never moved or deleted after submission.
  ```bash
  git tag -a pre-defense-1 -m "Pre-defense 1 submission"
  git push origin pre-defense-1
  ```

## Diagrams
- Edit sources in `diagrams/src/*.puml` and re-export:
  ```bash
  plantuml -tpng -o ../export diagrams/src/*.puml
  plantuml -tsvg -o ../export diagrams/src/*.puml
  ```
- Commit the source and the export together.
