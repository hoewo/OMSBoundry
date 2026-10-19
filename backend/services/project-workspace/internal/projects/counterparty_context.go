package projects

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
)

const CounterpartyContextSchema = "project-counterparty-context-v1"

// Project-specific communication facts, never a Party master or identity.
type CounterpartyContact struct {
	Name         string `json:"name"`
	ContactName  string `json:"contact_name"`
	ContactPhone string `json:"contact_phone"`
}
type CounterpartyContext struct {
	SchemaVersion string `json:"schema_version"`
	Role          string `json:"role"`
	CounterpartyContact
	RecordedProjectRevision int64     `json:"recorded_project_revision"`
	RecordedAt              time.Time `json:"recorded_at"`
}
type CounterpartyContextView struct {
	ProjectID       string               `json:"project_id"`
	ProjectRevision int64                `json:"project_revision"`
	ProjectStatus   string               `json:"project_status"`
	Context         *CounterpartyContext `json:"context"`
}
type CounterpartyContextInput struct {
	CommandID        string `json:"command_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Reason           string `json:"reason"`
	CounterpartyContact
}

var contactPhonePattern = regexp.MustCompile(`^[0-9+\-\s()]*$`)

func NormalizeCounterpartyContext(in CounterpartyContextInput) (CounterpartyContextInput, error) {
	in.CommandID = strings.ToLower(strings.TrimSpace(in.CommandID))
	in.Reason = strings.TrimSpace(in.Reason)
	in.Name = strings.TrimSpace(in.Name)
	in.ContactName = strings.TrimSpace(in.ContactName)
	in.ContactPhone = strings.TrimSpace(in.ContactPhone)
	if !ValidUUID(in.CommandID) || in.ExpectedRevision < 1 || in.Reason == "" || len([]rune(in.Reason)) > 500 || in.Name == "" || len([]rune(in.Name)) > 200 || len([]rune(in.ContactName)) > 80 || len([]rune(in.ContactPhone)) > 50 || !contactPhonePattern.MatchString(in.ContactPhone) {
		return in, fmt.Errorf("counterparty context requires command, revision, reason, name and a valid contact phone")
	}
	for _, value := range []string{in.Name, in.ContactName} {
		if strings.IndexFunc(value, unicode.IsControl) >= 0 {
			return in, fmt.Errorf("contact name contains a control character")
		}
	}
	return in, nil
}
func (s *Service) CounterpartyContext(ctx context.Context, tenant, id string) (CounterpartyContextView, error) {
	if !ValidUUID(id) {
		return CounterpartyContextView{}, fmt.Errorf("invalid project id")
	}
	return s.store.CounterpartyContext(ctx, tenant, id)
}
func (s *Service) ChangeCounterpartyContext(ctx context.Context, actor Actor, id string, input CounterpartyContextInput) (CounterpartyContextView, error) {
	in, err := NormalizeCounterpartyContext(input)
	if err != nil {
		return CounterpartyContextView{}, err
	}
	if !ValidUUID(id) {
		return CounterpartyContextView{}, fmt.Errorf("invalid project id")
	}
	return s.store.ChangeCounterpartyContext(ctx, actor, id, in)
}
