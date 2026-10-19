package main

import (
	"context"
	"testing"
	"time"

	"omsbizfoundry/bonus-distribution/internal/config"
)

type fakeMoneyCredentials struct{}

func (fakeMoneyCredentials) Credential(context.Context, string, string, string, string) (string, error) {
	return "workload-token", nil
}

func (fakeMoneyCredentials) Invalidate(string, string, string, string) {}

func TestNewMoneyPayableSelectsCredentialMode(t *testing.T) {
	tests := []struct {
		name        string
		provider    config.IdentityProvider
		accessToken string
		credentials fakeMoneyCredentials
	}{
		{name: "template static token", provider: config.IdentityTemplate, accessToken: "static-money-token"},
		{name: "iam workload credential", provider: config.IdentityIAM, credentials: fakeMoneyCredentials{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := config.Config{IdentityProvider: test.provider, MoneyBaseURL: "http://money-flow-ledger:8083", MoneyAccessToken: test.accessToken, ReferenceTimeout: time.Second}
			var credentials any
			if test.provider == config.IdentityIAM {
				credentials = test.credentials
			}
			var source interface {
				Credential(context.Context, string, string, string, string) (string, error)
				Invalidate(string, string, string, string)
			}
			if credentials != nil {
				source = credentials.(fakeMoneyCredentials)
			}
			client, err := newMoneyPayable(cfg, source)
			if err != nil {
				t.Fatal(err)
			}
			if client == nil {
				t.Fatal("expected configured Money payable client")
			}
		})
	}
}

func TestNewMoneyPayableRejectsIAMWithoutWorkloadCredentials(t *testing.T) {
	cfg := config.Config{IdentityProvider: config.IdentityIAM, MoneyBaseURL: "http://money-flow-ledger:8083", ReferenceTimeout: time.Second}
	if _, err := newMoneyPayable(cfg, nil); err == nil {
		t.Fatal("expected IAM Money client without workload credentials to fail closed")
	}
}
