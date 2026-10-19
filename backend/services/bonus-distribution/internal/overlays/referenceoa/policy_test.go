package referenceoa

import (
	"context"
	"testing"

	"omsbizfoundry/bonus-distribution/internal/bonus"
)

func TestEvaluatorUsesExactReferenceOAFormulaAndHalfUpRounding(t *testing.T) {
	evaluator := Evaluator{}
	request := bonus.PolicyRequest{Key: PolicyKey, Version: PolicyVersion, RateBasisPoints: 1000, Rounding: bonus.RoundingHalfUp}
	snapshot, amount, err := evaluator.Evaluate(context.Background(), bonus.TypeBusiness, 1_000_000, "CNY", request)
	if err != nil {
		t.Fatal(err)
	}
	if amount != 93_897 {
		t.Fatalf("base 1000000 at 10%% / 1.065 must round to 93897, got %d", amount)
	}
	if snapshot.Key != PolicyKey || snapshot.Version != PolicyVersion || snapshot.ParametersDigest == "" {
		t.Fatalf("unexpected policy snapshot: %#v", snapshot)
	}

	_, tiny, err := evaluator.Evaluate(context.Background(), bonus.TypeProduction, 1065, "CNY", bonus.PolicyRequest{Key: PolicyKey, Version: PolicyVersion, RateBasisPoints: 1, Rounding: bonus.RoundingHalfUp})
	if err != nil || tiny != 0 {
		t.Fatalf("exact sub-minor result must round deterministically to zero: %d %v", tiny, err)
	}
}

func TestEvaluatorRejectsUnknownOrFloatLikePolicyInputs(t *testing.T) {
	evaluator := Evaluator{}
	cases := []bonus.PolicyRequest{
		{Key: "arbitrary-formula", Version: PolicyVersion, RateBasisPoints: 1000, Rounding: bonus.RoundingHalfUp},
		{Key: PolicyKey, Version: "latest", RateBasisPoints: 1000, Rounding: bonus.RoundingHalfUp},
		{Key: PolicyKey, Version: PolicyVersion, RateBasisPoints: 10001, Rounding: bonus.RoundingHalfUp},
		{Key: PolicyKey, Version: PolicyVersion, RateBasisPoints: 1000, Rounding: "bankers"},
	}
	for _, request := range cases {
		if _, _, err := evaluator.Evaluate(context.Background(), bonus.TypeBusiness, 1000, "CNY", request); err == nil {
			t.Fatalf("invalid policy must fail: %#v", request)
		}
	}
}
