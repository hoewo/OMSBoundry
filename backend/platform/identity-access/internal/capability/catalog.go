package capability

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
)

var (
	identifierPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{2,63}$`)
	actionPattern     = regexp.MustCompile(`^[a-z][a-z0-9-]*(\.[a-z][a-z0-9-]*)+$`)
	resourcePattern   = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)
	versionPattern    = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?$`)
)

type item struct {
	ID            string   `json:"id"`
	Description   string   `json:"description"`
	Risk          string   `json:"risk"`
	ResourceTypes []string `json:"resource_types"`
}
type manifest struct {
	SchemaVersion        string   `json:"schema_version"`
	ApplicationID        string   `json:"application_id"`
	ApplicationVersion   string   `json:"application_version"`
	Audience             string   `json:"audience"`
	CapabilityNamespace  string   `json:"capability_namespace,omitempty"`
	CapabilityNamespaces []string `json:"capability_namespaces,omitempty"`
	Capabilities         []item   `json:"capabilities"`
}

type Catalog struct {
	actions map[string]map[string]map[string]struct{}
}

func Load(paths []string) (*Catalog, error) {
	catalog := &Catalog{actions: map[string]map[string]map[string]struct{}{}}
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read capability manifest %s: %w", path, err)
		}
		decoder := json.NewDecoder(strings.NewReader(string(data)))
		decoder.DisallowUnknownFields()
		var value manifest
		if err := decoder.Decode(&value); err != nil {
			return nil, fmt.Errorf("decode capability manifest %s: %w", path, err)
		}
		var trailing any
		if err := decoder.Decode(&trailing); err != io.EOF {
			return nil, fmt.Errorf("capability manifest %s contains trailing JSON", path)
		}
		if err := validate(value); err != nil {
			return nil, fmt.Errorf("validate capability manifest %s: %w", path, err)
		}
		if _, duplicate := catalog.actions[value.Audience]; duplicate {
			return nil, fmt.Errorf("duplicate capability audience %s", value.Audience)
		}
		actions := map[string]map[string]struct{}{}
		for _, capability := range value.Capabilities {
			resources := make(map[string]struct{}, len(capability.ResourceTypes))
			for _, resourceType := range capability.ResourceTypes {
				resources[resourceType] = struct{}{}
			}
			actions[capability.ID] = resources
		}
		catalog.actions[value.Audience] = actions
	}
	return catalog, nil
}

func (c *Catalog) Contains(audience, action string) bool {
	if c == nil {
		return false
	}
	_, found := c.actions[audience][action]
	return found
}
func (c *Catalog) AllowsResource(audience, action, resourceType string) bool {
	if c == nil {
		return false
	}
	_, found := c.actions[audience][action][resourceType]
	return found
}
func (c *Catalog) Audiences() []string {
	values := make([]string, 0, len(c.actions))
	for audience := range c.actions {
		values = append(values, audience)
	}
	sort.Strings(values)
	return values
}

func (c *Catalog) Actions() map[string][]string {
	values := make(map[string][]string, len(c.actions))
	for audience, actions := range c.actions {
		items := make([]string, 0, len(actions))
		for action := range actions {
			items = append(items, action)
		}
		sort.Strings(items)
		values[audience] = items
	}
	return values
}

func validate(value manifest) error {
	if value.SchemaVersion != "1" || !identifierPattern.MatchString(value.ApplicationID) || value.Audience != value.ApplicationID || !versionPattern.MatchString(value.ApplicationVersion) || len(value.Capabilities) == 0 {
		return fmt.Errorf("manifest identity is invalid")
	}
	namespaces, err := manifestNamespaces(value)
	if err != nil {
		return err
	}
	seen := map[string]struct{}{}
	for _, capability := range value.Capabilities {
		if !actionPattern.MatchString(capability.ID) || !hasNamespace(capability.ID, namespaces) || strings.TrimSpace(capability.Description) == "" {
			return fmt.Errorf("capability %q is invalid", capability.ID)
		}
		if capability.Risk != "read" && capability.Risk != "write" && capability.Risk != "privileged" {
			return fmt.Errorf("capability %q risk is invalid", capability.ID)
		}
		if len(capability.ResourceTypes) == 0 {
			return fmt.Errorf("capability %q has no resource types", capability.ID)
		}
		resourceTypes := map[string]struct{}{}
		for _, resourceType := range capability.ResourceTypes {
			if !resourcePattern.MatchString(resourceType) {
				return fmt.Errorf("capability %q resource type is invalid", capability.ID)
			}
			if _, duplicate := resourceTypes[resourceType]; duplicate {
				return fmt.Errorf("capability %q has duplicate resource type", capability.ID)
			}
			resourceTypes[resourceType] = struct{}{}
		}
		if _, duplicate := seen[capability.ID]; duplicate {
			return fmt.Errorf("duplicate capability %q", capability.ID)
		}
		seen[capability.ID] = struct{}{}
	}
	return nil
}

func manifestNamespaces(value manifest) ([]string, error) {
	if value.CapabilityNamespace != "" && len(value.CapabilityNamespaces) != 0 {
		return nil, fmt.Errorf("single and multiple capability namespaces cannot both be declared")
	}
	namespaces := append([]string(nil), value.CapabilityNamespaces...)
	if len(namespaces) == 0 {
		namespace := value.CapabilityNamespace
		if namespace == "" {
			namespace = value.Audience
		}
		namespaces = []string{namespace}
	}
	seen := map[string]bool{}
	for _, namespace := range namespaces {
		if !identifierPattern.MatchString(namespace) || seen[namespace] {
			return nil, fmt.Errorf("capability namespace is invalid")
		}
		seen[namespace] = true
	}
	return namespaces, nil
}

func hasNamespace(action string, namespaces []string) bool {
	for _, namespace := range namespaces {
		if strings.HasPrefix(action, namespace+".") {
			return true
		}
	}
	return false
}
