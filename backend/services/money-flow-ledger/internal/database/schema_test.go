package database

import (
	"slices"
	"strings"
	"testing"
)

func TestClassifySchema(t *testing.T) {
	t.Parallel()
	expected := []string{"000001.sql", "000002.sql", "000003.sql"}
	tests := []struct {
		name           string
		applied        []string
		tableExists    bool
		relations      int64
		wantState      SchemaState
		wantPending    []string
		wantUnknown    []string
		wantReady      bool
		wantReasonPart string
	}{
		{name: "empty", wantState: SchemaEmpty, wantPending: expected},
		{name: "empty migration table", tableExists: true, wantState: SchemaEmpty, wantPending: expected},
		{name: "empty migration table with unmanaged relations", tableExists: true, relations: 1, wantState: SchemaIncompatible, wantReasonPart: "user relations"},
		{name: "pending prefix", tableExists: true, applied: []string{"000001.sql"}, wantState: SchemaPending, wantPending: []string{"000002.sql", "000003.sql"}},
		{name: "current", tableExists: true, applied: expected, wantState: SchemaCurrent, wantPending: []string{}, wantReady: true},
		{name: "unknown", tableExists: true, applied: []string{"000001.sql", "999999.sql"}, wantState: SchemaIncompatible, wantUnknown: []string{"999999.sql"}, wantReasonPart: "unknown"},
		{name: "gap", tableExists: true, applied: []string{"000002.sql"}, wantState: SchemaIncompatible, wantReasonPart: "strict prefix"},
		{name: "unmanaged relations", relations: 2, wantState: SchemaIncompatible, wantReasonPart: "user relations"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status := classifySchema(expected, test.applied, test.tableExists, test.relations)
			if status.State != test.wantState || status.ServerReady != test.wantReady ||
				!slices.Equal(status.Pending, test.wantPending) || !slices.Equal(status.Unknown, test.wantUnknown) ||
				(test.wantReasonPart != "" && !strings.Contains(status.Reason, test.wantReasonPart)) {
				t.Fatalf("classifySchema() = %#v", status)
			}
		})
	}
}

func TestEmbeddedMigrationListIsOrdered(t *testing.T) {
	t.Parallel()
	versions, err := migrationVersions()
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 26 || versions[0] != "000001_bootstrap.sql" || versions[len(versions)-1] != "000026_internal_finance_payments.sql" {
		t.Fatalf("migrationVersions() = %#v", versions)
	}
	status := classifySchema(versions, append([]string(nil), versions...), true, 0)
	if status.State != SchemaCurrent || !status.ServerReady {
		t.Fatalf("current schema status = %#v", status)
	}
}
