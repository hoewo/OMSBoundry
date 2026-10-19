package bonus

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	TypeBusiness   = "business"
	TypeProduction = "production"

	StateDraft      = "draft"
	StatePublished  = "published"
	StateSuperseded = "superseded"
	StateWithdrawn  = "withdrawn"

	SourceContractSettlement = "contract_settlement"
	SourceMoneyReceiptSet    = "money_receipt_set"
	SourceMoneyReceiptItem   = "money_transaction_allocation"

	RoundingHalfUp = "half_up"

	ActionBusinessCalculate    = "bonus-pool.business.calculate"
	ActionBusinessPublish      = "bonus-pool.business.publish"
	ActionBusinessSupersede    = "bonus-pool.business.supersede"
	ActionBusinessWithdraw     = "bonus-pool.business.withdraw"
	ActionProductionCalculate  = "bonus-pool.production.calculate"
	ActionProductionPublish    = "bonus-pool.production.publish"
	ActionProductionSupersede  = "bonus-pool.production.supersede"
	ActionProductionWithdraw   = "bonus-pool.production.withdraw"
	ActionPoolRead             = "bonus-pool.read"
	ActionPoolHistory          = "bonus-pool.history.read"
	ActionAllocationManage     = "bonus-allocation.manage"
	ActionAllocationRead       = "bonus-allocation.read"
	ActionAllocationAmountRead = "bonus-allocation.amount.read"
	ActionSuggestionCreate     = "bonus-payable-suggestion.create"
	ActionSuggestionRead       = "bonus-payable-suggestion.read"
	ActionSuggestionWithdraw   = "bonus-payable-suggestion.withdraw"

	SuggestionTargetUnavailable = "target_unavailable"
	SuggestionProposed          = "proposed"
	SuggestionAccepted          = "accepted"
	SuggestionRejected          = "rejected"
	SuggestionWithdrawn         = "withdrawn"
)

var (
	ErrNotFound             = errors.New("bonus pool not found")
	ErrRevisionConflict     = errors.New("revision conflict")
	ErrCommandConflict      = errors.New("command payload conflict")
	ErrStateConflict        = errors.New("bonus pool state conflict")
	ErrForbidden            = errors.New("action forbidden")
	ErrReferenceInvalid     = errors.New("reference invalid")
	ErrReferenceUnavailable = errors.New("reference unavailable")
	ErrPolicyInvalid        = errors.New("bonus policy invalid")
	ErrAllocationInvalid    = errors.New("bonus allocation invalid")
	ErrTargetUnavailable    = errors.New("Money payable suggestion target unavailable")

	uuidPattern     = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)
	digestPattern   = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

type Actor struct {
	ID             string          `json:"id"`
	TenantID       string          `json:"tenant_id"`
	ProjectIDs     []string        `json:"project_ids"`
	ResourceScopes []ResourceScope `json:"resource_scopes,omitempty"`
	TenantScope    bool            `json:"tenant_scope,omitempty"`
	Actions        []string        `json:"actions"`
	Credential     string          `json:"credential_id"`
	SubjectKind    string          `json:"subject_kind"`
}

type ResourceScope struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type ProjectRef struct {
	ID       string `json:"id"`
	Revision int64  `json:"revision"`
}

type ProjectSnapshot struct {
	ID          string `json:"id"`
	Revision    int64  `json:"revision"`
	Code        string `json:"code"`
	DisplayName string `json:"display_name"`
	Digest      string `json:"digest"`
}

type ProjectReference struct {
	ProjectSnapshot
	TenantID string `json:"tenant_id"`
	State    string `json:"state"`
}

type BusinessSourceRef struct {
	SettlementID string `json:"settlement_id"`
	Revision     int64  `json:"revision"`
}

type ProductionSourceRef struct {
	ReceiptSetID string `json:"receipt_set_id"`
	Revision     int64  `json:"revision"`
}

type SourceItemSnapshot struct {
	Type                string `json:"type"`
	ID                  string `json:"id"`
	Revision            int64  `json:"revision"`
	TransactionID       string `json:"transaction_id,omitempty"`
	TransactionRevision int64  `json:"transaction_revision,omitempty"`
	AmountMinor         int64  `json:"amount_minor"`
	Currency            string `json:"currency"`
	State               string `json:"state"`
	Digest              string `json:"digest"`
}

type SourceSnapshot struct {
	Type             string               `json:"type"`
	ID               string               `json:"id"`
	Revision         int64                `json:"revision"`
	ProjectID        string               `json:"project_id"`
	Currency         string               `json:"currency"`
	TotalAmountMinor int64                `json:"total_amount_minor"`
	State            string               `json:"state"`
	Digest           string               `json:"digest"`
	Items            []SourceItemSnapshot `json:"items"`
}

type PersonRef struct {
	ID       string `json:"id"`
	Revision int64  `json:"revision"`
}

type AssignmentRef struct {
	ID       string `json:"id"`
	Revision int64  `json:"revision"`
}

type PersonSnapshot struct {
	ID          string `json:"id"`
	Revision    int64  `json:"revision"`
	DisplayName string `json:"display_name"`
	Digest      string `json:"digest"`
}

type AssignmentSnapshot struct {
	ID        string `json:"id"`
	Revision  int64  `json:"revision"`
	ProjectID string `json:"project_id"`
	PersonID  string `json:"person_id"`
	RoleKey   string `json:"role_key"`
	Digest    string `json:"digest"`
}

type ParticipantReference struct {
	TenantID   string             `json:"tenant_id"`
	BonusType  string             `json:"bonus_type"`
	Active     bool               `json:"active"`
	Eligible   bool               `json:"eligible"`
	Person     PersonSnapshot     `json:"person"`
	Assignment AssignmentSnapshot `json:"assignment"`
}

type PolicyRequest struct {
	Key             string `json:"key"`
	Version         string `json:"version"`
	RateBasisPoints int64  `json:"rate_basis_points"`
	Rounding        string `json:"rounding"`
}

type PolicySnapshot struct {
	Key              string `json:"key"`
	Version          string `json:"version"`
	BonusType        string `json:"bonus_type"`
	RateBasisPoints  int64  `json:"rate_basis_points"`
	Rounding         string `json:"rounding"`
	Formula          string `json:"formula"`
	ParametersDigest string `json:"parameters_digest"`
}

type AllocationRequest struct {
	Person           PersonRef     `json:"person"`
	Assignment       AssignmentRef `json:"assignment"`
	RatioBasisPoints int64         `json:"ratio_basis_points"`
	Reason           string        `json:"reason"`
}

type BonusAllocation struct {
	ID               string             `json:"id"`
	TenantID         string             `json:"tenant_id"`
	PoolID           string             `json:"pool_id"`
	PoolRevision     int64              `json:"pool_revision"`
	ProjectID        string             `json:"project_id"`
	BonusType        string             `json:"bonus_type"`
	Person           PersonSnapshot     `json:"person"`
	Assignment       AssignmentSnapshot `json:"assignment"`
	RatioBasisPoints int64              `json:"ratio_basis_points"`
	AmountMinor      int64              `json:"amount_minor"`
	Currency         string             `json:"currency"`
	Reason           string             `json:"reason"`
	State            string             `json:"state"`
	Revision         int64              `json:"revision"`
	CreatedAt        time.Time          `json:"created_at"`
	UpdatedAt        time.Time          `json:"updated_at"`
}

type SupersededReference struct {
	PoolID   string `json:"pool_id"`
	Revision int64  `json:"revision"`
}

type MoneyPartyRef struct {
	ID        string `json:"id"`
	Revision  int64  `json:"revision"`
	Kind      string `json:"kind"`
	LegalName string `json:"legal_name"`
}

type PayableSourceSnapshot struct {
	AllocationID       string          `json:"allocation_id"`
	AllocationRevision int64           `json:"allocation_revision"`
	AllocationDigest   string          `json:"allocation_digest"`
	PoolID             string          `json:"pool_id"`
	PoolRevision       int64           `json:"pool_revision"`
	Project            ProjectSnapshot `json:"project"`
	Person             PersonSnapshot  `json:"person"`
}

type ObligationReference struct {
	ID       string `json:"id"`
	Revision int64  `json:"revision"`
}

type BonusPayableSuggestion struct {
	ID             string                `json:"id"`
	TenantID       string                `json:"tenant_id"`
	Source         PayableSourceSnapshot `json:"source"`
	LedgerOwner    MoneyPartyRef         `json:"ledger_owner"`
	AmountMinor    int64                 `json:"amount_minor"`
	Currency       string                `json:"currency"`
	DueOn          string                `json:"due_on"`
	Purpose        string                `json:"purpose"`
	State          string                `json:"state"`
	Revision       int64                 `json:"revision"`
	TargetRevision int64                 `json:"target_revision,omitempty"`
	RequestDigest  string                `json:"request_digest"`
	Obligation     *ObligationReference  `json:"obligation,omitempty"`
	DecisionReason string                `json:"decision_reason,omitempty"`
	CreatedBy      string                `json:"created_by"`
	UpdatedBy      string                `json:"updated_by"`
	CreatedAt      time.Time             `json:"created_at"`
	UpdatedAt      time.Time             `json:"updated_at"`
}

type BonusPayableSuggestionRevision struct {
	SuggestionID string                 `json:"suggestion_id"`
	TenantID     string                 `json:"tenant_id"`
	Revision     int64                  `json:"revision"`
	EventKind    string                 `json:"event_kind"`
	CommandID    string                 `json:"command_id"`
	Snapshot     BonusPayableSuggestion `json:"snapshot"`
	Reason       string                 `json:"reason"`
	ActorRef     string                 `json:"actor_ref"`
	OccurredAt   time.Time              `json:"occurred_at"`
}

type ProposePayableSuggestionInput struct {
	CommandID          string        `json:"command_id"`
	AllocationID       string        `json:"allocation_id"`
	AllocationRevision int64         `json:"allocation_revision"`
	LedgerOwner        MoneyPartyRef `json:"ledger_owner"`
	DueOn              string        `json:"due_on"`
	Purpose            string        `json:"purpose"`
}

type SuggestionCommandInput struct {
	CommandID        string `json:"command_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Reason           string `json:"reason,omitempty"`
}

type PayableSubmission struct {
	SuggestionID  string
	TenantID      string
	ProjectID     string
	Source        PayableSourceSnapshot
	LedgerOwner   MoneyPartyRef
	AmountMinor   int64
	Currency      string
	DueOn         string
	Purpose       string
	RequestDigest string
}

type PayableIntake struct {
	SuggestionID   string
	TenantID       string
	RequestDigest  string
	Status         string
	Revision       int64
	Obligation     *ObligationReference
	DecisionReason string
}

type BonusPool struct {
	ID               string               `json:"id"`
	TenantID         string               `json:"tenant_id"`
	Project          ProjectSnapshot      `json:"project"`
	BonusType        string               `json:"bonus_type"`
	State            string               `json:"state"`
	Source           SourceSnapshot       `json:"source"`
	Policy           PolicySnapshot       `json:"policy"`
	Currency         string               `json:"currency"`
	AmountMinor      int64                `json:"amount_minor"`
	UnallocatedMinor int64                `json:"unallocated_minor"`
	Allocations      []BonusAllocation    `json:"allocations"`
	Supersedes       *SupersededReference `json:"supersedes,omitempty"`
	SupersededBy     *SupersededReference `json:"superseded_by,omitempty"`
	Revision         int64                `json:"revision"`
	CreatedBy        string               `json:"created_by"`
	UpdatedBy        string               `json:"updated_by"`
	CreatedAt        time.Time            `json:"created_at"`
	UpdatedAt        time.Time            `json:"updated_at"`
}

type PoolRevision struct {
	PoolID     string    `json:"pool_id"`
	TenantID   string    `json:"tenant_id"`
	Revision   int64     `json:"revision"`
	EventKind  string    `json:"event_kind"`
	CommandID  string    `json:"command_id"`
	Snapshot   BonusPool `json:"snapshot"`
	Reason     string    `json:"reason"`
	ActorRef   string    `json:"actor_ref"`
	OccurredAt time.Time `json:"occurred_at"`
}

type AllocationRevision struct {
	AllocationID string          `json:"allocation_id"`
	PoolID       string          `json:"pool_id"`
	TenantID     string          `json:"tenant_id"`
	Revision     int64           `json:"revision"`
	EventKind    string          `json:"event_kind"`
	CommandID    string          `json:"command_id"`
	Snapshot     BonusAllocation `json:"snapshot"`
	Reason       string          `json:"reason"`
	ActorRef     string          `json:"actor_ref"`
	OccurredAt   time.Time       `json:"occurred_at"`
}

type CreateBusinessDraftInput struct {
	CommandID   string              `json:"command_id"`
	Project     ProjectRef          `json:"project"`
	Source      BusinessSourceRef   `json:"source"`
	Policy      PolicyRequest       `json:"policy"`
	Allocations []AllocationRequest `json:"allocations"`
}

type CreateProductionDraftInput struct {
	CommandID   string              `json:"command_id"`
	Project     ProjectRef          `json:"project"`
	Source      ProductionSourceRef `json:"source"`
	Policy      PolicyRequest       `json:"policy"`
	Allocations []AllocationRequest `json:"allocations"`
}

type PoolCommandInput struct {
	CommandID        string `json:"command_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Reason           string `json:"reason,omitempty"`
}

type SupersedeInput struct {
	CommandID                   string `json:"command_id"`
	PublishedPoolID             string `json:"published_pool_id"`
	ExpectedPublishedRevision   int64  `json:"expected_published_revision"`
	ReplacementPoolID           string `json:"replacement_pool_id"`
	ExpectedReplacementRevision int64  `json:"expected_replacement_revision"`
	Reason                      string `json:"reason"`
}

type SupersessionResult struct {
	Superseded BonusPool `json:"superseded"`
	Published  BonusPool `json:"published"`
}

func normalizeActor(value Actor) (Actor, error) {
	value.ID = normalizeUUID(value.ID)
	value.TenantID = normalizeUUID(value.TenantID)
	value.Credential = strings.TrimSpace(value.Credential)
	value.SubjectKind = strings.TrimSpace(value.SubjectKind)
	value.ProjectIDs = uniqueStrings(value.ProjectIDs)
	value.Actions = uniqueStrings(value.Actions)
	for i := range value.ProjectIDs {
		value.ProjectIDs[i] = normalizeUUID(value.ProjectIDs[i])
		if !validUUID(value.ProjectIDs[i]) {
			return Actor{}, fmt.Errorf("invalid actor project scope")
		}
	}
	seenScopes := map[string]bool{}
	scopes := make([]ResourceScope, 0, len(value.ResourceScopes))
	for _, scope := range value.ResourceScopes {
		scope.Type = strings.TrimSpace(scope.Type)
		scope.ID = normalizeUUID(scope.ID)
		key := scope.Type + "\x00" + scope.ID
		if (scope.Type != "bonus_pool" && scope.Type != "bonus_allocation") || !validUUID(scope.ID) || seenScopes[key] {
			return Actor{}, fmt.Errorf("invalid actor resource scope")
		}
		seenScopes[key] = true
		scopes = append(scopes, scope)
	}
	value.ResourceScopes = scopes
	if !validUUID(value.ID) || !validUUID(value.TenantID) || !validText(value.Credential, 1, 256) || !validText(value.SubjectKind, 1, 100) {
		return Actor{}, fmt.Errorf("invalid actor")
	}
	return value, nil
}

func normalizeProjectRef(value ProjectRef) (ProjectRef, error) {
	value.ID = normalizeUUID(value.ID)
	if !validUUID(value.ID) || value.Revision <= 0 {
		return ProjectRef{}, fmt.Errorf("invalid project reference")
	}
	return value, nil
}

func normalizeBusinessSourceRef(value BusinessSourceRef) (BusinessSourceRef, error) {
	value.SettlementID = normalizeUUID(value.SettlementID)
	if !validUUID(value.SettlementID) || value.Revision <= 0 {
		return BusinessSourceRef{}, fmt.Errorf("invalid business source reference")
	}
	return value, nil
}

func normalizeProductionSourceRef(value ProductionSourceRef) (ProductionSourceRef, error) {
	value.ReceiptSetID = normalizeUUID(value.ReceiptSetID)
	if !validUUID(value.ReceiptSetID) || value.Revision <= 0 {
		return ProductionSourceRef{}, fmt.Errorf("invalid production source reference")
	}
	return value, nil
}

func normalizePolicyRequest(value PolicyRequest) (PolicyRequest, error) {
	value.Key = strings.TrimSpace(value.Key)
	value.Version = strings.TrimSpace(value.Version)
	value.Rounding = strings.TrimSpace(value.Rounding)
	if !validText(value.Key, 1, 100) || !validText(value.Version, 1, 50) || value.RateBasisPoints <= 0 || value.RateBasisPoints > 10000 || value.Rounding != RoundingHalfUp {
		return PolicyRequest{}, ErrPolicyInvalid
	}
	return value, nil
}

func normalizeAllocationRequests(values []AllocationRequest) ([]AllocationRequest, error) {
	if len(values) == 0 || len(values) > 1000 {
		return nil, ErrAllocationInvalid
	}
	result := append([]AllocationRequest(nil), values...)
	seenPeople, seenAssignments := map[string]bool{}, map[string]bool{}
	var total int64
	for i := range result {
		result[i].Person.ID = normalizeUUID(result[i].Person.ID)
		result[i].Assignment.ID = normalizeUUID(result[i].Assignment.ID)
		result[i].Reason = strings.TrimSpace(result[i].Reason)
		if !validUUID(result[i].Person.ID) || result[i].Person.Revision <= 0 || !validUUID(result[i].Assignment.ID) || result[i].Assignment.Revision <= 0 || result[i].RatioBasisPoints <= 0 || result[i].RatioBasisPoints > 10000 || !validText(result[i].Reason, 1, 500) || seenPeople[result[i].Person.ID] || seenAssignments[result[i].Assignment.ID] {
			return nil, ErrAllocationInvalid
		}
		seenPeople[result[i].Person.ID], seenAssignments[result[i].Assignment.ID] = true, true
		total += result[i].RatioBasisPoints
		if total > 10000 {
			return nil, ErrAllocationInvalid
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Person.ID < result[j].Person.ID })
	return result, nil
}

func normalizePoolCommand(value PoolCommandInput) (PoolCommandInput, error) {
	value.CommandID = normalizeUUID(value.CommandID)
	value.Reason = strings.TrimSpace(value.Reason)
	if !validUUID(value.CommandID) || value.ExpectedRevision <= 0 || len([]rune(value.Reason)) > 500 {
		return PoolCommandInput{}, fmt.Errorf("invalid pool command")
	}
	return value, nil
}

func normalizeSupersede(value SupersedeInput) (SupersedeInput, error) {
	value.CommandID = normalizeUUID(value.CommandID)
	value.PublishedPoolID = normalizeUUID(value.PublishedPoolID)
	value.ReplacementPoolID = normalizeUUID(value.ReplacementPoolID)
	value.Reason = strings.TrimSpace(value.Reason)
	if !validUUID(value.CommandID) || !validUUID(value.PublishedPoolID) || !validUUID(value.ReplacementPoolID) || value.PublishedPoolID == value.ReplacementPoolID || value.ExpectedPublishedRevision <= 0 || value.ExpectedReplacementRevision <= 0 || !validText(value.Reason, 1, 500) {
		return SupersedeInput{}, fmt.Errorf("invalid supersession")
	}
	return value, nil
}

func validateProjectReference(value ProjectReference, tenantID string, ref ProjectRef) (ProjectSnapshot, error) {
	value.ID = normalizeUUID(value.ID)
	value.TenantID = normalizeUUID(value.TenantID)
	value.State = strings.TrimSpace(value.State)
	value.Digest = strings.TrimSpace(value.Digest)
	if value.TenantID != tenantID || value.ID != ref.ID || value.Revision != ref.Revision || value.State != "active" || !validText(value.Code, 1, 100) || !validText(value.DisplayName, 1, 300) || !digestPattern.MatchString(value.Digest) {
		return ProjectSnapshot{}, ErrReferenceInvalid
	}
	return value.ProjectSnapshot, nil
}

func validateSourceSnapshot(value SourceSnapshot, tenantProject ProjectSnapshot, bonusType string, sourceID string, revision int64) (SourceSnapshot, error) {
	value.Type = strings.TrimSpace(value.Type)
	value.ID = normalizeUUID(value.ID)
	value.ProjectID = normalizeUUID(value.ProjectID)
	value.Currency = strings.ToUpper(strings.TrimSpace(value.Currency))
	value.State = strings.TrimSpace(value.State)
	value.Digest = strings.TrimSpace(value.Digest)
	wantedType := SourceContractSettlement
	if bonusType == TypeProduction {
		wantedType = SourceMoneyReceiptSet
	}
	if value.Type != wantedType || value.ID != sourceID || value.Revision != revision || value.ProjectID != tenantProject.ID || value.State != "confirmed" || !currencyPattern.MatchString(value.Currency) || value.TotalAmountMinor <= 0 || !digestPattern.MatchString(value.Digest) || len(value.Items) == 0 {
		return SourceSnapshot{}, ErrReferenceInvalid
	}
	items := append([]SourceItemSnapshot(nil), value.Items...)
	seen := map[string]bool{}
	var total int64
	for i := range items {
		items[i].Type = strings.TrimSpace(items[i].Type)
		items[i].ID = normalizeUUID(items[i].ID)
		items[i].TransactionID = normalizeUUID(items[i].TransactionID)
		items[i].Currency = strings.ToUpper(strings.TrimSpace(items[i].Currency))
		items[i].State = strings.TrimSpace(items[i].State)
		items[i].Digest = strings.TrimSpace(items[i].Digest)
		if !validUUID(items[i].ID) || items[i].Revision <= 0 || items[i].AmountMinor <= 0 || items[i].Currency != value.Currency || items[i].State != "confirmed" || !digestPattern.MatchString(items[i].Digest) || seen[items[i].Type+":"+items[i].ID] {
			return SourceSnapshot{}, ErrReferenceInvalid
		}
		if bonusType == TypeBusiness && (len(items) != 1 || items[i].Type != SourceContractSettlement || items[i].ID != sourceID || items[i].Revision != revision) {
			return SourceSnapshot{}, ErrReferenceInvalid
		}
		if bonusType == TypeProduction && (items[i].Type != SourceMoneyReceiptItem || !validUUID(items[i].TransactionID) || items[i].TransactionRevision <= 0) {
			return SourceSnapshot{}, ErrReferenceInvalid
		}
		seen[items[i].Type+":"+items[i].ID] = true
		var ok bool
		total, ok = checkedAdd(total, items[i].AmountMinor)
		if !ok {
			return SourceSnapshot{}, ErrReferenceInvalid
		}
	}
	if total != value.TotalAmountMinor {
		return SourceSnapshot{}, ErrReferenceInvalid
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Type == items[j].Type {
			if items[i].ID == items[j].ID {
				return items[i].Revision < items[j].Revision
			}
			return items[i].ID < items[j].ID
		}
		return items[i].Type < items[j].Type
	})
	value.Items = items
	return value, nil
}

func validateParticipant(value ParticipantReference, tenantID, bonusType string, project ProjectSnapshot, request AllocationRequest) (PersonSnapshot, AssignmentSnapshot, error) {
	value.TenantID = normalizeUUID(value.TenantID)
	value.BonusType = strings.TrimSpace(value.BonusType)
	value.Person.ID = normalizeUUID(value.Person.ID)
	value.Person.Digest = strings.TrimSpace(value.Person.Digest)
	value.Assignment.ID = normalizeUUID(value.Assignment.ID)
	value.Assignment.ProjectID = normalizeUUID(value.Assignment.ProjectID)
	value.Assignment.PersonID = normalizeUUID(value.Assignment.PersonID)
	value.Assignment.RoleKey = strings.TrimSpace(value.Assignment.RoleKey)
	value.Assignment.Digest = strings.TrimSpace(value.Assignment.Digest)
	if value.TenantID != tenantID || value.BonusType != bonusType || !value.Active || !value.Eligible || value.Person.ID != request.Person.ID || value.Person.Revision != request.Person.Revision || !validText(value.Person.DisplayName, 1, 300) || !digestPattern.MatchString(value.Person.Digest) || value.Assignment.ID != request.Assignment.ID || value.Assignment.Revision != request.Assignment.Revision || value.Assignment.ProjectID != project.ID || value.Assignment.PersonID != value.Person.ID || !validText(value.Assignment.RoleKey, 1, 100) || !digestPattern.MatchString(value.Assignment.Digest) {
		return PersonSnapshot{}, AssignmentSnapshot{}, ErrReferenceInvalid
	}
	return value.Person, value.Assignment, nil
}

func validatePolicySnapshot(value PolicySnapshot, bonusType string, request PolicyRequest, amountMinor, baseMinor int64) (PolicySnapshot, error) {
	value.Key = strings.TrimSpace(value.Key)
	value.Version = strings.TrimSpace(value.Version)
	value.BonusType = strings.TrimSpace(value.BonusType)
	value.Rounding = strings.TrimSpace(value.Rounding)
	value.Formula = strings.TrimSpace(value.Formula)
	value.ParametersDigest = strings.TrimSpace(value.ParametersDigest)
	if value.Key != request.Key || value.Version != request.Version || value.BonusType != bonusType || value.RateBasisPoints != request.RateBasisPoints || value.Rounding != request.Rounding || !validText(value.Formula, 1, 500) || !digestPattern.MatchString(value.ParametersDigest) || amountMinor < 0 || amountMinor > baseMinor {
		return PolicySnapshot{}, ErrPolicyInvalid
	}
	return value, nil
}

func exactRatioAmount(amountMinor, ratioBasisPoints int64) (int64, error) {
	if amountMinor < 0 || ratioBasisPoints <= 0 || ratioBasisPoints > 10000 {
		return 0, ErrAllocationInvalid
	}
	numerator := new(big.Int).Mul(big.NewInt(amountMinor), big.NewInt(ratioBasisPoints))
	return quotientHalfUp(numerator, big.NewInt(10000))
}

func quotientHalfUp(numerator, denominator *big.Int) (int64, error) {
	if numerator.Sign() < 0 || denominator.Sign() <= 0 {
		return 0, fmt.Errorf("invalid exact arithmetic")
	}
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(numerator, denominator, remainder)
	if new(big.Int).Lsh(remainder, 1).Cmp(denominator) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	if !quotient.IsInt64() {
		return 0, fmt.Errorf("exact amount overflow")
	}
	return quotient.Int64(), nil
}

func checkedAdd(a, b int64) (int64, bool) {
	value := new(big.Int).Add(big.NewInt(a), big.NewInt(b))
	return value.Int64(), value.IsInt64()
}

func normalizeUUID(value string) string { return strings.ToLower(strings.TrimSpace(value)) }
func validUUID(value string) bool       { return uuidPattern.MatchString(value) }
func validText(value string, min, max int) bool {
	size := len([]rune(strings.TrimSpace(value)))
	return size >= min && size <= max
}

func uniqueStrings(values []string) []string {
	result, seen := make([]string, 0, len(values)), map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

func newUUID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(value[:])
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32], nil
}
