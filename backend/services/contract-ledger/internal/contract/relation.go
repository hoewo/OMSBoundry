package contract

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const AuditActionContractRelationLinked = "contract_relation_linked"

type RelationType string

const (
	RelationTypeFundingSourceFor       RelationType = "funding_source_for"
	RelationTypeEntrustmentBasisFor    RelationType = "entrustment_basis_for"
	RelationTypeSubcontractOf          RelationType = "subcontract_of"
	RelationTypeSpecificUnderFramework RelationType = "specific_under_framework"
	RelationTypeSupersedes             RelationType = "supersedes"
	RelationTypeReferences             RelationType = "references"
)

func (value RelationType) Valid() bool {
	switch value {
	case RelationTypeFundingSourceFor,
		RelationTypeEntrustmentBasisFor,
		RelationTypeSubcontractOf,
		RelationTypeSpecificUnderFramework,
		RelationTypeSupersedes,
		RelationTypeReferences:
		return true
	default:
		return false
	}
}

func (value RelationType) Acyclic() bool {
	return value.Valid() && value != RelationTypeReferences
}

func (value RelationType) Label() string {
	switch value {
	case RelationTypeFundingSourceFor:
		return "资金来源／上游依据"
	case RelationTypeEntrustmentBasisFor:
		return "委托／上游依据"
	case RelationTypeSubcontractOf:
		return "分包合同"
	case RelationTypeSpecificUnderFramework:
		return "框架下具体合同"
	case RelationTypeSupersedes:
		return "替代"
	case RelationTypeReferences:
		return "引用"
	default:
		return string(value)
	}
}

type ContractRelationSnapshot struct {
	ContractID      string
	Revision        int64
	NumberNamespace string
	ContractNumber  string
	Title           string
}

type ContractRelation struct {
	ID          string
	Source      ContractRelationSnapshot
	Target      ContractRelationSnapshot
	Type        RelationType
	EffectiveOn string
	Note        string
	Actor       ActorRef
	CreatedAt   time.Time
}

type LinkContractRelationInput struct {
	CommandID              string
	SourceContractID       string
	TargetContractID       string
	RelationType           string
	ExpectedSourceRevision int64
	ExpectedTargetRevision int64
	EffectiveOn            string
	Note                   string
	Actor                  ActorRef
}

type ContractRelationCommand struct {
	CommandID              string
	AuditID                string
	RelationID             string
	SourceContractID       string
	TargetContractID       string
	RelationType           RelationType
	ExpectedSourceRevision int64
	ExpectedTargetRevision int64
	EffectiveOn            string
	Note                   string
	Actor                  ActorRef
	OccurredAt             time.Time
}

type ContractRelationResult struct {
	Relation ContractRelation
	Replayed bool
}

type ContractRelationStateConflictError struct {
	Side   string
	Status Status
}

func (e *ContractRelationStateConflictError) Error() string {
	if e.Side == "target" {
		return "目标合同尚未形成可关联的正式 revision"
	}
	return "来源合同尚未形成可关联的正式 revision"
}

type ContractRelationRevisionConflictError struct {
	Side     string
	Expected int64
	Actual   int64
}

func (e *ContractRelationRevisionConflictError) Error() string {
	if e.Side == "target" {
		return "目标合同 revision 已变化，请确认最新内容后重试"
	}
	return "来源合同 revision 已变化，请确认最新内容后重试"
}

type DuplicateContractRelationError struct{}

func (e *DuplicateContractRelationError) Error() string {
	return "两份合同之间已经存在相同类型和方向的关系"
}

type ContractRelationCycleError struct{}

func (e *ContractRelationCycleError) Error() string { return "该关系会形成不允许的有向环" }

func (s *Service) LinkRelation(ctx context.Context, input LinkContractRelationInput) (ContractRelationResult, error) {
	normalized, err := normalizeLinkContractRelationInput(input)
	if err != nil {
		return ContractRelationResult{}, err
	}
	relationID, err := s.newID()
	if err != nil {
		return ContractRelationResult{}, fmt.Errorf("generate contract relation ID: %w", err)
	}
	auditID, err := s.newID()
	if err != nil {
		return ContractRelationResult{}, fmt.Errorf("generate contract relation audit ID: %w", err)
	}
	command := ContractRelationCommand{
		CommandID: normalized.CommandID, AuditID: auditID, RelationID: relationID,
		SourceContractID: normalized.SourceContractID, TargetContractID: normalized.TargetContractID,
		RelationType:           RelationType(normalized.RelationType),
		ExpectedSourceRevision: normalized.ExpectedSourceRevision,
		ExpectedTargetRevision: normalized.ExpectedTargetRevision,
		EffectiveOn:            normalized.EffectiveOn, Note: normalized.Note, Actor: normalized.Actor,
		OccurredAt: s.now(),
	}
	persistedCommand, persistedRelation, created, err := s.repository.LinkRelation(ctx, command)
	if err != nil {
		return ContractRelationResult{}, fmt.Errorf("link contracts: %w", err)
	}
	if !created && !sameContractRelationIntent(persistedCommand, command) {
		return ContractRelationResult{}, &IdempotencyConflictError{CommandID: command.CommandID}
	}
	return ContractRelationResult{Relation: persistedRelation, Replayed: !created}, nil
}

func normalizeLinkContractRelationInput(input LinkContractRelationInput) (LinkContractRelationInput, error) {
	commandID, err := normalizeUUID("command_id", "命令标识无效，请刷新页面后重试", input.CommandID)
	if err != nil {
		return LinkContractRelationInput{}, err
	}
	sourceID, err := normalizeUUID("source_contract_id", "来源合同标识无效", input.SourceContractID)
	if err != nil {
		return LinkContractRelationInput{}, err
	}
	targetID, err := normalizeUUID("target_contract_id", "目标合同标识无效", input.TargetContractID)
	if err != nil {
		return LinkContractRelationInput{}, err
	}
	if sourceID == targetID {
		return LinkContractRelationInput{}, &ValidationError{Field: "target_contract_id", Message: "合同不能与自身建立关系"}
	}
	relationType := RelationType(strings.ToLower(strings.TrimSpace(input.RelationType)))
	if !relationType.Valid() {
		return LinkContractRelationInput{}, &ValidationError{Field: "relation_type", Message: "合同关系类型无效"}
	}
	if input.ExpectedSourceRevision < 1 {
		return LinkContractRelationInput{}, &ValidationError{Field: "expected_source_revision", Message: "来源合同 revision 无效，请刷新页面后重试"}
	}
	if input.ExpectedTargetRevision < 1 {
		return LinkContractRelationInput{}, &ValidationError{Field: "expected_target_revision", Message: "目标合同 revision 无效，请重新选择目标合同"}
	}
	effectiveOn, err := normalizeDate("effective_on", "关系生效日期格式无效", input.EffectiveOn)
	if err != nil {
		return LinkContractRelationInput{}, err
	}
	note, err := optionalText("note", "关系说明不能超过 500 个字符", input.Note, 500)
	if err != nil {
		return LinkContractRelationInput{}, err
	}
	actor, err := normalizeActor(input.Actor)
	if err != nil {
		return LinkContractRelationInput{}, err
	}
	return LinkContractRelationInput{
		CommandID: commandID, SourceContractID: sourceID, TargetContractID: targetID,
		RelationType: string(relationType), ExpectedSourceRevision: input.ExpectedSourceRevision,
		ExpectedTargetRevision: input.ExpectedTargetRevision, EffectiveOn: effectiveOn, Note: note, Actor: actor,
	}, nil
}

func sameContractRelationIntent(left, right ContractRelationCommand) bool {
	return left.CommandID == right.CommandID &&
		left.SourceContractID == right.SourceContractID &&
		left.TargetContractID == right.TargetContractID &&
		left.RelationType == right.RelationType &&
		left.ExpectedSourceRevision == right.ExpectedSourceRevision &&
		left.ExpectedTargetRevision == right.ExpectedTargetRevision &&
		left.EffectiveOn == right.EffectiveOn &&
		left.Note == right.Note &&
		left.Actor == right.Actor
}
