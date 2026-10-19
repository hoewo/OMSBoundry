package party

import (
	"context"
	"strings"
	"testing"
)

type cursorStore struct {
	items    []Party
	received ListFilter
}

func (s *cursorStore) Ping(context.Context) error { return nil }
func (s *cursorStore) Create(context.Context, Actor, CreateInput, string) (Party, error) {
	return Party{}, nil
}
func (s *cursorStore) List(_ context.Context, _ string, filter ListFilter) ([]Party, error) {
	s.received = filter
	return s.items, nil
}
func (s *cursorStore) Get(context.Context, string, string) (Party, error) { return Party{}, nil }
func (s *cursorStore) History(context.Context, string, string, *int64, int) (HistoryPage, error) {
	return HistoryPage{}, nil
}
func (s *cursorStore) Update(context.Context, Actor, string, UpdateInput, string) (Party, error) {
	return Party{}, nil
}
func (s *cursorStore) ChangeStatus(context.Context, Actor, string, StatusInput, string) (Party, error) {
	return Party{}, nil
}

func TestCursorBindsFiltersAndRejectsTampering(t *testing.T) {
	store := &cursorStore{items: []Party{{ID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", LegalName: "甲公司"}, {ID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", LegalName: "乙公司"}}}
	service, err := NewService(store, strings.Repeat("c", 32))
	if err != nil {
		t.Fatal(err)
	}
	page, err := service.List(context.Background(), "11111111-1111-4111-8111-111111111111", ListFilter{Query: "公司", Limit: 1}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.NextCursor == "" {
		t.Fatalf("expected one item and cursor: %#v", page)
	}
	store.items = nil
	if _, err = service.List(context.Background(), "11111111-1111-4111-8111-111111111111", ListFilter{Query: "公司", Limit: 1}, page.NextCursor); err != nil {
		t.Fatal(err)
	}
	if store.received.Cursor == nil || store.received.Cursor.PartyID != "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa" {
		t.Fatalf("cursor was not decoded: %#v", store.received.Cursor)
	}
	if _, err = service.List(context.Background(), "11111111-1111-4111-8111-111111111111", ListFilter{Query: "另一条件", Limit: 1}, page.NextCursor); err == nil {
		t.Fatal("cursor reuse with a different filter must fail")
	}
	corrupt := page.NextCursor[:len(page.NextCursor)-1] + "A"
	if _, err = service.List(context.Background(), "11111111-1111-4111-8111-111111111111", ListFilter{Query: "公司", Limit: 1}, corrupt); err == nil {
		t.Fatal("tampered cursor must fail")
	}
}

func TestReferenceRejectsInactiveParty(t *testing.T) {
	store := &cursorStore{}
	store.items = nil
	service, err := NewService(&getStore{Party: Party{ID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", TenantID: "11111111-1111-4111-8111-111111111111", Status: StatusInactive}}, strings.Repeat("k", 32))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Reference(context.Background(), "11111111-1111-4111-8111-111111111111", "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"); err != ErrInactive {
		t.Fatalf("expected inactive error, got %v", err)
	}
}

type getStore struct {
	cursorStore
	Party Party
}

func (s *getStore) Get(context.Context, string, string) (Party, error) { return s.Party, nil }
