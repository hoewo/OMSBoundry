package projects

import (
	"context"
	"fmt"
	"strings"
)

// Scope is populated from verified credentials, never from query parameters.
type DirectoryQuery struct {
	Offset      int
	Limit       int
	Keyword     string
	Status      string
	ScopeKind   string
	ResourceIDs []string
}

type DirectoryEntry struct {
	Project
	Assignments   []Assignment     `json:"assignments"`
	ResourceLinks []ResourceLink   `json:"resource_links"`
	Children      []DirectoryEntry `json:"children"`
}

type DirectoryPage struct {
	SchemaVersion string           `json:"schema_version"`
	Items         []DirectoryEntry `json:"items"`
	Total         int              `json:"total"`
	Offset        int              `json:"offset"`
	Limit         int              `json:"limit"`
	HasMore       bool             `json:"has_more"`
}

func normalizeDirectoryQuery(query DirectoryQuery) (DirectoryQuery, error) {
	query.Keyword = strings.TrimSpace(query.Keyword)
	query.Status = strings.TrimSpace(query.Status)
	if query.Limit == 0 {
		query.Limit = 100
	}
	if query.Status == "" {
		query.Status = "active"
	}
	if query.Offset < 0 || query.Limit < 1 || query.Limit > 100 || len([]rune(query.Keyword)) > 200 {
		return DirectoryQuery{}, fmt.Errorf("invalid directory pagination or keyword")
	}
	if query.Status != "active" && query.Status != "all" {
		return DirectoryQuery{}, fmt.Errorf("invalid directory status")
	}
	switch query.ScopeKind {
	case "tenant":
		if len(query.ResourceIDs) != 0 {
			return DirectoryQuery{}, fmt.Errorf("tenant directory scope must not contain resource IDs")
		}
	case "resources":
		for _, id := range query.ResourceIDs {
			if !ValidUUID(id) {
				return DirectoryQuery{}, fmt.Errorf("invalid directory resource scope")
			}
		}
	default:
		return DirectoryQuery{}, fmt.Errorf("invalid directory scope")
	}
	return query, nil
}

func (s *Service) Directory(ctx context.Context, tenantID string, query DirectoryQuery) (DirectoryPage, error) {
	query, err := normalizeDirectoryQuery(query)
	if err != nil {
		return DirectoryPage{}, err
	}
	return s.store.Directory(ctx, tenantID, query)
}
