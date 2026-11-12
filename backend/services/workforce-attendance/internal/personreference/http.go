package personreference

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"omsbizfoundry/workforce-attendance/internal/attendance"
)

const maxResponseBytes = 256 << 10

type CredentialSource interface {
	CredentialForTenant(context.Context, string, string, string, string, string) (string, error)
	InvalidateForTenant(string, string, string, string, string)
}

type HTTPClient struct {
	client               *http.Client
	baseURL, staticToken string
	credentials          CredentialSource
}

func NewHTTPClient(baseURL, staticToken string, credentials CredentialSource, timeout time.Duration) (*HTTPClient, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	parsed, err := url.Parse(baseURL)
	hasStatic := strings.TrimSpace(staticToken) != ""
	hasDynamic := credentials != nil
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || hasStatic == hasDynamic || timeout <= 0 || timeout > 30*time.Second {
		return nil, fmt.Errorf("People employment configuration is invalid")
	}
	return &HTTPClient{client: &http.Client{Timeout: timeout}, baseURL: baseURL, staticToken: staticToken, credentials: credentials}, nil
}

func (c *HTTPClient) Snapshot(ctx context.Context, tenantID, personID string) (attendance.EmploymentSnapshot, error) {
	token := c.staticToken
	var err error
	if c.credentials != nil {
		token, err = c.credentials.CredentialForTenant(ctx, tenantID, "people-groups", "people-groups.person.reference.read", "person", personID)
		if err != nil || strings.TrimSpace(token) == "" {
			return attendance.EmploymentSnapshot{}, attendance.ErrDependency
		}
	}
	return c.snapshot(ctx, tenantID, personID, token, true)
}

func (c *HTTPClient) snapshot(ctx context.Context, tenantID, personID, token string, retry bool) (attendance.EmploymentSnapshot, error) {
	personBody, status, err := c.get(ctx, "/api/v1/references/persons/"+url.PathEscape(personID), token)
	if status == http.StatusUnauthorized && retry && c.credentials != nil {
		c.credentials.InvalidateForTenant(tenantID, "people-groups", "people-groups.person.reference.read", "person", personID)
		fresh, credentialErr := c.credentials.CredentialForTenant(ctx, tenantID, "people-groups", "people-groups.person.reference.read", "person", personID)
		if credentialErr != nil {
			return attendance.EmploymentSnapshot{}, attendance.ErrDependency
		}
		return c.snapshot(ctx, tenantID, personID, fresh, false)
	}
	if err != nil || status < 200 || status >= 300 {
		return attendance.EmploymentSnapshot{}, attendance.ErrDependency
	}
	historyBody, historyStatus, err := c.get(ctx, "/api/v1/persons/"+url.PathEscape(personID)+"/employment-history", token)
	if historyStatus == http.StatusUnauthorized && retry && c.credentials != nil {
		c.credentials.InvalidateForTenant(tenantID, "people-groups", "people-groups.person.reference.read", "person", personID)
		fresh, credentialErr := c.credentials.CredentialForTenant(ctx, tenantID, "people-groups", "people-groups.person.reference.read", "person", personID)
		if credentialErr != nil {
			return attendance.EmploymentSnapshot{}, attendance.ErrDependency
		}
		return c.snapshot(ctx, tenantID, personID, fresh, false)
	}
	if err != nil || historyStatus < 200 || historyStatus >= 300 {
		return attendance.EmploymentSnapshot{}, attendance.ErrDependency
	}
	return decodeSnapshot(personBody, historyBody, tenantID, personID)
}

func (c *HTTPClient) get(ctx context.Context, path, token string) ([]byte, int, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, 0, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	response, err := c.client.Do(request)
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil || len(body) > maxResponseBytes {
		return nil, response.StatusCode, attendance.ErrDependency
	}
	return body, response.StatusCode, nil
}

type personWire struct {
	SchemaVersion     string `json:"schema_version"`
	SourceApplication string `json:"source_application"`
	ID                string `json:"id"`
	TenantID          string `json:"tenant_id"`
	ResourceType      string `json:"resource_type"`
	DisplayName       string `json:"display_name"`
	Status            string `json:"status"`
	Revision          int64  `json:"revision"`
}

type employmentEventWire struct {
	Sequence       int64  `json:"sequence"`
	PersonRevision int64  `json:"person_revision"`
	EventKind      string `json:"event_kind"`
	EffectiveDate  string `json:"effective_date"`
}

func decodeSnapshot(personBody, historyBody []byte, tenantID, personID string) (attendance.EmploymentSnapshot, error) {
	var person personWire
	if err := strictDecode(personBody, &person); err != nil || person.SchemaVersion != "1" || person.SourceApplication != "people-groups" || person.ResourceType != "person" || person.ID != personID || person.TenantID != tenantID || strings.TrimSpace(person.DisplayName) == "" || person.Revision < 1 {
		return attendance.EmploymentSnapshot{}, attendance.ErrDependency
	}
	var events []employmentEventWire
	if err := strictDecode(historyBody, &events); err != nil || len(events) == 0 {
		return attendance.EmploymentSnapshot{}, attendance.ErrDependency
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].Sequence == events[j].Sequence {
			return events[i].EffectiveDate < events[j].EffectiveDate
		}
		return events[i].Sequence < events[j].Sequence
	})
	intervals := make([]attendance.EmploymentInterval, 0, 2)
	var open string
	var lastSequence int64
	maxRevision := person.Revision
	for _, event := range events {
		date, dateErr := time.Parse("2006-01-02", event.EffectiveDate)
		if dateErr != nil || event.Sequence <= lastSequence || event.PersonRevision < 1 {
			return attendance.EmploymentSnapshot{}, attendance.ErrDependency
		}
		lastSequence = event.Sequence
		if event.PersonRevision > maxRevision {
			maxRevision = event.PersonRevision
		}
		switch event.EventKind {
		case "hired", "rehired":
			if open != "" {
				return attendance.EmploymentSnapshot{}, attendance.ErrDependency
			}
			open = event.EffectiveDate
		case "offboarded":
			if open == "" {
				return attendance.EmploymentSnapshot{}, attendance.ErrDependency
			}
			through := date.AddDate(0, 0, -1).Format("2006-01-02")
			if through < open {
				return attendance.EmploymentSnapshot{}, attendance.ErrDependency
			}
			intervals = append(intervals, attendance.EmploymentInterval{From: open, Through: through})
			open = ""
		default:
			return attendance.EmploymentSnapshot{}, attendance.ErrDependency
		}
	}
	if open != "" {
		intervals = append(intervals, attendance.EmploymentInterval{From: open})
	}
	if len(intervals) == 0 {
		return attendance.EmploymentSnapshot{}, attendance.ErrDependency
	}
	digestInput := append(append([]byte{}, personBody...), '\n')
	digestInput = append(digestInput, historyBody...)
	digest := sha256.Sum256(digestInput)
	return attendance.EmploymentSnapshot{
		PersonID:       person.ID,
		DisplayName:    strings.TrimSpace(person.DisplayName),
		SourceRevision: maxRevision,
		SourceDigest:   fmt.Sprintf("sha256:%x", digest),
		Intervals:      intervals,
	}, nil
}

func strictDecode(body []byte, destination any) error {
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return fmt.Errorf("response contains trailing JSON")
	}
	return nil
}

var _ attendance.EmploymentPort = (*HTTPClient)(nil)
