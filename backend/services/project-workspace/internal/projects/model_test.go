package projects

import (
	"testing"
	"time"
)

func TestNormalizeCreate(t *testing.T) {
	start := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	value, err := NormalizeCreate(CreateInput{
		CommandID:        "AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA",
		ProjectNumber:    " OA-001 ",
		Name:             " 演示设计项目 ",
		Summary:          " 首个闭环 ",
		PlannedStartDate: &start,
		PlannedEndDate:   &end,
		ProjectTypeKeys:  []string{" Design "},
	})
	if err != nil {
		t.Fatalf("NormalizeCreate() error = %v", err)
	}
	if value.CommandID != "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa" || value.ProjectNumber != "OA-001" || value.Name != "演示设计项目" || value.ProjectTypeKeys[0] != "design" {
		t.Fatalf("NormalizeCreate() = %#v", value)
	}
}

func TestNormalizeCreateRejectsInvalidInputs(t *testing.T) {
	base := CreateInput{CommandID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", ProjectNumber: "OA-001", Name: "项目", ProjectTypeKeys: []string{"design"}}
	tests := []struct {
		name   string
		mutate func(*CreateInput)
	}{
		{"command", func(v *CreateInput) { v.CommandID = "not-a-uuid" }},
		{"number", func(v *CreateInput) { v.ProjectNumber = "bad number" }},
		{"name", func(v *CreateInput) { v.Name = "" }},
		{"type", func(v *CreateInput) { v.ProjectTypeKeys = []string{"Design Work"} }},
		{"duplicate type", func(v *CreateInput) { v.ProjectTypeKeys = []string{"design", "design"} }},
		{"date order", func(v *CreateInput) {
			start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
			end := start.AddDate(0, 0, -1)
			v.PlannedStartDate, v.PlannedEndDate = &start, &end
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := base
			input.ProjectTypeKeys = append([]string(nil), base.ProjectTypeKeys...)
			test.mutate(&input)
			if _, err := NormalizeCreate(input); err == nil {
				t.Fatal("NormalizeCreate() error = nil")
			}
		})
	}
}

func TestNormalizePackageAllowsUnnumberedPackageButRejectsInvalidNumber(t *testing.T) {
	value, err := NormalizePackage(PackageCreateInput{CommandID: "AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA", Name: " 华东项目包 ", ProjectTypeKeys: []string{" Design "}})
	if err != nil || value.ProjectNumber != "" || value.Name != "华东项目包" || value.ProjectTypeKeys[0] != "design" {
		t.Fatalf("NormalizePackage()=%#v error=%v", value, err)
	}
	if _, err := NormalizePackage(PackageCreateInput{CommandID: value.CommandID, ProjectNumber: "bad number", Name: "项目包", ProjectTypeKeys: []string{"design"}}); err == nil {
		t.Fatal("invalid optional package number was accepted")
	}
}

func TestValidResourceUsesClosedTypeCatalog(t *testing.T) {
	if !ValidResource("party-directory", "party", "contractor-party") {
		t.Fatal("expected project party relationship to be valid")
	}
	if !ValidResource("contract-ledger", "contract", "primary-contract") {
		t.Fatal("expected contract link to be valid")
	}
	if !ValidResource("business-engagement", "business-engagement", "engagement-materials") {
		t.Fatal("expected business engagement material source to be valid")
	}
	if ValidResource("party-directory", "contract", "contractor-party") || ValidResource("contract-ledger", "obligation", "wrong-owner") || ValidResource("unknown", "contract", "primary") {
		t.Fatal("unexpected resource combination accepted")
	}
}
