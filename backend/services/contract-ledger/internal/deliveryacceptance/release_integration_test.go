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
	"mime/multipart"
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

	"omsbizfoundry/contract-ledger/internal/testdatabase"
)

const (
	bootstrapUsername = "delivery-admin"
	bootstrapPassword = "delivery-admin-password"
	latestSchema      = "000016_empty_database_contract_restore.sql"
)

var (
	csrfPattern = regexp.MustCompile(`name="csrf_token" value="([^"]+)"`)
	bundleFiles = []string{
		"bin/contract-ledger-backup",
		"bin/contract-ledger-ops",
		"bin/contract-ledger-server",
		"config/contract-ledger.env.example",
		"docs/BACKUP-RESTORE.md",
		"docs/INSTALL-UPGRADE.md",
	}
	evidenceContent = []byte("OMSBizFoundry Contract clean-room evidence\n")
)

func TestContractReleaseCleanInstallBackupRestore(t *testing.T) {
	archivePath := requireAbsoluteFile(t, "DELIVERY_BUNDLE_PATH")
	checksumPath := requireAbsoluteFile(t, "DELIVERY_BUNDLE_SHA256_PATH")
	pgDumpBinary := requireAbsoluteFile(t, "DELIVERY_PG_DUMP_BIN")
	pgRestoreBinary := requireAbsoluteFile(t, "DELIVERY_PG_RESTORE_BIN")
	verifyChecksum(t, archivePath, checksumPath)
	bundleRoot := extractBundle(t, archivePath)

	serverBinary := filepath.Join(bundleRoot, "bin", "contract-ledger-server")
	opsBinary := filepath.Join(bundleRoot, "bin", "contract-ledger-ops")
	backupBinary := filepath.Join(bundleRoot, "bin", "contract-ledger-backup")
	assertBuildIdentity(t, serverBinary, opsBinary, backupBinary)

	baseDatabaseURL := testdatabase.RequireURL(t)
	source := testdatabase.New(t, baseDatabaseURL)
	sourceEvidenceRoot := filepath.Join(t.TempDir(), "source-evidence")
	check := runOpsCheck(t, opsBinary, source.URL, sourceEvidenceRoot, false)
	if check.PostgreSQL.Major != 18 || check.Schema.State != "empty" || check.Schema.ServerReady ||
		!check.Evidence.Writable || check.Evidence.Root != sourceEvidenceRoot {
		t.Fatalf("empty source check = %#v", check)
	}
	runCommand(t, runtimeEnvironment(source.URL, sourceEvidenceRoot), opsBinary, "migrate")
	check = runOpsCheck(t, opsBinary, source.URL, sourceEvidenceRoot, true)
	if check.Schema.State != "current" || check.Schema.Current != latestSchema ||
		check.Schema.Latest != latestSchema || !check.Schema.ServerReady {
		t.Fatalf("current source check = %#v", check)
	}

	sourceAddress := unusedAddress(t)
	sourceServer := startServer(t, serverBinary, source.URL, sourceEvidenceRoot, sourceAddress, true)
	sourceURL := "http://" + sourceAddress
	waitReady(t, sourceServer, sourceURL)
	sourceClient := newClient(t)
	login(t, sourceClient, sourceURL)

	createParty(t, sourceClient, sourceURL, "organization", "交付验收本方组织", "CL-DELIVERY-OWNER")
	createParty(t, sourceClient, sourceURL, "organization", "交付验收往来组织", "CL-DELIVERY-COUNTERPARTY")
	contractID := createContract(t, sourceClient, sourceURL)
	uploadEvidence(t, sourceClient, sourceURL, contractID)
	evidence := readEvidenceFact(t, source, contractID)
	verifyEvidenceFile(t, sourceEvidenceRoot, evidence)
	downloadEvidence(t, sourceClient, sourceURL, contractID, evidence)
	verifyDatabaseFacts(t, source, contractID, 2)

	backupPath := filepath.Join(t.TempDir(), "contract-source-backup.zip")
	backupEnvironment := cleanEnvironment(
		"DATABASE_URL="+source.URL,
		"POSTGRES_CLIENT_DATABASE_URL="+source.URL,
		"EVIDENCE_STORAGE_DIR="+sourceEvidenceRoot,
	)
	runCommand(t, backupEnvironment, backupBinary, "create",
		"--pg-dump-bin", pgDumpBinary,
		"--output", backupPath,
	)
	archiveDigest := verifyBackupArchive(t, backupPath, source.Name, evidence)

	handoffPath := filepath.Join(t.TempDir(), "contract-handoff-backup.zip")
	copyFile(t, backupPath, handoffPath)
	if handoffDigest := fileSHA256(t, handoffPath); handoffDigest != archiveDigest {
		t.Fatalf("handoff archive digest = %s, want %s", handoffDigest, archiveDigest)
	}
	sourceServer.stop(t)

	target := testdatabase.New(t, baseDatabaseURL)
	targetEvidenceRoot := filepath.Join(t.TempDir(), "target-evidence")
	restoreEnvironment := cleanEnvironment(
		"DATABASE_URL="+target.URL,
		"POSTGRES_CLIENT_DATABASE_URL="+target.URL,
		"EVIDENCE_STORAGE_DIR="+targetEvidenceRoot,
	)
	runCommand(t, restoreEnvironment, backupBinary, "restore",
		"--pg-restore-bin", pgRestoreBinary,
		"--input", handoffPath,
	)
	check = runOpsCheck(t, opsBinary, target.URL, targetEvidenceRoot, true)
	if check.Schema.State != "current" || check.Schema.Current != latestSchema || !check.Schema.ServerReady {
		t.Fatalf("restored target check = %#v", check)
	}
	assertRestoredSessionAndMarkerState(t, target, targetEvidenceRoot)
	verifyEvidenceFile(t, targetEvidenceRoot, evidence)

	targetAddress := unusedAddress(t)
	targetServer := startServer(t, serverBinary, target.URL, targetEvidenceRoot, targetAddress, false)
	targetURL := "http://" + targetAddress
	waitReady(t, targetServer, targetURL)
	assertOldSessionRejected(t, sourceClient, targetURL)
	restoredClient := newClient(t)
	login(t, restoredClient, targetURL)
	detail := getPage(t, restoredClient, targetURL+"/contracts/"+contractID)
	assertContainsAll(t, "restored contract detail", detail,
		"交付验收合同", "1000 CNY", "交付验收文件证据", "delivery-proof.txt")
	downloadEvidence(t, restoredClient, targetURL, contractID, evidence)
	verifyDatabaseFacts(t, target, contractID, 2)
	createParty(t, restoredClient, targetURL, "individual", "恢复后新增主体", "CL-DELIVERY-RESTORED")
	assertPartyCount(t, target, 3)
	assertPartyCount(t, source, 2)
	targetServer.stop(t)

	nonemptyTarget := testdatabase.New(t, baseDatabaseURL)
	nonemptyMigrationEvidence := filepath.Join(t.TempDir(), "nonempty-migration-evidence")
	runCommand(t, runtimeEnvironment(nonemptyTarget.URL, nonemptyMigrationEvidence), opsBinary, "migrate")
	nonemptyRestoreEvidence := filepath.Join(t.TempDir(), "nonempty-restore-evidence")
	runCommandFails(t, cleanEnvironment(
		"DATABASE_URL="+nonemptyTarget.URL,
		"POSTGRES_CLIENT_DATABASE_URL="+nonemptyTarget.URL,
		"EVIDENCE_STORAGE_DIR="+nonemptyRestoreEvidence,
	), "restore target database is not empty", backupBinary, "restore",
		"--pg-restore-bin", pgRestoreBinary,
		"--input", handoffPath,
	)
	if _, err := os.Lstat(nonemptyRestoreEvidence); !os.IsNotExist(err) {
		t.Fatalf("nonempty rejection created evidence root: %v", err)
	}
	nonemptyCheck := runOpsCheck(t, opsBinary, nonemptyTarget.URL, nonemptyMigrationEvidence, true)
	if nonemptyCheck.Schema.State != "current" || nonemptyCheck.Schema.Current != latestSchema {
		t.Fatalf("nonempty rejection changed schema = %#v", nonemptyCheck)
	}

	existingEvidenceTarget := testdatabase.New(t, baseDatabaseURL)
	existingEvidenceRoot := filepath.Join(t.TempDir(), "existing-evidence")
	if err := os.Mkdir(existingEvidenceRoot, 0o700); err != nil {
		t.Fatalf("create existing evidence root: %v", err)
	}
	sentinelPath := filepath.Join(existingEvidenceRoot, "sentinel")
	if err := os.WriteFile(sentinelPath, []byte("preserve"), 0o600); err != nil {
		t.Fatalf("write existing evidence sentinel: %v", err)
	}
	runCommandFails(t, cleanEnvironment(
		"DATABASE_URL="+existingEvidenceTarget.URL,
		"POSTGRES_CLIENT_DATABASE_URL="+existingEvidenceTarget.URL,
		"EVIDENCE_STORAGE_DIR="+existingEvidenceRoot,
	), "target evidence root already exists", backupBinary, "restore",
		"--pg-restore-bin", pgRestoreBinary,
		"--input", handoffPath,
	)
	if relations := countUserRelations(t, existingEvidenceTarget); relations != 0 {
		t.Fatalf("existing-evidence rejection target relations = %d, want 0", relations)
	}
	if value, err := os.ReadFile(sentinelPath); err != nil || string(value) != "preserve" {
		t.Fatalf("existing evidence sentinel = %q error=%v", value, err)
	}

	corruptPath := filepath.Join(t.TempDir(), "contract-corrupt-evidence-backup.zip")
	tamperEvidenceEntry(t, handoffPath, corruptPath, evidence.ArchivePath)
	corruptTarget := testdatabase.New(t, baseDatabaseURL)
	corruptEvidenceRoot := filepath.Join(t.TempDir(), "corrupt-target-evidence")
	runCommandFails(t, cleanEnvironment(
		"DATABASE_URL="+corruptTarget.URL,
		"POSTGRES_CLIENT_DATABASE_URL="+corruptTarget.URL,
		"EVIDENCE_STORAGE_DIR="+corruptEvidenceRoot,
	), "integrity mismatch", backupBinary, "restore",
		"--pg-restore-bin", pgRestoreBinary,
		"--input", corruptPath,
	)
	if relations := countUserRelations(t, corruptTarget); relations != 0 {
		t.Fatalf("corrupt restore target relations = %d, want 0", relations)
	}
	if _, err := os.Lstat(corruptEvidenceRoot); !os.IsNotExist(err) {
		t.Fatalf("corrupt restore published evidence root: %v", err)
	}
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
		if current.Service != "contract-ledger" || current.Version == "" || current.Version == "dev" ||
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
	Evidence struct {
		Root     string `json:"root"`
		Writable bool   `json:"writable"`
	} `json:"evidence"`
}

func runOpsCheck(t *testing.T, binary, databaseURL, evidenceRoot string, requireCurrent bool) opsCheck {
	t.Helper()
	arguments := []string{"check"}
	if requireCurrent {
		arguments = append(arguments, "--require-current")
	}
	output := runCommand(t, runtimeEnvironment(databaseURL, evidenceRoot), binary, arguments...)
	var value opsCheck
	if err := json.Unmarshal(output, &value); err != nil {
		t.Fatalf("decode ops check: %v: %q", err, output)
	}
	return value
}

func runtimeEnvironment(databaseURL, evidenceRoot string) []string {
	return cleanEnvironment("DATABASE_URL="+databaseURL, "EVIDENCE_STORAGE_DIR="+evidenceRoot)
}

type runningServer struct {
	command *exec.Cmd
	output  bytes.Buffer
	done    chan error
	stopped bool
}

func startServer(t *testing.T, binary, databaseURL, evidenceRoot, address string, bootstrap bool) *runningServer {
	t.Helper()
	environment := cleanEnvironment(
		"DATABASE_URL="+databaseURL,
		"EVIDENCE_STORAGE_DIR="+evidenceRoot,
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
			server.stopForCleanup()
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

func (s *runningServer) stopForCleanup() {
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
	server.stopForCleanup()
	t.Fatalf("release server did not become ready\n%s", server.output.String())
}

func login(t *testing.T, client *http.Client, baseURL string) {
	t.Helper()
	page := getPage(t, client, baseURL+"/login")
	response := postForm(t, client, baseURL+"/login", url.Values{
		"csrf_token": {hiddenValue(t, csrfPattern, page, "CSRF")},
		"username":   {bootstrapUsername},
		"password":   {bootstrapPassword},
		"next":       {"/"},
	})
	defer closeResponse(t, response)
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != "/" {
		t.Fatalf("login status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	home := getPage(t, client, baseURL+"/")
	assertContainsAll(t, "authenticated home", home, "交付验收管理员", "administrator", "注销")
}

func createParty(t *testing.T, client *http.Client, baseURL, kind, name, identifier string) {
	t.Helper()
	page := getPage(t, client, baseURL+"/parties")
	response := postForm(t, client, baseURL+"/parties", url.Values{
		"csrf_token":              {hiddenValue(t, csrfPattern, page, "CSRF")},
		"kind":                    {kind},
		"legal_name":              {name},
		"registration_identifier": {identifier},
	})
	defer closeResponse(t, response)
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != "/parties?created=1" {
		t.Fatalf("create party status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
}

func createContract(t *testing.T, client *http.Client, baseURL string) string {
	t.Helper()
	page := getPage(t, client, baseURL+"/contracts")
	response := postForm(t, client, baseURL+"/contracts", url.Values{
		"csrf_token":       {formHiddenValue(t, page, "/contracts", "csrf_token")},
		"command_id":       {formHiddenValue(t, page, "/contracts", "command_id")},
		"number_namespace": {"delivery"},
		"contract_number":  {"CL-M6-003"},
		"title":            {"交付验收合同"},
		"category":         {"通用验收"},
		"currency":         {"CNY"},
		"total_amount":     {"1000.00"},
		"signed_on":        {"2026-09-20"},
		"effective_on":     {"2026-09-21"},
	})
	defer closeResponse(t, response)
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("create contract status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	location, err := url.Parse(response.Header.Get("Location"))
	if err != nil || location.Path != "/contracts" || location.Query().Get("created") != "1" || location.Query().Get("contract_id") == "" {
		t.Fatalf("create contract redirect = %q error=%v", response.Header.Get("Location"), err)
	}
	return location.Query().Get("contract_id")
}

func uploadEvidence(t *testing.T, client *http.Client, baseURL, contractID string) {
	t.Helper()
	contractPath := "/contracts/" + contractID
	evidencePath := contractPath + "/evidence"
	page := getPage(t, client, baseURL+contractPath)
	response := postMultipart(t, client, baseURL+evidencePath, url.Values{
		"csrf_token":       {formHiddenValue(t, page, evidencePath, "csrf_token")},
		"command_id":       {formHiddenValue(t, page, evidencePath, "command_id")},
		"evidence_target":  {"contract:" + contractID},
		"evidence_title":   {"交付验收文件证据"},
		"evidence_purpose": {"验证实际发布包的文件持久化、备份和恢复"},
	}, "evidence_file", "delivery-proof.txt", evidenceContent)
	defer closeResponse(t, response)
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != contractPath+"?evidence_uploaded=1" {
		t.Fatalf("upload evidence status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
}

type evidenceFact struct {
	DocumentID  string
	VersionID   string
	StorageKey  string
	ArchivePath string
	SHA256      string
	SizeBytes   int64
}

func readEvidenceFact(t *testing.T, environment testdatabase.Environment, contractID string) evidenceFact {
	t.Helper()
	var value evidenceFact
	err := environment.Pool.QueryRow(context.Background(), `
		SELECT d.id::text, v.id::text, v.storage_key, v.sha256, v.size_bytes
		FROM contract_evidence_documents d
		JOIN contract_evidence_versions v ON v.document_id = d.id
		WHERE d.contract_id = $1 AND d.target_type = 'contract' AND d.target_id = $1
	`, contractID).Scan(&value.DocumentID, &value.VersionID, &value.StorageKey, &value.SHA256, &value.SizeBytes)
	if err != nil {
		t.Fatalf("read evidence fact: %v", err)
	}
	value.ArchivePath = path.Join("evidence", "objects", value.StorageKey[:2], value.StorageKey)
	expectedDigest := sha256.Sum256(evidenceContent)
	if value.SizeBytes != int64(len(evidenceContent)) || value.SHA256 != hex.EncodeToString(expectedDigest[:]) {
		t.Fatalf("evidence metadata = %#v", value)
	}
	return value
}

func verifyEvidenceFile(t *testing.T, evidenceRoot string, evidence evidenceFact) {
	t.Helper()
	objectPath := filepath.Join(evidenceRoot, "objects", evidence.StorageKey[:2], evidence.StorageKey)
	info, err := os.Stat(objectPath)
	if err != nil {
		t.Fatalf("stat evidence object: %v", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || info.Size() != evidence.SizeBytes {
		t.Fatalf("evidence object mode=%s size=%d", info.Mode(), info.Size())
	}
	value, err := os.ReadFile(objectPath)
	if err != nil || !bytes.Equal(value, evidenceContent) {
		t.Fatalf("evidence object bytes=%q error=%v", value, err)
	}
}

func downloadEvidence(t *testing.T, client *http.Client, baseURL, contractID string, evidence evidenceFact) {
	t.Helper()
	target := fmt.Sprintf("%s/contracts/%s/evidence/%s/versions/%s/download", baseURL, contractID, evidence.DocumentID, evidence.VersionID)
	response, err := client.Get(target)
	if err != nil {
		t.Fatalf("download evidence: %v", err)
	}
	defer closeResponse(t, response)
	content := readResponseBytes(t, response)
	if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "application/octet-stream" ||
		!strings.Contains(response.Header.Get("Cache-Control"), "no-store") ||
		!strings.Contains(response.Header.Get("Content-Disposition"), "delivery-proof.txt") ||
		!bytes.Equal(content, evidenceContent) {
		t.Fatalf("evidence download status=%d headers=%#v body=%q", response.StatusCode, response.Header, content)
	}
}

func verifyDatabaseFacts(t *testing.T, environment testdatabase.Environment, contractID string, expectedParties int) {
	t.Helper()
	var users, parties, contracts, evidenceDocuments, evidenceVersions, evidenceCommands int
	var status, amount string
	err := environment.Pool.QueryRow(context.Background(), `
		SELECT
			(SELECT count(*) FROM local_users),
			(SELECT count(*) FROM parties),
			(SELECT count(*) FROM contracts),
			(SELECT count(*) FROM contract_evidence_documents),
			(SELECT count(*) FROM contract_evidence_versions WHERE content_status = 'available'),
			(SELECT count(*) FROM contract_evidence_commands),
			status,
			total_amount::text
		FROM contracts
		WHERE id = $1
	`, contractID).Scan(&users, &parties, &contracts, &evidenceDocuments, &evidenceVersions, &evidenceCommands, &status, &amount)
	if err != nil {
		t.Fatalf("read contract delivery facts: %v", err)
	}
	if users != 1 || parties != expectedParties || contracts != 1 || evidenceDocuments != 1 ||
		evidenceVersions != 1 || evidenceCommands != 1 || status != "draft" || amount != "1000.000000" {
		t.Fatalf("delivery facts users=%d parties=%d contracts=%d evidence=%d/%d commands=%d status=%s amount=%s",
			users, parties, contracts, evidenceDocuments, evidenceVersions, evidenceCommands, status, amount)
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
		Counts            struct {
			Contracts                 int64 `json:"contracts"`
			Parties                   int64 `json:"parties"`
			EvidenceVersions          int64 `json:"evidence_versions"`
			AvailableEvidenceVersions int64 `json:"available_evidence_versions"`
			MissingEvidenceVersions   int64 `json:"missing_evidence_versions"`
		} `json:"counts"`
	} `json:"database"`
	Evidence struct {
		Layout  string `json:"layout"`
		Objects []struct {
			StorageKey string `json:"storage_key"`
			Path       string `json:"path"`
			SizeBytes  int64  `json:"size_bytes"`
			SHA256     string `json:"sha256"`
		} `json:"objects"`
	} `json:"evidence"`
}

func verifyBackupArchive(t *testing.T, archivePath, sourceDatabaseName string, evidence evidenceFact) string {
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
	if len(reader.File) != 3 {
		t.Fatalf("backup ZIP entries = %d, want 3", len(reader.File))
	}
	entries := make(map[string]*zip.File, len(reader.File))
	for _, entry := range reader.File {
		if _, duplicate := entries[entry.Name]; duplicate {
			t.Fatalf("duplicate backup ZIP entry %q", entry.Name)
		}
		if entry.Method != zip.Store || !entry.Mode().IsRegular() || entry.Mode().Perm() != 0o600 ||
			path.Clean(entry.Name) != entry.Name || strings.HasPrefix(entry.Name, "../") {
			t.Fatalf("invalid backup ZIP entry %q method=%d mode=%s", entry.Name, entry.Method, entry.Mode())
		}
		entries[entry.Name] = entry
	}
	manifestEntry, manifestOK := entries["manifest.json"]
	dumpEntry, dumpOK := entries["database.dump"]
	evidenceEntry, evidenceOK := entries[evidence.ArchivePath]
	if !manifestOK || !dumpOK || !evidenceOK {
		t.Fatalf("backup ZIP entries = %v", mapKeys(entries))
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
	counts := manifest.Database.Counts
	if manifest.Format != "omsbizfoundry.contract-ledger/full-backup-v1" || manifest.Version != 1 ||
		manifest.Database.Name != sourceDatabaseName || !strings.HasPrefix(manifest.Database.PostgreSQLVersion, "18.") ||
		manifest.Database.SchemaVersion != latestSchema || manifest.Database.DumpPath != "database.dump" ||
		!manifest.Database.SessionsExcluded || counts.Contracts != 1 || counts.Parties != 2 ||
		counts.EvidenceVersions != 1 || counts.AvailableEvidenceVersions != 1 || counts.MissingEvidenceVersions != 0 ||
		manifest.Evidence.Layout != "local-store-v1" || len(manifest.Evidence.Objects) != 1 {
		t.Fatalf("backup manifest identity = %#v", manifest)
	}
	if manifest.Database.DumpBytes != int64(dumpEntry.UncompressedSize64) ||
		manifest.Database.DumpSHA256 != hashZIPFile(t, dumpEntry) {
		t.Fatalf("backup dump metadata bytes=%d/%d sha=%s", manifest.Database.DumpBytes, dumpEntry.UncompressedSize64, manifest.Database.DumpSHA256)
	}
	object := manifest.Evidence.Objects[0]
	if object.StorageKey != evidence.StorageKey || object.Path != evidence.ArchivePath ||
		object.SizeBytes != evidence.SizeBytes || object.SHA256 != evidence.SHA256 ||
		object.SizeBytes != int64(evidenceEntry.UncompressedSize64) || hashZIPFile(t, evidenceEntry) != evidence.SHA256 ||
		!bytes.Equal(readZIPFile(t, evidenceEntry, 1<<20), evidenceContent) {
		t.Fatalf("backup evidence object = %#v", object)
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

func tamperEvidenceEntry(t *testing.T, sourcePath, targetPath, evidencePath string) {
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
	found := false
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
		if entry.Name == evidencePath {
			first := []byte{0}
			if _, err := io.ReadFull(input, first); err != nil {
				_ = input.Close()
				t.Fatalf("read evidence entry for tamper: %v", err)
			}
			first[0] ^= 0xff
			if _, err := output.Write(first); err != nil {
				_ = input.Close()
				t.Fatalf("write corrupt evidence byte: %v", err)
			}
			found = true
		}
		_, copyErr := io.Copy(output, input)
		closeErr := input.Close()
		if copyErr != nil || closeErr != nil {
			t.Fatalf("copy corrupt ZIP entry %s: copy=%v close=%v", entry.Name, copyErr, closeErr)
		}
	}
	if !found {
		t.Fatalf("evidence entry %s missing from source backup", evidencePath)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close corrupt ZIP: %v", err)
	}
	if err := target.Close(); err != nil {
		t.Fatalf("close corrupt backup: %v", err)
	}
}

func assertRestoredSessionAndMarkerState(t *testing.T, environment testdatabase.Environment, evidenceRoot string) {
	t.Helper()
	var sessions int
	if err := environment.Pool.QueryRow(context.Background(), `SELECT count(*) FROM local_sessions`).Scan(&sessions); err != nil {
		t.Fatalf("count restored sessions: %v", err)
	}
	if sessions != 0 {
		t.Fatalf("restored sessions = %d, want 0", sessions)
	}
	marker := filepath.Join(evidenceRoot, ".contract-ledger-restore-in-progress")
	if _, err := os.Lstat(marker); !os.IsNotExist(err) {
		t.Fatalf("restored evidence marker remains: %v", err)
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
	actual := fileSHA256(t, archivePath)
	checksum, err := os.ReadFile(checksumPath)
	if err != nil {
		t.Fatalf("read release checksum: %v", err)
	}
	fields := strings.Fields(string(checksum))
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
				t.Fatalf("release root is a regular file: %q", header.Name)
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
	actual := mapKeys(seen)
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

func postMultipart(t *testing.T, client *http.Client, target string, values url.Values, fileField, filename string, content []byte) *http.Response {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for name, fieldValues := range values {
		for _, value := range fieldValues {
			if err := writer.WriteField(name, value); err != nil {
				t.Fatalf("write multipart field %q: %v", name, err)
			}
		}
	}
	part, err := writer.CreateFormFile(fileField, filename)
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	request, err := http.NewRequest(http.MethodPost, target, &body)
	if err != nil {
		t.Fatalf("create multipart request: %v", err)
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("POST multipart %s: %v", target, err)
	}
	return response
}

func hiddenValue(t *testing.T, pattern *regexp.Regexp, page, label string) string {
	t.Helper()
	match := pattern.FindStringSubmatch(page)
	if len(match) != 2 {
		t.Fatalf("%s field not found", label)
	}
	return match[1]
}

func formHiddenValue(t *testing.T, body, action, name string) string {
	t.Helper()
	formPattern := regexp.MustCompile(`(?s)<form[^>]*action="` + regexp.QuoteMeta(action) + `"[^>]*>(.*?)</form>`)
	formMatch := formPattern.FindStringSubmatch(body)
	if len(formMatch) != 2 {
		t.Fatalf("page missing form action %q", action)
	}
	inputPattern := regexp.MustCompile(`name="` + regexp.QuoteMeta(name) + `" value="([^"]*)"`)
	inputMatch := inputPattern.FindStringSubmatch(formMatch[1])
	if len(inputMatch) != 2 {
		t.Fatalf("form %q missing input %q", action, name)
	}
	return inputMatch[1]
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

func TestContractAcceptanceRequiresActualBundle(t *testing.T) {
	for _, name := range []string{
		"DELIVERY_BUNDLE_PATH", "DELIVERY_BUNDLE_SHA256_PATH", "DELIVERY_PG_DUMP_BIN", "DELIVERY_PG_RESTORE_BIN",
	} {
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
