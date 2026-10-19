package ledgerrestore

import (
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"omsbizfoundry/contract-ledger/internal/audit"
	"omsbizfoundry/contract-ledger/internal/ledgerexport"
)

var (
	currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)
	decimalPattern  = regexp.MustCompile(`^-?[0-9]+(?:\.[0-9]+)?$`)
	sha256Pattern   = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

func validateLedger(ledger ledgerexport.Ledger) error {
	if ledger.DraftParties == nil || ledger.Revisions == nil || ledger.Instruments == nil ||
		ledger.Relations == nil || ledger.Settlements == nil || ledger.Evidence == nil || ledger.Audit == nil {
		return invalid("ledger", "ledger 的集合字段不能缺失或为 null")
	}
	if err := validateContract(ledger); err != nil {
		return err
	}
	links, parties, err := validateDraftParties(ledger.DraftParties)
	if err != nil {
		return err
	}
	revisions, instruments, err := validateFormalHistory(ledger, links)
	if err != nil {
		return err
	}
	settlements, err := validateSettlements(ledger, revisions)
	if err != nil {
		return err
	}
	if err := validateClosure(ledger, revisions, settlements); err != nil {
		return err
	}
	if err := validateEvidence(ledger, instruments, settlements); err != nil {
		return err
	}
	if err := validateAuditAndCommands(ledger); err != nil {
		return err
	}
	_ = parties
	return nil
}

func validateContract(ledger ledgerexport.Ledger) error {
	c := ledger.Contract
	if !validUUID(c.ID) {
		return invalid("contract.id", "合同标识无效")
	}
	if err := requiredText("contract.number_namespace", c.NumberNamespace, 64); err != nil {
		return err
	}
	if err := requiredText("contract.contract_number", c.ContractNumber, 100); err != nil {
		return err
	}
	if err := requiredText("contract.title", c.Title, 200); err != nil {
		return err
	}
	if err := optionalText("contract.category", c.Category, 100); err != nil {
		return err
	}
	if !currencyPattern.MatchString(c.Currency) {
		return invalid("contract.currency", "合同币种无效")
	}
	if err := optionalAmount("contract.total_amount", c.TotalAmount, false); err != nil {
		return err
	}
	if err := optionalDate("contract.signed_on", c.SignedOn); err != nil {
		return err
	}
	if err := optionalDate("contract.effective_on", c.EffectiveOn); err != nil {
		return err
	}
	createdAt, err := parseTimestamp("contract.created_at", c.CreatedAt)
	if err != nil {
		return err
	}
	updatedAt, err := parseTimestamp("contract.updated_at", c.UpdatedAt)
	if err != nil {
		return err
	}
	if updatedAt.Before(createdAt) {
		return invalid("contract.updated_at", "合同更新时间不能早于创建时间")
	}
	if c.DraftVersion < 1 {
		return invalid("contract.draft_version", "合同草稿版本无效")
	}
	switch c.Status {
	case "draft":
		if c.CurrentRevision != nil || ledger.Cancellation != nil || len(ledger.Revisions) != 0 || len(ledger.Instruments) != 0 || ledger.Closure != nil {
			return invalid("contract.status", "草稿合同包含不允许的正式或取消事实")
		}
	case "cancelled":
		if c.CurrentRevision != nil || ledger.Cancellation == nil || len(ledger.Revisions) != 0 || len(ledger.Instruments) != 0 || ledger.Closure != nil {
			return invalid("contract.status", "已取消合同的生命周期事实不完整")
		}
	case "active", "terminated", "closed":
		if c.CurrentRevision == nil || *c.CurrentRevision < 1 || ledger.Cancellation != nil {
			return invalid("contract.status", "正式合同的 current revision 或取消事实无效")
		}
		if c.SignedOn == nil || c.EffectiveOn == nil {
			return invalid("contract", "正式合同必须有签署和生效日期")
		}
		if c.Status == "closed" && ledger.Closure == nil {
			return invalid("contract.status", "已关闭合同缺少关闭事实")
		}
		if c.Status != "closed" && ledger.Closure != nil {
			return invalid("contract.status", "未关闭合同不能包含关闭事实")
		}
	default:
		return invalid("contract.status", "合同状态无效")
	}
	if ledger.Cancellation != nil {
		cancel := ledger.Cancellation
		if !validUUID(cancel.CommandID) || cancel.ExpectedDraftVersion < 1 || cancel.ResultingDraftVersion != cancel.ExpectedDraftVersion+1 || cancel.ResultingDraftVersion != c.DraftVersion {
			return invalid("cancellation", "合同取消命令版本或标识无效")
		}
		if err := requiredText("cancellation.reason", cancel.Reason, 500); err != nil {
			return err
		}
		if err := validateActor("cancellation.actor", cancel.Actor); err != nil {
			return err
		}
		if _, err := parseTimestamp("cancellation.cancelled_at", cancel.CancelledAt); err != nil {
			return err
		}
	}
	return nil
}

func validateDraftParties(values []ledgerexport.DraftParty) (map[string]ledgerexport.DraftParty, map[string]ledgerexport.DraftParty, error) {
	links := make(map[string]ledgerexport.DraftParty, len(values))
	parties := make(map[string]ledgerexport.DraftParty, len(values))
	for index, value := range values {
		field := fmt.Sprintf("draft_parties[%d]", index)
		if !validUUID(value.ID) || !validUUID(value.PartyID) {
			return nil, nil, invalid(field, "草稿主体或关联标识无效")
		}
		if _, exists := links[value.ID]; exists {
			return nil, nil, invalid(field+".id", "草稿主体关联标识重复")
		}
		if value.PartyKind != "organization" && value.PartyKind != "individual" {
			return nil, nil, invalid(field+".party_kind", "主体类型无效")
		}
		if err := requiredText(field+".party_legal_name", value.PartyLegalName, 200); err != nil {
			return nil, nil, err
		}
		if err := optionalText(field+".registration_identifier", value.RegistrationIdentifier, 100); err != nil {
			return nil, nil, err
		}
		if value.PartyRevision < 1 {
			return nil, nil, invalid(field+".party_revision", "主体 revision 无效")
		}
		if err := requiredText(field+".role", value.Role, 100); err != nil {
			return nil, nil, err
		}
		addedAt, err := parseTimestamp(field+".added_at", value.AddedAt)
		if err != nil {
			return nil, nil, err
		}
		if value.RemovedAt != nil {
			removedAt, err := parseTimestamp(field+".removed_at", *value.RemovedAt)
			if err != nil {
				return nil, nil, err
			}
			if removedAt.Before(addedAt) {
				return nil, nil, invalid(field+".removed_at", "主体移除时间不能早于添加时间")
			}
		}
		if prior, exists := parties[value.PartyID]; exists && !sameParty(prior, value) {
			return nil, nil, invalid(field+".party_id", "同一主体标识对应了不同主体资料")
		}
		links[value.ID] = value
		parties[value.PartyID] = value
	}
	return links, parties, nil
}

func validateFormalHistory(ledger ledgerexport.Ledger, links map[string]ledgerexport.DraftParty) (map[int64]ledgerexport.Revision, map[string]ledgerexport.Instrument, error) {
	revisions := make(map[int64]ledgerexport.Revision, len(ledger.Revisions))
	instruments := make(map[string]ledgerexport.Instrument, len(ledger.Instruments))
	if len(ledger.Revisions) != len(ledger.Instruments) {
		return nil, nil, invalid("revisions", "Revision 与合同文书数量不一致")
	}
	if ledger.Contract.CurrentRevision != nil && int64(len(ledger.Revisions)) != *ledger.Contract.CurrentRevision {
		return nil, nil, invalid("contract.current_revision", "current revision 与历史长度不一致")
	}
	for index, instrument := range ledger.Instruments {
		field := fmt.Sprintf("instruments[%d]", index)
		expected := int64(index + 1)
		if !validUUID(instrument.ID) || !validUUID(instrument.SourceCommandID) {
			return nil, nil, invalid(field, "合同文书或来源命令标识无效")
		}
		if _, exists := instruments[instrument.ID]; exists {
			return nil, nil, invalid(field+".id", "合同文书标识重复")
		}
		if instrument.EffectiveOrder != expected || instrument.ResultingRevision != expected {
			return nil, nil, invalid(field, "合同文书顺序必须与 resulting revision 连续一致")
		}
		if err := requiredText(field+".instrument_number", instrument.InstrumentNumber, 100); err != nil {
			return nil, nil, err
		}
		if err := requiredText(field+".title", instrument.Title, 200); err != nil {
			return nil, nil, err
		}
		if err := requiredText(field+".change_summary", instrument.ChangeSummary, 1000); err != nil {
			return nil, nil, err
		}
		if _, err := parseDate(field+".signed_on", instrument.SignedOn); err != nil {
			return nil, nil, err
		}
		if _, err := parseDate(field+".effective_on", instrument.EffectiveOn); err != nil {
			return nil, nil, err
		}
		if err := validateActor(field+".actor", instrument.Actor); err != nil {
			return nil, nil, err
		}
		if _, err := parseTimestamp(field+".recorded_at", instrument.RecordedAt); err != nil {
			return nil, nil, err
		}
		switch instrument.InstrumentType {
		case "original_contract":
			if expected != 1 || instrument.AmountEffect != "original" {
				return nil, nil, invalid(field, "原合同文书语义无效")
			}
			if err := optionalAmount(field+".amount_value", instrument.AmountValue, false); err != nil {
				return nil, nil, err
			}
		case "supplemental_agreement":
			if expected < 2 || (instrument.AmountEffect != "adjustment" && instrument.AmountEffect != "restatement" && instrument.AmountEffect != "unchanged") {
				return nil, nil, invalid(field, "补充协议金额语义无效")
			}
			if instrument.AmountEffect == "unchanged" && instrument.AmountValue != nil {
				return nil, nil, invalid(field+".amount_value", "金额不变时不能有金额值")
			}
			if instrument.AmountEffect != "unchanged" {
				if instrument.AmountValue == nil {
					return nil, nil, invalid(field+".amount_value", "补充协议缺少金额值")
				}
				if err := validAmount(field+".amount_value", *instrument.AmountValue, instrument.AmountEffect == "adjustment"); err != nil {
					return nil, nil, err
				}
				if instrument.AmountEffect == "adjustment" && decimalIsZero(*instrument.AmountValue) {
					return nil, nil, invalid(field+".amount_value", "金额调整值不能为零")
				}
			}
		case "termination_instrument":
			if expected < 2 || instrument.AmountEffect != "unchanged" || instrument.AmountValue != nil {
				return nil, nil, invalid(field, "终止文书金额语义无效")
			}
		case "correction_instrument":
			if expected < 2 || instrument.AmountEffect != "corrected" {
				return nil, nil, invalid(field, "更正文书金额语义无效")
			}
			if err := optionalAmount(field+".amount_value", instrument.AmountValue, false); err != nil {
				return nil, nil, err
			}
		default:
			return nil, nil, invalid(field+".instrument_type", "合同文书类型无效")
		}
		instruments[instrument.ID] = instrument
	}
	for index, revision := range ledger.Revisions {
		field := fmt.Sprintf("revisions[%d]", index)
		expected := int64(index + 1)
		if revision.Revision != expected || !validUUID(revision.SourceInstrumentID) {
			return nil, nil, invalid(field, "合同 Revision 顺序或来源文书无效")
		}
		instrument, found := instruments[revision.SourceInstrumentID]
		if !found || instrument.ResultingRevision != revision.Revision {
			return nil, nil, invalid(field+".source_instrument_id", "合同 Revision 来源文书引用无效")
		}
		if expected == 1 {
			if revision.ActivationCommandID == nil || !validUUID(*revision.ActivationCommandID) || revision.SourceDraftVersion == nil || *revision.SourceDraftVersion != ledger.Contract.DraftVersion || instrument.SourceCommandID != *revision.ActivationCommandID {
				return nil, nil, invalid(field, "首个 Revision 的激活来源无效")
			}
		} else if revision.ActivationCommandID != nil || revision.SourceDraftVersion != nil || instrument.SourceCommandID == "" {
			return nil, nil, invalid(field, "后续 Revision 不能包含激活草稿来源")
		}
		if err := validateRevisionFields(field, revision); err != nil {
			return nil, nil, err
		}
		if !sameActor(revision.Actor, instrument.Actor) || revision.RecordedAt != instrument.RecordedAt {
			return nil, nil, invalid(field, "Revision 与来源文书的操作者或记录时间不一致")
		}
		if revision.Parties == nil || len(revision.Parties) == 0 {
			return nil, nil, invalid(field+".parties", "正式 Revision 必须包含主体快照")
		}
		for partyIndex, snapshot := range revision.Parties {
			partyField := fmt.Sprintf("%s.parties[%d]", field, partyIndex)
			if snapshot.Position != int64(partyIndex+1) {
				return nil, nil, invalid(partyField+".position", "主体快照位置必须连续")
			}
			link, found := links[snapshot.SourcePartyLinkID]
			if !found || !sameSnapshotAndLink(snapshot, link) {
				return nil, nil, invalid(partyField, "主体快照与草稿主体引用不一致")
			}
		}
		if index > 0 && !sameSnapshotList(revision.Parties, ledger.Revisions[index-1].Parties) {
			return nil, nil, invalid(field+".parties", "后续 Revision 必须延续上一版主体快照")
		}
		if err := validateRevisionTransition(field, ledger.Revisions, ledger.Instruments, index); err != nil {
			return nil, nil, err
		}
		revisions[revision.Revision] = revision
	}
	if len(ledger.Revisions) > 0 {
		latest := ledger.Revisions[len(ledger.Revisions)-1]
		if !sameContractProjection(ledger.Contract, latest) {
			return nil, nil, invalid("contract", "合同当前投影与最新 Revision 不一致")
		}
	}
	terminated := false
	for _, instrument := range ledger.Instruments {
		if instrument.InstrumentType == "termination_instrument" {
			terminated = true
		}
	}
	if ledger.Contract.Status == "active" && terminated {
		return nil, nil, invalid("contract.status", "已激活合同不能包含终止文书")
	}
	if ledger.Contract.Status == "terminated" && !terminated {
		return nil, nil, invalid("contract.status", "已终止合同缺少终止文书")
	}
	return revisions, instruments, nil
}

func validateRevisionFields(field string, value ledgerexport.Revision) error {
	if err := requiredText(field+".number_namespace", value.NumberNamespace, 64); err != nil {
		return err
	}
	if err := requiredText(field+".contract_number", value.ContractNumber, 100); err != nil {
		return err
	}
	if err := requiredText(field+".title", value.Title, 200); err != nil {
		return err
	}
	if err := optionalText(field+".category", value.Category, 100); err != nil {
		return err
	}
	if !currencyPattern.MatchString(value.Currency) {
		return invalid(field+".currency", "Revision 币种无效")
	}
	if err := optionalAmount(field+".total_amount", value.TotalAmount, false); err != nil {
		return err
	}
	if _, err := parseDate(field+".signed_on", value.SignedOn); err != nil {
		return err
	}
	if _, err := parseDate(field+".effective_on", value.EffectiveOn); err != nil {
		return err
	}
	if err := validateActor(field+".actor", value.Actor); err != nil {
		return err
	}
	_, err := parseTimestamp(field+".recorded_at", value.RecordedAt)
	return err
}

func validateSettlements(ledger ledgerexport.Ledger, revisions map[int64]ledgerexport.Revision) (map[string]ledgerexport.Settlement, error) {
	values := make(map[string]ledgerexport.Settlement, len(ledger.Settlements))
	finalSeen := false
	for index, value := range ledger.Settlements {
		field := fmt.Sprintf("settlements[%d]", index)
		if !validUUID(value.ID) || value.Sequence != int64(index+1) {
			return nil, invalid(field, "结算标识或 sequence 无效")
		}
		if _, found := revisions[value.SourceRevision]; !found {
			return nil, invalid(field+".source_revision", "结算来源 Revision 不存在")
		}
		if value.Kind != "progress" && value.Kind != "final" {
			return nil, invalid(field+".kind", "结算类型无效")
		}
		if finalSeen {
			return nil, invalid(field+".kind", "最终结算之后不能再有结算")
		}
		if value.Kind == "final" {
			finalSeen = true
		}
		if err := requiredText(field+".statement_number", value.StatementNumber, 100); err != nil {
			return nil, err
		}
		if err := requiredText(field+".title", value.Title, 200); err != nil {
			return nil, err
		}
		if err := validAmount(field+".amount", value.Amount, false); err != nil {
			return nil, err
		}
		if !currencyPattern.MatchString(value.Currency) || value.Currency != revisions[value.SourceRevision].Currency {
			return nil, invalid(field+".currency", "结算币种与来源 Revision 不一致")
		}
		if _, err := parseDate(field+".confirmed_on", value.ConfirmedOn); err != nil {
			return nil, err
		}
		if err := requiredText(field+".basis", value.Basis, 1000); err != nil {
			return nil, err
		}
		if err := validateActor(field+".actor", value.Actor); err != nil {
			return nil, err
		}
		if _, err := parseTimestamp(field+".recorded_at", value.RecordedAt); err != nil {
			return nil, err
		}
		if value.ConfirmingParties == nil || len(value.ConfirmingParties) == 0 {
			return nil, invalid(field+".confirming_parties", "结算至少需要一个确认主体")
		}
		revisionParties := revisions[value.SourceRevision].Parties
		positions := make(map[int64]bool)
		for partyIndex, snapshot := range value.ConfirmingParties {
			partyField := fmt.Sprintf("%s.confirming_parties[%d]", field, partyIndex)
			if positions[snapshot.Position] || snapshot.Position < 1 || snapshot.Position > int64(len(revisionParties)) {
				return nil, invalid(partyField+".position", "结算确认主体位置无效或重复")
			}
			positions[snapshot.Position] = true
			if !sameSnapshot(snapshot, revisionParties[snapshot.Position-1]) {
				return nil, invalid(partyField, "结算确认主体与来源 Revision 不一致")
			}
		}
		if _, exists := values[value.ID]; exists {
			return nil, invalid(field+".id", "结算标识重复")
		}
		values[value.ID] = value
	}
	return values, nil
}

func validateClosure(ledger ledgerexport.Ledger, revisions map[int64]ledgerexport.Revision, settlements map[string]ledgerexport.Settlement) error {
	if ledger.Closure == nil {
		return nil
	}
	value := ledger.Closure
	if !validUUID(value.ID) {
		return invalid("closure.id", "关闭事实标识无效")
	}
	if _, found := revisions[value.SourceRevision]; !found {
		return invalid("closure.source_revision", "关闭来源 Revision 不存在")
	}
	if value.SourceSettlementSequence != int64(len(ledger.Settlements)) {
		return invalid("closure.source_settlement_sequence", "关闭来源结算 sequence 不一致")
	}
	switch value.SettlementDisposition {
	case "final_recorded":
		if value.FinalSettlementID == nil {
			return invalid("closure.final_settlement_id", "关闭缺少最终结算引用")
		}
		settlement, found := settlements[*value.FinalSettlementID]
		if !found || settlement.Kind != "final" {
			return invalid("closure.final_settlement_id", "关闭引用的最终结算无效")
		}
	case "not_required":
		if value.FinalSettlementID != nil {
			return invalid("closure.final_settlement_id", "无需结算时不能引用最终结算")
		}
	default:
		return invalid("closure.settlement_disposition", "关闭结算处置无效")
	}
	if _, err := parseDate("closure.closed_on", value.ClosedOn); err != nil {
		return err
	}
	if err := requiredText("closure.reason", value.Reason, 1000); err != nil {
		return err
	}
	if err := validateActor("closure.actor", value.Actor); err != nil {
		return err
	}
	_, err := parseTimestamp("closure.recorded_at", value.RecordedAt)
	return err
}

func validateEvidence(ledger ledgerexport.Ledger, instruments map[string]ledgerexport.Instrument, settlements map[string]ledgerexport.Settlement) error {
	documents := make(map[string]bool)
	versions := make(map[string]bool)
	for index, document := range ledger.Evidence {
		field := fmt.Sprintf("evidence[%d]", index)
		if !validUUID(document.ID) || documents[document.ID] {
			return invalid(field+".id", "Evidence Document 标识无效或重复")
		}
		documents[document.ID] = true
		switch document.TargetType {
		case "contract":
			if document.TargetID != ledger.Contract.ID {
				return invalid(field+".target_id", "合同 Evidence 目标无效")
			}
		case "instrument":
			if _, found := instruments[document.TargetID]; !found {
				return invalid(field+".target_id", "Evidence 文书目标不存在")
			}
		case "settlement":
			if _, found := settlements[document.TargetID]; !found {
				return invalid(field+".target_id", "Evidence 结算目标不存在")
			}
		case "closure":
			if ledger.Closure == nil || document.TargetID != ledger.Closure.ID {
				return invalid(field+".target_id", "Evidence 关闭目标不存在")
			}
		default:
			return invalid(field+".target_type", "Evidence 目标类型无效")
		}
		if err := requiredText(field+".title", document.Title, 200); err != nil {
			return err
		}
		if err := requiredText(field+".purpose", document.Purpose, 500); err != nil {
			return err
		}
		if err := validateActor(field+".actor", document.Actor); err != nil {
			return err
		}
		if _, err := parseTimestamp(field+".created_at", document.CreatedAt); err != nil {
			return err
		}
		if document.Versions == nil || len(document.Versions) == 0 {
			return invalid(field+".versions", "Evidence Document 必须至少包含一个 Version")
		}
		for versionIndex, version := range document.Versions {
			versionField := fmt.Sprintf("%s.versions[%d]", field, versionIndex)
			if !validUUID(version.ID) || versions[version.ID] {
				return invalid(versionField+".id", "Evidence Version 标识无效或重复")
			}
			versions[version.ID] = true
			if version.Version != int64(versionIndex+1) {
				return invalid(versionField+".version", "Evidence Version 必须连续")
			}
			if err := requiredFilename(versionField+".original_filename", version.OriginalFilename); err != nil {
				return err
			}
			if err := requiredText(versionField+".detected_media_type", version.DetectedMediaType, 200); err != nil {
				return err
			}
			if version.SizeBytes < 1 || !sha256Pattern.MatchString(version.SHA256) {
				return invalid(versionField, "Evidence 文件大小或 SHA-256 无效")
			}
			if err := validateActor(versionField+".actor", version.Actor); err != nil {
				return err
			}
			if _, err := parseTimestamp(versionField+".recorded_at", version.RecordedAt); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateAuditAndCommands(ledger ledgerexport.Ledger) error {
	ids := make(map[string]bool)
	actionCommands := make(map[string]bool)
	for index, entry := range ledger.Audit {
		field := fmt.Sprintf("audit[%d]", index)
		if !validUUID(entry.ID) || ids[entry.ID] {
			return invalid(field+".id", "审计标识无效或重复")
		}
		ids[entry.ID] = true
		if err := requiredText(field+".action", entry.Action, 100); err != nil {
			return err
		}
		if entry.CommandID != nil {
			if !validUUID(*entry.CommandID) {
				return invalid(field+".command_id", "审计命令标识无效")
			}
			key := entry.Action + ":" + *entry.CommandID
			if actionCommands[key] {
				return invalid(field+".command_id", "审计 action 与 command 组合重复")
			}
			actionCommands[key] = true
		}
		if err := validateActor(field+".actor", entry.Actor); err != nil {
			return err
		}
		if _, err := parseTimestamp(field+".occurred_at", entry.OccurredAt); err != nil {
			return err
		}
		if err := requiredText(field+".summary", entry.Summary, 500); err != nil {
			return err
		}
		var details map[string]any
		if len(entry.Details) == 0 || json.Unmarshal(entry.Details, &details) != nil || details == nil {
			return invalid(field+".details", "审计 details 必须是 JSON object")
		}
	}
	if _, err := uniqueAudit(ledger.Audit, "contract_draft_created", "", ""); err != nil {
		return err
	}
	createdAudit, _ := uniqueAudit(ledger.Audit, "contract_draft_created", "", "")
	if createdAudit.OccurredAt != ledger.Contract.CreatedAt {
		return invalid("audit", "合同创建审计时间与合同创建时间不一致")
	}
	if ledger.Cancellation != nil {
		entry, err := uniqueAudit(ledger.Audit, "contract_draft_cancelled", "", "")
		if err != nil || entry.CommandID == nil || *entry.CommandID != ledger.Cancellation.CommandID ||
			!sameActor(entry.Actor, ledger.Cancellation.Actor) || entry.OccurredAt != ledger.Cancellation.CancelledAt {
			return invalid("cancellation.command_id", "找不到合同取消来源审计")
		}
	}
	for _, instrument := range ledger.Instruments {
		action := "contract_activated"
		key := ""
		value := ""
		switch instrument.InstrumentType {
		case "supplemental_agreement":
			action, key, value = "contract_change_activated", "instrument_id", instrument.ID
		case "termination_instrument":
			action, key, value = "contract_terminated", "instrument_id", instrument.ID
		case "correction_instrument":
			action, key, value = "contract_correction_recorded", "instrument_id", instrument.ID
		}
		entry, err := uniqueAudit(ledger.Audit, action, key, value)
		if err != nil || entry.CommandID == nil || *entry.CommandID != instrument.SourceCommandID ||
			!sameActor(entry.Actor, instrument.Actor) || entry.OccurredAt != instrument.RecordedAt {
			return invalid("audit", "找不到合同文书的来源命令审计")
		}
		if instrument.InstrumentType == "correction_instrument" {
			details, _ := auditDetails(entry)
			expectedStatus, _ := details["expected_status"].(string)
			if expectedStatus != "active" && expectedStatus != "terminated" && expectedStatus != "closed" {
				return invalid("audit", "更正文书来源审计缺少有效 expected_status")
			}
		}
	}
	for _, settlement := range ledger.Settlements {
		entry, err := uniqueAudit(ledger.Audit, "contract_settlement_recorded", "settlement_id", settlement.ID)
		if err != nil {
			return err
		}
		if !sameActor(entry.Actor, settlement.Actor) || entry.OccurredAt != settlement.RecordedAt {
			return invalid("audit", "结算来源审计的操作者或时间不一致")
		}
	}
	if ledger.Closure != nil {
		entry, err := uniqueAudit(ledger.Audit, "contract_closed", "closure_id", ledger.Closure.ID)
		if err != nil {
			return err
		}
		if !sameActor(entry.Actor, ledger.Closure.Actor) || entry.OccurredAt != ledger.Closure.RecordedAt {
			return invalid("audit", "关闭来源审计的操作者或时间不一致")
		}
	}
	for _, document := range ledger.Evidence {
		for index, version := range document.Versions {
			action := "contract_evidence_version_added"
			if index == 0 {
				action = "contract_evidence_created"
			}
			entry, err := uniqueAudit(ledger.Audit, action, "version_id", version.ID)
			if err != nil {
				return err
			}
			if !sameActor(entry.Actor, version.Actor) || entry.OccurredAt != version.RecordedAt {
				return invalid("audit", "Evidence 来源审计的操作者或时间不一致")
			}
		}
	}
	return nil
}

func uniqueAudit(entries []ledgerexport.AuditEntry, action, detailKey, detailValue string) (ledgerexport.AuditEntry, error) {
	var result ledgerexport.AuditEntry
	count := 0
	for _, entry := range entries {
		if entry.Action != action {
			continue
		}
		if detailKey != "" {
			details, ok := auditDetails(entry)
			if !ok || details[detailKey] != detailValue {
				continue
			}
		}
		result = entry
		count++
	}
	if count != 1 || result.CommandID == nil {
		return ledgerexport.AuditEntry{}, invalid("audit", fmt.Sprintf("来源审计 %s 缺失或不唯一", action))
	}
	return result, nil
}

func auditDetails(entry ledgerexport.AuditEntry) (map[string]any, bool) {
	var details map[string]any
	if json.Unmarshal(entry.Details, &details) != nil || details == nil {
		return nil, false
	}
	return details, true
}

func validUUID(value string) bool {
	return uuidPattern.MatchString(value) && value == strings.ToLower(value)
}

func requiredText(field, value string, max int) error {
	if value == "" || value != strings.TrimSpace(value) || !utf8.ValidString(value) || len([]rune(value)) > max {
		return invalid(field, "文本为空、含首尾空白或超过长度限制")
	}
	return nil
}

func optionalText(field string, value *string, max int) error {
	if value == nil {
		return nil
	}
	return requiredText(field, *value, max)
}

func requiredFilename(field, value string) error {
	if err := requiredText(field, value, 255); err != nil {
		return err
	}
	if strings.ContainsAny(value, "/\\") {
		return invalid(field, "Evidence 文件名不能包含路径分隔符")
	}
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return invalid(field, "Evidence 文件名不能包含控制字符")
		}
	}
	return nil
}

func validateActor(field string, value ledgerexport.Actor) error {
	normalized, err := audit.NormalizeActor(audit.ActorRef{Kind: value.Kind, ID: value.ID, DisplayName: value.DisplayName})
	if err != nil || normalized.Kind != value.Kind || normalized.ID != value.ID || normalized.DisplayName != value.DisplayName {
		return invalid(field, "操作者引用无效或未规范化")
	}
	return nil
}

func parseTimestamp(field, value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, invalid(field, "时间格式无效")
	}
	return parsed, nil
}

func parseDate(field, value string) (time.Time, error) {
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, invalid(field, "日期格式无效")
	}
	return parsed, nil
}

func optionalDate(field string, value *string) error {
	if value == nil {
		return nil
	}
	_, err := parseDate(field, *value)
	return err
}

func optionalAmount(field string, value *string, signed bool) error {
	if value == nil {
		return nil
	}
	return validAmount(field, *value, signed)
}

func validAmount(field, value string, signed bool) error {
	if !decimalPattern.MatchString(value) || (!signed && strings.HasPrefix(value, "-")) {
		return invalid(field, "十进制金额格式无效")
	}
	normalized := strings.TrimPrefix(value, "-")
	parts := strings.SplitN(normalized, ".", 2)
	if len(parts[0]) > 18 || (len(parts) == 2 && len(parts[1]) > 6) {
		return invalid(field, "十进制金额超过 numeric(24,6) 精度")
	}
	return nil
}

func decimalIsZero(value string) bool {
	value = strings.TrimPrefix(value, "-")
	value = strings.ReplaceAll(value, ".", "")
	return strings.Trim(value, "0") == ""
}

func sameParty(a, b ledgerexport.DraftParty) bool {
	return a.PartyKind == b.PartyKind && a.PartyLegalName == b.PartyLegalName &&
		stringPtrEqual(a.RegistrationIdentifier, b.RegistrationIdentifier) && a.PartyRevision == b.PartyRevision
}

func sameSnapshotAndLink(snapshot ledgerexport.PartySnapshot, link ledgerexport.DraftParty) bool {
	return snapshot.SourcePartyLinkID == link.ID && snapshot.SourcePartyID == link.PartyID &&
		snapshot.SourcePartyRevision == link.PartyRevision && snapshot.PartyKind == link.PartyKind &&
		snapshot.LegalName == link.PartyLegalName && stringPtrEqual(snapshot.RegistrationIdentifier, link.RegistrationIdentifier) &&
		snapshot.Role == link.Role
}

func sameSnapshot(a, b ledgerexport.PartySnapshot) bool {
	return a.Position == b.Position && a.SourcePartyLinkID == b.SourcePartyLinkID && a.SourcePartyID == b.SourcePartyID &&
		a.SourcePartyRevision == b.SourcePartyRevision && a.PartyKind == b.PartyKind && a.LegalName == b.LegalName &&
		stringPtrEqual(a.RegistrationIdentifier, b.RegistrationIdentifier) && a.Role == b.Role
}

func sameContractProjection(contract ledgerexport.ContractSnapshot, revision ledgerexport.Revision) bool {
	return contract.NumberNamespace == revision.NumberNamespace && contract.ContractNumber == revision.ContractNumber &&
		contract.Title == revision.Title && stringPtrEqual(contract.Category, revision.Category) && contract.Currency == revision.Currency &&
		stringPtrEqual(contract.TotalAmount, revision.TotalAmount) && contract.SignedOn != nil && *contract.SignedOn == revision.SignedOn &&
		contract.EffectiveOn != nil && *contract.EffectiveOn == revision.EffectiveOn
}

func validateRevisionTransition(field string, revisions []ledgerexport.Revision, instruments []ledgerexport.Instrument, index int) error {
	current := revisions[index]
	instrument := instruments[index]
	if index == 0 {
		if instrument.InstrumentNumber != current.ContractNumber || instrument.Title != current.Title ||
			instrument.SignedOn != current.SignedOn || instrument.EffectiveOn != current.EffectiveOn ||
			!amountPtrEqual(instrument.AmountValue, current.TotalAmount) {
			return invalid(field, "首个 Revision 与原合同文书不一致")
		}
		return nil
	}
	previous := revisions[index-1]
	if current.NumberNamespace != previous.NumberNamespace || current.ContractNumber != previous.ContractNumber || current.Currency != previous.Currency {
		return invalid(field, "后续 Revision 改变了不允许变更的编号或币种")
	}
	switch instrument.InstrumentType {
	case "supplemental_agreement":
		if current.Title != previous.Title || !stringPtrEqual(current.Category, previous.Category) ||
			current.SignedOn != previous.SignedOn || current.EffectiveOn != previous.EffectiveOn {
			return invalid(field, "补充协议 Revision 改变了金额以外的合同核心字段")
		}
		expected, ok := resultingAmount(previous.TotalAmount, instrument.AmountEffect, instrument.AmountValue)
		if !ok || !amountPtrEqual(expected, current.TotalAmount) {
			return invalid(field+".total_amount", "补充协议金额结果与上一 Revision 不一致")
		}
	case "termination_instrument":
		if !sameRevisionProjection(previous, current) {
			return invalid(field, "终止 Revision 必须保持上一版合同核心字段")
		}
	case "correction_instrument":
		if !amountPtrEqual(instrument.AmountValue, current.TotalAmount) {
			return invalid(field+".total_amount", "更正文书金额与 Revision 不一致")
		}
		if sameRevisionProjection(previous, current) {
			return invalid(field, "更正 Revision 没有改变任何可更正字段")
		}
	}
	return nil
}

func resultingAmount(previous *string, effect string, value *string) (*string, bool) {
	switch effect {
	case "unchanged":
		return previous, true
	case "restatement":
		return value, value != nil
	case "adjustment":
		if previous == nil || value == nil {
			return nil, false
		}
		left, leftOK := new(big.Rat).SetString(*previous)
		right, rightOK := new(big.Rat).SetString(*value)
		if !leftOK || !rightOK {
			return nil, false
		}
		result := new(big.Rat).Add(left, right)
		if result.Sign() < 0 {
			return nil, false
		}
		text := result.FloatString(6)
		return &text, true
	default:
		return nil, false
	}
}

func amountPtrEqual(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	left, leftOK := new(big.Rat).SetString(*a)
	right, rightOK := new(big.Rat).SetString(*b)
	return leftOK && rightOK && left.Cmp(right) == 0
}

func sameRevisionProjection(a, b ledgerexport.Revision) bool {
	return a.NumberNamespace == b.NumberNamespace && a.ContractNumber == b.ContractNumber &&
		a.Title == b.Title && stringPtrEqual(a.Category, b.Category) && a.Currency == b.Currency &&
		amountPtrEqual(a.TotalAmount, b.TotalAmount) && a.SignedOn == b.SignedOn && a.EffectiveOn == b.EffectiveOn
}

func sameSnapshotList(a, b []ledgerexport.PartySnapshot) bool {
	if len(a) != len(b) {
		return false
	}
	for index := range a {
		if !sameSnapshot(a[index], b[index]) {
			return false
		}
	}
	return true
}

func sameActor(a, b ledgerexport.Actor) bool {
	return a.Kind == b.Kind && a.ID == b.ID && a.DisplayName == b.DisplayName
}

func stringPtrEqual(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}
