package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"omsbizfoundry/money-flow-ledger/internal/buildinfo"
	"omsbizfoundry/money-flow-ledger/internal/config"
	"omsbizfoundry/money-flow-ledger/internal/database"
	"omsbizfoundry/money-flow-ledger/internal/partymigration"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	if err := run(); err != nil {
		logger.Error("Money Flow Ledger operation failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	if buildinfo.Requested(os.Args[1:]) {
		return buildinfo.Write(os.Stdout)
	}
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: ops check|migrate|link-iam-subject|party-migration-preflight|party-migration-map|party-runtime-transition|party-directory-project|payable-party-map|version [options]")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	switch os.Args[1] {
	case "check":
		return runCheck(ctx, os.Args[2:])
	case "migrate":
		if len(os.Args) != 2 {
			return fmt.Errorf("migrate does not accept arguments")
		}
		return runMigrate(ctx)
	case "link-iam-subject":
		return runLinkIAMSubject(ctx, os.Args[2:])
	case "party-migration-preflight":
		return runPartyMigrationPreflight(ctx, os.Args[2:])
	case "party-migration-map":
		return runPartyMigrationMap(ctx, os.Args[2:])
	case "party-runtime-transition":
		return runPartyRuntimeTransition(ctx, os.Args[2:])
	case "party-directory-project":
		return runPartyDirectoryProject(ctx, os.Args[2:])
	case "payable-party-map":
		return runPayablePartyMap(ctx, os.Args[2:])
	default:
		return fmt.Errorf("unknown operation %q; expected check, migrate, link-iam-subject, party-migration-preflight, party-migration-map, party-runtime-transition, party-directory-project, payable-party-map, or version", os.Args[1])
	}
}

var operationUUIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
var operationDigestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

func runPayablePartyMap(ctx context.Context, arguments []string) error {
	flags := flag.NewFlagSet("ops payable-party-map", flag.ContinueOnError)
	commandID := flags.String("command-id", "", "idempotent reviewed mapping command UUID")
	tenantID := flags.String("tenant-id", "", "installation tenant UUID")
	personID := flags.String("person-id", "", "People & Groups Person UUID")
	personRevision := flags.Int64("person-revision", 0, "exact People Person revision")
	personDigest := flags.String("person-digest", "", "exact People Person sha256 digest")
	localPartyID := flags.String("local-party-id", "", "reviewed Money individual Party UUID")
	reviewerKind := flags.String("reviewed-by-actor-kind", "iam_subject", "reviewer actor kind")
	reviewerID := flags.String("reviewed-by-actor-id", "", "reviewer actor identifier")
	reviewerName := flags.String("reviewed-by-display-name", "", "reviewer display name")
	reason := flags.String("reason", "", "reviewed mapping reason")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("payable-party-map does not accept positional arguments")
	}
	for name, value := range map[string]*string{"command-id": commandID, "tenant-id": tenantID, "person-id": personID, "local-party-id": localPartyID} {
		*value = strings.ToLower(strings.TrimSpace(*value))
		if !operationUUIDPattern.MatchString(*value) {
			return fmt.Errorf("%s must be a lowercase RFC 4122 UUID", name)
		}
	}
	*personDigest = strings.ToLower(strings.TrimSpace(*personDigest))
	*reviewerKind, *reviewerID = strings.TrimSpace(*reviewerKind), strings.TrimSpace(*reviewerID)
	*reviewerName, *reason = strings.TrimSpace(*reviewerName), strings.TrimSpace(*reason)
	if *personRevision < 1 || !operationDigestPattern.MatchString(*personDigest) {
		return fmt.Errorf("person-revision and person-digest must identify one exact People Person snapshot")
	}
	if !regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`).MatchString(*reviewerKind) || len([]rune(*reviewerID)) < 1 || len([]rune(*reviewerID)) > 100 || len([]rune(*reviewerName)) < 1 || len([]rune(*reviewerName)) > 100 || len([]rune(*reason)) < 1 || len([]rune(*reason)) > 1000 {
		return fmt.Errorf("reviewer identity, display name and reason are required and must fit their documented limits")
	}
	pool, _, err := openRuntime(ctx)
	if err != nil {
		return err
	}
	defer pool.Close()
	if _, err = database.RequireCurrentSchema(ctx, pool); err != nil {
		return err
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return fmt.Errorf("begin payable Party mapping transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, "money-flow-payable-party-map:"+*tenantID); err != nil {
		return fmt.Errorf("lock payable Party mappings: %w", err)
	}
	var localRevision int64
	var localKind, localName string
	if err = tx.QueryRow(ctx, `SELECT revision,kind,legal_name FROM parties WHERE id=$1`, *localPartyID).Scan(&localRevision, &localKind, &localName); err != nil {
		return fmt.Errorf("load Money local Party: %w", err)
	}
	if localKind != "individual" {
		return fmt.Errorf("People Person payable mapping requires an individual Money Party")
	}
	runtimeState, err := partymigration.NewRuntimeRegistry(pool).State(ctx, *tenantID)
	if err != nil {
		return fmt.Errorf("load Money Party runtime state: %w", err)
	}
	if runtimeState.DirectoryAuthorityEstablished {
		if runtimeState.Mode != partymigration.RuntimeModeDirectory {
			return fmt.Errorf("People Person payable mapping requires active Party Directory mode after authority has been established")
		}
		var reviewedDirectoryProjection bool
		if err = tx.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM party_migration_mappings
				WHERE tenant_id=$1 AND migration_batch_id=$2 AND local_party_id=$3 AND target_kind='individual'
				UNION ALL
				SELECT 1 FROM party_directory_runtime_projections
				WHERE tenant_id=$1 AND local_party_id=$3 AND target_kind='individual'
			)
		`, *tenantID, runtimeState.MigrationBatchID, *localPartyID).Scan(&reviewedDirectoryProjection); err != nil {
			return fmt.Errorf("verify Money individual Party Directory projection: %w", err)
		}
		if !reviewedDirectoryProjection {
			return fmt.Errorf("People Person payable mapping requires a reviewed Party Directory individual projection")
		}
	}
	request := struct {
		CommandID, TenantID, PersonID, PersonDigest, LocalPartyID, LocalPartyKind, LocalPartyName string
		PersonRevision, LocalPartyRevision                                                        int64
		ReviewerKind, ReviewerID, ReviewerName, Reason                                            string
	}{*commandID, *tenantID, *personID, *personDigest, *localPartyID, localKind, localName, *personRevision, localRevision, *reviewerKind, *reviewerID, *reviewerName, *reason}
	raw, err := json.Marshal(request)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(raw)
	requestSHA256 := hex.EncodeToString(digest[:])
	var existingRequestSHA256 string
	var existingRecordedAt time.Time
	err = tx.QueryRow(ctx, `SELECT request_sha256,recorded_at FROM payable_party_source_mappings WHERE command_id=$1`, *commandID).Scan(&existingRequestSHA256, &existingRecordedAt)
	if err == nil {
		if existingRequestSHA256 != requestSHA256 {
			return fmt.Errorf("payable Party mapping command %s already exists with a different request", *commandID)
		}
		if err = tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit payable Party mapping replay: %w", err)
		}
		return writeJSON(map[string]any{"service": "money-flow-ledger", "tenant_id": *tenantID, "command_id": *commandID, "person_id": *personID, "person_revision": *personRevision, "person_digest": *personDigest, "local_party_id": *localPartyID, "local_party_revision": localRevision, "recorded_at": existingRecordedAt, "request_sha256": requestSHA256, "replayed": true})
	}
	if err != pgx.ErrNoRows {
		return fmt.Errorf("load payable Party mapping command: %w", err)
	}
	now := time.Now().UTC()
	result, err := tx.Exec(ctx, `INSERT INTO payable_party_source_mappings(tenant_id,command_id,source_application,source_resource_type,source_resource_id,source_revision,source_digest,local_party_id,local_party_revision,local_party_kind,local_party_legal_name,reviewed_by_actor_kind,reviewed_by_actor_id,reviewed_by_actor_display_name,review_reason,recorded_at,request_sha256)
		VALUES($1,$2,'people-groups','person',$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`, *tenantID, *commandID, *personID, *personRevision, *personDigest, *localPartyID, localRevision, localKind, localName, *reviewerKind, *reviewerID, *reviewerName, *reason, now, requestSHA256)
	if err != nil {
		return fmt.Errorf("record immutable payable Party mapping: %w", err)
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("payable Party mapping was not recorded")
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit payable Party mapping: %w", err)
	}
	return writeJSON(map[string]any{"service": "money-flow-ledger", "tenant_id": *tenantID, "command_id": *commandID, "person_id": *personID, "person_revision": *personRevision, "person_digest": *personDigest, "local_party_id": *localPartyID, "local_party_revision": localRevision, "recorded_at": now, "request_sha256": requestSHA256, "replayed": false})
}

func runPartyMigrationPreflight(ctx context.Context, arguments []string) error {
	flags := flag.NewFlagSet("ops party-migration-preflight", flag.ContinueOnError)
	tenantID := flags.String("tenant-id", "", "target Party Directory tenant UUID")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("party-migration-preflight does not accept positional arguments")
	}
	*tenantID = strings.ToLower(strings.TrimSpace(*tenantID))
	if !operationUUIDPattern.MatchString(*tenantID) {
		return fmt.Errorf("tenant-id must be a lowercase RFC 4122 UUID")
	}
	operationContext, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	pool, _, err := openRuntime(operationContext)
	if err != nil {
		return err
	}
	defer pool.Close()
	if _, err := database.RequireCurrentSchema(operationContext, pool); err != nil {
		return err
	}
	report, err := partymigration.Inspect(operationContext, pool, *tenantID)
	if err != nil {
		return err
	}
	if err := writeJSON(report); err != nil {
		return err
	}
	if !report.ReadyForReview {
		return fmt.Errorf("Money Flow Party migration preflight found %d blocking issue(s)", len(report.BlockingIssues))
	}
	return nil
}

func runLinkIAMSubject(ctx context.Context, arguments []string) error {
	flags := flag.NewFlagSet("ops link-iam-subject", flag.ContinueOnError)
	localUserID := flags.String("local-user-id", "", "historic Money Flow local user UUID")
	tenantID := flags.String("tenant-id", "", "installation tenant UUID")
	subjectID := flags.String("subject-id", "", "IAM subject UUID")
	linkedBy := flags.String("linked-by-subject-id", "", "IAM administrator subject UUID")
	displayName := flags.String("linked-by-display-name", "", "IAM administrator display name snapshot")
	reason := flags.String("reason", "", "reviewed reason for the explicit identity link")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("link-iam-subject does not accept positional arguments")
	}
	for name, value := range map[string]string{"local-user-id": *localUserID, "tenant-id": *tenantID, "subject-id": *subjectID, "linked-by-subject-id": *linkedBy} {
		if !operationUUIDPattern.MatchString(strings.ToLower(strings.TrimSpace(value))) {
			return fmt.Errorf("%s must be a lowercase RFC 4122 UUID", name)
		}
	}
	if strings.TrimSpace(*displayName) == "" || len([]rune(strings.TrimSpace(*displayName))) > 100 {
		return fmt.Errorf("linked-by-display-name is required and cannot exceed 100 characters")
	}
	if strings.TrimSpace(*reason) == "" || len([]rune(strings.TrimSpace(*reason))) > 500 {
		return fmt.Errorf("reason is required and cannot exceed 500 characters")
	}
	pool, _, err := openRuntime(ctx)
	if err != nil {
		return err
	}
	defer pool.Close()
	if _, err := database.RequireCurrentSchema(ctx, pool); err != nil {
		return err
	}
	now := time.Now().UTC()
	result, err := pool.Exec(ctx, `INSERT INTO local_user_iam_subject_links(local_user_id,tenant_id,iam_subject_id,linked_by_iam_subject_id,linked_by_display_name,reason,linked_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, *localUserID, *tenantID, *subjectID, *linkedBy, strings.TrimSpace(*displayName), strings.TrimSpace(*reason), now)
	if err != nil {
		return fmt.Errorf("record immutable local-user IAM-subject link: %w", err)
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("identity link was not recorded")
	}
	return writeJSON(map[string]any{"service": "money-flow-ledger", "local_user_id": *localUserID, "tenant_id": *tenantID, "iam_subject_id": *subjectID, "linked_at": now})
}

type checkReport struct {
	Service    string                    `json:"service"`
	Build      buildinfo.Info            `json:"build"`
	PostgreSQL database.PostgreSQLStatus `json:"postgresql"`
	Schema     database.SchemaStatus     `json:"schema"`
}

type migrationReport struct {
	Service    string                    `json:"service"`
	Build      buildinfo.Info            `json:"build"`
	PostgreSQL database.PostgreSQLStatus `json:"postgresql"`
	Before     database.SchemaStatus     `json:"before"`
	After      database.SchemaStatus     `json:"after"`
}

func runCheck(ctx context.Context, arguments []string) error {
	flags := flag.NewFlagSet("ops check", flag.ContinueOnError)
	requireCurrent := flags.Bool("require-current", false, "fail unless all embedded migrations are applied")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("check does not accept positional arguments")
	}
	operationContext, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	pool, postgresStatus, err := openRuntime(operationContext)
	if err != nil {
		return err
	}
	defer pool.Close()
	schemaStatus, err := database.InspectSchema(operationContext, pool)
	if err != nil {
		return err
	}
	if err := writeJSON(checkReport{
		Service: "money-flow-ledger", Build: buildinfo.Current(), PostgreSQL: postgresStatus, Schema: schemaStatus,
	}); err != nil {
		return err
	}
	if schemaStatus.State == database.SchemaIncompatible {
		return fmt.Errorf("database schema is incompatible: %s", schemaStatus.Reason)
	}
	if *requireCurrent && schemaStatus.State != database.SchemaCurrent {
		return fmt.Errorf("database schema is %s; run the matching release migration before server startup", schemaStatus.State)
	}
	return nil
}

func runMigrate(ctx context.Context) error {
	pool, postgresStatus, err := openRuntime(ctx)
	if err != nil {
		return err
	}
	defer pool.Close()
	before, err := database.InspectSchema(ctx, pool)
	if err != nil {
		return err
	}
	if err := database.Migrate(ctx, pool); err != nil {
		return err
	}
	after, err := database.RequireCurrentSchema(ctx, pool)
	if err != nil {
		return err
	}
	return writeJSON(migrationReport{
		Service: "money-flow-ledger", Build: buildinfo.Current(), PostgreSQL: postgresStatus, Before: before, After: after,
	})
}

func openRuntime(ctx context.Context) (*pgxpool.Pool, database.PostgreSQLStatus, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, database.PostgreSQLStatus{}, err
	}
	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, database.PostgreSQLStatus{}, err
	}
	postgresStatus, err := database.CheckPostgreSQL(ctx, pool)
	if err != nil {
		pool.Close()
		return nil, database.PostgreSQLStatus{}, err
	}
	return pool, postgresStatus, nil
}

func writeJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("write operation report: %w", err)
	}
	return nil
}
