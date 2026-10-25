package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"omsbizfoundry/identity-access/internal/database"
	"omsbizfoundry/identity-access/internal/identity"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(arguments []string) error {
	if len(arguments) == 0 || arguments[0] != "create" {
		return fmt.Errorf("usage: tenantctl create --tenant-id UUID --tenant-name NAME --admin-username USERNAME --admin-display-name NAME --admin-password-file PATH --operator-id ID --operator-name NAME")
	}
	flags := flag.NewFlagSet("tenantctl create", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	var input identity.TenantProvisionInput
	var passwordFile string
	flags.StringVar(&input.TenantID, "tenant-id", "", "new tenant UUID")
	flags.StringVar(&input.TenantName, "tenant-name", "", "new tenant name")
	flags.StringVar(&input.AdminUsername, "admin-username", "", "first administrator login name")
	flags.StringVar(&input.AdminDisplayName, "admin-display-name", "", "first administrator display name")
	flags.StringVar(&passwordFile, "admin-password-file", "", "file containing the first administrator password")
	flags.StringVar(&input.OperatorID, "operator-id", "", "platform operator identifier for audit")
	flags.StringVar(&input.OperatorDisplayName, "operator-name", "", "platform operator display name for audit")
	if err := flags.Parse(arguments[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 || strings.TrimSpace(passwordFile) == "" {
		return fmt.Errorf("tenant provisioning arguments are incomplete")
	}
	password, err := os.ReadFile(passwordFile)
	if err != nil {
		return fmt.Errorf("read administrator password file: %w", err)
	}
	input.AdminPassword = strings.TrimSuffix(strings.TrimSuffix(string(password), "\n"), "\r")
	for index := range password {
		password[index] = 0
	}

	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		databaseURL = "postgres://identity_access:identity_access_dev@127.0.0.1:55434/identity_access?sslmode=disable"
	}
	root, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(root, 60*time.Second)
	defer cancel()
	pool, err := database.Open(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if _, err := database.RequireCurrentSchema(ctx, pool); err != nil {
		return err
	}
	provisioner, err := identity.NewTenantProvisioner(identity.NewPostgresRepository(pool))
	if err != nil {
		return err
	}
	subject, err := provisioner.Provision(ctx, input)
	input.AdminPassword = ""
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]string{
		"status": "created", "tenant_id": subject.TenantID, "administrator_subject_id": subject.ID, "administrator_username": subject.Username,
	})
}
