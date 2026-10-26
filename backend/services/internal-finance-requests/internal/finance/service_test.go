package finance

import (
	"math"
	"strings"
	"testing"
)

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
