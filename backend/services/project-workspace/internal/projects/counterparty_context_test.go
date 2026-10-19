package projects

import (
	"context"
	"strings"
	"testing"
)

func TestCounterpartyContextValidationAndNoMasterDependency(t *testing.T) {
	base := CounterpartyContextInput{CommandID: testCommand, ExpectedRevision: 3, Reason: "修改项目甲方信息", CounterpartyContact: CounterpartyContact{Name: "  测试甲方  ", ContactName: " 李工 ", ContactPhone: " +86 (0571) 123-456 "}}
	normalized, err := NormalizeCounterpartyContext(base)
	if err != nil || normalized.Name != "测试甲方" || normalized.ContactPhone != "+86 (0571) 123-456" {
		t.Fatalf("normalize=%#v err=%v", normalized, err)
	}
	for _, edit := range []func(*CounterpartyContextInput){func(v *CounterpartyContextInput) { v.Name = " " }, func(v *CounterpartyContextInput) { v.Name = strings.Repeat("甲", 201) }, func(v *CounterpartyContextInput) { v.ContactName = strings.Repeat("名", 81) }, func(v *CounterpartyContextInput) { v.ContactPhone = "abc" }, func(v *CounterpartyContextInput) { v.ExpectedRevision = 0 }, func(v *CounterpartyContextInput) { v.CommandID = "bad" }, func(v *CounterpartyContextInput) { v.Name = "甲\n方" }, func(v *CounterpartyContextInput) { v.Reason = "" }} {
		in := base
		edit(&in)
		if _, err := NormalizeCounterpartyContext(in); err == nil {
			t.Fatalf("accepted %#v", in)
		}
	}
	store := &storeStub{}
	refs := &referencesStub{peopleErr: ErrReferenceUnavailable, partyErr: ErrReferenceUnavailable}
	service, _ := NewService(store, refs)
	if _, err := service.ChangeCounterpartyContext(context.Background(), Actor{TenantID: testTenant}, testProject, base); err != nil || store.contextCalls != 1 || store.contextInput.Name != "测试甲方" || refs.peopleCalls != 0 || refs.partyCalls != 0 {
		t.Fatalf("err=%v writes=%d", err, store.contextCalls)
	}
	if _, err := service.ChangeCounterpartyContext(context.Background(), Actor{TenantID: testTenant}, "invalid", base); err == nil || store.contextCalls != 1 {
		t.Fatal("invalid id reached store")
	}
}
