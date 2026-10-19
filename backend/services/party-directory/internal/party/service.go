package party

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

type Store interface {
	Ping(context.Context) error
	Create(context.Context, Actor, CreateInput, string) (Party, error)
	List(context.Context, string, ListFilter) ([]Party, error)
	Get(context.Context, string, string) (Party, error)
	History(context.Context, string, string, *int64, int) (HistoryPage, error)
	Update(context.Context, Actor, string, UpdateInput, string) (Party, error)
	ChangeStatus(context.Context, Actor, string, StatusInput, string) (Party, error)
}

type Service struct {
	store     Store
	cursorKey []byte
}

func NewService(store Store, cursorKey string) (*Service, error) {
	if store == nil || len(cursorKey) < 32 {
		return nil, fmt.Errorf("store and a 32+ byte cursor key are required")
	}
	return &Service{store: store, cursorKey: []byte(cursorKey)}, nil
}

func (s *Service) Ping(ctx context.Context) error { return s.store.Ping(ctx) }

func (s *Service) Create(ctx context.Context, actor Actor, input CreateInput) (Party, error) {
	input, err := NormalizeCreate(input)
	if err != nil {
		return Party{}, err
	}
	hash, err := requestHash("create", "", actor.ID, input)
	if err != nil {
		return Party{}, err
	}
	return s.store.Create(ctx, actor, input, hash)
}

func (s *Service) List(ctx context.Context, tenantID string, filter ListFilter, rawCursor string) (Page, error) {
	filter, err := NormalizeList(filter)
	if err != nil {
		return Page{}, err
	}
	if rawCursor != "" {
		cursor, err := s.decodeCursor(rawCursor, filter)
		if err != nil {
			return Page{}, fmt.Errorf("invalid cursor")
		}
		filter.Cursor = &cursor
	}
	items, err := s.store.List(ctx, tenantID, filter)
	if err != nil {
		return Page{}, err
	}
	page := Page{Items: items}
	if len(items) > filter.Limit {
		visible := items[:filter.Limit]
		last := visible[len(visible)-1]
		page.Items = visible
		page.NextCursor, err = s.encodeCursor(filter, Cursor{NormalizedName: NormalizeName(last.LegalName), LegalName: last.LegalName, PartyID: last.ID})
	}
	return page, err
}

func (s *Service) Get(ctx context.Context, tenantID, id string) (Party, error) {
	id = strings.ToLower(strings.TrimSpace(id))
	if !ValidUUID(id) {
		return Party{}, fmt.Errorf("invalid party id")
	}
	return s.store.Get(ctx, tenantID, id)
}

func (s *Service) Reference(ctx context.Context, tenantID, id string) (Reference, error) {
	value, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return Reference{}, err
	}
	if value.Status != StatusActive {
		return Reference{}, ErrInactive
	}
	return Reference{SchemaVersion: "1", SourceApplication: "party-directory", ID: value.ID, TenantID: value.TenantID, Kind: value.Kind, LegalName: value.LegalName, RegistrationIdentifier: value.RegistrationIdentifier, Status: value.Status, Revision: value.Revision}, nil
}

func (s *Service) History(ctx context.Context, tenantID, id string, before *int64, limit int) (HistoryPage, error) {
	id = strings.ToLower(strings.TrimSpace(id))
	if !ValidUUID(id) || (before != nil && *before < 2) {
		return HistoryPage{}, fmt.Errorf("invalid history query")
	}
	if limit == 0 {
		limit = 50
	}
	if limit < 1 || limit > 100 {
		return HistoryPage{}, fmt.Errorf("invalid history limit")
	}
	return s.store.History(ctx, tenantID, id, before, limit)
}

func (s *Service) Update(ctx context.Context, actor Actor, id string, input UpdateInput) (Party, error) {
	id = strings.ToLower(strings.TrimSpace(id))
	if !ValidUUID(id) {
		return Party{}, fmt.Errorf("invalid party id")
	}
	input, err := NormalizeUpdate(input)
	if err != nil {
		return Party{}, err
	}
	hash, err := requestHash("update", id, actor.ID, input)
	if err != nil {
		return Party{}, err
	}
	return s.store.Update(ctx, actor, id, input, hash)
}

func (s *Service) ChangeStatus(ctx context.Context, actor Actor, id string, input StatusInput) (Party, error) {
	id = strings.ToLower(strings.TrimSpace(id))
	if !ValidUUID(id) {
		return Party{}, fmt.Errorf("invalid party id")
	}
	input, err := NormalizeStatus(input)
	if err != nil {
		return Party{}, err
	}
	hash, err := requestHash("status", id, actor.ID, input)
	if err != nil {
		return Party{}, err
	}
	return s.store.ChangeStatus(ctx, actor, id, input, hash)
}

type cursorEnvelope struct {
	Query, Kind, Status string
	Cursor              Cursor `json:"cursor"`
}

func (s *Service) encodeCursor(filter ListFilter, cursor Cursor) (string, error) {
	raw, err := json.Marshal(cursorEnvelope{Query: filter.Query, Kind: filter.Kind, Status: filter.Status, Cursor: cursor})
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, s.cursorKey)
	_, _ = mac.Write(raw)
	return base64.RawURLEncoding.EncodeToString(raw) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (s *Service) decodeCursor(value string, filter ListFilter) (Cursor, error) {
	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		return Cursor{}, fmt.Errorf("bad cursor")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Cursor{}, err
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Cursor{}, err
	}
	mac := hmac.New(sha256.New, s.cursorKey)
	_, _ = mac.Write(raw)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return Cursor{}, fmt.Errorf("bad signature")
	}
	var envelope cursorEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return Cursor{}, err
	}
	if envelope.Query != filter.Query || envelope.Kind != filter.Kind || envelope.Status != filter.Status || !ValidUUID(envelope.Cursor.PartyID) {
		return Cursor{}, fmt.Errorf("cursor filter mismatch")
	}
	return envelope.Cursor, nil
}

func requestHash(operation, id, actorID string, value any) (string, error) {
	raw, err := json.Marshal(struct {
		Operation, ID, ActorID string
		Value                  any
	}{operation, id, actorID, value})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
