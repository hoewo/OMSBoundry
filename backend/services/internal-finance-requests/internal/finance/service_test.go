package finance

import (
	"context"
	"math"
	"strings"
	"testing"
)

type submitReplayStore struct {
	Store
	current     Request
	calls       int
	input       SubmitInput
	requestHash string
}

type deleteDraftStore struct {
	Store
	input DeleteDraftInput
}

func (s *deleteDraftStore) DeleteDraft(_ context.Context, _ Actor, _ string, input DeleteDraftInput, _ string) (Request, error) {
	s.input = input
	return Request{Status: "draft"}, nil
}

func (s *submitReplayStore) Get(context.Context, string, string) (Request, error) {
	return s.current, nil
}

func (s *submitReplayStore) Submit(_ context.Context, _ Actor, _ string, input SubmitInput, requestHash string) (Request, error) {
	s.calls++
	s.input = input
	s.requestHash = requestHash
	return s.current, nil
}

func TestSubmitDelegatesAdvancedRevisionToStoreForIdempotentReplay(t *testing.T) {
	actor := Actor{ID: "22222222-2222-4222-8222-222222222222", TenantID: "11111111-1111-4111-8111-111111111111", CredentialID: "33333333-3333-4333-8333-333333333333"}
	input := SubmitInput{CommandID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", ExpectedRevision: 1}
	store := &submitReplayStore{current: Request{ID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", TenantID: actor.TenantID, ApplicantSubjectID: actor.ID, Status: "payable", Revision: 2}}
	service, err := NewService(store)
	if err != nil {
		t.Fatal(err)
	}
	value, err := service.Submit(context.Background(), actor, store.current.ID, input)
	if err != nil || value.Status != "payable" || store.calls != 1 {
		t.Fatalf("replayed submit=%#v calls=%d err=%v", value, store.calls, err)
	}
	if store.input.ValidatedDetails != nil {
		t.Fatalf("replay unexpectedly revalidated details: %#v", store.input.ValidatedDetails)
	}
	if store.requestHash != hash(input) {
		t.Fatalf("replay hash=%q want=%q", store.requestHash, hash(input))
	}
}

func TestDeleteDraftRequiresAndNormalizesReason(t *testing.T) {
	actor := Actor{ID: "22222222-2222-4222-8222-222222222222", TenantID: "11111111-1111-4111-8111-111111111111", CredentialID: "33333333-3333-4333-8333-333333333333"}
	store := &deleteDraftStore{}
	service, err := NewService(store)
	if err != nil {
		t.Fatal(err)
	}
	id := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	command := "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	if _, err = service.DeleteDraft(context.Background(), actor, id, DeleteDraftInput{CommandID: command, Confirmation: "DELETE"}); err == nil {
		t.Fatal("draft deletion without a reason was accepted")
	}
	if _, err = service.DeleteDraft(context.Background(), actor, id, DeleteDraftInput{CommandID: command, Confirmation: "DELETE", Reason: "  重复草稿  "}); err != nil {
		t.Fatal(err)
	}
	if store.input.Reason != "重复草稿" {
		t.Fatalf("normalized reason=%q", store.input.Reason)
	}
}

func TestTenantWideDeleteRequiresObservedRevision(t *testing.T) {
	actor := Actor{ID: "22222222-2222-4222-8222-222222222222", TenantID: "11111111-1111-4111-8111-111111111111", CredentialID: "33333333-3333-4333-8333-333333333333"}
	store := &deleteDraftStore{}
	service, _ := NewService(store)
	id := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	command := "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	input := DeleteDraftInput{CommandID: command, Confirmation: "DELETE", Reason: "重复付款申请", TenantWide: true}
	if _, err := service.DeleteDraft(context.Background(), actor, id, input); err == nil {
		t.Fatal("tenant-wide deletion without expected revision was accepted")
	}
	input.ExpectedRevision = 3
	if _, err := service.DeleteDraft(context.Background(), actor, id, input); err != nil {
		t.Fatal(err)
	}
	if !store.input.TenantWide || store.input.ExpectedRevision != 3 || store.input.Reason != "重复付款申请" {
		t.Fatalf("trusted tenant-wide delete input=%#v", store.input)
	}
}

func TestTotalLinesRejectsOverflow(t *testing.T) {
	if value, err := totalLines([]Line{{AmountMinor: math.MaxInt64 - 1}, {AmountMinor: 1}}); err != nil || value != math.MaxInt64 {
		t.Fatalf("valid boundary total=%d err=%v", value, err)
	}
	if _, err := totalLines([]Line{{AmountMinor: math.MaxInt64}, {AmountMinor: 1}}); err == nil {
		t.Fatal("overflowing exact-money total must be rejected")
	}
}

func TestNormalizePaymentDetailsSupportsCashAndBankTransfer(t *testing.T) {
	base := func() DetailSnapshot {
		return DetailSnapshot{
			Applicant: ReferenceSnapshot{ID: "11111111-1111-4111-8111-111111111111", Revision: 1, Digest: "sha256:" + strings.Repeat("a", 64), DisplayName: "申请人"},
			Payee:     PayeeSnapshot{Kind: "one_time", EntityKind: "organization", DisplayName: "收款单位"},
			Lines:     []Line{{ID: "22222222-2222-4222-8222-222222222222", Purpose: "付款说明", AmountMinor: 10000}},
		}
	}

	cash := base()
	cash.Payee.PaymentMethod = "cash"
	cash.Payee.BankName = "不应保留的银行"
	cash.Payee.AccountNumber = "6222000012345678"
	if err := normalizeDetails("payment", &cash, true); err != nil {
		t.Fatalf("cash payment rejected: %v", err)
	}
	if cash.Payee.BankName != "" || cash.Payee.AccountNumber != "" || cash.Payee.AccountLast4 != "" {
		t.Fatalf("cash payment retained bank details: %#v", cash.Payee)
	}

	bank := base()
	bank.Payee.PaymentMethod = "bank_transfer"
	bank.Payee.BankName = "测试银行"
	bank.Payee.AccountNumber = "6222000012345678"
	if err := normalizeDetails("payment", &bank, true); err != nil {
		t.Fatalf("bank transfer rejected: %v", err)
	}
	if bank.Payee.AccountLast4 != "5678" {
		t.Fatalf("bank transfer last4 = %q", bank.Payee.AccountLast4)
	}

	missingAccount := base()
	missingAccount.Payee.PaymentMethod = "bank_transfer"
	if err := normalizeDetails("payment", &missingAccount, true); err == nil {
		t.Fatal("bank transfer without an account was accepted")
	}
}

func TestNormalizePaymentDetailsAcceptsConfirmedInvoiceSnapshot(t *testing.T) {
	details := DetailSnapshot{
		Applicant: ReferenceSnapshot{ID: "11111111-1111-4111-8111-111111111111", Revision: 1, Digest: "sha256:" + strings.Repeat("a", 64), DisplayName: "申请人"},
		Payee:     PayeeSnapshot{Kind: "one_time", EntityKind: "organization", DisplayName: "销售方公司", PaymentMethod: "cash"},
		Lines: []Line{{
			ID:           "22222222-2222-4222-8222-222222222222",
			Purpose:      "采购付款",
			AmountMinor:  268000,
			BusinessDate: "2026-11-06",
			Invoice: &InvoiceSnapshot{
				Code:        " 2493-2000 ",
				Number:      " 000038888512 ",
				IssuerName:  " 销售方公司 ",
				InvoiceDate: "2026-11-05",
				AmountMinor: 268000,
			},
		}},
	}
	if err := normalizeDetails("payment", &details, true); err != nil {
		t.Fatalf("payment invoice snapshot rejected: %v", err)
	}
	if got := details.Lines[0].Invoice; got.Code != "24932000" || got.Number != "000038888512" || got.IssuerName != "销售方公司" {
		t.Fatalf("payment invoice snapshot was not normalized: %#v", got)
	}

	details.Lines[0].Invoice.AmountMinor--
	if err := normalizeDetails("payment", &details, true); err == nil {
		t.Fatal("payment invoice amount different from its request line was accepted")
	}
}

func TestNormalizePaymentDetailsAcceptsFullyDigitalInvoiceWithoutLegacyCode(t *testing.T) {
	details := DetailSnapshot{
		Applicant: ReferenceSnapshot{ID: "11111111-1111-4111-8111-111111111111", Revision: 1, Digest: "sha256:" + strings.Repeat("a", 64), DisplayName: "申请人"},
		Payee:     PayeeSnapshot{Kind: "one_time", EntityKind: "organization", DisplayName: "销售方公司", PaymentMethod: "cash"},
		Lines: []Line{{
			ID: "22222222-2222-4222-8222-222222222222", Purpose: "采购付款", AmountMinor: 268000,
			Invoice: &InvoiceSnapshot{Number: " 24932000000038888512 ", IssuerName: "销售方公司", InvoiceDate: "2026-11-05", AmountMinor: 268000},
		}},
	}
	if err := normalizeDetails("payment", &details, true); err != nil {
		t.Fatalf("fully digital invoice without legacy code rejected: %v", err)
	}
	if got := details.Lines[0].Invoice; got.Code != "" || got.Number != "24932000000038888512" {
		t.Fatalf("fully digital invoice identity was not normalized: %#v", got)
	}
	details.Lines[0].Invoice.Number = ""
	if err := normalizeDetails("payment", &details, true); err == nil {
		t.Fatal("invoice snapshot without invoice number was accepted")
	}
}

func TestNormalizeReimbursementDetailsAcceptsConfirmedInvoiceSnapshot(t *testing.T) {
	person := ReferenceSnapshot{ID: "11111111-1111-4111-8111-111111111111", Revision: 1, Digest: "sha256:" + strings.Repeat("a", 64), DisplayName: "申请人"}
	details := DetailSnapshot{
		Applicant: person,
		Payee:     PayeeSnapshot{Kind: "person", Reference: person, DisplayName: "申请人"},
		Lines: []Line{{
			ID:           "22222222-2222-4222-8222-222222222222",
			Purpose:      "差旅报销",
			AmountMinor:  268000,
			BusinessDate: "2026-11-06",
			CategoryKey:  "travel",
			CategoryName: "差旅费",
			CategoryRev:  1,
			Evidence:     []EvidenceRef{{FileID: "33333333-3333-4333-8333-333333333333", Version: 1, Purpose: "invoice"}},
			Invoice: &InvoiceSnapshot{
				Code:        " 2493-2000 ",
				Number:      " 000038888512 ",
				IssuerName:  " 销售方公司 ",
				InvoiceDate: "2026-11-05",
				AmountMinor: 268000,
			},
		}},
	}
	if err := normalizeDetails("reimbursement", &details, true); err != nil {
		t.Fatalf("reimbursement invoice snapshot rejected: %v", err)
	}
	if got := details.Lines[0].Invoice; got.Code != "24932000" || got.Number != "000038888512" || got.IssuerName != "销售方公司" {
		t.Fatalf("reimbursement invoice snapshot was not normalized: %#v", got)
	}
}
