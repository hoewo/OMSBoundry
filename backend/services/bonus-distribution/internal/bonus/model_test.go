package bonus

import (
	"errors"
	"math"
	"testing"
)

func TestExactRatioAmountUsesIntegerHalfUpArithmetic(t *testing.T) {
	cases := []struct {
		amount int64
		ratio  int64
		want   int64
	}{
		{100_000, 6000, 60_000},
		{1, 5000, 1},
		{1, 4999, 0},
		{999, 3333, 333},
	}
	for _, item := range cases {
		got, err := exactRatioAmount(item.amount, item.ratio)
		if err != nil || got != item.want {
			t.Fatalf("exactRatioAmount(%d,%d)=%d,%v want %d", item.amount, item.ratio, got, err, item.want)
		}
	}
	if _, err := exactRatioAmount(math.MaxInt64, 10000); err != nil {
		t.Fatalf("big integer intermediate must avoid overflow: %v", err)
	}
	if _, err := exactRatioAmount(1, 0); !errors.Is(err, ErrAllocationInvalid) {
		t.Fatalf("zero ratio must fail: %v", err)
	}
}

func TestAllocationNormalizationRejectsAmbiguousOrOverAllocatedRequests(t *testing.T) {
	base := []AllocationRequest{
		{Person: PersonRef{ID: "00000000-0000-4000-8000-000000000001", Revision: 1}, Assignment: AssignmentRef{ID: "00000000-0000-4000-8000-000000000011", Revision: 1}, RatioBasisPoints: 6000, Reason: "经营负责人"},
		{Person: PersonRef{ID: "00000000-0000-4000-8000-000000000002", Revision: 1}, Assignment: AssignmentRef{ID: "00000000-0000-4000-8000-000000000012", Revision: 1}, RatioBasisPoints: 4000, Reason: "经营参与人"},
	}
	if _, err := normalizeAllocationRequests(base); err != nil {
		t.Fatalf("valid complete allocation failed: %v", err)
	}
	over := append([]AllocationRequest(nil), base...)
	over[1].RatioBasisPoints = 4001
	if _, err := normalizeAllocationRequests(over); !errors.Is(err, ErrAllocationInvalid) {
		t.Fatalf("over allocation must fail: %v", err)
	}
	duplicate := append([]AllocationRequest(nil), base...)
	duplicate[1].Person = duplicate[0].Person
	if _, err := normalizeAllocationRequests(duplicate); !errors.Is(err, ErrAllocationInvalid) {
		t.Fatalf("duplicate person across roles must fail closed in v1: %v", err)
	}
}
