package combineddelivery_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
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

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	bootstrapUsername = "combined-admin"
	bootstrapPassword = "combined-delivery-password"
)

var (
	csrfPattern    = regexp.MustCompile(`name="csrf_token" value="([^"]+)"`)
	commandPattern = regexp.MustCompile(`name="command_id" value="([^"]+)"`)
)

type bundleSpec struct {
	archiveEnv  string
	checksumEnv string
	service     string
	files       []string
	serverPath  string
	opsPath     string
	backupPath  string
}

var contractBundle = bundleSpec{
	archiveEnv: "CONTRACT_DELIVERY_BUNDLE_PATH", checksumEnv: "CONTRACT_DELIVERY_BUNDLE_SHA256_PATH",
	service: "contract-ledger",
	files: []string{
		"bin/contract-ledger-backup", "bin/contract-ledger-ops", "bin/contract-ledger-server",
		"config/contract-ledger.env.example", "docs/BACKUP-RESTORE.md", "docs/INSTALL-UPGRADE.md",
	},
	serverPath: "bin/contract-ledger-server", opsPath: "bin/contract-ledger-ops", backupPath: "bin/contract-ledger-backup",
}

var moneyBundle = bundleSpec{
	archiveEnv: "MONEY_FLOW_DELIVERY_BUNDLE_PATH", checksumEnv: "MONEY_FLOW_DELIVERY_BUNDLE_SHA256_PATH",
	service: "money-flow-ledger",
	files: []string{
		"bin/money-flow-ledger-backup", "bin/money-flow-ledger-ops", "bin/money-flow-ledger-server",
		"config/money-flow-ledger.env.example", "docs/BACKUP-RESTORE.md", "docs/INSTALL-UPGRADE.md",
	},
	serverPath: "bin/money-flow-ledger-server", opsPath: "bin/money-flow-ledger-ops", backupPath: "bin/money-flow-ledger-backup",
}

func TestActualBundlesCoexistAndIsolateSingleSideFailure(t *testing.T) {
	inputs := requireAcceptanceInputs(t)
	contractRoot := verifyAndExtractBundle(t, contractBundle, inputs[contractBundle.archiveEnv], inputs[contractBundle.checksumEnv])
	moneyRoot := verifyAndExtractBundle(t, moneyBundle, inputs[moneyBundle.archiveEnv], inputs[moneyBundle.checksumEnv])

	contractServerBinary := filepath.Join(contractRoot, contractBundle.serverPath)
	contractOpsBinary := filepath.Join(contractRoot, contractBundle.opsPath)
	contractBackupBinary := filepath.Join(contractRoot, contractBundle.backupPath)
	moneyServerBinary := filepath.Join(moneyRoot, moneyBundle.serverPath)
	moneyOpsBinary := filepath.Join(moneyRoot, moneyBundle.opsPath)
	moneyBackupBinary := filepath.Join(moneyRoot, moneyBundle.backupPath)
	assertBuildIdentity(t, contractBundle.service, contractServerBinary, contractOpsBinary, contractBackupBinary)
	assertBuildIdentity(t, moneyBundle.service, moneyServerBinary, moneyOpsBinary, moneyBackupBinary)

	contractDatabase := newDatabase(t, inputs["CONTRACT_TEST_DATABASE_URL"], "combined_contract_test_")
	moneyDatabase := newDatabase(t, inputs["MONEY_FLOW_TEST_DATABASE_URL"], "combined_money_test_")
	if contractDatabase.Name == moneyDatabase.Name || contractDatabase.URL == moneyDatabase.URL {
		t.Fatalf("combined databases are not distinct: contract=%s money=%s", contractDatabase.Name, moneyDatabase.Name)
	}
	contractEvidenceRoot := filepath.Join(t.TempDir(), "contract-evidence")
	moneyEvidenceTrap := filepath.Join(t.TempDir(), "money-evidence-must-not-exist")

	runCommand(t, contractRuntimeEnvironment(contractDatabase.URL, contractEvidenceRoot), contractOpsBinary, "migrate")
	runCommand(t, moneyRuntimeEnvironment(moneyDatabase.URL), moneyOpsBinary, "migrate")
	assertSchemaCurrent(t, contractOpsBinary, contractRuntimeEnvironment(contractDatabase.URL, contractEvidenceRoot))
	assertSchemaCurrent(t, moneyOpsBinary, moneyRuntimeEnvironment(moneyDatabase.URL))

	contractAddress := unusedAddress(t)
	moneyAddress := unusedAddress(t)
	if contractAddress == moneyAddress {
		t.Fatalf("combined server addresses collide: %s", contractAddress)
	}
	contractURL := "http://" + contractAddress
	moneyURL := "http://" + moneyAddress
	contractServer := startServer(t, contractServerBinary, contractServerEnvironment(contractDatabase.URL, contractEvidenceRoot, contractAddress, true))
	moneyServer := startServer(t, moneyServerBinary, moneyServerEnvironment(moneyDatabase.URL, moneyAddress, true))
	waitReady(t, contractServer, contractURL, "contract-ledger")
	waitReady(t, moneyServer, moneyURL, "money-flow-ledger")

	browser, jar := newBrowser(t)
	login(t, browser, contractURL, "组合验收合同管理员")
	login(t, browser, moneyURL, "组合验收资金管理员")
	assertAuthenticated(t, browser, contractURL, "组合验收合同管理员")
	assertAuthenticated(t, browser, moneyURL, "组合验收资金管理员")
	assertApplicationCookies(t, jar, contractURL)

	createContractParty(t, browser, contractURL, "组合合同主体一", "COMBINED-CONTRACT-1")
	createMoneyParty(t, browser, moneyURL, "组合资金主体一", "COMBINED-MONEY-1")
	assertPartyCount(t, contractDatabase, 1)
	assertPartyCount(t, moneyDatabase, 1)
	assertDatabaseBoundary(t, contractDatabase, moneyDatabase)
	assertEvidenceBoundary(t, contractEvidenceRoot, moneyEvidenceTrap)

	contractServer.stop(t)
	assertReadyHTTP(t, moneyURL, "money-flow-ledger")
	assertAuthenticated(t, browser, moneyURL, "组合验收资金管理员")
	createMoneyParty(t, browser, moneyURL, "合同停机期间资金主体", "COMBINED-MONEY-2")
	assertPartyCount(t, moneyDatabase, 2)
	assertPartyCount(t, contractDatabase, 1)

	restartedContract := startServer(t, contractServerBinary, contractServerEnvironment(contractDatabase.URL, contractEvidenceRoot, contractAddress, false))
	waitReady(t, restartedContract, contractURL, "contract-ledger")
	assertAuthenticated(t, browser, contractURL, "组合验收合同管理员")
	createContractParty(t, browser, contractURL, "合同重启后主体", "COMBINED-CONTRACT-2")
	assertPartyCount(t, contractDatabase, 2)

	moneyServer.stop(t)
	assertReadyHTTP(t, contractURL, "contract-ledger")
	assertAuthenticated(t, browser, contractURL, "组合验收合同管理员")
	createContractParty(t, browser, contractURL, "资金停机期间合同主体", "COMBINED-CONTRACT-3")
	assertPartyCount(t, contractDatabase, 3)
	assertPartyCount(t, moneyDatabase, 2)

	restartedMoney := startServer(t, moneyServerBinary, moneyServerEnvironment(moneyDatabase.URL, moneyAddress, false))
	waitReady(t, restartedMoney, moneyURL, "money-flow-ledger")
	assertAuthenticated(t, browser, moneyURL, "组合验收资金管理员")
	createMoneyParty(t, browser, moneyURL, "资金重启后主体", "COMBINED-MONEY-3")
	assertPartyCount(t, moneyDatabase, 3)
	assertDatabaseBoundary(t, contractDatabase, moneyDatabase)
	assertEvidenceBoundary(t, contractEvidenceRoot, moneyEvidenceTrap)
	assertApplicationCookies(t, jar, moneyURL)

	restartedContract.stop(t)
	assertReadyHTTP(t, moneyURL, "money-flow-ledger")
	restartedMoney.stop(t)
}

func requireAcceptanceInputs(t *testing.T) map[string]string {
	t.Helper()
	names := []string{
		contractBundle.archiveEnv, contractBundle.checksumEnv,
		moneyBundle.archiveEnv, moneyBundle.checksumEnv,
		"CONTRACT_TEST_DATABASE_URL", "MONEY_FLOW_TEST_DATABASE_URL",
	}
	values := make(map[string]string, len(names))
	present := 0
	for _, name := range names {
		values[name] = strings.TrimSpace(os.Getenv(name))
		if values[name] != "" {
			present++
		}
	}
	if present == 0 {
		t.Skip("combined actual-bundle acceptance inputs are not set")
	}
	for _, name := range names {
		if values[name] == "" {
			t.Fatalf("%s is required when combined acceptance is enabled", name)
		}
	}
	for _, name := range []string{
		contractBundle.archiveEnv, contractBundle.checksumEnv,
		moneyBundle.archiveEnv, moneyBundle.checksumEnv,
	} {
		if !filepath.IsAbs(values[name]) {
			t.Fatalf("%s must be an absolute path", name)
		}
		info, err := os.Stat(values[name])
		if err != nil || !info.Mode().IsRegular() {
			t.Fatalf("%s is not a regular file: %v", name, err)
		}
	}
	return values
}

func verifyAndExtractBundle(t *testing.T, spec bundleSpec, archivePath, checksumPath string) string {
	t.Helper()
	verifyChecksum(t, archivePath, checksumPath)
	archive, err := os.Open(archivePath)
	if err != nil {
		t.Fatalf("open %s release: %v", spec.service, err)
	}
	defer archive.Close()
	gzipReader, err := gzip.NewReader(archive)
	if err != nil {
		t.Fatalf("open %s release gzip: %v", spec.service, err)
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
			t.Fatalf("read %s release tar: %v", spec.service, err)
		}
		isDirectory := header.FileInfo().IsDir() || strings.HasSuffix(header.Name, "/")
		rawName := header.Name
		if isDirectory {
			rawName = strings.TrimSuffix(rawName, "/")
		}
		cleaned := path.Clean(rawName)
		if cleaned != rawName || path.IsAbs(cleaned) || cleaned == "." || strings.HasPrefix(cleaned, "../") {
			t.Fatalf("unsafe %s release path %q", spec.service, header.Name)
		}
		parts := strings.Split(cleaned, "/")
		if root == "" {
			root = parts[0]
		}
		if parts[0] != root {
			t.Fatalf("%s release has multiple roots", spec.service)
		}
		destination := filepath.Join(target, filepath.FromSlash(cleaned))
		switch {
		case isDirectory:
			if err := os.MkdirAll(destination, 0o700); err != nil {
				t.Fatalf("create %s release directory: %v", spec.service, err)
			}
		case header.Typeflag == tar.TypeReg || header.Typeflag == tar.TypeRegA:
			relative := strings.Join(parts[1:], "/")
			if relative == "" {
				t.Fatalf("%s release root is a regular file: %q", spec.service, header.Name)
			}
			if _, duplicate := seen[relative]; duplicate {
				t.Fatalf("duplicate %s release file %s", spec.service, relative)
			}
			seen[relative] = struct{}{}
			if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
				t.Fatalf("create %s release parent: %v", spec.service, err)
			}
			mode := os.FileMode(0o600)
			if strings.HasPrefix(relative, "bin/") {
				mode = 0o700
			}
			file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
			if err != nil {
				t.Fatalf("create %s release file %s: %v", spec.service, relative, err)
			}
			_, copyErr := io.Copy(file, tarReader)
			closeErr := file.Close()
			if copyErr != nil || closeErr != nil {
				t.Fatalf("extract %s release file %s: copy=%v close=%v", spec.service, relative, copyErr, closeErr)
			}
		default:
			t.Fatalf("unsupported %s release entry type %d for %s", spec.service, header.Typeflag, header.Name)
		}
	}
	actual := mapKeys(seen)
	if strings.Join(actual, "\n") != strings.Join(spec.files, "\n") {
		t.Fatalf("%s release files = %v, want %v", spec.service, actual, spec.files)
	}
	return filepath.Join(target, root)
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

type buildIdentity struct {
	Service   string `json:"service"`
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuiltAt   string `json:"built_at"`
	GoVersion string `json:"go_version"`
	Target    string `json:"target"`
}

func assertBuildIdentity(t *testing.T, service string, binaries ...string) {
	t.Helper()
	var first buildIdentity
	for index, binary := range binaries {
		output := runCommand(t, cleanEnvironment(), binary, "version")
		var current buildIdentity
		if err := json.Unmarshal(output, &current); err != nil {
			t.Fatalf("decode %s version: %v: %q", binary, err, output)
		}
		if current.Service != service || current.Version == "" || current.Version == "dev" ||
			current.Commit == "" || current.Commit == "unknown" || current.BuiltAt == "" || current.BuiltAt == "unknown" ||
			current.GoVersion == "" || current.Target == "" {
			t.Fatalf("untraceable build identity for %s: %#v", binary, current)
		}
		if index == 0 {
			first = current
			continue
		}
		if current != first {
			t.Fatalf("%s build identity mismatch: %#v != %#v", service, current, first)
		}
	}
}

type databaseEnvironment struct {
	Pool *pgxpool.Pool
	URL  string
	Name string
}

func newDatabase(t *testing.T, baseDatabaseURL, prefix string) databaseEnvironment {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	baseConfig, err := pgxpool.ParseConfig(baseDatabaseURL)
	if err != nil {
		t.Fatalf("parse database URL: %v", err)
	}
	adminConfig := baseConfig.Copy()
	adminConfig.ConnConfig.Database = "postgres"
	adminPool, err := pgxpool.NewWithConfig(ctx, adminConfig)
	if err != nil {
		t.Fatalf("open database administrator: %v", err)
	}
	if err := adminPool.Ping(ctx); err != nil {
		adminPool.Close()
		t.Fatalf("ping database administrator: %v", err)
	}
	randomBytes := make([]byte, 6)
	if _, err := rand.Read(randomBytes); err != nil {
		adminPool.Close()
		t.Fatalf("generate database name: %v", err)
	}
	databaseName := prefix + hex.EncodeToString(randomBytes)
	parsedURL, err := url.Parse(baseDatabaseURL)
	if err != nil || (parsedURL.Scheme != "postgres" && parsedURL.Scheme != "postgresql") {
		adminPool.Close()
		t.Fatalf("database URL must use postgres scheme: %v", err)
	}
	parsedURL.Path = "/" + databaseName
	parsedURL.RawPath = ""
	quotedName := pgx.Identifier{databaseName}.Sanitize()
	if _, err := adminPool.Exec(ctx, "CREATE DATABASE "+quotedName); err != nil {
		adminPool.Close()
		t.Fatalf("create database %s: %v", databaseName, err)
	}
	targetConfig := baseConfig.Copy()
	targetConfig.ConnConfig.Database = databaseName
	pool, err := pgxpool.NewWithConfig(ctx, targetConfig)
	if err != nil {
		_, _ = adminPool.Exec(context.Background(), "DROP DATABASE "+quotedName)
		adminPool.Close()
		t.Fatalf("open database %s: %v", databaseName, err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		_, _ = adminPool.Exec(context.Background(), "DROP DATABASE "+quotedName)
		adminPool.Close()
		t.Fatalf("ping database %s: %v", databaseName, err)
	}
	t.Cleanup(func() {
		pool.Close()
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cleanupCancel()
		_, _ = adminPool.Exec(cleanupContext, `
			SELECT pg_terminate_backend(pid)
			FROM pg_stat_activity
			WHERE datname = $1 AND pid <> pg_backend_pid()
		`, databaseName)
		if _, err := adminPool.Exec(cleanupContext, "DROP DATABASE "+quotedName); err != nil {
			t.Errorf("drop database %s: %v", databaseName, err)
		}
		adminPool.Close()
	})
	return databaseEnvironment{Pool: pool, URL: strings.TrimSpace(parsedURL.String()), Name: databaseName}
}

func assertSchemaCurrent(t *testing.T, opsBinary string, environment []string) {
	t.Helper()
	output := runCommand(t, environment, opsBinary, "check", "--require-current")
	var report struct {
		Schema struct {
			State       string `json:"state"`
			ServerReady bool   `json:"server_ready"`
		} `json:"schema"`
	}
	if err := json.Unmarshal(output, &report); err != nil {
		t.Fatalf("decode ops current check: %v: %q", err, output)
	}
	if report.Schema.State != "current" || !report.Schema.ServerReady {
		t.Fatalf("ops current check = %#v", report)
	}
}

func contractRuntimeEnvironment(databaseURL, evidenceRoot string) []string {
	return cleanEnvironment("DATABASE_URL="+databaseURL, "EVIDENCE_STORAGE_DIR="+evidenceRoot)
}

func moneyRuntimeEnvironment(databaseURL string) []string {
	return cleanEnvironment("DATABASE_URL=" + databaseURL)
}

func contractServerEnvironment(databaseURL, evidenceRoot, address string, bootstrap bool) []string {
	values := []string{
		"DATABASE_URL=" + databaseURL, "EVIDENCE_STORAGE_DIR=" + evidenceRoot, "HTTP_ADDR=" + address,
		"MIGRATION_MODE=verify", "SESSION_COOKIE_SECURE=false", "SESSION_TTL=30m", "SHUTDOWN_TIMEOUT=3s",
	}
	if bootstrap {
		values = append(values,
			"BOOTSTRAP_ADMIN_USERNAME="+bootstrapUsername,
			"BOOTSTRAP_ADMIN_DISPLAY_NAME=组合验收合同管理员",
			"BOOTSTRAP_ADMIN_PASSWORD="+bootstrapPassword,
		)
	}
	return cleanEnvironment(values...)
}

func moneyServerEnvironment(databaseURL, address string, bootstrap bool) []string {
	values := []string{
		"DATABASE_URL=" + databaseURL, "HTTP_ADDR=" + address,
		"MIGRATION_MODE=verify", "SESSION_COOKIE_SECURE=false", "SESSION_TTL=30m", "SHUTDOWN_TIMEOUT=3s",
	}
	if bootstrap {
		values = append(values,
			"BOOTSTRAP_ADMIN_USERNAME="+bootstrapUsername,
			"BOOTSTRAP_ADMIN_DISPLAY_NAME=组合验收资金管理员",
			"BOOTSTRAP_ADMIN_PASSWORD="+bootstrapPassword,
		)
	}
	return cleanEnvironment(values...)
}

type runningServer struct {
	command *exec.Cmd
	output  bytes.Buffer
	done    chan error
	stopped bool
}

func startServer(t *testing.T, binary string, environment []string) *runningServer {
	t.Helper()
	command := exec.Command(binary)
	server := &runningServer{command: command, done: make(chan error, 1)}
	command.Env = environment
	command.Stdout = &server.output
	command.Stderr = &server.output
	if err := command.Start(); err != nil {
		t.Fatalf("start %s: %v", binary, err)
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
		t.Fatalf("interrupt server: %v", err)
	}
	select {
	case err := <-s.done:
		s.stopped = true
		if err != nil {
			t.Fatalf("server stopped with error: %v\n%s", err, s.output.String())
		}
	case <-time.After(5 * time.Second):
		_ = s.command.Process.Kill()
		<-s.done
		s.stopped = true
		t.Fatalf("server did not stop gracefully\n%s", s.output.String())
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

func waitReady(t *testing.T, server *runningServer, baseURL, service string) {
	t.Helper()
	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-server.done:
			server.stopped = true
			t.Fatalf("%s exited before ready: %v\n%s", service, err, server.output.String())
		default:
		}
		response, err := http.Get(baseURL + "/health/ready")
		if err == nil {
			body, readErr := io.ReadAll(response.Body)
			_ = response.Body.Close()
			if readErr == nil && response.StatusCode == http.StatusOK && strings.Contains(string(body), `"service":"`+service+`"`) && strings.Contains(string(body), `"status":"ready"`) {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	server.stopForCleanup()
	t.Fatalf("%s did not become ready\n%s", service, server.output.String())
}

func assertReadyHTTP(t *testing.T, baseURL, service string) {
	t.Helper()
	response, err := http.Get(baseURL + "/health/ready")
	if err != nil {
		t.Fatalf("GET %s ready: %v", service, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != http.StatusOK || !strings.Contains(string(body), `"service":"`+service+`"`) || !strings.Contains(string(body), `"status":"ready"`) {
		t.Fatalf("%s ready status=%d body=%q error=%v", service, response.StatusCode, body, err)
	}
}

func newBrowser(t *testing.T) (*http.Client, *cookiejar.Jar) {
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
	}, jar
}

func login(t *testing.T, client *http.Client, baseURL, displayName string) {
	t.Helper()
	page := getPage(t, client, baseURL+"/login")
	response := postForm(t, client, baseURL+"/login", url.Values{
		"csrf_token": {hiddenValue(t, csrfPattern, page, "CSRF")},
		"username":   {bootstrapUsername}, "password": {bootstrapPassword}, "next": {"/"},
	})
	defer closeResponse(t, response)
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != "/" {
		t.Fatalf("login %s status=%d location=%q body=%q", baseURL, response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	assertAuthenticated(t, client, baseURL, displayName)
}

func assertAuthenticated(t *testing.T, client *http.Client, baseURL, displayName string) {
	t.Helper()
	home := getPage(t, client, baseURL+"/")
	if !strings.Contains(home, displayName) || !strings.Contains(home, "administrator") || !strings.Contains(home, "注销") {
		t.Fatalf("%s home is not authenticated for %s", baseURL, displayName)
	}
}

func assertApplicationCookies(t *testing.T, jar *cookiejar.Jar, baseURL string) {
	t.Helper()
	parsed, err := url.Parse(baseURL)
	if err != nil {
		t.Fatalf("parse cookie URL: %v", err)
	}
	actual := make(map[string]int)
	for _, cookie := range jar.Cookies(parsed) {
		actual[cookie.Name]++
	}
	for _, name := range []string{
		"contract_ledger_session", "contract_ledger_csrf", "money_flow_ledger_session", "money_flow_ledger_csrf",
	} {
		if actual[name] != 1 {
			t.Fatalf("combined cookie %s count=%d all=%v", name, actual[name], actual)
		}
	}
}

func createContractParty(t *testing.T, client *http.Client, baseURL, name, identifier string) {
	t.Helper()
	page := getPage(t, client, baseURL+"/parties")
	response := postForm(t, client, baseURL+"/parties", url.Values{
		"csrf_token": {hiddenValue(t, csrfPattern, page, "contract CSRF")},
		"kind":       {"organization"}, "legal_name": {name}, "registration_identifier": {identifier},
	})
	defer closeResponse(t, response)
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != "/parties?created=1" {
		t.Fatalf("create Contract party status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
}

func createMoneyParty(t *testing.T, client *http.Client, baseURL, name, identifier string) {
	t.Helper()
	page := getPage(t, client, baseURL+"/parties")
	response := postForm(t, client, baseURL+"/parties", url.Values{
		"csrf_token": {hiddenValue(t, csrfPattern, page, "money CSRF")},
		"command_id": {hiddenValue(t, commandPattern, page, "money command")},
		"kind":       {"organization"}, "legal_name": {name}, "registration_identifier": {identifier},
	})
	defer closeResponse(t, response)
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("create Money Flow party status=%d location=%q body=%q", response.StatusCode, response.Header.Get("Location"), readResponse(t, response))
	}
	location, err := url.Parse(response.Header.Get("Location"))
	if err != nil || location.Path != "/parties" || location.Query().Get("created") != "1" || location.Query().Get("party_id") == "" {
		t.Fatalf("create Money Flow party redirect=%q error=%v", response.Header.Get("Location"), err)
	}
}

func assertPartyCount(t *testing.T, environment databaseEnvironment, expected int) {
	t.Helper()
	var actual int
	if err := environment.Pool.QueryRow(context.Background(), `SELECT count(*) FROM parties`).Scan(&actual); err != nil {
		t.Fatalf("count %s parties: %v", environment.Name, err)
	}
	if actual != expected {
		t.Fatalf("%s parties=%d, want %d", environment.Name, actual, expected)
	}
}

func assertDatabaseBoundary(t *testing.T, contract, money databaseEnvironment) {
	t.Helper()
	var contractHasObligations, contractHasCash, moneyHasContracts, moneyHasEvidence bool
	if err := contract.Pool.QueryRow(context.Background(), `
		SELECT to_regclass('public.obligations') IS NOT NULL,
		       to_regclass('public.cash_transactions') IS NOT NULL
	`).Scan(&contractHasObligations, &contractHasCash); err != nil {
		t.Fatalf("inspect Contract database boundary: %v", err)
	}
	if err := money.Pool.QueryRow(context.Background(), `
		SELECT to_regclass('public.contracts') IS NOT NULL,
		       to_regclass('public.contract_evidence_versions') IS NOT NULL
	`).Scan(&moneyHasContracts, &moneyHasEvidence); err != nil {
		t.Fatalf("inspect Money Flow database boundary: %v", err)
	}
	if contractHasObligations || contractHasCash || moneyHasContracts || moneyHasEvidence {
		t.Fatalf("cross-application tables detected contract=%t/%t money=%t/%t",
			contractHasObligations, contractHasCash, moneyHasContracts, moneyHasEvidence)
	}
}

func assertEvidenceBoundary(t *testing.T, contractRoot, moneyTrap string) {
	t.Helper()
	for _, directory := range []string{contractRoot, filepath.Join(contractRoot, ".tmp"), filepath.Join(contractRoot, "objects")} {
		info, err := os.Stat(directory)
		if err != nil || !info.IsDir() {
			t.Fatalf("Contract Evidence directory %s invalid: %v", directory, err)
		}
	}
	if _, err := os.Lstat(moneyTrap); !os.IsNotExist(err) {
		t.Fatalf("Money Flow created or used an Evidence path: %v", err)
	}
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

func getPage(t *testing.T, client *http.Client, target string) string {
	t.Helper()
	response, err := client.Get(target)
	if err != nil {
		t.Fatalf("GET %s: %v", target, err)
	}
	defer closeResponse(t, response)
	body := readResponse(t, response)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status=%d location=%q body=%q", target, response.StatusCode, response.Header.Get("Location"), body)
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
	content, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	return string(content)
}

func closeResponse(t *testing.T, response *http.Response) {
	t.Helper()
	if err := response.Body.Close(); err != nil {
		t.Errorf("close response: %v", err)
	}
}

func mapKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func TestCombinedAcceptanceInputPaths(t *testing.T) {
	for _, name := range []string{
		contractBundle.archiveEnv, contractBundle.checksumEnv,
		moneyBundle.archiveEnv, moneyBundle.checksumEnv,
	} {
		if value := os.Getenv(name); value != "" && !filepath.IsAbs(value) {
			t.Fatalf("%s must be absolute when provided", name)
		}
	}
}

func Example_combinedAcceptanceInputs() {
	fmt.Println("CONTRACT_DELIVERY_BUNDLE_PATH=/absolute/contract.tar.gz")
	fmt.Println("MONEY_FLOW_DELIVERY_BUNDLE_PATH=/absolute/money-flow.tar.gz")
	// Output:
	// CONTRACT_DELIVERY_BUNDLE_PATH=/absolute/contract.tar.gz
	// MONEY_FLOW_DELIVERY_BUNDLE_PATH=/absolute/money-flow.tar.gz
}
