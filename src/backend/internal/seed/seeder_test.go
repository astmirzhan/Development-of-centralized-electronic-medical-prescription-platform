package seed

import "testing"

func TestEmbeddedDatasetsValid(t *testing.T) {
	icdCodes, err := loadJSON[[]icd10Seed]("data/icd10_codes.json")
	if err != nil {
		t.Fatalf("load icd10_codes.json: %v", err)
	}
	if len(icdCodes) < 20 {
		t.Fatalf("expected at least 20 ICD-10 codes, got %d", len(icdCodes))
	}

	medicines, err := loadJSON[[]medicineSeed]("data/medicines.json")
	if err != nil {
		t.Fatalf("load medicines.json: %v", err)
	}
	if len(medicines) < 15 {
		t.Fatalf("expected at least 15 INN medicines, got %d", len(medicines))
	}

	ddis, err := loadJSON[[]ddiSeed]("data/drug_interactions.json")
	if err != nil {
		t.Fatalf("load drug_interactions.json: %v", err)
	}
	if len(ddis) < 8 {
		t.Fatalf("expected at least 8 DDI entries, got %d", len(ddis))
	}

	severities := map[string]bool{}
	for _, d := range ddis {
		severities[d.SeverityLevel] = true
	}
	for _, expected := range []string{"critical", "moderate", "minor"} {
		if !severities[expected] {
			t.Fatalf("missing DDI severity %q in seed data", expected)
		}
	}
}
