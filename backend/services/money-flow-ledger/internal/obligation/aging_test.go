package obligation

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCurrentAgingClassifiesBoundariesFiltersAndSumsExactly(t *testing.T) {
	repository := &repositoryStub{listResult: []Obligation{
		agingObligation("00000000-0000-4000-8000-000000000009", DirectionReceivable, "CNY", "2026-12-31", "0.1", StatusOpen),
		agingObligation("00000000-0000-4000-8000-000000000008", DirectionReceivable, "CNY", "2027-01-01", "0.2", StatusOpen),
		agingObligation("00000000-0000-4000-8000-000000000007", DirectionReceivable, "CNY", "2026-12-30", "1", StatusOpen),
		agingObligation("00000000-0000-4000-8000-000000000006", DirectionReceivable, "CNY", "2026-12-01", "30", StatusOpen),
		agingObligation("00000000-0000-4000-8000-000000000005", DirectionReceivable, "CNY", "2026-11-30", "31", StatusOpen),
		agingObligation("00000000-0000-4000-8000-000000000004", DirectionReceivable, "CNY", "2026-11-01", "60", StatusOpen),
		agingObligation("00000000-0000-4000-8000-000000000003", DirectionReceivable, "CNY", "2026-10-31", "61", StatusOpen),
		agingObligation("00000000-0000-4000-8000-000000000002", DirectionReceivable, "CNY", "2026-10-02", "90", StatusOpen),
		agingObligation("00000000-0000-4000-8000-000000000001", DirectionReceivable, "CNY", "2026-10-01", "91.000001", StatusOpen),
		agingObligation("10000000-0000-4000-8000-000000000001", DirectionPayable, "CNY", "2026-12-30", "5", StatusOpen),
		agingObligation("20000000-0000-4000-8000-000000000001", DirectionReceivable, "USD", "2026-12-30", "7", StatusOpen),
		agingObligation("30000000-0000-4000-8000-000000000001", DirectionReceivable, "CNY", "2026-01-01", "0.000000", StatusOpen),
		agingObligation("30000000-0000-4000-8000-000000000002", DirectionReceivable, "CNY", "2026-01-01", "999", StatusDraft),
		agingObligation("30000000-0000-4000-8000-000000000003", DirectionReceivable, "CNY", "2026-01-01", "999", StatusClosed),
	}}
	service := NewService(repository)
	service.now = func() time.Time { return time.Date(2026, 9, 19, 23, 59, 0, 0, time.UTC) }

	report, err := service.CurrentAging(context.Background(), CurrentAgingInput{AsOfDate: "2026-12-31"})
	if err != nil {
		t.Fatalf("CurrentAging() error = %v", err)
	}
	if report.AsOfDate != "2026-12-31" || len(report.Items) != 11 || len(report.Summaries) != 3 {
		t.Fatalf("report = %#v", report)
	}
	wantBuckets := []AgingBucket{
		AgingBucketNotDue, AgingBucketNotDue,
		AgingBucketDays1To30, AgingBucketDays1To30,
		AgingBucketDays31To60, AgingBucketDays31To60,
		AgingBucketDays61To90, AgingBucketDays61To90,
		AgingBucketDays91Plus,
	}
	wantDays := []int{0, 0, 30, 1, 60, 31, 90, 61, 91}
	for i := range wantBuckets {
		if report.Items[i].Bucket != wantBuckets[i] || report.Items[i].DaysPastDue != wantDays[i] {
			t.Fatalf("item[%d] = %#v", i, report.Items[i])
		}
	}
	receivableCNY := report.Summaries[0]
	if receivableCNY.Direction != DirectionReceivable || receivableCNY.Currency != "CNY" ||
		receivableCNY.ObligationCount != 9 || receivableCNY.OverdueCount != 7 ||
		receivableCNY.OutstandingAmount != "364.300001" || receivableCNY.OverdueAmount != "364.000001" ||
		receivableCNY.NotDueAmount != "0.3" || receivableCNY.Days1To30Amount != "31" ||
		receivableCNY.Days31To60Amount != "91" || receivableCNY.Days61To90Amount != "151" ||
		receivableCNY.Days91PlusAmount != "91.000001" {
		t.Fatalf("receivable CNY summary = %#v", receivableCNY)
	}
	if report.Summaries[1].Direction != DirectionReceivable || report.Summaries[1].Currency != "USD" ||
		report.Summaries[1].OutstandingAmount != "7" || report.Summaries[2].Direction != DirectionPayable ||
		report.Summaries[2].Currency != "CNY" || report.Summaries[2].OutstandingAmount != "5" {
		t.Fatalf("separate summaries = %#v", report.Summaries)
	}
}

func TestCurrentAgingDefaultsToTodayAndRejectsPastOrInvalidDates(t *testing.T) {
	service := NewService(&repositoryStub{})
	service.now = func() time.Time { return time.Date(2026, 9, 20, 1, 2, 3, 0, time.UTC) }

	report, err := service.CurrentAging(context.Background(), CurrentAgingInput{})
	if err != nil || report.AsOfDate != "2026-09-20" || !report.GeneratedAt.Equal(service.now()) {
		t.Fatalf("default report = %#v, %v", report, err)
	}
	for _, value := range []string{"2026-09-19", "2026-02-30", "20-09-2026", "2026-9-20"} {
		_, err := service.CurrentAging(context.Background(), CurrentAgingInput{AsOfDate: value})
		var validation *ValidationError
		if !errors.As(err, &validation) || validation.Field != "as_of_date" {
			t.Fatalf("date %q error = %v", value, err)
		}
	}
}

func TestCurrentAgingDoesNotHideRepositoryOrStoredFactErrors(t *testing.T) {
	repositoryError := errors.New("database unavailable")
	service := NewService(&repositoryStub{listErr: repositoryError})
	service.now = func() time.Time { return time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC) }
	if _, err := service.CurrentAging(context.Background(), CurrentAgingInput{}); !errors.Is(err, repositoryError) {
		t.Fatalf("repository error = %v", err)
	}

	service = NewService(&repositoryStub{listResult: []Obligation{
		agingObligation("00000000-0000-4000-8000-000000000001", DirectionReceivable, "CNY", "invalid", "1", StatusOpen),
	}})
	service.now = func() time.Time { return time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC) }
	if _, err := service.CurrentAging(context.Background(), CurrentAgingInput{}); err == nil {
		t.Fatal("invalid stored due date error = nil")
	}

	service = NewService(&repositoryStub{listResult: []Obligation{
		agingObligation("00000000-0000-4000-8000-000000000001", DirectionReceivable, "CNY", "2026-09-20", "not-money", StatusOpen),
	}})
	service.now = func() time.Time { return time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC) }
	if _, err := service.CurrentAging(context.Background(), CurrentAgingInput{}); err == nil {
		t.Fatal("invalid stored amount error = nil")
	}
}

func agingObligation(id string, direction Direction, currency, dueOn, outstanding, status string) Obligation {
	return Obligation{
		ID: id, Direction: direction, Title: id,
		Currency: currency, DueOn: dueOn, OutstandingAmount: outstanding, Status: status,
	}
}
