package accesscontract_test

import (
	"slices"
	"testing"
)

func TestStatisticsExportCapabilitiesDeclareExactOwnerResources(t *testing.T) {
	for _, v := range []struct{ file, action, resource string }{
		{"money-flow-ledger", "money-flow-ledger.export", "cash_transaction"},
		{"billing-invoice", "billing-invoice.export", "invoice_registration"},
	} {
		manifest := readStrict[capabilityManifest](t, "examples/"+v.file+"-capabilities.json")
		found := false
		for _, action := range manifest.Capabilities {
			if action.ID == v.action {
				found = action.Risk == "privileged" && slices.Contains(action.ResourceTypes, v.resource)
				if v.file == "money-flow-ledger" && !slices.Contains(action.ResourceTypes, "party") {
					t.Fatal("statistics export removed original party scope")
				}
			}
		}
		if !found {
			t.Fatalf("%s must declare exact %s export", v.file, v.resource)
		}
	}
}
