# Tests

- `e2e/` - Playwright scenarios (doctor issue, pharmacist dispense)
- `load/` - k6 scripts (AT-09 concurrency, NFR-PERF01)
- `security/` - ZAP / gitleaks configs
- `evaluation/` - evaluation study materials (no personal data)

Go unit and integration tests live next to the code (`*_test.go`). See [docs/09-verification-and-evaluation-plan.md](../docs/09-verification-and-evaluation-plan.md).
