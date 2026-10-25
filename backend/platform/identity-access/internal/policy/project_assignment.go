package policy

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"omsbizfoundry/identity-access/internal/identity"
)

var (
	uuidPattern       = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	identifierPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{2,63}$`)
	actionPattern     = regexp.MustCompile(`^[a-z][a-z0-9-]*(\.[a-z][a-z0-9-]*)+$`)
	versionPattern    = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?$`)
)

type Config struct {
	ProjectBaseURL   string
	PeopleBaseURL    string
	ServiceSubjectID string
	Issuer           string
	PolicyPaths      []string
	Timeout          time.Duration
}

type projectAssignmentPolicy struct {
	SchemaVersion       string              `json:"schema_version"`
	PolicyID            string              `json:"policy_id"`
	PolicyVersion       string              `json:"policy_version"`
	Audience            string              `json:"audience"`
	CapabilityNamespace string              `json:"capability_namespace,omitempty"`
	Roles               map[string][]string `json:"roles"`
}

type ProjectAssignmentEvaluator struct {
	projectBaseURL   string
	peopleBaseURL    string
	serviceSubjectID string
	issuer           string
	policies         map[string]projectAssignmentPolicy
	signer           identity.Signer
	client           *http.Client
	now              func() time.Time
}

// Validate checks every configured policy document without constructing a
// runtime evaluator or contacting any policy fact source.
func Validate(paths []string) error {
	_, err := loadPolicies(paths)
	return err
}

type assignmentFact struct {
	ID             string     `json:"id"`
	TenantID       string     `json:"tenant_id"`
	ProjectID      string     `json:"project_id"`
	ResourceType   string     `json:"resource_type"`
	SubjectKind    string     `json:"subject_kind"`
	SubjectID      string     `json:"subject_id"`
	SourceRevision int64      `json:"source_revision"`
	RoleKey        string     `json:"role_key"`
	Status         string     `json:"status"`
	Revision       int64      `json:"revision"`
	EffectiveFrom  *time.Time `json:"effective_from,omitempty"`
	EndedAt        *time.Time `json:"ended_at,omitempty"`
	DisplayName    string     `json:"display_name"`
}

type personFact struct {
	SchemaVersion     string `json:"schema_version"`
	SourceApplication string `json:"source_application"`
	FactKind          string `json:"fact_kind"`
	ID                string `json:"id"`
	TenantID          string `json:"tenant_id"`
	Status            string `json:"status"`
	Revision          int64  `json:"revision"`
	IAMSubjectID      string `json:"iam_subject_id"`
}

func NewProjectAssignmentEvaluator(config Config, signer identity.Signer) (*ProjectAssignmentEvaluator, error) {
	if signer == nil {
		return nil, fmt.Errorf("policy signer is required")
	}
	projectURL, err := validBaseURL(config.ProjectBaseURL)
	if err != nil {
		return nil, fmt.Errorf("project policy endpoint: %w", err)
	}
	peopleURL, err := validBaseURL(config.PeopleBaseURL)
	if err != nil {
		return nil, fmt.Errorf("people policy endpoint: %w", err)
	}
	if !uuidPattern.MatchString(config.ServiceSubjectID) || strings.TrimSpace(config.Issuer) == "" {
		return nil, fmt.Errorf("policy service identity is invalid")
	}
	if config.Timeout <= 0 || config.Timeout > 30*time.Second {
		return nil, fmt.Errorf("policy timeout must be between 1ns and 30s")
	}
	values, err := loadPolicies(config.PolicyPaths)
	if err != nil {
		return nil, err
	}
	return &ProjectAssignmentEvaluator{
		projectBaseURL: projectURL, peopleBaseURL: peopleURL,
		serviceSubjectID: config.ServiceSubjectID,
		issuer:           strings.TrimRight(config.Issuer, "/"), policies: values, signer: signer,
		client: &http.Client{Timeout: config.Timeout}, now: func() time.Time { return time.Now().UTC() },
	}, nil
}

func (e *ProjectAssignmentEvaluator) Evaluate(ctx context.Context, principal identity.Principal, audience string, scope identity.Scope, reference identity.BusinessFactReference) (identity.PolicyDecision, error) {
	selectedPolicy, found := e.policies[audience]
	if !uuidPattern.MatchString(principal.TenantID) || !found || reference.Kind != "project_assignment" || reference.Source != "project-workspace" || !uuidPattern.MatchString(reference.ID) || reference.Revision < 1 {
		return identity.PolicyDecision{}, &identity.AuthorizationError{}
	}
	if scope.Kind != "resources" || len(scope.Resources) != 1 || scope.Resources[0].Type != "project" || !uuidPattern.MatchString(scope.Resources[0].ID) {
		return identity.PolicyDecision{}, &identity.AuthorizationError{}
	}
	projectID := scope.Resources[0].ID
	var assignment assignmentFact
	if err := e.get(ctx, principal.TenantID, "project-workspace", "project-workspace.policy-fact.read", identity.Scope{Kind: "resources", Resources: []identity.ResourceRef{{Type: "project", ID: projectID}}}, e.projectBaseURL+"/api/v1/policy-facts/projects/"+url.PathEscape(projectID)+"/assignments/"+url.PathEscape(reference.ID), &assignment); err != nil {
		return identity.PolicyDecision{}, err
	}
	now := e.now()
	if assignment.ID != reference.ID || assignment.ProjectID != projectID || assignment.TenantID != principal.TenantID || assignment.ResourceType != "project-assignment" || assignment.SubjectKind != "person" || assignment.Status != "active" || assignment.Revision != reference.Revision || assignment.SourceRevision < 1 || !uuidPattern.MatchString(assignment.SubjectID) || (assignment.EffectiveFrom != nil && assignment.EffectiveFrom.After(now)) || assignment.EndedAt != nil {
		return identity.PolicyDecision{}, &identity.AuthorizationError{}
	}
	var person personFact
	if err := e.get(ctx, principal.TenantID, "people-groups", "people-groups.policy-fact.read", identity.Scope{Kind: "resources", Resources: []identity.ResourceRef{{Type: "person", ID: assignment.SubjectID}}}, e.peopleBaseURL+"/api/v1/policy-facts/persons/"+url.PathEscape(assignment.SubjectID), &person); err != nil {
		return identity.PolicyDecision{}, err
	}
	if person.SchemaVersion != "1" || person.SourceApplication != "people-groups" || person.FactKind != "person-iam-subject-link" || person.ID != assignment.SubjectID || person.TenantID != principal.TenantID || person.Status != "active" || person.Revision != assignment.SourceRevision || person.IAMSubjectID != principal.SubjectID {
		return identity.PolicyDecision{}, &identity.AuthorizationError{}
	}
	actions := append([]string(nil), selectedPolicy.Roles[assignment.RoleKey]...)
	if len(actions) == 0 {
		return identity.PolicyDecision{}, &identity.AuthorizationError{}
	}
	sort.Strings(actions)
	decisionID, err := randomUUID()
	if err != nil {
		return identity.PolicyDecision{}, err
	}
	return identity.PolicyDecision{
		DecisionID: decisionID, PolicyID: selectedPolicy.PolicyID, PolicyVersion: selectedPolicy.PolicyVersion,
		SourceApplication: "project-workspace", FactKind: reference.Kind, FactID: reference.ID,
		FactRevision: reference.Revision, Actions: actions,
	}, nil
}

func (e *ProjectAssignmentEvaluator) get(ctx context.Context, tenantID, audience, action string, scope identity.Scope, endpoint string, destination any) error {
	token, err := e.serviceCredential(tenantID, audience, action, scope)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	response, err := e.client.Do(request)
	if err != nil {
		return &identity.DependencyUnavailableError{Dependency: audience, Err: fmt.Errorf("read policy fact: %w", err)}
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusForbidden || response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusUnprocessableEntity {
		return &identity.AuthorizationError{}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return &identity.DependencyUnavailableError{Dependency: audience, Err: fmt.Errorf("policy fact endpoint returned HTTP %d", response.StatusCode)}
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return &identity.DependencyUnavailableError{Dependency: audience, Err: fmt.Errorf("decode policy fact: %w", err)}
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return &identity.DependencyUnavailableError{Dependency: audience, Err: fmt.Errorf("policy fact contains trailing JSON")}
	}
	return nil
}

func (e *ProjectAssignmentEvaluator) serviceCredential(tenantID, audience, action string, scope identity.Scope) (string, error) {
	now := e.now()
	id, err := randomUUID()
	if err != nil {
		return "", err
	}
	claims := identity.CapabilityClaims{
		Version: "1", Issuer: e.issuer, Subject: e.serviceSubjectID, Tenant: tenantID,
		Audience: audience, Name: "IAM Policy Provider", Actions: []string{action}, Scope: scope,
		IssuedAt: now.Unix(), NotBefore: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix(), AuthTime: now.Unix(), ID: id,
	}
	return e.signer.Sign(claims)
}

func loadPolicies(paths []string) (map[string]projectAssignmentPolicy, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("at least one project assignment policy is required")
	}
	result := make(map[string]projectAssignmentPolicy, len(paths))
	for _, path := range paths {
		value, err := loadPolicy(path)
		if err != nil {
			return nil, err
		}
		if _, exists := result[value.Audience]; exists {
			return nil, fmt.Errorf("duplicate project assignment policy audience %q", value.Audience)
		}
		result[value.Audience] = value
	}
	return result, nil
}

func loadPolicy(path string) (projectAssignmentPolicy, error) {
	data, err := os.ReadFile(strings.TrimSpace(path))
	if err != nil {
		return projectAssignmentPolicy{}, fmt.Errorf("read project assignment policy: %w", err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	var value projectAssignmentPolicy
	if err := decoder.Decode(&value); err != nil {
		return projectAssignmentPolicy{}, fmt.Errorf("decode project assignment policy: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return projectAssignmentPolicy{}, fmt.Errorf("project assignment policy contains trailing JSON")
	}
	if value.SchemaVersion != "1" || strings.TrimSpace(value.PolicyID) == "" || !versionPattern.MatchString(value.PolicyVersion) || !identifierPattern.MatchString(value.Audience) || len(value.Roles) == 0 {
		return projectAssignmentPolicy{}, fmt.Errorf("project assignment policy identity is invalid")
	}
	namespace := value.CapabilityNamespace
	if namespace == "" {
		namespace = value.Audience
	}
	if !identifierPattern.MatchString(namespace) {
		return projectAssignmentPolicy{}, fmt.Errorf("project assignment policy capability namespace is invalid")
	}
	for role, actions := range value.Roles {
		if strings.TrimSpace(role) == "" || len(actions) == 0 {
			return projectAssignmentPolicy{}, fmt.Errorf("project assignment policy role is invalid")
		}
		seen := map[string]bool{}
		for _, action := range actions {
			if !actionPattern.MatchString(action) || !strings.HasPrefix(action, namespace+".") || seen[action] {
				return projectAssignmentPolicy{}, fmt.Errorf("project assignment policy action is invalid")
			}
			seen[action] = true
		}
	}
	return value, nil
}

func validBaseURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(raw), "/"))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("base URL must be absolute http or https")
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}

func randomUUID() (string, error) {
	data := make([]byte, 16)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	data[6] = (data[6] & 0x0f) | 0x40
	data[8] = (data[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", data[0:4], data[4:6], data[6:8], data[8:10], data[10:16]), nil
}
