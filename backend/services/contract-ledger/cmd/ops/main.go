package main

import (
	"context"
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

	"github.com/jackc/pgx/v5/pgxpool"

	"omsbizfoundry/contract-ledger/internal/buildinfo"
	"omsbizfoundry/contract-ledger/internal/config"
	"omsbizfoundry/contract-ledger/internal/database"
	"omsbizfoundry/contract-ledger/internal/evidence"
	"omsbizfoundry/contract-ledger/internal/evidencemigration"
	"omsbizfoundry/contract-ledger/internal/partymigration"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	if err := run(); err != nil {
		logger.Error("Contract Ledger operation failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	if buildinfo.Requested(os.Args[1:]) {
		return buildinfo.Write(os.Stdout)
	}
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: ops check|migrate|link-iam-subject|evidence-migration-preflight|party-migration-preflight|party-migration-map|party-runtime-transition|version [options]")
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
	case "evidence-migration-preflight":
		return runEvidenceMigrationPreflight(ctx, os.Args[2:])
	case "party-migration-preflight":
		return runPartyMigrationPreflight(ctx, os.Args[2:])
	case "party-migration-map":
		return runPartyMigrationMap(ctx, os.Args[2:])
	case "party-runtime-transition":
		return runPartyRuntimeTransition(ctx, os.Args[2:])
	default:
		return fmt.Errorf("unknown operation %q; expected check, migrate, link-iam-subject, evidence-migration-preflight, party-migration-preflight, party-migration-map, party-runtime-transition, or version", os.Args[1])
	}
}

var operationUUIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

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
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	operationContext, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	pool, err := database.Open(operationContext, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if _, err := database.CheckPostgreSQL(operationContext, pool); err != nil {
		return err
	}
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
		return fmt.Errorf("Contract Party migration preflight found %d blocking issue(s)", len(report.BlockingIssues))
	}
	return nil
}

func runEvidenceMigrationPreflight(ctx context.Context, arguments []string) error {
	flags := flag.NewFlagSet("ops evidence-migration-preflight", flag.ContinueOnError)
	tenantID := flags.String("tenant-id", "", "target Evidence tenant UUID")
	storageRoot := flags.String("storage-root", "", "legacy Contract Evidence root; defaults to EVIDENCE_STORAGE_DIR")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("evidence-migration-preflight does not accept positional arguments")
	}
	*tenantID = strings.ToLower(strings.TrimSpace(*tenantID))
	if !operationUUIDPattern.MatchString(*tenantID) {
		return fmt.Errorf("tenant-id must be a lowercase RFC 4122 UUID")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	root := strings.TrimSpace(*storageRoot)
	if root == "" {
		root = cfg.EvidenceStorageDirectory
	}
	operationContext, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	pool, err := database.Open(operationContext, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if _, err := database.CheckPostgreSQL(operationContext, pool); err != nil {
		return err
	}
	if _, err := database.RequireCurrentSchema(operationContext, pool); err != nil {
		return err
	}
	report, err := evidencemigration.Inspect(operationContext, pool, root, *tenantID)
	if err != nil {
		return err
	}
	if err := writeJSON(report); err != nil {
		return err
	}
	if !report.Ready {
		return fmt.Errorf("Contract Evidence migration preflight found %d blocking issue(s)", len(report.Issues))
	}
	return nil
}

func runLinkIAMSubject(ctx context.Context, arguments []string) error {
	flags := flag.NewFlagSet("ops link-iam-subject", flag.ContinueOnError)
	localUserID := flags.String("local-user-id", "", "historic Contract local user UUID")
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
	pool, _, _, err := openRuntime(ctx)
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
	return writeJSON(map[string]any{"service": "contract-ledger", "local_user_id": *localUserID, "tenant_id": *tenantID, "iam_subject_id": *subjectID, "linked_at": now})
}

type evidenceReport struct {
	Root     string `json:"root"`
	Writable bool   `json:"writable"`
}

type checkReport struct {
	Service    string                    `json:"service"`
	Build      buildinfo.Info            `json:"build"`
	PostgreSQL database.PostgreSQLStatus `json:"postgresql"`
	Schema     database.SchemaStatus     `json:"schema"`
	Evidence   evidenceReport            `json:"evidence"`
}

type migrationReport struct {
	Service    string                    `json:"service"`
	Build      buildinfo.Info            `json:"build"`
	PostgreSQL database.PostgreSQLStatus `json:"postgresql"`
	Before     database.SchemaStatus     `json:"before"`
	After      database.SchemaStatus     `json:"after"`
	Evidence   evidenceReport            `json:"evidence"`
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
	pool, store, postgresStatus, err := openRuntime(operationContext)
	if err != nil {
		return err
	}
	defer pool.Close()
	schemaStatus, err := database.InspectSchema(operationContext, pool)
	if err != nil {
		return err
	}
	report := checkReport{
		Service: "contract-ledger", Build: buildinfo.Current(), PostgreSQL: postgresStatus,
		Schema: schemaStatus, Evidence: evidenceReport{Root: store.Root(), Writable: true},
	}
	if err := writeJSON(report); err != nil {
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
	pool, store, postgresStatus, err := openRuntime(ctx)
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
		Service: "contract-ledger", Build: buildinfo.Current(), PostgreSQL: postgresStatus,
		Before: before, After: after, Evidence: evidenceReport{Root: store.Root(), Writable: true},
	})
}

func openRuntime(ctx context.Context) (*pgxpool.Pool, *evidence.LocalStore, database.PostgreSQLStatus, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, database.PostgreSQLStatus{}, err
	}
	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, nil, database.PostgreSQLStatus{}, err
	}
	fail := func(cause error) (*pgxpool.Pool, *evidence.LocalStore, database.PostgreSQLStatus, error) {
		pool.Close()
		return nil, nil, database.PostgreSQLStatus{}, cause
	}
	postgresStatus, err := database.CheckPostgreSQL(ctx, pool)
	if err != nil {
		return fail(err)
	}
	store, err := evidence.NewLocalStore(cfg.EvidenceStorageDirectory)
	if err != nil {
		return fail(err)
	}
	if err := store.CheckWritable(ctx); err != nil {
		return fail(err)
	}
	return pool, store, postgresStatus, nil
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
