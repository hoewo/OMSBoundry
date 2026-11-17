package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	contract "omsbizfoundry/invoice-recognition-contract"
	"omsbizfoundry/invoice-recognition/internal/auth"
	"omsbizfoundry/invoice-recognition/internal/recognition"
)

const (
	testTenant   = "11111111-1111-4111-8111-111111111111"
	testSubject  = "22222222-2222-4222-8222-222222222222"
	testProposal = "55555555-5555-4555-8555-555555555555"
	testToken    = "invoice-recognition-template-token"
)

type useCasesStub struct{}

func (useCasesStub) Ping(context.Context) error { return nil }
func (useCasesStub) Start(context.Context, recognition.Actor, recognition.StartInput) (contract.Proposal, error) {
	return contract.Proposal{}, nil
}
func (useCasesStub) Get(context.Context, string, string) (contract.Proposal, error) {
	return contract.Proposal{}, nil
}
func (useCasesStub) List(context.Context, string) ([]contract.Proposal, error) { return nil, nil }
func (useCasesStub) ReviewImage(_ context.Context, tenantID, proposalID string) (recognition.ReviewImage, error) {
	if tenantID != testTenant || proposalID != testProposal {
		return recognition.ReviewImage{}, recognition.ErrNotFound
	}
	return recognition.ReviewImage{MIMEType: "image/png", Content: []byte("review-page")}, nil
}
func (useCasesStub) Confirm(context.Context, recognition.Actor, recognition.ConfirmInput) (contract.Confirmation, error) {
	return contract.Confirmation{}, nil
}
func (useCasesStub) Discard(context.Context, recognition.Actor, recognition.DiscardInput) (contract.Proposal, error) {
	return contract.Proposal{}, nil
}

func TestReviewImageReturnsProtectedNoStoreImage(t *testing.T) {
	authenticator, err := auth.NewTemplateProvider(testTenant, testSubject, "Reviewer", testToken)
	if err != nil {
		t.Fatal(err)
	}
	server, err := New(useCasesStub{}, authenticator)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/proposals/"+testProposal+"/review-image", nil)
	request.Header.Set("Authorization", "Bearer "+testToken)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "image/png" || response.Header().Get("Cache-Control") != "no-store" || response.Body.String() != "review-page" {
		t.Fatalf("status=%d headers=%v body=%q", response.Code, response.Header(), response.Body.String())
	}
}

func TestReviewImageRequiresCredential(t *testing.T) {
	authenticator, err := auth.NewTemplateProvider(testTenant, testSubject, "Reviewer", testToken)
	if err != nil {
		t.Fatal(err)
	}
	server, err := New(useCasesStub{}, authenticator)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/proposals/"+testProposal+"/review-image", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
	}
}
