package payablesuggestioncontract

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

const (
	SchemaVersion                        = "1"
	SourceApplication                    = "cost-outsourcing"
	SourceApplicationCostOutsourcing     = SourceApplication
	SourceApplicationBonusDistribution   = "bonus-distribution"
	SourceCostEntry                      = "cost_entry"
	SourceEngagement                     = "outsourcing_engagement"
	SourceBonusAllocation                = "bonus_allocation"
	PartySourceApplicationPartyDirectory = "party-directory"
	PartySourceApplicationMoneyFlow      = "money-flow-ledger"
	PartySourceApplicationPeopleGroups   = "people-groups"
	PartyResourceParty                   = "party"
	PartyResourcePerson                  = "person"
	StatusPending                        = "pending"
	StatusAccepted                       = "accepted"
	StatusRejected                       = "rejected"
	StatusWithdrawn                      = "withdrawn"
	DecisionAccept                       = "accept"
	DecisionReject                       = "reject"
)

type SourceSnapshot struct {
	Application  string `json:"application"`
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id"`
	Revision     int64  `json:"revision"`
	Digest       string `json:"digest"`
}

type PartySnapshot struct {
	SourceApplication string `json:"source_application,omitempty"`
	ResourceType      string `json:"resource_type,omitempty"`
	ID                string `json:"id"`
	Revision          int64  `json:"revision"`
	Kind              string `json:"kind"`
	LegalName         string `json:"legal_name"`
	Digest            string `json:"digest,omitempty"`
}

type Submission struct {
	SchemaVersion string         `json:"schema_version"`
	SuggestionID  string         `json:"suggestion_id"`
	TenantID      string         `json:"tenant_id"`
	ProjectID     string         `json:"project_id"`
	Source        SourceSnapshot `json:"source"`
	LedgerOwner   PartySnapshot  `json:"ledger_owner"`
	Counterparty  PartySnapshot  `json:"counterparty"`
	AmountMinor   int64          `json:"amount_minor"`
	Currency      string         `json:"currency"`
	DueOn         string         `json:"due_on"`
	Purpose       string         `json:"purpose"`
	RequestDigest string         `json:"request_digest"`
}

type ObligationReference struct {
	ID       string `json:"id"`
	Revision int64  `json:"revision"`
}

type Intake struct {
	SchemaVersion  string               `json:"schema_version"`
	SuggestionID   string               `json:"suggestion_id"`
	TenantID       string               `json:"tenant_id"`
	RequestDigest  string               `json:"request_digest"`
	Status         string               `json:"status"`
	Revision       int64                `json:"revision"`
	Obligation     *ObligationReference `json:"obligation,omitempty"`
	DecisionReason string               `json:"decision_reason,omitempty"`
}

type Decision struct {
	CommandID        string `json:"command_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Decision         string `json:"decision"`
	Reason           string `json:"reason"`
}

type Withdrawal struct {
	CommandID        string `json:"command_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Reason           string `json:"reason"`
}

func Digest(value Submission) (string, error) {
	value.RequestDigest = ""
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}
