//go:build integration

package design

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"omsbizfoundry/design-operations/internal/database"
	"omsbizfoundry/design-operations/internal/testdatabase"
)

func materialOwnedFacts(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	ctx := context.Background()
	rows, err := pool.Query(ctx, `SELECT tablename FROM pg_tables WHERE schemaname='public' ORDER BY tablename`)
	if err != nil {
		t.Fatal(err)
	}
	tables := []string{}
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			t.Fatal(err)
		}
		tables = append(tables, table)
	}
	rows.Close()
	if rows.Err() != nil {
		t.Fatal(rows.Err())
	}
	facts := make([][2]string, 0, len(tables))
	for _, table := range tables {
		var canonical string
		query := `SELECT count(*)::text || ':' || md5(coalesce(string_agg(to_jsonb(t)::text,E'\n' ORDER BY to_jsonb(t)::text),'')) FROM ` + pgx.Identifier{"public", table}.Sanitize() + ` t`
		if err := pool.QueryRow(ctx, query).Scan(&canonical); err != nil {
			t.Fatal(err)
		}
		facts = append(facts, [2]string{table, canonical})
	}
	raw, _ := json.Marshal(facts)
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func TestMaterialReferencePostgresCurrentVersionsAndZeroWrites(t *testing.T) {
	ctx := context.Background()
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := NewPostgresStore(environment.Pool)
	refs := &stableReferences{projectTypeKeys: []string{"design", "consultation"}}
	writer, _ := NewService(store, refs)
	reader, _ := NewService(store, &materialReferenceNoUpstream{})
	actor := Actor{Kind: "test_subject", ID: intActor, TenantID: intTenant}
	for index, typeKey := range []string{DesignTypeKey, ConsultationTypeKey} {
		created, err := writer.Create(ctx, actor, CreateInput{CommandID: command(200 + index*10), ProjectID: intProject, ProjectSourceRevision: 4, TypeKey: typeKey})
		if err != nil {
			t.Fatal(err)
		}
		id := created.WorkItem.ID
		if _, err := writer.SetOwner(ctx, actor, id, OwnerInput{CommandID: command(201 + index*10), ExpectedRevision: 1, AssignmentID: intAssignment, AssignmentSourceRevision: 1}); err != nil {
			t.Fatal(err)
		}
		for version := 1; version <= 2; version++ {
			if _, err := writer.PutDeliverable(ctx, actor, id, "scheme", DeliverableInput{CommandID: command(201 + index*10 + version), ExpectedRevision: int64(1 + version), RequirementKey: RequirementPrimary, EvidenceFileID: workItemPerson, EvidenceVersion: version, EvidenceSourceRevision: int64(version)}); err != nil {
				t.Fatal(err)
			}
		}
		before := materialOwnedFacts(t, environment.Pool)
		for count := 0; count < 3; count++ {
			projection, err := reader.ReferenceMaterials(ctx, intTenant, id)
			if err != nil || projection.Revision != 4 || len(projection.Items) != 1 || projection.Items[0].Version != 2 || projection.Items[0].EvidenceSourceRevision != 2 {
				t.Fatalf("current projection=%#v err=%v", projection, err)
			}
		}
		if _, err := reader.ReferenceMaterials(ctx, workItemPerson, id); !errors.Is(err, ErrNotFound) {
			t.Fatalf("cross-tenant read err=%v", err)
		}
		if materialOwnedFacts(t, environment.Pool) != before {
			t.Fatal("material reads changed owned canonical facts")
		}
	}
}

type materialSnapshotBarrier struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (b *materialSnapshotBarrier) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if strings.Contains(data.SQL, "FROM design_stages WHERE tenant_id=") {
		b.once.Do(func() {
			close(b.entered)
			select {
			case <-b.release:
			case <-ctx.Done():
			}
		})
	}
	return ctx
}
func (*materialSnapshotBarrier) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func TestMaterialReferencePostgresSnapshotSurvivesConcurrentReplacement(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	if err := database.Migrate(ctx, environment.Pool); err != nil {
		t.Fatal(err)
	}
	store, _ := NewPostgresStore(environment.Pool)
	writer, _ := NewService(store, &stableReferences{})
	actor := Actor{Kind: "test_subject", ID: intActor, TenantID: intTenant}
	created, err := writer.Create(ctx, actor, CreateInput{CommandID: command(240), ProjectID: intProject, ProjectSourceRevision: 4})
	if err != nil {
		t.Fatal(err)
	}
	id := created.WorkItem.ID
	if _, err := writer.SetOwner(ctx, actor, id, OwnerInput{CommandID: command(241), ExpectedRevision: 1, AssignmentID: intAssignment, AssignmentSourceRevision: 1}); err != nil {
		t.Fatal(err)
	}
	input := DeliverableInput{CommandID: command(242), ExpectedRevision: 2, RequirementKey: RequirementPrimary, EvidenceFileID: workItemPerson, EvidenceVersion: 1, EvidenceSourceRevision: 1}
	if _, err := writer.PutDeliverable(ctx, actor, id, "scheme", input); err != nil {
		t.Fatal(err)
	}
	barrier := &materialSnapshotBarrier{entered: make(chan struct{}), release: make(chan struct{})}
	config := environment.Pool.Config().Copy()
	config.ConnConfig.Tracer = barrier
	readerPool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer readerPool.Close()
	readerStore, _ := NewPostgresStore(readerPool)
	reader, _ := NewService(readerStore, &materialReferenceNoUpstream{})
	type result struct {
		value MaterialReferenceProjection
		err   error
	}
	finished := make(chan result, 1)
	go func() {
		value, err := reader.ReferenceMaterials(ctx, intTenant, id)
		finished <- result{value, err}
	}()
	select {
	case <-barrier.entered:
	case <-ctx.Done():
		t.Fatal("reader did not reach the inter-query barrier")
	}
	input.CommandID, input.ExpectedRevision, input.EvidenceVersion, input.EvidenceSourceRevision = command(243), 3, 2, 2
	_, writeErr := writer.PutDeliverable(ctx, actor, id, "scheme", input)
	close(barrier.release)
	if writeErr != nil {
		t.Fatal(writeErr)
	}
	first := <-finished
	if first.err != nil || first.value.Revision != 3 || len(first.value.Items) != 1 || first.value.Items[0].Version != 1 {
		t.Fatalf("mixed pre/post-command snapshot: %#v err=%v", first.value, first.err)
	}
	second, err := reader.ReferenceMaterials(ctx, intTenant, id)
	if err != nil || second.Revision != 4 || len(second.Items) != 1 || second.Items[0].Version != 2 || second.Digest == first.value.Digest {
		t.Fatalf("new snapshot=%#v err=%v", second, err)
	}
}
