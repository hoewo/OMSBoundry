package payroll

import (
	"math"
	"testing"
)

func TestCalculatePayrollLineUsesIntegerRoundingAndSeparateEmployerCost(t *testing.T) {
	components := []ContributionComponent{
		{Key: "down", BaseKind: "social", IndividualRateBPS: 333, EmployerRateBPS: 0, BaseFloorMinor: 0, BaseCapMinor: 0, RoundingMode: "down"},
		{Key: "half", BaseKind: "social", IndividualRateBPS: 5000, EmployerRateBPS: 5000, IndividualFixedMinor: 300, BaseFloorMinor: 0, BaseCapMinor: 0, RoundingMode: "half_up"},
		{Key: "up", BaseKind: "housing", IndividualRateBPS: 333, EmployerRateBPS: 0, BaseFloorMinor: 0, BaseCapMinor: 0, RoundingMode: "yuan_half_up"},
	}
	gross, details, employee, employer, preTax, net, cost, err := CalculatePayrollLine(10001, 101, 101, 999, 100, []Adjustment{{AmountMinor: 1}, {AmountMinor: -1}}, components)
	if err != nil {
		t.Fatal(err)
	}
	if gross != 11000 || len(details) != 3 || details[0].IndividualMinor != 3 || details[1].IndividualMinor != 351 || details[1].EmployerMinor != 51 || details[2].IndividualMinor != 0 || details[1].IndividualFixedMinor != 300 {
		t.Fatalf("unexpected rounded details: gross=%d details=%#v", gross, details)
	}
	if employee != 354 || employer != 51 || preTax != 10646 || net != 10546 || cost != 11051 {
		t.Fatalf("unexpected totals employee=%d employer=%d preTax=%d net=%d cost=%d", employee, employer, preTax, net, cost)
	}
}

func TestCalculateContributionSupportsBeijingHousingYuanRounding(t *testing.T) {
	value, err := calculateContribution(3634800, 254000, 3634800, 500, "yuan_half_up")
	if err != nil || value != 181700 {
		t.Fatalf("beijing housing contribution=%d err=%v", value, err)
	}
	value, err = calculateContribution(3634800, 254000, 3634800, 1200, "yuan_half_up")
	if err != nil || value != 436200 {
		t.Fatalf("beijing housing 12 percent contribution=%d err=%v", value, err)
	}
}

func TestCalculatePayrollLineRejectsOverflowAndUnboundedInputs(t *testing.T) {
	if _, _, _, _, _, _, _, err := CalculatePayrollLine(maxPayrollMinor+1, 0, 0, 0, 0, nil, nil); err == nil {
		t.Fatal("amount over supported exact range must fail")
	}
	if _, _, _, _, _, _, _, err := CalculatePayrollLine(maxPayrollMinor, 0, 0, maxPayrollMinor, 0, []Adjustment{{AmountMinor: math.MaxInt64}}, nil); err == nil {
		t.Fatal("overflowing adjustment must fail")
	}
	if _, err := calculateContribution(maxPayrollMinor, 0, maxPayrollMinor, 10001, "half_up"); err == nil {
		t.Fatal("invalid contribution rate must fail")
	}
}
