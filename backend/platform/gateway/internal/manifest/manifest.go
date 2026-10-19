package manifest

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"
)

var identifierPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{2,63}$`)
var actionPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*(\.[a-z][a-z0-9-]*)+$`)
var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type Application struct {
	ID                   string              `json:"id"`
	Version              string              `json:"version"`
	Enabled              bool                `json:"enabled"`
	Required             bool                `json:"required"`
	RoutePrefix          string              `json:"route_prefix"`
	Upstream             string              `json:"upstream"`
	ReadinessPath        string              `json:"readiness_path"`
	Audience             string              `json:"audience"`
	CapabilityNamespace  string              `json:"capability_namespace,omitempty"`
	CapabilityNamespaces []string            `json:"capability_namespaces,omitempty"`
	EntryAction          string              `json:"entry_action,omitempty"`
	EntryActions         []string            `json:"entry_actions,omitempty"`
	CapabilityManifest   string              `json:"capability_manifest,omitempty"`
	ShowInWorkbench      *bool               `json:"show_in_workbench,omitempty"`
	WorkItemProjection   *WorkItemProjection `json:"work_item_projection,omitempty"`
	MetricProjection     *MetricProjection   `json:"metric_projection,omitempty"`
}
type WorkItemProjection struct {
	SchemaVersion string `json:"schema_version"`
	Path          string `json:"path"`
	Action        string `json:"action"`
}

func (app Application) capabilityNamespaces() ([]string, error) {
	if app.CapabilityNamespace != "" && app.CapabilityNamespaces != nil {
		return nil, fmt.Errorf("single and multiple capability namespaces are exclusive")
	}
	namespaces := app.CapabilityNamespaces
	if namespaces == nil {
		namespace := app.CapabilityNamespace
		if namespace == "" {
			namespace = app.Audience
		}
		namespaces = []string{namespace}
	}
	if len(namespaces) == 0 || len(namespaces) > 16 {
		return nil, fmt.Errorf("capability namespaces must be bounded and nonempty")
	}
	seen := map[string]bool{}
	for _, namespace := range namespaces {
		if !identifierPattern.MatchString(namespace) || seen[namespace] {
			return nil, fmt.Errorf("invalid or duplicate capability namespace")
		}
		seen[namespace] = true
	}
	return namespaces, nil
}

func nativeNamespace(action string, namespaces []string) bool {
	for _, namespace := range namespaces {
		if strings.HasPrefix(action, namespace+".") {
			return true
		}
	}
	return false
}

type MetricProjection struct {
	SchemaVersion string `json:"schema_version"`
	Path          string `json:"path"`
	Action        string `json:"action"`
}
type Suite struct {
	SchemaVersion string        `json:"schema_version"`
	SuiteID       string        `json:"suite_id"`
	TenantID      string        `json:"tenant_id"`
	Applications  []Application `json:"applications"`
}

// EntryCapabilities is an admission ANY set, not a grant or final authorization.
// The singular form retains the original Manifest v1 behavior.
func (app Application) EntryCapabilities() []string {
	if app.EntryAction != "" {
		return []string{app.EntryAction}
	}
	return app.EntryActions
}

func Load(path string) (Suite, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Suite{}, fmt.Errorf("read suite manifest: %w", err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	var suite Suite
	if err := decoder.Decode(&suite); err != nil {
		return Suite{}, fmt.Errorf("decode suite manifest: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return Suite{}, fmt.Errorf("suite manifest contains trailing JSON")
	}
	if err := validate(suite); err != nil {
		return Suite{}, err
	}
	slices.SortFunc(suite.Applications, func(left, right Application) int { return len(right.RoutePrefix) - len(left.RoutePrefix) })
	return suite, nil
}
func (s Suite) Application(id string) (Application, bool) {
	for _, app := range s.Applications {
		if app.ID == id {
			return app, true
		}
	}
	return Application{}, false
}
func (s Suite) Match(path string) (Application, bool) {
	for _, app := range s.Applications {
		if !app.Enabled {
			continue
		}
		if path == app.RoutePrefix || strings.HasPrefix(path, app.RoutePrefix+"/") {
			return app, true
		}
	}
	return Application{}, false
}

func validate(suite Suite) error {
	if suite.SchemaVersion != "1" || !identifierPattern.MatchString(suite.SuiteID) || !uuidPattern.MatchString(suite.TenantID) || len(suite.Applications) < 2 {
		return fmt.Errorf("suite manifest identity is invalid")
	}
	ids := map[string]struct{}{}
	audiences := map[string]struct{}{}
	routes := map[string]struct{}{}
	iamValid := false
	for _, app := range suite.Applications {
		if !identifierPattern.MatchString(app.ID) || app.Audience != app.ID || !strings.HasPrefix(app.RoutePrefix, "/") || !strings.HasPrefix(app.ReadinessPath, "/") {
			return fmt.Errorf("suite application %q is invalid", app.ID)
		}
		upstream, err := url.Parse(app.Upstream)
		if err != nil || upstream.Scheme != "http" || upstream.Host == "" || upstream.Path != "" || upstream.RawQuery != "" || upstream.Fragment != "" {
			return fmt.Errorf("suite application %q upstream is invalid", app.ID)
		}
		if _, ok := ids[app.ID]; ok {
			return fmt.Errorf("duplicate application %q", app.ID)
		}
		if _, ok := audiences[app.Audience]; ok {
			return fmt.Errorf("duplicate audience %q", app.Audience)
		}
		for route := range routes {
			if overlap(route, app.RoutePrefix) {
				return fmt.Errorf("overlapping route %q", app.RoutePrefix)
			}
		}
		ids[app.ID] = struct{}{}
		audiences[app.Audience] = struct{}{}
		routes[app.RoutePrefix] = struct{}{}
		if app.ID == "identity-access" {
			iamValid = app.Enabled && app.Required
		}
		platform := app.ID == "identity-access" || app.ID == "unified-workbench"
		if platform && (app.EntryAction != "" || app.EntryActions != nil) {
			return fmt.Errorf("platform application %q cannot declare entry action", app.ID)
		}
		namespaces, namespaceErr := app.capabilityNamespaces()
		if namespaceErr != nil {
			return fmt.Errorf("application %q capability namespace is invalid", app.ID)
		}
		if !platform {
			if (app.EntryAction != "" && app.EntryActions != nil) || len(app.EntryCapabilities()) == 0 || len(app.EntryCapabilities()) > 16 {
				return fmt.Errorf("application %q entry capabilities are invalid", app.ID)
			}
			seen := map[string]struct{}{}
			for _, action := range app.EntryCapabilities() {
				if !actionPattern.MatchString(action) || !nativeNamespace(action, namespaces) {
					return fmt.Errorf("application %q entry action is invalid", app.ID)
				}
				if _, exists := seen[action]; exists {
					return fmt.Errorf("application %q has duplicate entry action", app.ID)
				}
				seen[action] = struct{}{}
			}
		}
		if app.WorkItemProjection != nil {
			projection := app.WorkItemProjection
			if platform || projection.SchemaVersion != "1" || !strings.HasPrefix(projection.Path, "/api/") || strings.Contains(projection.Path, "?") || !actionPattern.MatchString(projection.Action) || !nativeNamespace(projection.Action, namespaces) || len(app.EntryCapabilities()) == 0 || app.CapabilityManifest == "" {
				return fmt.Errorf("application %q work item projection is invalid", app.ID)
			}
		}
		if app.MetricProjection != nil {
			projection := app.MetricProjection
			if platform || projection.SchemaVersion != "1" || !strings.HasPrefix(projection.Path, "/api/") || strings.Contains(projection.Path, "?") || !actionPattern.MatchString(projection.Action) || !nativeNamespace(projection.Action, namespaces) || len(app.EntryCapabilities()) == 0 || app.CapabilityManifest == "" {
				return fmt.Errorf("application %q metric projection is invalid", app.ID)
			}
		}
	}
	if !iamValid {
		return fmt.Errorf("identity-access must be enabled and required")
	}
	return nil
}
func overlap(left, right string) bool {
	return left == right || strings.HasPrefix(left, right+"/") || strings.HasPrefix(right, left+"/")
}
