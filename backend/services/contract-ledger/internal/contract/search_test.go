package contract

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestSearchNormalizesFiltersBeforeRepository(t *testing.T) {
	repository := &repositoryStub{search: []Contract{{ID: testContractID, Title: "年度服务合同"}}}
	page, err := NewService(repository).Search(context.Background(), SearchContractsInput{
		Keyword: "  HT-001  ", Status: " ACTIVE ", Currency: " cny ",
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if repository.receivedSearch.Keyword != "HT-001" || repository.receivedSearch.Status != StatusActive ||
		repository.receivedSearch.Currency != "CNY" || repository.receivedSearch.Limit != 51 {
		t.Fatalf("normalized query = %#v", repository.receivedSearch)
	}
	if len(page.Contracts) != 1 || page.Keyword != "HT-001" || page.Status != StatusActive || page.Currency != "CNY" || page.NextCursor != "" {
		t.Fatalf("page = %#v", page)
	}
}

func TestSearchRejectsInvalidFiltersBeforeRepository(t *testing.T) {
	tests := []struct {
		name  string
		input SearchContractsInput
		field string
	}{
		{name: "keyword", input: SearchContractsInput{Keyword: strings.Repeat("界", 101)}, field: "q"},
		{name: "status", input: SearchContractsInput{Status: "archived"}, field: "status"},
		{name: "currency", input: SearchContractsInput{Currency: "CNYY"}, field: "currency"},
		{name: "cursor", input: SearchContractsInput{Cursor: "not-a-cursor"}, field: "cursor"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &repositoryStub{}
			_, err := NewService(repository).Search(context.Background(), test.input)
			var validation *ValidationError
			if !errors.As(err, &validation) || validation.Field != test.field {
				t.Fatalf("error = %v, want ValidationError field %q", err, test.field)
			}
			if repository.receivedSearch.Limit != 0 {
				t.Fatalf("invalid filter reached repository: %#v", repository.receivedSearch)
			}
		})
	}
}

func TestSearchUsesFilterBoundKeysetCursor(t *testing.T) {
	createdAt := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	values := make([]Contract, 51)
	for index := range values {
		values[index] = Contract{
			ID:        fmt.Sprintf("10000000-0000-4000-8000-%012d", index+1),
			Title:     fmt.Sprintf("合同 %d", index+1),
			CreatedAt: createdAt.Add(-time.Duration(index) * time.Second),
		}
	}
	repository := &repositoryStub{search: values}
	service := NewService(repository)
	page, err := service.Search(context.Background(), SearchContractsInput{
		Keyword: "合同", Status: "active", Currency: "CNY",
	})
	if err != nil {
		t.Fatalf("Search(first page) error = %v", err)
	}
	if len(page.Contracts) != 50 || page.NextCursor == "" || page.Contracts[49].ID != values[49].ID {
		t.Fatalf("first page count=%d cursor=%q last=%#v", len(page.Contracts), page.NextCursor, page.Contracts[len(page.Contracts)-1])
	}
	repository.search = nil
	second, err := service.Search(context.Background(), SearchContractsInput{
		Keyword: "合同", Status: "active", Currency: "CNY", Cursor: page.NextCursor,
	})
	if err != nil {
		t.Fatalf("Search(second page) error = %v", err)
	}
	if len(second.Contracts) != 0 || repository.receivedSearch.AfterID != values[49].ID ||
		!repository.receivedSearch.AfterCreatedAt.Equal(values[49].CreatedAt) {
		t.Fatalf("second query=%#v page=%#v", repository.receivedSearch, second)
	}

	mixedRepository := &repositoryStub{}
	_, err = NewService(mixedRepository).Search(context.Background(), SearchContractsInput{
		Keyword: "合同", Status: "terminated", Currency: "CNY", Cursor: page.NextCursor,
	})
	var validation *ValidationError
	if !errors.As(err, &validation) || validation.Field != "cursor" {
		t.Fatalf("mixed filter cursor error = %v", err)
	}
	if mixedRepository.receivedSearch.Limit != 0 {
		t.Fatalf("mixed cursor reached repository: %#v", mixedRepository.receivedSearch)
	}
}

func TestEscapeContractSearchKeywordTreatsWildcardsLiterally(t *testing.T) {
	if got, want := escapeContractSearchKeyword(`100%_done\draft`), `100\%\_done\\draft`; got != want {
		t.Fatalf("escapeContractSearchKeyword() = %q, want %q", got, want)
	}
}
