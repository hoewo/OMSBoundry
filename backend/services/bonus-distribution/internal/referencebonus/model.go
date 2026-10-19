// Package referencebonus is an explicitly selected customer overlay. Manual
// declarations are not confirmed-source pools or records of actual payment.
package referencebonus

import (
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	core "omsbizfoundry/bonus-distribution/internal/bonus"
)

const Profile = "reference-oa-bonus-v2"
const ProfessionalProfile = "reference-oa-bonus-v3"
const ProfessionalSource = "reference-oa-production-personnel-v1"
const ResourceType = "reference_oa_bonus_allocation"
const (
	ActionRead       = "bonus-manual-allocation.read"
	ActionAmountRead = "bonus-manual-allocation.amount.read"
	ActionRecord     = "bonus-manual-allocation.record"
	ActionWithdraw   = "bonus-manual-allocation.withdraw"
	ActionHistory    = "bonus-manual-allocation.history.read"
)

var Actions = []string{ActionRead, ActionAmountRead, ActionRecord, ActionWithdraw, ActionHistory}
var uuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
var integer = regexp.MustCompile(`^(0|[1-9][0-9]*)$`)
var digest = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

type ItemInput struct {
	AssignmentSource string             `json:"assignment_source,omitempty"`
	PersonRef        core.PersonRef     `json:"person_ref"`
	AssignmentRef    core.AssignmentRef `json:"assignment_ref"`
	Method           string             `json:"method"`
	RatioBasisPoints int64              `json:"ratio_basis_points"`
	FixedAmountMinor string             `json:"fixed_amount_minor"`
}
type CreateInput struct {
	CommandID       string          `json:"command_id"`
	ProjectRef      core.ProjectRef `json:"project_ref"`
	Scope           string          `json:"scope"`
	BaseAmountMinor string          `json:"base_amount_minor"`
	OccurredOn      string          `json:"occurred_on"`
	Description     string          `json:"description"`
	Items           []ItemInput     `json:"items"`
}
type WithdrawInput struct {
	CommandID        string `json:"command_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Reason           string `json:"reason"`
}
type Record struct {
	Profile          string                  `json:"profile,omitempty"`
	AssignmentSource string                  `json:"assignment_source,omitempty"`
	Discipline       *DisciplineSnapshot     `json:"discipline,omitempty"`
	ID               string                  `json:"id"`
	BatchID          string                  `json:"batch_id"`
	TenantID         string                  `json:"tenant_id"`
	Project          core.ProjectSnapshot    `json:"project"`
	Scope            string                  `json:"scope"`
	Currency         string                  `json:"currency"`
	BaseAmountMinor  int64                   `json:"base_amount_minor"`
	Person           core.PersonSnapshot     `json:"person"`
	Assignment       core.AssignmentSnapshot `json:"assignment"`
	Method           string                  `json:"method"`
	RatioBasisPoints int64                   `json:"ratio_basis_points"`
	FixedAmountMinor int64                   `json:"fixed_amount_minor"`
	AmountMinor      int64                   `json:"amount_minor"`
	OccurredOn       string                  `json:"occurred_on"`
	Description      string                  `json:"description"`
	State            string                  `json:"state"`
	Revision         int64                   `json:"revision"`
	CreatedBy        string                  `json:"created_by"`
	UpdatedBy        string                  `json:"updated_by"`
	CreatedAt        time.Time               `json:"created_at"`
	UpdatedAt        time.Time               `json:"updated_at"`
}
type DisciplineSnapshot struct {
	ID          string `json:"id"`
	Revision    int64  `json:"revision"`
	DisplayName string `json:"display_name"`
}
type Participant struct {
	core.ParticipantReference
	AssignmentSource string              `json:"assignment_source,omitempty"`
	Discipline       *DisciplineSnapshot `json:"discipline,omitempty"`
}

func (v Record) EffectiveProfile() string {
	if v.Profile == "" {
		return Profile
	}
	return v.Profile
}
func ProfessionalRole(role string) bool {
	return role == "designer" || role == "participant" || role == "reviewer" || role == "auditor" || role == "approver" || role == "estimator"
}
func validDiscipline(role string, v *DisciplineSnapshot) bool {
	if role == "approver" || role == "estimator" {
		return v == nil
	}
	return v != nil && uuid.MatchString(v.ID) && v.Revision > 0 && strings.TrimSpace(v.DisplayName) != ""
}

type Revision struct {
	Snapshot  Record `json:"snapshot"`
	CommandID string `json:"command_id"`
	Operation string `json:"operation"`
	Reason    string `json:"reason"`
}

func SideKind(scope string) string {
	switch scope {
	case core.TypeBusiness:
		return "business_bonus_project"
	case core.TypeProduction:
		return "production_bonus_project"
	}
	return ""
}

// EligibleRole is an overlay policy, not a weakening of the existing Pool policy.
func EligibleRole(scope, role string) bool {
	if scope == core.TypeBusiness {
		return role == "business_owner" || role == "commercial_contributor"
	}
	if scope == core.TypeProduction {
		return role == "production_owner" || role == "participant"
	}
	return false
}
func ValidActor(a core.Actor) bool {
	return uuid.MatchString(a.ID) && uuid.MatchString(a.TenantID) && len(a.Credential) > 0 && len(a.Credential) <= 256 && (a.SubjectKind == "iam_subject" || a.SubjectKind == "template_subject")
}
func Can(a core.Actor, action, project, scope, id string) bool {
	if !ValidActor(a) || !uuid.MatchString(project) || SideKind(scope) == "" {
		return false
	}
	allowed := false
	for _, v := range a.Actions {
		allowed = allowed || v == action
	}
	if !allowed {
		return false
	}
	if a.TenantScope {
		return true
	}
	for _, v := range a.ResourceScopes {
		if v.Type == SideKind(scope) && v.ID == project || id != "" && v.Type == ResourceType && v.ID == id {
			return true
		}
	}
	return false // Generic project and legacy Pool scopes do not authorize this object.
}
func minor(v string) (int64, error) {
	if !integer.MatchString(v) {
		return 0, fmt.Errorf("invalid exact manual bonus amount")
	}
	n, e := strconv.ParseInt(v, 10, 64)
	if e != nil {
		return 0, fmt.Errorf("invalid manual bonus amount range")
	}
	return n, nil
}
func normalize(in CreateInput) (CreateInput, int64, []int64, error) {
	if !uuid.MatchString(in.CommandID) || !uuid.MatchString(in.ProjectRef.ID) || in.ProjectRef.Revision < 1 || SideKind(in.Scope) == "" {
		return in, 0, nil, fmt.Errorf("invalid manual bonus identity or scope")
	}
	base, err := minor(in.BaseAmountMinor)
	if err != nil {
		return in, 0, nil, err
	}
	date, err := time.Parse("2006-01-02", in.OccurredOn)
	if err != nil || date.Year() < 1 || date.Format("2006-01-02") != in.OccurredOn {
		return in, 0, nil, fmt.Errorf("invalid manual bonus date")
	}
	in.Description = strings.TrimSpace(in.Description)
	if utf8.RuneCountInString(in.Description) > 1000 {
		return in, 0, nil, fmt.Errorf("invalid manual bonus description length")
	}
	if len(in.Items) < 1 || len(in.Items) > 100 {
		return in, 0, nil, fmt.Errorf("invalid manual bonus item count")
	}
	seen := map[string]bool{}
	values := make([]int64, len(in.Items))
	var totalRatio, totalAmount int64
	for i, v := range in.Items {
		key := v.AssignmentSource + ":" + v.AssignmentRef.ID
		if (v.AssignmentSource != "" && (v.AssignmentSource != ProfessionalSource || in.Scope != core.TypeProduction)) || !uuid.MatchString(v.PersonRef.ID) || v.PersonRef.Revision < 1 || !uuid.MatchString(v.AssignmentRef.ID) || v.AssignmentRef.Revision < 1 || seen[key] {
			return in, 0, nil, fmt.Errorf("invalid or duplicate manual bonus assignment")
		}
		seen[key] = true
		fixed, e := minor(v.FixedAmountMinor)
		if e != nil {
			return in, 0, nil, e
		}
		switch v.Method {
		case "ratio":
			if base <= 0 || v.RatioBasisPoints < 1 || v.RatioBasisPoints > 10000 || fixed != 0 {
				return in, 0, nil, fmt.Errorf("invalid ratio manual bonus input")
			}
			totalRatio += v.RatioBasisPoints
			if totalRatio > 10000 {
				return in, 0, nil, fmt.Errorf("invalid manual bonus ratio total")
			}
			product := new(big.Int).Mul(big.NewInt(base), big.NewInt(v.RatioBasisPoints))
			product.Add(product, big.NewInt(5000))
			product.Quo(product, big.NewInt(10000))
			values[i] = product.Int64()
		case "fixed":
			if fixed <= 0 || v.RatioBasisPoints != 0 {
				return in, 0, nil, fmt.Errorf("invalid fixed manual bonus input")
			}
			values[i] = fixed
		default:
			return in, 0, nil, fmt.Errorf("invalid manual bonus method")
		}
		if values[i] <= 0 || totalAmount > math.MaxInt64-values[i] {
			return in, 0, nil, fmt.Errorf("invalid manual bonus amount or total overflow")
		}
		totalAmount += values[i]
	}
	return in, base, values, nil
}
func newID() string {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		panic(e)
	}
	b[6] = b[6]&15 | 64
	b[8] = b[8]&63 | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}
