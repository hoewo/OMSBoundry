package peoplegroups

import (
	"context"
	"errors"
	"testing"
)

const (
	referenceTenant  = "11111111-1111-4111-8111-111111111111"
	referenceSubject = "22222222-2222-4222-8222-222222222222"
)

type subjectReferenceStub struct {
	value IAMSubjectReference
	err   error
}

func (s subjectReferenceStub) ValidateIAMSubject(context.Context, string, string) (IAMSubjectReference, error) {
	return s.value, s.err
}

func TestValidateIAMSubjectKeepsSourceStatusAndRevision(t *testing.T) {
	service := &Service{iamSubjectRefs: subjectReferenceStub{value: IAMSubjectReference{
		SchemaVersion: "1", SourceApplication: "identity-access", ID: referenceSubject, TenantID: referenceTenant,
		Status: "disabled", Revision: 4,
	}}}
	status, revision, err := service.validateIAMSubject(t.Context(), referenceTenant, referenceSubject)
	if err != nil || status != "disabled" || revision != 4 {
		t.Fatalf("status=%q revision=%d err=%v", status, revision, err)
	}
}

func TestValidateIAMSubjectRejectsCrossTenantAndUnavailable(t *testing.T) {
	service := &Service{iamSubjectRefs: subjectReferenceStub{value: IAMSubjectReference{
		SchemaVersion: "1", SourceApplication: "identity-access", ID: referenceSubject,
		TenantID: "33333333-3333-4333-8333-333333333333", Status: "active", Revision: 1,
	}}}
	if _, _, err := service.validateIAMSubject(t.Context(), referenceTenant, referenceSubject); !errors.Is(err, ErrSubjectReferenceInvalid) {
		t.Fatalf("cross tenant error=%v", err)
	}
	service.iamSubjectRefs = subjectReferenceStub{err: ErrSubjectReferenceUnavailable}
	if _, _, err := service.validateIAMSubject(t.Context(), referenceTenant, referenceSubject); !errors.Is(err, ErrSubjectReferenceUnavailable) {
		t.Fatalf("unavailable error=%v", err)
	}
}

func TestValidateIAMSubjectRejectsUnverifiedLink(t *testing.T) {
	service := &Service{}
	if _, _, err := service.validateIAMSubject(t.Context(), referenceTenant, referenceSubject); !errors.Is(err, ErrSubjectReferenceInvalid) {
		t.Fatalf("unverified subject link error=%v", err)
	}
	if status, revision, err := service.validateIAMSubject(t.Context(), referenceTenant, ""); err != nil || status != "" || revision != 0 {
		t.Fatalf("empty optional subject status=%q revision=%d err=%v", status, revision, err)
	}
}
