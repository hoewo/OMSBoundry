// Package referencecost is the explicitly selected reference-oa-cost-v1 overlay.
// It does not weaken or create shadow instances of the generic CostEntry.
package referencecost

import (
	"crypto/rand"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	core "omsbizfoundry/cost-outsourcing/internal/costoutsourcing"
)

const Profile = "reference-oa-cost-v1"
const ResourceType = "reference_oa_cost_entry"

var uuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
var decimal = regexp.MustCompile(`^(0|[1-9][0-9]*)$`)
var digest = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
var categories = map[string]bool{"vehicle": true, "accommodation": true, "transport": true, "expert_fee": true, "bid_signup_fee": true, "bid_service_fee": true, "other": true}

type Fields struct {
	Category     string             `json:"category"`
	AmountMinor  string             `json:"amount_minor"`
	OccurredOn   *string            `json:"occurred_on"`
	Description  *string            `json:"description"`
	EvidenceRefs []core.EvidenceRef `json:"evidence_refs"`
}
type CreateInput struct {
	CommandID  string          `json:"command_id"`
	ProjectRef core.ProjectRef `json:"project_ref"`
	Scope      string          `json:"scope"`
	Fields     Fields          `json:"fields"`
}
type ChangeInput struct {
	CommandID        string `json:"command_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Reason           string `json:"reason"`
	Fields           Fields `json:"fields"`
}
type WithdrawInput struct {
	CommandID        string `json:"command_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Reason           string `json:"reason"`
}

// Record is an internal persistence snapshot. HTTP must use its redacted view.
type Record struct {
	ID          string                  `json:"id"`
	TenantID    string                  `json:"tenant_id"`
	Project     core.ProjectSnapshot    `json:"project"`
	Scope       string                  `json:"scope"`
	Category    string                  `json:"category"`
	Currency    string                  `json:"currency"`
	AmountMinor int64                   `json:"amount_minor"`
	OccurredOn  *string                 `json:"occurred_on"`
	Description *string                 `json:"description"`
	Evidence    []core.EvidenceSnapshot `json:"evidence"`
	State       string                  `json:"state"`
	Revision    int64                   `json:"revision"`
	CreatedBy   string                  `json:"created_by"`
	UpdatedBy   string                  `json:"updated_by"`
	CreatedAt   time.Time               `json:"created_at"`
	UpdatedAt   time.Time               `json:"updated_at"`
}
type Revision struct {
	Record    Record `json:"snapshot"`
	CommandID string `json:"command_id"`
	Operation string `json:"operation"`
	Reason    string `json:"reason"`
}

func SideKind(scope string) string {
	if scope == "business" {
		return "business_cost_project"
	}
	if scope == "production" {
		return "production_cost_project"
	}
	return ""
}
func Can(a core.Actor, action, projectID, scope, id string) bool {
	if !ValidActor(a) || SideKind(scope) == "" || !uuid.MatchString(projectID) {
		return false
	}
	allowed := false
	for _, v := range a.Actions {
		if v == action {
			allowed = true
		}
	}
	if !allowed {
		return false
	}
	if a.TenantScope {
		return true
	}
	for _, s := range a.ResourceScopes {
		if s.Type == SideKind(scope) && s.ID == projectID || id != "" && s.Type == ResourceType && s.ID == id {
			return true
		}
	}
	// Existing generic project grants deliberately do not authorize this overlay.
	return false
}
func ValidActor(a core.Actor) bool {
	return uuid.MatchString(a.ID) && uuid.MatchString(a.TenantID) && uuid.MatchString(a.CredentialID) && (a.Kind == "iam_subject" || a.Kind == "template_subject")
}
func normalizeFields(f Fields, scope string) (Fields, int64, error) {
	f.Category = strings.TrimSpace(f.Category)
	if !categories[f.Category] || SideKind(scope) == "" || scope == "production" && (f.Category == "bid_signup_fee" || f.Category == "bid_service_fee") {
		return f, 0, fmt.Errorf("invalid cost category for scope")
	}
	if !decimal.MatchString(f.AmountMinor) {
		return f, 0, fmt.Errorf("invalid exact cost amount")
	}
	amount, err := strconv.ParseInt(f.AmountMinor, 10, 64)
	if err != nil {
		return f, 0, fmt.Errorf("invalid cost amount range")
	}
	if f.OccurredOn != nil {
		v := strings.TrimSpace(*f.OccurredOn)
		if v == "" {
			f.OccurredOn = nil
		} else {
			d, err := time.Parse("2006-01-02", v)
			if err != nil || d.Format("2006-01-02") != v || d.Year() < 1 {
				return f, 0, fmt.Errorf("invalid cost date")
			}
			f.OccurredOn = &v
		}
	}
	if f.Description != nil {
		v := strings.TrimSpace(*f.Description)
		if utf8.RuneCountInString(v) > 1000 {
			return f, 0, fmt.Errorf("invalid cost description length")
		}
		if v == "" {
			f.Description = nil
		} else {
			f.Description = &v
		}
	}
	if len(f.EvidenceRefs) > 30 {
		return f, 0, fmt.Errorf("invalid cost file count")
	}
	seen := map[string]bool{}
	for i, v := range f.EvidenceRefs {
		v.FileID = strings.ToLower(strings.TrimSpace(v.FileID))
		if !uuid.MatchString(v.FileID) || v.Version < 1 || v.Slot != "invoice" {
			return f, 0, fmt.Errorf("invalid cost evidence reference")
		}
		key := fmt.Sprintf("%s:%d", v.FileID, v.Version)
		if seen[key] {
			return f, 0, fmt.Errorf("duplicate cost evidence reference")
		}
		seen[key] = true
		f.EvidenceRefs[i] = v
	}
	if f.EvidenceRefs == nil {
		f.EvidenceRefs = []core.EvidenceRef{}
	}
	return f, amount, nil
}
func validateCommand(id string) error {
	if !uuid.MatchString(id) {
		return fmt.Errorf("invalid command id")
	}
	return nil
}
func reason(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || utf8.RuneCountInString(value) > 500 {
		return "", fmt.Errorf("invalid cost change reason")
	}
	return value, nil
}
func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = b[6]&15 | 64
	b[8] = b[8]&63 | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}
