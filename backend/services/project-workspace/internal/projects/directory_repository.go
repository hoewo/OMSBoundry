package projects

import (
	"context"

	"github.com/jackc/pgx/v5"
)

const directoryProjectColumns = "id,tenant_id,project_number,node_type,parent_project_id,sort_order,name,summary,planned_start_date,planned_end_date,project_type_keys,client_party_id,client_party_source_revision,client_party_kind,client_party_legal_name,status,revision,cancellation_reason,created_by,created_at,updated_at"

func (s *PostgresStore) Directory(ctx context.Context, tenantID string, input DirectoryQuery) (DirectoryPage, error) {
	query, err := normalizeDirectoryQuery(input)
	if err != nil {
		return DirectoryPage{}, err
	}
	page := DirectoryPage{SchemaVersion: "1", Items: []DirectoryEntry{}, Offset: query.Offset, Limit: query.Limit}
	ids := query.ResourceIDs
	if ids == nil {
		ids = []string{}
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return page, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	// Every occurrence of visible applies both Tenant and signed resource scope.
	cte := `WITH visible AS (SELECT * FROM projects WHERE tenant_id=$1 AND ($4 OR status='active') AND ($2 OR id=ANY($3::uuid[]))), roots AS (
	 SELECT p.* FROM visible p WHERE NOT EXISTS (SELECT 1 FROM visible parent WHERE parent.id=p.parent_project_id)
	 AND ($5='' OR strpos(lower(p.name||' '||p.project_number),lower($5))>0
	 OR EXISTS (SELECT 1 FROM visible child WHERE child.parent_project_id=p.id AND strpos(lower(child.name||' '||child.project_number),lower($5))>0))) `
	args := []any{tenantID, query.ScopeKind == "tenant", ids, query.Status == "all", query.Keyword}
	if err := tx.QueryRow(ctx, cte+`SELECT count(*) FROM roots`, args...).Scan(&page.Total); err != nil {
		return page, err
	}
	rows, err := tx.Query(ctx, cte+`SELECT `+directoryProjectColumns+` FROM roots ORDER BY created_at DESC,id DESC LIMIT $6 OFFSET $7`, append(args, query.Limit, query.Offset)...)
	if err != nil {
		return page, err
	}
	rootIndexes := map[string]int{}
	rootIDs := []string{}
	for rows.Next() {
		project, err := scanProject(rows)
		if err != nil {
			rows.Close()
			return page, err
		}
		rootIndexes[project.ID] = len(page.Items)
		rootIDs = append(rootIDs, project.ID)
		page.Items = append(page.Items, DirectoryEntry{Project: project, Assignments: []Assignment{}, ResourceLinks: []ResourceLink{}, Children: []DirectoryEntry{}})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return page, err
	}
	allIDs := append([]string{}, rootIDs...)
	childIndexes := map[string][2]int{}
	if len(rootIDs) > 0 {
		rows, err = tx.Query(ctx, `SELECT `+directoryProjectColumns+` FROM projects WHERE tenant_id=$1 AND ($5 OR status='active') AND parent_project_id=ANY($2::uuid[]) AND ($3 OR id=ANY($4::uuid[])) ORDER BY sort_order,created_at,id`, tenantID, rootIDs, query.ScopeKind == "tenant", ids, query.Status == "all")
		if err != nil {
			return page, err
		}
		for rows.Next() {
			project, err := scanProject(rows)
			if err != nil {
				rows.Close()
				return page, err
			}
			rootIndex := rootIndexes[*project.ParentProjectID]
			childIndexes[project.ID] = [2]int{rootIndex, len(page.Items[rootIndex].Children)}
			page.Items[rootIndex].Children = append(page.Items[rootIndex].Children, DirectoryEntry{Project: project, Assignments: []Assignment{}, ResourceLinks: []ResourceLink{}, Children: []DirectoryEntry{}})
			allIDs = append(allIDs, project.ID)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return page, err
		}
		rows, err = tx.Query(ctx, `SELECT id,tenant_id,project_id,subject_kind,subject_id,source_revision,display_snapshot,role_key,effective_from,ended_at,end_reason,created_project_revision,ended_project_revision,created_at FROM project_assignments WHERE tenant_id=$1 AND project_id=ANY($2::uuid[]) AND ended_at IS NULL AND effective_from<=now() AND role_key IN ('business_owner','production_owner') ORDER BY project_id,role_key,id`, tenantID, allIDs)
		if err != nil {
			return page, err
		}
		for rows.Next() {
			assignment, err := scanAssignment(rows)
			if err != nil {
				rows.Close()
				return page, err
			}
			if index, ok := rootIndexes[assignment.ProjectID]; ok {
				page.Items[index].Assignments = append(page.Items[index].Assignments, assignment)
			} else if index, ok := childIndexes[assignment.ProjectID]; ok {
				child := &page.Items[index[0]].Children[index[1]]
				child.Assignments = append(child.Assignments, assignment)
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return page, err
		}
		rows, err = tx.Query(ctx, `SELECT id,tenant_id,project_id,target_service,resource_type,resource_id,source_revision,relation_key,note,created_project_revision,unlinked_project_revision,linked_at,unlinked_at FROM project_resource_links WHERE tenant_id=$1 AND project_id=ANY($2::uuid[]) AND unlinked_at IS NULL ORDER BY project_id,relation_key,linked_at,id`, tenantID, allIDs)
		if err != nil {
			return page, err
		}
		for rows.Next() {
			link, err := scanLink(rows)
			if err != nil {
				rows.Close()
				return page, err
			}
			if index, ok := rootIndexes[link.ProjectID]; ok {
				page.Items[index].ResourceLinks = append(page.Items[index].ResourceLinks, link)
			} else if index, ok := childIndexes[link.ProjectID]; ok {
				child := &page.Items[index[0]].Children[index[1]]
				child.ResourceLinks = append(child.ResourceLinks, link)
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return page, err
		}
	}
	page.HasMore = page.Offset+len(page.Items) < page.Total
	return page, tx.Commit(ctx)
}
