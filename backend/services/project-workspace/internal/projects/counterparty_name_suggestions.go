package projects

import (
	"context"
	"fmt"
	"strings"
	"unicode"
)

const CounterpartyNamesSchema = "project-counterparty-name-suggestions-v1"

// Scope fields are populated only from a verified native credential.
type CounterpartyNamesQuery struct {
	Query       string
	Limit       int
	ScopeKind   string
	ResourceIDs []string
}
type CounterpartyNames struct {
	SchemaVersion string   `json:"schema_version"`
	Names         []string `json:"names"`
}

func NormalizeCounterpartyNamesQuery(q CounterpartyNamesQuery) (CounterpartyNamesQuery, error) {
	q.Query = strings.TrimSpace(q.Query)
	if q.Limit == 0 {
		q.Limit = 10
	}
	if q.Limit < 1 || q.Limit > 50 || len([]rune(q.Query)) > 200 || strings.IndexFunc(q.Query, unicode.IsControl) >= 0 {
		return q, fmt.Errorf("invalid name suggestion query or limit")
	}
	switch q.ScopeKind {
	case "tenant":
		if len(q.ResourceIDs) > 0 {
			return q, fmt.Errorf("tenant scope cannot contain resource IDs")
		}
	case "resources":
		for _, id := range q.ResourceIDs {
			if !ValidUUID(id) {
				return q, fmt.Errorf("invalid name suggestion resource scope")
			}
		}
	default:
		return q, fmt.Errorf("invalid name suggestion scope")
	}
	return q, nil
}
func (s *Service) CounterpartyNameSuggestions(ctx context.Context, tenant string, input CounterpartyNamesQuery) (CounterpartyNames, error) {
	q, err := NormalizeCounterpartyNamesQuery(input)
	if err != nil {
		return CounterpartyNames{}, err
	}
	if q.Query == "" {
		return CounterpartyNames{SchemaVersion: CounterpartyNamesSchema, Names: []string{}}, nil
	}
	return s.store.CounterpartyNameSuggestions(ctx, tenant, q)
}
func (s *PostgresStore) CounterpartyNameSuggestions(ctx context.Context, tenant string, input CounterpartyNamesQuery) (CounterpartyNames, error) {
	q, err := NormalizeCounterpartyNamesQuery(input)
	out := CounterpartyNames{SchemaVersion: CounterpartyNamesSchema, Names: []string{}}
	if err != nil {
		return out, err
	}
	if q.Query == "" {
		return out, nil
	}
	ids := q.ResourceIDs
	if ids == nil {
		ids = []string{}
	}
	// A single read statement applies the same tenant/resource filter to both
	// sources. These are Project-owned historical names, not a Party master search.
	rows, err := s.pool.Query(ctx, `WITH visible AS (
  SELECT id,client_party_legal_name FROM projects WHERE tenant_id=$1 AND ($2 OR id=ANY($3::uuid[]))
 ), historical_names AS (
  SELECT client_party_legal_name AS name FROM visible
  UNION ALL
  SELECT h.payload->'context'->>'name' AS name FROM project_history_events h
  JOIN visible p ON p.id=h.project_id WHERE h.tenant_id=$1 AND h.payload->>'schema_version'=$4
 ) SELECT DISTINCT name COLLATE "C" FROM historical_names
 WHERE name IS NOT NULL AND name<>'' AND strpos(lower(name),lower($5))>0
 ORDER BY name COLLATE "C" LIMIT $6`, tenant, q.ScopeKind == "tenant", ids, CounterpartyContextSchema, q.Query, q.Limit)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return out, err
		}
		out.Names = append(out.Names, name)
	}
	return out, rows.Err()
}
