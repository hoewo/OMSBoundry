package partymigration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const ReportFormat = "omsbizfoundry.contract-party-migration-preflight/v1"

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type LegacyParty struct {
	ID                      string
	Kind                    string
	LegalName               string
	RegistrationIdentifier  *string
	Revision                int64
	CreatedAt               time.Time
	UpdatedAt               time.Time
	DraftLinkCount          int64
	RevisionSnapshotCount   int64
	SettlementSnapshotCount int64
}

type Usage struct {
	DraftLinks          int64 `json:"draft_links"`
	RevisionSnapshots   int64 `json:"revision_snapshots"`
	SettlementSnapshots int64 `json:"settlement_snapshots"`
}

func (u Usage) Total() int64 { return u.DraftLinks + u.RevisionSnapshots + u.SettlementSnapshots }

type Signal struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Item struct {
	LocalPartyID           string   `json:"local_party_id"`
	Kind                   string   `json:"kind"`
	LegalName              string   `json:"legal_name"`
	RegistrationIdentifier *string  `json:"registration_identifier"`
	LocalRevision          int64    `json:"local_revision"`
	CreatedAt              string   `json:"created_at"`
	UpdatedAt              string   `json:"updated_at"`
	LocalSnapshotSHA256    string   `json:"local_snapshot_sha256"`
	Usage                  Usage    `json:"usage"`
	DecisionStatus         string   `json:"decision_status"`
	AllowedDecisions       []string `json:"allowed_decisions"`
	Signals                []Signal `json:"signals,omitempty"`
}

type Counts struct {
	Parties                     int `json:"parties"`
	Referenced                  int `json:"referenced"`
	Unreferenced                int `json:"unreferenced"`
	Unresolved                  int `json:"unresolved"`
	DuplicateRegistrationGroups int `json:"duplicate_registration_groups"`
	BlockingIssues              int `json:"blocking_issues"`
}

type Report struct {
	Format                string   `json:"format"`
	TenantID              string   `json:"tenant_id"`
	ReadyForReview        bool     `json:"ready_for_review"`
	ReadyForCutover       bool     `json:"ready_for_cutover"`
	AutomaticMergeAllowed bool     `json:"automatic_merge_allowed"`
	Counts                Counts   `json:"counts"`
	BlockingIssues        []Signal `json:"blocking_issues,omitempty"`
	Items                 []Item   `json:"items"`
}

func Inspect(ctx context.Context, pool *pgxpool.Pool, tenantID string) (Report, error) {
	if pool == nil {
		return Report{}, fmt.Errorf("Contract database pool is required")
	}
	rows, err := pool.Query(ctx, `
		SELECT p.id::text, p.kind, p.legal_name, p.registration_identifier,
			p.revision, p.created_at, p.updated_at,
			(SELECT count(*) FROM contract_draft_parties d WHERE d.party_id = p.id),
			(SELECT count(*) FROM contract_revision_parties r WHERE r.source_party_id = p.id),
			(SELECT count(*) FROM contract_settlement_parties s WHERE s.source_party_id = p.id)
		FROM parties p
		ORDER BY p.id
	`)
	if err != nil {
		return Report{}, fmt.Errorf("query legacy Contract parties: %w", err)
	}
	defer rows.Close()
	parties := make([]LegacyParty, 0)
	for rows.Next() {
		var value LegacyParty
		if err := rows.Scan(
			&value.ID, &value.Kind, &value.LegalName, &value.RegistrationIdentifier,
			&value.Revision, &value.CreatedAt, &value.UpdatedAt,
			&value.DraftLinkCount, &value.RevisionSnapshotCount, &value.SettlementSnapshotCount,
		); err != nil {
			return Report{}, fmt.Errorf("scan legacy Contract party: %w", err)
		}
		parties = append(parties, value)
	}
	if err := rows.Err(); err != nil {
		return Report{}, fmt.Errorf("read legacy Contract parties: %w", err)
	}
	return Evaluate(tenantID, parties)
}

func Evaluate(tenantID string, parties []LegacyParty) (Report, error) {
	tenantID = strings.ToLower(strings.TrimSpace(tenantID))
	if !uuidPattern.MatchString(tenantID) {
		return Report{}, fmt.Errorf("tenant ID must be a lowercase RFC 4122 UUID")
	}
	ordered := append([]LegacyParty(nil), parties...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })
	report := Report{
		Format: ReportFormat, TenantID: tenantID, AutomaticMergeAllowed: false,
		Items: make([]Item, 0, len(ordered)),
	}
	registrationGroups := make(map[string][]string)
	for _, value := range ordered {
		if value.RegistrationIdentifier != nil {
			key := strings.ToLower(strings.TrimSpace(*value.RegistrationIdentifier))
			if key != "" {
				registrationGroups[key] = append(registrationGroups[key], value.ID)
			}
		}
	}
	duplicateIDs := make(map[string]struct{})
	for _, ids := range registrationGroups {
		if len(ids) > 1 {
			report.Counts.DuplicateRegistrationGroups++
			for _, id := range ids {
				duplicateIDs[id] = struct{}{}
			}
		}
	}
	for _, value := range ordered {
		item := Item{
			LocalPartyID: value.ID, Kind: value.Kind, LegalName: value.LegalName,
			RegistrationIdentifier: value.RegistrationIdentifier, LocalRevision: value.Revision,
			CreatedAt: value.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: value.UpdatedAt.UTC().Format(time.RFC3339Nano),
			Usage:            Usage{DraftLinks: value.DraftLinkCount, RevisionSnapshots: value.RevisionSnapshotCount, SettlementSnapshots: value.SettlementSnapshotCount},
			DecisionStatus:   "unresolved",
			AllowedDecisions: []string{"create_directory_party", "map_existing_directory_party"},
		}
		if !uuidPattern.MatchString(strings.ToLower(value.ID)) || (value.Kind != "organization" && value.Kind != "individual") || strings.TrimSpace(value.LegalName) == "" || value.Revision < 1 || value.UpdatedAt.Before(value.CreatedAt) {
			issue := Signal{Code: "invalid_legacy_party", Message: fmt.Sprintf("local Party %s has an invalid identity, kind, name, revision, or timestamp", value.ID)}
			item.Signals = append(item.Signals, issue)
			report.BlockingIssues = append(report.BlockingIssues, issue)
		}
		if _, duplicated := duplicateIDs[value.ID]; duplicated {
			item.Signals = append(item.Signals, Signal{Code: "duplicate_registration_identifier", Message: "registration identifier is shared by multiple local Parties; review each record and do not merge automatically"})
		}
		digest, err := snapshotDigest(value)
		if err != nil {
			return Report{}, err
		}
		item.LocalSnapshotSHA256 = digest
		if item.Usage.Total() > 0 {
			report.Counts.Referenced++
		} else {
			report.Counts.Unreferenced++
		}
		report.Items = append(report.Items, item)
	}
	report.Counts.Parties = len(report.Items)
	report.Counts.Unresolved = len(report.Items)
	report.Counts.BlockingIssues = len(report.BlockingIssues)
	report.ReadyForReview = len(report.BlockingIssues) == 0
	// Preflight never authorizes cutover. Every item requires an explicit, target-validated decision.
	report.ReadyForCutover = false
	return report, nil
}

func snapshotDigest(value LegacyParty) (string, error) {
	payload, err := json.Marshal(struct {
		Format                 string  `json:"format"`
		LocalPartyID           string  `json:"local_party_id"`
		Kind                   string  `json:"kind"`
		LegalName              string  `json:"legal_name"`
		RegistrationIdentifier *string `json:"registration_identifier"`
		Revision               int64   `json:"revision"`
	}{
		Format: "omsbizfoundry.contract-local-party-snapshot/v1", LocalPartyID: value.ID,
		Kind: value.Kind, LegalName: value.LegalName, RegistrationIdentifier: value.RegistrationIdentifier, Revision: value.Revision,
	})
	if err != nil {
		return "", fmt.Errorf("encode local Party snapshot: %w", err)
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}
