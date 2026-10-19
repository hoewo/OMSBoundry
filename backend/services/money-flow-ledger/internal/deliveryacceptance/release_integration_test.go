//go:build integration

package deliveryacceptance_test

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"omsbizfoundry/money-flow-ledger/internal/testdatabase"
)

const (
	bootstrapUsername = "delivery-admin"
	bootstrapPassword = "delivery-admin-password"
	exportFormatID    = "omsbizfoundry.money-flow-ledger/owner-ledger-export-v2"
	latestSchema      = "000016_local_user_iam_subject_links.sql"
)

var (
	csrfPattern    = regexp.MustCompile(`name="csrf_token" value="([^"]+)"`)
	commandPattern = regexp.MustCompile(`name="command_id" value="([^"]+)"`)
	bundleFiles    = []string{
		"bin/money-flow-ledger-backup",
		"bin/money-flow-ledger-ops",
		"bin/money-flow-ledger-server",
		"config/money-flow-ledger.env.example",
		"docs/BACKUP-RESTORE.md",
		"docs/INSTALL-UPGRADE.md",
	}
	exportFiles = []string{
		"allocation_reversals.csv",
		"allocations.csv",
		"audit_entries.csv",
		"cash_transaction_reversals.csv",
		"cash_transactions.csv",
		"internal_transfers.csv",
		"manifest.json",
		"obligation_amount_adjustments.csv",
		"obligation_cash_expectations.csv",
		"obligation_closures.csv",
		"obligation_remissions.csv",
		"obligation_revisions.csv",
		"obligations.csv",
		"parties.csv",
	}
)

func TestMoneyFlowReleaseCleanInstall(t *testing.T) {
	archivePath := requireAbsoluteFile(t, "DELIVERY_BUNDLE_PATH")
	checksumPath := requireAbsoluteFile(t, "DELIVERY_BUNDLE_SHA256_PATH")
	verifyChecksum(t, archivePath, checksumPath)
	bundleRoot := extractBundle(t, archivePath)

	serverBinary := filepath.Join(bundleRoot, "bin", "money-flow-ledger-server")
	opsBinary := filepath.Join(bundleRoot, "bin", "money-flow-ledger-ops")
	backupBinary := filepath.Join(bundleRoot, "bin", "money-flow-ledger-backup")
	assertBuildIdentity(t, serverBinary, opsBinary, backupBinary)

	environment := testdatabase.New(t, testdatabase.RequireURL(t))
	check := runOpsCheck(t, opsBinary, environment.URL, false)
	if check.PostgreSQL.Major != 18 || check.Schema.State != "empty" || check.Schema.ServerReady {
		t.Fatalf("empty schema check = %#v", check)
	}
	runCommand(t, cleanEnvironment("DATABASE_URL="+environment.URL), opsBinary, "migrate")
	check = runOpsCheck(t, opsBinary, environment.URL, true)
	if check.Schema.State != "current" || check.Schema.Current != latestSchema ||
		check.Schema.Latest != latestSchema || !check.Schema.ServerReady {
		t.Fatalf("current schema check = %#v", check)
	}

	firstAddress := unusedAddress(t)
	first := startServer(t, serverBinary, environment.URL, firstAddress, true)
	firstURL := "http://" + firstAddress
	waitReady(t, first, firstURL)
	client := newClient(t)
	login(t, client, firstURL, bootstrapUsername, bootstrapPassword)

	ownerID := createParty(t, client, firstURL, "organization", "交付验收本方组织", "DELIVERY-OWNER-001")
	counterpartyID := createParty(t, client, firstURL, "individual", "交付验收往来方", "")
	obligationID := createAndConfirmObligation(t, client, firstURL, ownerID, counterpartyID)
	recordCashExpectation(t, client, firstURL, obligationID)
	cashID := createCashReceipt(t, client, firstURL, ownerID, counterpartyID)
	allocationID := applyAllocation(t, client, firstURL, cashID, obligationID)

	forecast := getPage(t, client, firstURL+"/cash-forecast?history_obligation_id="+obligationID)
	assertContainsAll(t, "cash forecast", forecast,
		"当前预计现金排期", "2099-10-20", "60.25 CNY", "交付验收应收")
	exportArchive := exportOwnerLedger(t, client, firstURL, ownerID)
	verifyOwnerExport(t, exportArchive, ownerID, obligationID, cashID, allocationID)
	verifyDatabaseFacts(t, environment, obligationID, cashID, allocationID)

	first.stop(t)
	secondAddress := unusedAddress(t)
	second := startServer(t, serverBinary, environment.URL, secondAddress, false)
	secondURL := "http://" + secondAddress
	waitReady(t, second, secondURL)
	restartedClient := newClient(t)
	login(t, restartedClient, secondURL, bootstrapUsername, bootstrapPassword)
	restartedForecast := getPage(t, restartedClient, secondURL+"/cash-forecast?history_obligation_id="+obligationID)
	assertContainsAll(t, "restarted cash forecast", restartedForecast,
		"2099-10-20", "60.25 CNY", "交付验收应收")
	var users int
	if err := environment.Pool.QueryRow(context.Background(), `SELECT count(*) FROM local_users`).Scan(&users); err != nil {
		t.Fatalf("count users after restart: %v", err)
	}
	if users != 1 {
		t.Fatalf("users after restart = %d, want 1", users)
	}
	second.stop(t)
}

func TestMoneyFlowReleaseBackupRestore(t *testing.T) {
	archivePath := requireAbsoluteFile(t, "DELIVERY_BUNDLE_PATH")
	checksumPath := requireAbsoluteFile(t, "DELIVERY_BUNDLE_SHA256_PATH")
	pgDumpBinary := requireAbsoluteFile(t, "DELIVERY_PG_DUMP_BIN")
	pgRestoreBinary := requireAbsoluteFile(t, "DELIVERY_PG_RESTORE_BIN")
	verifyChecksum(t, archivePath, checksumPath)
	bundleRoot := extractBundle(t, archivePath)

	serverBinary := filepath.Join(bundleRoot, "bin", "money-flow-ledger-server")
	opsBinary := filepath.Join(bundleRoot, "bin", "money-flow-ledger-ops")
	backupBinary := filepath.Join(bundleRoot, "bin", "money-flow-ledger-backup")
	assertBuildIdentity(t, serverBinary, opsBinary, backupBinary)

	baseDatabaseURL := testdatabase.RequireURL(t)
	source := testdatabase.New(t, baseDatabaseURL)
	runCommand(t, cleanEnvironment("DATABASE_URL="+source.URL), opsBinary, "migrate")

	sourceAddress := unusedAddress(t)
	sourceServer := startServer(t, serverBinary, source.URL, sourceAddress, true)
	sourceURL := "http://" + sourceAddress
	waitReady(t, sourceServer, sourceURL)
	sourceClient := newClient(t)
	login(t, sourceClient, sourceURL, bootstrapUsername, bootstrapPassword)

	ownerID := createParty(t, sourceClient, sourceURL, "organization", "交付验收本方组织", "DELIVERY-OWNER-001")
	counterpartyID := createParty(t, sourceClient, sourceURL, "individual", "交付验收往来方", "")
	obligationID := createAndConfirmObligation(t, sourceClient, sourceURL, ownerID, counterpartyID)
	recordCashExpectation(t, sourceClient, sourceURL, obligationID)
	cashID := createCashReceipt(t, sourceClient, sourceURL, ownerID, counterpartyID)
	allocationID := applyAllocation(t, sourceClient, sourceURL, cashID, obligationID)
	verifyDatabaseFacts(t, source, obligationID, cashID, allocationID)

	backupPath := filepath.Join(t.TempDir(), "money-flow-source-backup.zip")
	backupEnvironment := cleanEnvironment(
		"DATABASE_URL="+source.URL,
		"POSTGRES_CLIENT_DATABASE_URL="+source.URL,
	)
	runCommand(t, backupEnvironment, backupBinary, "create",
		"--pg-dump-bin", pgDumpBinary,
		"--output", backupPath,
	)
	archiveDigest := verifyBackupArchive(t, backupPath, source.Name)

	handoffPath := filepath.Join(t.TempDir(), "money-flow-handoff-backup.zip")
	copyFile(t, backupPath, handoffPath)
	if handoffDigest := fileSHA256(t, handoffPath); handoffDigest != archiveDigest {
		t.Fatalf("handoff archive digest = %s, want %s", handoffDigest, archiveDigest)
	}
	sourceServer.stop(t)

	target := testdatabase.New(t, baseDatabaseURL)
	restoreEnvironment := cleanEnvironment(
		"DATABASE_URL="+target.URL,
		"POSTGRES_CLIENT_DATABASE_URL="+target.URL,
	)
	runCommand(t, restoreEnvironment, backupBinary, "restore",
		"--pg-restore-bin", pgRestoreBinary,
		"--input", handoffPath,
	)
	check := runOpsCheck(t, opsBinary, target.URL, true)
	if check.Schema.State != "current" || check.Schema.Current != latestSchema || !check.Schema.ServerReady {
		t.Fatalf("restored schema check = %#v", check)
	}
	var restoredSessions int
	if err := target.Pool.QueryRow(context.Background(), `SELECT count(*) FROM local_sessions`).Scan(&restoredSessions); err != nil {
		t.Fatalf("count restored sessions: %v", err)
	}
	if restoredSessions != 0 {
		t.Fatalf("restored sessions = %d, want 0", restoredSessions)
	}

	targetAddress := unusedAddress(t)
	targetServer := startServer(t, serverBinary, target.URL, targetAddress, false)
	targetURL := "http://" + targetAddress
	waitReady(t, targetServer, targetURL)
	assertOldSessionRejected(t, sourceClient, targetURL)

	restoredClient := newClient(t)
	login(t, restoredClient, targetURL, bootstrapUsername, bootstrapPassword)
	restoredForecast := getPage(t, restoredClient, targetURL+"/cash-forecast?history_obligation_id="+obligationID)
	assertContainsAll(t, "restored cash forecast", restoredForecast,
		"当前预计现金排期", "2099-10-20", "60.25 CNY", "交付验收应收")
	verifyDatabaseFacts(t, target, obligationID, cashID, allocationID)
	createParty(t, restoredClient, targetURL, "organization", "恢复后新增主体", "DELIVERY-RESTORED-001")
	assertPartyCount(t, target, 3)
	assertPartyCount(t, source, 2)
	targetServer.stop(t)

	nonemptyTarget := testdatabase.New(t, baseDatabaseURL)
	runCommand(t, cleanEnvironment("DATABASE_URL="+nonemptyTarget.URL), opsBinary, "migrate")
	nonemptyEnvironment := cleanEnvironment(
		"DATABASE_URL="+nonemptyTarget.URL,
		"POSTGRES_CLIENT_DATABASE_URL="+nonemptyTarget.URL,
	)
	runCommandFails(t, nonemptyEnvironment, "restore target database is not empty", backupBinary, "restore",
		"--pg-restore-bin", pgRestoreBinary,
		"--input", handoffPath,
	)
	nonemptyCheck := runOpsCheck(t, opsBinary, nonemptyTarget.URL, true)
	if nonemptyCheck.Schema.State != "current" || nonemptyCheck.Schema.Current != latestSchema {
		t.Fatalf("nonempty rejection changed schema = %#v", nonemptyCheck)
	}

	corruptPath := filepath.Join(t.TempDir(), "money-flow-corrupt-backup.zip")
	tamperBackupDump(t, handoffPath, corruptPath)
	corruptTarget := testdatabase.New(t, baseDatabaseURL)
	corruptEnvironment := cleanEnvironment(
		"DATABASE_URL="+corruptTarget.URL,
		"POSTGRES_CLIENT_DATABASE_URL="+corruptTarget.URL,
	)
	runCommandFails(t, corruptEnvironment, "integrity mismatch", backupBinary, "restore",
		"--pg-restore-bin", pgRestoreBinary,
		"--input", corruptPath,
	)
	if relations := countUserRelations(t, corruptTarget); relations != 0 {
		t.Fatalf("corrupt restore target relations = %d, want 0", relations)
	}
}

type backupManifest struct {
	Format      string `json:"format"`
	Version     int    `json:"version"`
	GeneratedAt string `json:"generated_at"`
	Database    struct {
		Name              string `json:"name"`
		PostgreSQLVersion string `json:"postgresql_version"`
		SchemaVersion     string `json:"schema_version"`
		DumpPath          string `json:"dump_path"`
		DumpBytes         int64  `json:"dump_bytes"`
		DumpSHA256        string `json:"dump_sha256"`
		SessionsExcluded  bool   `json:"sessions_excluded"`
		Tables            []struct {
			Name string `json:"name"`
			Rows int64  `json:"rows"`
		} `json:"tables"`
	} `json:"database"`
}

func verifyBackupArchive(t *testing.T, archivePath, sourceDatabaseName string) string {
	t.Helper()
	info, err := os.Stat(archivePath)
	if err != nil {
		t.Fatalf("stat backup archive: %v", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		t.Fatalf("backup archive mode = %s, want regular 0600", info.Mode())
	}
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatalf("open backup ZIP: %v", err)
	}
	defer reader.Close()
	if len(reader.File) != 2 {
		t.Fatalf("backup ZIP entries = %d, want 2", len(reader.File))
	}
	entries := make(map[string]*zip.File, len(reader.File))
	for _, entry := range reader.File {
		if _, duplicate := entries[entry.Name]; duplicate {
			t.Fatalf("duplicate backup ZIP entry %q", entry.Name)
		}
		if entry.Method != zip.Store || !entry.Mode().IsRegular() || entry.Mode().Perm() != 0o600 ||
			path.Clean(entry.Name) != entry.Name || strings.Contains(entry.Name, "/") {
			t.Fatalf("invalid backup ZIP entry %q method=%d mode=%s", entry.Name, entry.Method, entry.Mode())
		}
		entries[entry.Name] = entry
	}
	manifestEntry, manifestOK := entries["manifest.json"]
	dumpEntry, dumpOK := entries["database.dump"]
	if !manifestOK || !dumpOK {
		t.Fatalf("backup ZIP entries = %v, want manifest.json and database.dump", mapKeys(entries))
	}

	manifestBytes := readZIPFile(t, manifestEntry, 1<<20)
	decoder := json.NewDecoder(bytes.NewReader(manifestBytes))
	decoder.DisallowUnknownFields()
	var manifest backupManifest
	if err := decoder.Decode(&manifest); err != nil {
		t.Fatalf("decode backup manifest: %v", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		t.Fatalf("backup manifest has trailing JSON: %v", err)
	}
	generatedAt, err := time.Parse(time.RFC3339Nano, manifest.GeneratedAt)
	if err != nil || generatedAt.Location() != time.UTC {
		t.Fatalf("backup generated_at = %q, want RFC3339 UTC", manifest.GeneratedAt)
	}
	if manifest.Format != "omsbizfoundry.money-flow-ledger/full-backup-v1" || manifest.Version != 1 ||
		manifest.Database.Name != sourceDatabaseName || !strings.HasPrefix(manifest.Database.PostgreSQLVersion, "18.") ||
		manifest.Database.SchemaVersion != latestSchema || manifest.Database.DumpPath != "database.dump" ||
		!manifest.Database.SessionsExcluded {
		t.Fatalf("backup manifest identity = %#v", manifest)
	}
	if manifest.Database.DumpBytes != int64(dumpEntry.UncompressedSize64) ||
		manifest.Database.DumpSHA256 != hashZIPFile(t, dumpEntry) {
		t.Fatalf("backup dump metadata bytes=%d/%d sha=%s", manifest.Database.DumpBytes, dumpEntry.UncompressedSize64, manifest.Database.DumpSHA256)
	}

	tableCounts := make(map[string]int64, len(manifest.Database.Tables))
	previous := ""
	for _, table := range manifest.Database.Tables {
		if table.Name == "" || table.Name == "local_sessions" || table.Rows < 0 ||
			(previous != "" && table.Name <= previous) {
			t.Fatalf("invalid backup table count after %q: %#v", previous, table)
		}
		previous = table.Name
		tableCounts[table.Name] = table.Rows
	}
	expectedCounts := map[string]int64{
		"local_users": 1, "parties": 2, "obligations": 1, "obligation_revisions": 1,
		"obligation_cash_expectations": 1, "cash_transactions": 1, "allocations": 1,
	}
	for table, expected := range expectedCounts {
		if actual, exists := tableCounts[table]; !exists || actual != expected {
			t.Fatalf("backup table %s rows=%d exists=%t, want %d", table, actual, exists, expected)
		}
	}
	if tableCounts["schema_migrations"] < 1 || tableCounts["audit_entries"] < 1 || tableCounts["party_create_commands"] < 2 {
		t.Fatalf("backup is missing migration, audit, or command facts: %#v", tableCounts)
	}
	return fileSHA256(t, archivePath)
}

func readZIPFile(t *testing.T, entry *zip.File, limit uint64) []byte {
	t.Helper()
	if entry.UncompressedSize64 > limit {
		t.Fatalf("ZIP entry %s exceeds %d bytes", entry.Name, limit)
	}
	opened, err := entry.Open()
	if err != nil {
		t.Fatalf("open ZIP entry %s: %v", entry.Name, err)
	}
	defer opened.Close()
	value, err := io.ReadAll(io.LimitReader(opened, int64(limit)+1))
	if err != nil {
		t.Fatalf("read ZIP entry %s: %v", entry.Name, err)
	}
	if uint64(len(value)) != entry.UncompressedSize64 {
		t.Fatalf("ZIP entry %s length = %d, want %d", entry.Name, len(value), entry.UncompressedSize64)
	}
	return value
}

func hashZIPFile(t *testing.T, entry *zip.File) string {
	t.Helper()
	opened, err := entry.Open()
	if err != nil {
		t.Fatalf("open ZIP entry %s: %v", entry.Name, err)
	}
	defer opened.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, opened); err != nil {
		t.Fatalf("hash ZIP entry %s: %v", entry.Name, err)
	}
	return hex.EncodeToString(digest.Sum(nil))
}

func mapKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func fileSHA256(t *testing.T, filePath string) string {
	t.Helper()
	file, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("open %s: %v", filePath, err)
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		t.Fatalf("hash %s: %v", filePath, err)
	}
	return hex.EncodeToString(digest.Sum(nil))
}

func copyFile(t *testing.T, sourcePath, targetPath string) {
	t.Helper()
	source, err := os.Open(sourcePath)
	if err != nil {
		t.Fatalf("open copy source: %v", err)
	}
	defer source.Close()
	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("create copy target: %v", err)
	}
	if _, err := io.Copy(target, source); err != nil {
		_ = target.Close()
		t.Fatalf("copy backup archive: %v", err)
	}
	if err := target.Close(); err != nil {
		t.Fatalf("close copied backup archive: %v", err)
	}
}

func tamperBackupDump(t *testing.T, sourcePath, targetPath string) {
	t.Helper()
	source, err := zip.OpenReader(sourcePath)
	if err != nil {
		t.Fatalf("open source backup for tamper: %v", err)
	}
	defer source.Close()
	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("create corrupt backup: %v", err)
	}
	writer := zip.NewWriter(target)
	foundDump := false
	for _, entry := range source.File {
		header := entry.FileHeader
		output, err := writer.CreateHeader(&header)
		if err != nil {
			t.Fatalf("create corrupt ZIP entry %s: %v", entry.Name, err)
		}
		input, err := entry.Open()
		if err != nil {
			t.Fatalf("open source ZIP entry %s: %v", entry.Name, err)
		}
		if entry.Name == "database.dump" {
			first := []byte{0}
			if _, err := io.ReadFull(input, first); err != nil {
				_ = input.Close()
				t.Fatalf("read database.dump for tamper: %v", err)
			}
			first[0] ^= 0xff
			if _, err := output.Write(first); err != nil {
				_ = input.Close()
				t.Fatalf("write corrupt database.dump byte: %v", err)
			}
			foundDump = true
		}
		_, copyErr := io.Copy(output, input)
		closeErr := input.Close()
		if copyErr != nil || closeErr != nil {
			t.Fatalf("copy corrupt ZIP entry %s: copy=%v close=%v", entry.Name, copyErr, closeErr)
		}
	}
	if !foundDump {
		t.Fatal("database.dump missing from source backup")
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close corrupt ZIP: %v", err)
	}
	if err := target.Close(); err != nil {
		t.Fatalf("close corrupt backup: %v", err)
	}
}

func assertOldSessionRejected(t *testing.T, client *http.Client, targetURL string) {
	t.Helper()
	response, err := client.Get(targetURL + "/")
	if err != nil {
		t.Fatalf("GET restored home with old session: %v", err)
	}
	defer closeResponse(t, response)
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != "/login?next=%2F" {
		t.Fatalf("old session status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
}

func assertPartyCount(t *testing.T, environment testdatabase.Environment, expected int) {
	t.Helper()
	var actual int
	if err := environment.Pool.QueryRow(context.Background(), `SELECT count(*) FROM parties`).Scan(&actual); err != nil {
		t.Fatalf("count parties: %v", err)
	}
	if actual != expected {
		t.Fatalf("parties = %d, want %d", actual, expected)
	}
}

func countUserRelations(t *testing.T, environment testdatabase.Environment) int64 {
	t.Helper()
	var count int64
	if err := environment.Pool.QueryRow(context.Background(), `
		SELECT count(*)
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname NOT IN ('pg_catalog', 'information_schema')
		  AND n.nspname !~ '^pg_toast'
		  AND c.relkind IN ('r', 'p', 'v', 'm', 'S', 'f')
	`).Scan(&count); err != nil {
		t.Fatalf("count user relations: %v", err)
	}
	return count
}

type buildIdentity struct {
	Service   string `json:"service"`
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuiltAt   string `json:"built_at"`
	GoVersion string `json:"go_version"`
	Target    string `json:"target"`
}

func assertBuildIdentity(t *testing.T, binaries ...string) {
	t.Helper()
	var first buildIdentity
	for index, binary := range binaries {
		output := runCommand(t, cleanEnvironment(), binary, "version")
		var current buildIdentity
		if err := json.Unmarshal(output, &current); err != nil {
			t.Fatalf("decode %s version: %v: %q", binary, err, output)
		}
		if current.Service != "money-flow-ledger" || current.Version == "" || current.Version == "dev" ||
			current.Commit == "" || current.Commit == "unknown" || current.BuiltAt == "" || current.BuiltAt == "unknown" ||
			current.GoVersion == "" || current.Target == "" {
			t.Fatalf("untraceable build identity for %s: %#v", binary, current)
		}
		if index == 0 {
			first = current
			continue
		}
		if current != first {
			t.Fatalf("build identity mismatch: %#v != %#v", current, first)
		}
	}
}

type opsCheck struct {
	PostgreSQL struct {
		Major int `json:"major"`
	} `json:"postgresql"`
	Schema struct {
		State       string `json:"state"`
		Current     string `json:"current"`
		Latest      string `json:"latest"`
		ServerReady bool   `json:"server_ready"`
	} `json:"schema"`
}

func runOpsCheck(t *testing.T, binary, databaseURL string, requireCurrent bool) opsCheck {
	t.Helper()
	arguments := []string{"check"}
	if requireCurrent {
		arguments = append(arguments, "--require-current")
	}
	output := runCommand(t, cleanEnvironment("DATABASE_URL="+databaseURL), binary, arguments...)
	var value opsCheck
	if err := json.Unmarshal(output, &value); err != nil {
		t.Fatalf("decode ops check: %v: %q", err, output)
	}
	return value
}

type runningServer struct {
	command *exec.Cmd
	output  bytes.Buffer
	done    chan error
	stopped bool
}

func startServer(t *testing.T, binary, databaseURL, address string, bootstrap bool) *runningServer {
	t.Helper()
	environment := cleanEnvironment(
		"DATABASE_URL="+databaseURL,
		"HTTP_ADDR="+address,
		"MIGRATION_MODE=verify",
		"SESSION_COOKIE_SECURE=false",
		"SESSION_TTL=30m",
		"SHUTDOWN_TIMEOUT=3s",
	)
	if bootstrap {
		environment = append(environment,
			"BOOTSTRAP_ADMIN_USERNAME="+bootstrapUsername,
			"BOOTSTRAP_ADMIN_DISPLAY_NAME=交付验收管理员",
			"BOOTSTRAP_ADMIN_PASSWORD="+bootstrapPassword,
		)
	}
	command := exec.Command(binary)
	server := &runningServer{command: command, done: make(chan error, 1)}
	command.Env = environment
	command.Stdout = &server.output
	command.Stderr = &server.output
	if err := command.Start(); err != nil {
		t.Fatalf("start release server: %v", err)
	}
	go func() { server.done <- command.Wait() }()
	t.Cleanup(func() {
		if !server.stopped {
			server.stopForCleanup(t)
		}
	})
	return server
}

func (s *runningServer) stop(t *testing.T) {
	t.Helper()
	if s.stopped {
		return
	}
	if err := s.command.Process.Signal(os.Interrupt); err != nil {
		t.Fatalf("interrupt release server: %v", err)
	}
	select {
	case err := <-s.done:
		s.stopped = true
		if err != nil {
			t.Fatalf("release server stopped with error: %v\n%s", err, s.output.String())
		}
	case <-time.After(5 * time.Second):
		_ = s.command.Process.Kill()
		<-s.done
		s.stopped = true
		t.Fatalf("release server did not stop gracefully\n%s", s.output.String())
	}
}

func (s *runningServer) stopForCleanup(t *testing.T) {
	t.Helper()
	if s.stopped {
		return
	}
	_ = s.command.Process.Signal(os.Interrupt)
	select {
	case <-s.done:
	case <-time.After(5 * time.Second):
		_ = s.command.Process.Kill()
		<-s.done
	}
	s.stopped = true
}

func waitReady(t *testing.T, server *runningServer, baseURL string) {
	t.Helper()
	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-server.done:
			server.stopped = true
			t.Fatalf("release server exited before ready: %v\n%s", err, server.output.String())
		default:
		}
		response, err := http.Get(baseURL + "/health/ready")
		if err == nil {
			body, readErr := io.ReadAll(response.Body)
			_ = response.Body.Close()
			if readErr == nil && response.StatusCode == http.StatusOK && strings.Contains(string(body), `"status":"ready"`) {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	server.stopForCleanup(t)
	t.Fatalf("release server did not become ready\n%s", server.output.String())
}

func login(t *testing.T, client *http.Client, baseURL, username, password string) {
	t.Helper()
	page := getPage(t, client, baseURL+"/login")
	response := postForm(t, client, baseURL+"/login", url.Values{
		"csrf_token": {hiddenValue(t, csrfPattern, page, "CSRF")},
		"next":       {"/"},
		"username":   {username},
		"password":   {password},
	})
	defer closeResponse(t, response)
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != "/" {
		t.Fatalf("login status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	home := getPage(t, client, baseURL+"/")
	assertContainsAll(t, "authenticated home", home, "交付验收管理员", "administrator", "注销")
}

func createParty(t *testing.T, client *http.Client, baseURL, kind, name, identifier string) string {
	t.Helper()
	page := getPage(t, client, baseURL+"/parties")
	response := postForm(t, client, baseURL+"/parties", url.Values{
		"csrf_token":              {hiddenValue(t, csrfPattern, page, "CSRF")},
		"command_id":              {hiddenValue(t, commandPattern, page, "command")},
		"kind":                    {kind},
		"legal_name":              {name},
		"registration_identifier": {identifier},
	})
	defer closeResponse(t, response)
	location := redirectLocation(t, response, "/parties")
	id := location.Query().Get("party_id")
	if location.Query().Get("created") != "1" || id == "" {
		t.Fatalf("party redirect = %q", location.String())
	}
	return id
}

func createAndConfirmObligation(t *testing.T, client *http.Client, baseURL, ownerID, counterpartyID string) string {
	t.Helper()
	page := getPage(t, client, baseURL+"/obligations")
	response := postForm(t, client, baseURL+"/obligations", url.Values{
		"csrf_token":              {hiddenValue(t, csrfPattern, page, "CSRF")},
		"command_id":              {hiddenValue(t, commandPattern, page, "command")},
		"direction":               {"receivable"},
		"ledger_owner_party_id":   {ownerID},
		"counterparty_party_id":   {counterpartyID},
		"title":                   {"交付验收应收"},
		"original_amount":         {"100.50"},
		"currency":                {"CNY"},
		"due_on":                  {"2099-10-31"},
		"manual_source_reference": {"M6-CLEAN-OBL-001"},
	})
	location := redirectLocation(t, response, "")
	closeResponse(t, response)
	if !strings.HasPrefix(location.Path, "/obligations/") || location.Query().Get("created") != "1" {
		t.Fatalf("obligation redirect = %q", location.String())
	}
	id := strings.TrimPrefix(location.Path, "/obligations/")
	detail := getPage(t, client, baseURL+location.RequestURI())
	response = postForm(t, client, baseURL+"/obligations/"+id+"/confirm", url.Values{
		"csrf_token":             {hiddenValue(t, csrfPattern, detail, "CSRF")},
		"command_id":             {hiddenValue(t, commandPattern, detail, "command")},
		"expected_draft_version": {"1"},
	})
	defer closeResponse(t, response)
	confirmation := redirectLocation(t, response, "/obligations/"+id)
	if confirmation.Query().Get("confirmed") != "1" {
		t.Fatalf("confirmation redirect = %q", confirmation.String())
	}
	return id
}

func recordCashExpectation(t *testing.T, client *http.Client, baseURL, obligationID string) {
	t.Helper()
	page := getPage(t, client, baseURL+"/cash-forecast")
	response := postForm(t, client, baseURL+"/cash-expectations", url.Values{
		"csrf_token":           {hiddenValue(t, csrfPattern, page, "CSRF")},
		"command_id":           {hiddenValue(t, commandPattern, page, "command")},
		"action":               {"set"},
		"obligation_selection": {obligationID + ":1"},
		"expected_on":          {"2099-10-20"},
		"expectation_reason":   {"交付验收预计收款"},
	})
	defer closeResponse(t, response)
	location := redirectLocation(t, response, "/cash-forecast")
	if location.Query().Get("recorded") != "1" {
		t.Fatalf("cash expectation redirect = %q", location.String())
	}
}

func createCashReceipt(t *testing.T, client *http.Client, baseURL, ownerID, counterpartyID string) string {
	t.Helper()
	page := getPage(t, client, baseURL+"/cash-transactions")
	response := postForm(t, client, baseURL+"/cash-transactions", url.Values{
		"csrf_token":                     {hiddenValue(t, csrfPattern, page, "CSRF")},
		"command_id":                     {hiddenValue(t, commandPattern, page, "command")},
		"type":                           {"receipt"},
		"ledger_owner_party_id":          {ownerID},
		"counterparty_party_id":          {counterpartyID},
		"description":                    {"交付验收实际收款"},
		"amount":                         {"40.25"},
		"currency":                       {"CNY"},
		"occurred_on":                    {"2026-09-20"},
		"account_reference":              {"交付验收基本户"},
		"manual_source_reference":        {"M6-CLEAN-CASH-001"},
		"external_transaction_reference": {"M6-CLEAN-BANK-001"},
	})
	defer closeResponse(t, response)
	location := redirectLocation(t, response, "")
	if !strings.HasPrefix(location.Path, "/cash-transactions/") || location.Query().Get("created") != "1" {
		t.Fatalf("cash redirect = %q", location.String())
	}
	return strings.TrimPrefix(location.Path, "/cash-transactions/")
}

func applyAllocation(t *testing.T, client *http.Client, baseURL, cashID, obligationID string) string {
	t.Helper()
	page := getPage(t, client, baseURL+"/allocations")
	response := postForm(t, client, baseURL+"/allocations", url.Values{
		"csrf_token":              {hiddenValue(t, csrfPattern, page, "CSRF")},
		"command_id":              {hiddenValue(t, commandPattern, page, "command")},
		"cash_transaction_id":     {cashID},
		"obligation_selection":    {obligationID + ":1"},
		"amount":                  {"40.25"},
		"manual_source_reference": {"M6-CLEAN-ALLOC-001"},
	})
	defer closeResponse(t, response)
	location := redirectLocation(t, response, "")
	if !strings.HasPrefix(location.Path, "/allocations/") || location.Query().Get("created") != "1" {
		t.Fatalf("allocation redirect = %q", location.String())
	}
	return strings.TrimPrefix(location.Path, "/allocations/")
}

func exportOwnerLedger(t *testing.T, client *http.Client, baseURL, ownerID string) []byte {
	t.Helper()
	page := getPage(t, client, baseURL+"/ledger-exports")
	response := postForm(t, client, baseURL+"/ledger-exports", url.Values{
		"csrf_token":            {hiddenValue(t, csrfPattern, page, "CSRF")},
		"ledger_owner_party_id": {ownerID},
	})
	defer closeResponse(t, response)
	content := readResponseBytes(t, response)
	if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "application/zip" ||
		response.Header.Get("Cache-Control") != "no-store" ||
		!strings.Contains(response.Header.Get("Content-Disposition"), "money-flow-ledger-"+ownerID+".zip") {
		t.Fatalf("export status=%d headers=%#v body=%q", response.StatusCode, response.Header, content)
	}
	return content
}

type exportManifest struct {
	FormatID            string `json:"format_id"`
	FormatVersion       int    `json:"format_version"`
	SourceSchemaVersion string `json:"source_schema_version"`
	LedgerOwner         struct {
		ID string `json:"id"`
	} `json:"ledger_owner"`
	Files []struct {
		Filename string `json:"filename"`
		Bytes    int    `json:"bytes"`
		SHA256   string `json:"sha256"`
	} `json:"files"`
}

func verifyOwnerExport(t *testing.T, archive []byte, ownerID, obligationID, cashID, allocationID string) {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatalf("open owner export: %v", err)
	}
	files := make(map[string][]byte, len(reader.File))
	for _, entry := range reader.File {
		if entry.FileInfo().IsDir() || path.Clean(entry.Name) != entry.Name || strings.Contains(entry.Name, "/") {
			t.Fatalf("invalid owner export entry %q", entry.Name)
		}
		opened, err := entry.Open()
		if err != nil {
			t.Fatalf("open owner export entry %s: %v", entry.Name, err)
		}
		content, err := io.ReadAll(opened)
		_ = opened.Close()
		if err != nil {
			t.Fatalf("read owner export entry %s: %v", entry.Name, err)
		}
		if _, duplicate := files[entry.Name]; duplicate {
			t.Fatalf("duplicate owner export entry %s", entry.Name)
		}
		files[entry.Name] = content
	}
	actualNames := make([]string, 0, len(files))
	for name := range files {
		actualNames = append(actualNames, name)
	}
	sort.Strings(actualNames)
	if strings.Join(actualNames, "\n") != strings.Join(exportFiles, "\n") {
		t.Fatalf("owner export files = %v, want %v", actualNames, exportFiles)
	}
	var manifest exportManifest
	if err := json.Unmarshal(files["manifest.json"], &manifest); err != nil {
		t.Fatalf("decode owner export manifest: %v", err)
	}
	if manifest.FormatID != exportFormatID || manifest.FormatVersion != 2 ||
		manifest.SourceSchemaVersion != latestSchema || manifest.LedgerOwner.ID != ownerID || len(manifest.Files) != 13 {
		t.Fatalf("owner export manifest = %#v", manifest)
	}
	for _, file := range manifest.Files {
		content, ok := files[file.Filename]
		if !ok {
			t.Fatalf("manifest references missing file %s", file.Filename)
		}
		digest := sha256.Sum256(content)
		if file.Bytes != len(content) || file.SHA256 != hex.EncodeToString(digest[:]) {
			t.Fatalf("manifest digest mismatch for %s", file.Filename)
		}
	}
	if !bytes.Contains(files["obligations.csv"], []byte(obligationID)) ||
		!bytes.Contains(files["obligation_cash_expectations.csv"], []byte("2099-10-20")) ||
		!bytes.Contains(files["cash_transactions.csv"], []byte(cashID)) ||
		!bytes.Contains(files["allocations.csv"], []byte(allocationID)) {
		t.Fatal("owner export is missing representative clean-room facts")
	}
}

func verifyDatabaseFacts(t *testing.T, environment testdatabase.Environment, obligationID, cashID, allocationID string) {
	t.Helper()
	var users, parties, obligations, revisions, expectations, cash, allocations int
	var outstanding string
	err := environment.Pool.QueryRow(context.Background(), `
		SELECT
			(SELECT count(*) FROM local_users),
			(SELECT count(*) FROM parties),
			(SELECT count(*) FROM obligations),
			(SELECT count(*) FROM obligation_revisions),
			(SELECT count(*) FROM obligation_cash_expectations),
			(SELECT count(*) FROM cash_transactions),
			(SELECT count(*) FROM allocations),
			(
				SELECT (revision.effective_amount
					- COALESCE((SELECT sum(amount) FROM allocations WHERE obligation_id = revision.obligation_id), 0)
					- COALESCE((SELECT sum(amount) FROM obligation_remissions WHERE obligation_id = revision.obligation_id), 0))::text
				FROM obligation_revisions revision
				WHERE revision.obligation_id = $1 AND revision.revision = 1
			)
	`, obligationID).Scan(&users, &parties, &obligations, &revisions, &expectations, &cash, &allocations, &outstanding)
	if err != nil {
		t.Fatalf("read clean-room facts: %v", err)
	}
	if users != 1 || parties != 2 || obligations != 1 || revisions != 1 || expectations != 1 || cash != 1 || allocations != 1 || outstanding != "60.250000" {
		t.Fatalf("clean-room counts users=%d parties=%d obligations=%d revisions=%d expectations=%d cash=%d allocations=%d outstanding=%s",
			users, parties, obligations, revisions, expectations, cash, allocations, outstanding)
	}
	var storedCashID, storedAllocationID string
	if err := environment.Pool.QueryRow(context.Background(), `
		SELECT cash.id::text, allocation.id::text
		FROM cash_transactions cash
		JOIN allocations allocation ON allocation.cash_transaction_id = cash.id
		WHERE cash.id = $1 AND allocation.id = $2
	`, cashID, allocationID).Scan(&storedCashID, &storedAllocationID); err != nil {
		t.Fatalf("read linked cash and allocation: %v", err)
	}
}

func requireAbsoluteFile(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Skip(name + " is not set")
	}
	if !filepath.IsAbs(value) {
		t.Fatalf("%s must be an absolute path", name)
	}
	info, err := os.Stat(value)
	if err != nil || !info.Mode().IsRegular() {
		t.Fatalf("%s is not a regular file: %v", name, err)
	}
	return value
}

func verifyChecksum(t *testing.T, archivePath, checksumPath string) {
	t.Helper()
	archive, err := os.Open(archivePath)
	if err != nil {
		t.Fatalf("open release archive: %v", err)
	}
	digest := sha256.New()
	if _, err := io.Copy(digest, archive); err != nil {
		_ = archive.Close()
		t.Fatalf("hash release archive: %v", err)
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close release archive: %v", err)
	}
	checksum, err := os.ReadFile(checksumPath)
	if err != nil {
		t.Fatalf("read release checksum: %v", err)
	}
	fields := strings.Fields(string(checksum))
	actual := hex.EncodeToString(digest.Sum(nil))
	if len(fields) != 2 || fields[0] != actual || fields[1] != filepath.Base(archivePath) {
		t.Fatalf("release checksum mismatch: %q", strings.TrimSpace(string(checksum)))
	}
}

func extractBundle(t *testing.T, archivePath string) string {
	t.Helper()
	archive, err := os.Open(archivePath)
	if err != nil {
		t.Fatalf("open release archive: %v", err)
	}
	defer archive.Close()
	gzipReader, err := gzip.NewReader(archive)
	if err != nil {
		t.Fatalf("open release gzip: %v", err)
	}
	defer gzipReader.Close()
	target := t.TempDir()
	tarReader := tar.NewReader(gzipReader)
	seen := make(map[string]struct{})
	root := ""
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read release tar: %v", err)
		}
		isDirectory := header.FileInfo().IsDir() || strings.HasSuffix(header.Name, "/")
		rawName := header.Name
		if isDirectory {
			rawName = strings.TrimSuffix(rawName, "/")
		}
		cleaned := path.Clean(rawName)
		if cleaned != rawName || path.IsAbs(cleaned) || cleaned == "." || strings.HasPrefix(cleaned, "../") {
			t.Fatalf("unsafe release path %q", header.Name)
		}
		parts := strings.Split(cleaned, "/")
		if root == "" {
			root = parts[0]
		}
		if parts[0] != root {
			t.Fatalf("release has multiple roots: %q and %q", root, parts[0])
		}
		destination := filepath.Join(target, filepath.FromSlash(cleaned))
		switch {
		case isDirectory:
			if err := os.MkdirAll(destination, 0o700); err != nil {
				t.Fatalf("create release directory: %v", err)
			}
		case header.Typeflag == tar.TypeReg || header.Typeflag == tar.TypeRegA:
			relative := strings.Join(parts[1:], "/")
			if relative == "" {
				t.Fatalf("release root is a regular file: name=%q type=%d mode=%s", header.Name, header.Typeflag, header.FileInfo().Mode())
			}
			if _, duplicate := seen[relative]; duplicate {
				t.Fatalf("duplicate release file %s", relative)
			}
			seen[relative] = struct{}{}
			if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
				t.Fatalf("create release parent: %v", err)
			}
			mode := os.FileMode(0o600)
			if strings.HasPrefix(relative, "bin/") {
				mode = 0o700
			}
			file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
			if err != nil {
				t.Fatalf("create release file %s: %v", relative, err)
			}
			_, copyErr := io.Copy(file, tarReader)
			closeErr := file.Close()
			if copyErr != nil || closeErr != nil {
				t.Fatalf("extract release file %s: copy=%v close=%v", relative, copyErr, closeErr)
			}
		default:
			t.Fatalf("unsupported release entry type %d for %s", header.Typeflag, header.Name)
		}
	}
	actual := make([]string, 0, len(seen))
	for name := range seen {
		actual = append(actual, name)
	}
	sort.Strings(actual)
	if strings.Join(actual, "\n") != strings.Join(bundleFiles, "\n") {
		t.Fatalf("release files = %v, want %v", actual, bundleFiles)
	}
	return filepath.Join(target, root)
}

func runCommand(t *testing.T, environment []string, binary string, arguments ...string) []byte {
	t.Helper()
	command := exec.Command(binary, arguments...)
	command.Env = environment
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("run %s %s: %v\n%s", binary, strings.Join(arguments, " "), err, output)
	}
	return output
}

func runCommandFails(t *testing.T, environment []string, expectedOutput, binary string, arguments ...string) []byte {
	t.Helper()
	command := exec.Command(binary, arguments...)
	command.Env = environment
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("run %s %s unexpectedly succeeded\n%s", binary, strings.Join(arguments, " "), output)
	}
	if !strings.Contains(string(output), expectedOutput) {
		t.Fatalf("run %s %s output does not contain %q: %v\n%s", binary, strings.Join(arguments, " "), expectedOutput, err, output)
	}
	return output
}

func cleanEnvironment(values ...string) []string {
	environment := []string{"PATH=" + os.Getenv("PATH"), "TMPDIR=" + os.TempDir(), "LANG=C.UTF-8"}
	return append(environment, values...)
}

func unusedAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve temporary address: %v", err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("release temporary address: %v", err)
	}
	return address
}

func newClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("create cookie jar: %v", err)
	}
	return &http.Client{
		Jar: jar,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: 10 * time.Second,
	}
}

func getPage(t *testing.T, client *http.Client, target string) string {
	t.Helper()
	response, err := client.Get(target)
	if err != nil {
		t.Fatalf("GET %s: %v", target, err)
	}
	defer closeResponse(t, response)
	body := readResponse(t, response)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status=%d body=%q", target, response.StatusCode, body)
	}
	return body
}

func postForm(t *testing.T, client *http.Client, target string, values url.Values) *http.Response {
	t.Helper()
	response, err := client.PostForm(target, values)
	if err != nil {
		t.Fatalf("POST %s: %v", target, err)
	}
	return response
}

func redirectLocation(t *testing.T, response *http.Response, expectedPath string) *url.URL {
	t.Helper()
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("redirect status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	location, err := url.Parse(response.Header.Get("Location"))
	if err != nil {
		t.Fatalf("parse redirect location: %v", err)
	}
	if expectedPath != "" && location.Path != expectedPath {
		t.Fatalf("redirect path=%q, want %q", location.Path, expectedPath)
	}
	return location
}

func hiddenValue(t *testing.T, pattern *regexp.Regexp, page, label string) string {
	t.Helper()
	match := pattern.FindStringSubmatch(page)
	if len(match) != 2 {
		t.Fatalf("%s field not found", label)
	}
	return match[1]
}

func readResponse(t *testing.T, response *http.Response) string {
	t.Helper()
	return string(readResponseBytes(t, response))
}

func readResponseBytes(t *testing.T, response *http.Response) []byte {
	t.Helper()
	content, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	return content
}

func closeResponse(t *testing.T, response *http.Response) {
	t.Helper()
	if err := response.Body.Close(); err != nil {
		t.Errorf("close response: %v", err)
	}
}

func assertContainsAll(t *testing.T, label, value string, expected ...string) {
	t.Helper()
	for _, item := range expected {
		if !strings.Contains(value, item) {
			t.Fatalf("%s does not contain %q", label, item)
		}
	}
}

func TestCleanInstallRequiresActualBundle(t *testing.T) {
	for _, name := range []string{"DELIVERY_BUNDLE_PATH", "DELIVERY_BUNDLE_SHA256_PATH"} {
		if value := os.Getenv(name); value != "" && !filepath.IsAbs(value) {
			t.Fatalf("%s must be absolute when provided", name)
		}
	}
}

func Example_acceptanceInput() {
	fmt.Println("DELIVERY_BUNDLE_PATH=/absolute/release.tar.gz")
	fmt.Println("DELIVERY_BUNDLE_SHA256_PATH=/absolute/release.tar.gz.sha256")
	// Output:
	// DELIVERY_BUNDLE_PATH=/absolute/release.tar.gz
	// DELIVERY_BUNDLE_SHA256_PATH=/absolute/release.tar.gz.sha256
}
