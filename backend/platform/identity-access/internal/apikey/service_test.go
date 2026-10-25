package apikey

import "testing"

func TestAllowedRequestActionCoversPublishedReimbursementLifecycle(t *testing.T) {
	for _, action := range []string{
		"open-api.reimbursement.create",
		"open-api.reimbursement.update",
		"open-api.reimbursement.submit",
		"open-api.reimbursement.withdraw",
		"open-api.reimbursement.restore",
		"open-api.reimbursement.delete",
		"open-api.reimbursement.read",
		"open-api.reimbursement.list",
	} {
		if !allowedRequestAction(action) {
			t.Errorf("published action %q is not accepted", action)
		}
	}
	if allowedRequestAction("open-api.reimbursement.pay") {
		t.Fatal("employee API must not accept payment confirmation")
	}
}
