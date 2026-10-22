package projects

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	PartyCandidateRoleClient     = "client"
	PartyCandidateRoleContractor = "contractor"
)

// PartyCandidate records that a neutral Party Directory subject is available
// for a project-side business role before the first project relationship exists.
// It does not change the Party profile or freeze a project counterparty snapshot.
type PartyCandidate struct {
	PartyID               string    `json:"party_id"`
	RoleKey               string    `json:"role_key"`
	SourceRevision        int64     `json:"source_revision"`
	RegistrationCommandID string    `json:"registration_command_id"`
	RegisteredBy          string    `json:"registered_by"`
	RegisteredAt          time.Time `json:"registered_at"`
}

type PartyCandidateInput struct {
	CommandID      string `json:"command_id"`
	PartyID        string `json:"party_id"`
	SourceRevision int64  `json:"source_revision"`
	RoleKey        string `json:"role_key"`
}

type PartyCandidatePage struct {
	SchemaVersion string           `json:"schema_version"`
	Items         []PartyCandidate `json:"items"`
}

type PartyCandidateStore interface {
	ListPartyCandidates(context.Context, string) ([]PartyCandidate, error)
	ReplayPartyCandidate(context.Context, string, PartyCandidateInput) (PartyCandidate, bool, error)
	RegisterPartyCandidate(context.Context, Actor, PartyCandidateInput) (PartyCandidate, error)
}

func normalizePartyCandidate(input PartyCandidateInput) (PartyCandidateInput, error) {
	input.CommandID = strings.ToLower(strings.TrimSpace(input.CommandID))
	input.PartyID = strings.ToLower(strings.TrimSpace(input.PartyID))
	input.RoleKey = strings.ToLower(strings.TrimSpace(input.RoleKey))
	if !ValidUUID(input.CommandID) || !ValidUUID(input.PartyID) || input.SourceRevision < 1 ||
		(input.RoleKey != PartyCandidateRoleClient && input.RoleKey != PartyCandidateRoleContractor) {
		return PartyCandidateInput{}, fmt.Errorf("invalid project party candidate")
	}
	return input, nil
}

func (s *Service) ListPartyCandidates(ctx context.Context, tenantID string) (PartyCandidatePage, error) {
	store, ok := s.store.(PartyCandidateStore)
	if !ok {
		return PartyCandidatePage{}, fmt.Errorf("project party candidate store is unavailable")
	}
	items, err := store.ListPartyCandidates(ctx, tenantID)
	if err != nil {
		return PartyCandidatePage{}, err
	}
	return PartyCandidatePage{SchemaVersion: "1", Items: items}, nil
}

func (s *Service) RegisterPartyCandidate(ctx context.Context, actor Actor, input PartyCandidateInput) (PartyCandidate, error) {
	normalized, err := normalizePartyCandidate(input)
	if err != nil {
		return PartyCandidate{}, err
	}
	store, ok := s.store.(PartyCandidateStore)
	if !ok {
		return PartyCandidate{}, fmt.Errorf("project party candidate store is unavailable")
	}
	if replay, found, err := store.ReplayPartyCandidate(ctx, actor.TenantID, normalized); err != nil {
		return PartyCandidate{}, err
	} else if found {
		return replay, nil
	}
	if _, err := s.references.ValidateParty(ctx, actor.TenantID, normalized.PartyID, normalized.SourceRevision); err != nil {
		return PartyCandidate{}, err
	}
	return store.RegisterPartyCandidate(ctx, actor, normalized)
}
