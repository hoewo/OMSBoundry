GO ?= go
NPM ?= npm
GO_PACKAGES = ./acceptance/access-contract/... ./acceptance/c6-cost-outsourcing-composition/... ./acceptance/combined-delivery/... ./acceptance/contract-ledger-demo/... ./acceptance/design-operations-demo/... ./acceptance/evidence-service-demo/... ./acceptance/money-flow-demo/... ./acceptance/people-groups-demo/... ./acceptance/project-workspace-demo/... ./acceptance/reference-oa-demo/... ./acceptance/scenario-evidence/... ./backend/services/approval-audit-results/... ./backend/services/billing-invoice/... ./backend/services/bonus-distribution/... ./backend/services/business-engagement/... ./backend/services/contract-ledger/... ./backend/services/cost-outsourcing/... ./backend/services/design-operations/... ./backend/services/evidence-service/... ./backend/services/internal-finance-requests/... ./backend/platform/gateway/... ./backend/platform/identity-access/... ./backend/services/money-flow-ledger/... ./backend/services/party-directory/... ./backend/services/payroll/... ./backend/services/people-groups/... ./backend/services/project-workspace/... ./frontend/oms-workbench/...
GO_SOURCE_DIRS = acceptance backend frontend packages
OUTPUT_DIR ?= dist

.PHONY: approval-audit-results-core-acceptance approval-audit-results-protected-http-acceptance architecture-boundary-static-audit billing-invoice-core-acceptance c6-cost-outsourcing-composition-acceptance check combined-delivery-acceptance contract-ledger-clean-install-backup-restore-acceptance contract-ledger-demo-static-check contract-ledger-release cost-outsourcing-core-acceptance d1-authorization-acceptance d1-business-fact-authorization-acceptance d1-c1-authorization-acceptance d1-c4-production-authorization-acceptance d1-c5-results-journey-acceptance d1-c6-cost-outsourcing-acceptance d1-full-oa-journey-scenario-test d1-resource-authorization-acceptance d1-delivery-acceptance d1-host-bootstrap-test d1-image-mode-test d1-image-promotion-test d1-ingress-mode-test d1-release d1-release-bundle-test d1-upgrade-selection-test design-operations-backup-restore-acceptance design-operations-demo-static-check design-operations-release evidence-service-backup-restore-acceptance evidence-service-demo-static-check evidence-service-release file-transfer-core-check level-a-demo-static-check local-oa-host-preflight-test local-oa-release local-oa-release-bundle-test local-oa-release-acceptance money-flow-demo-static-check money-flow-ledger-backup-restore-acceptance money-flow-ledger-clean-install-acceptance money-flow-ledger-release p4-payroll-execution-acceptance party-directory-delivery-recovery-acceptance party-directory-release people-groups-demo-static-check project-workspace-demo-static-check project-workspace-release release-provenance-test scenario-evidence-check scenario-evidence-compare scenario-evidence-readiness fmt fmt-check test tidy vet

check: fmt-check architecture-boundary-static-audit release-provenance-test vet test scenario-evidence-check d1-full-oa-journey-scenario-test file-transfer-core-check level-a-demo-static-check contract-ledger-demo-static-check money-flow-demo-static-check people-groups-demo-static-check evidence-service-demo-static-check project-workspace-demo-static-check design-operations-demo-static-check d1-host-bootstrap-test d1-image-mode-test d1-image-promotion-test d1-ingress-mode-test d1-upgrade-selection-test d1-release-bundle-test local-oa-host-preflight-test local-oa-release-bundle-test

architecture-boundary-static-audit:
	acceptance/architecture-boundary-static-audit/self-test
	acceptance/architecture-boundary-static-audit/verify

d1-full-oa-journey-scenario-test:
	acceptance/d1-full-oa-journey/verify-scenario

file-transfer-core-check:
	@test -d packages/file-transfer-core/node_modules || $(NPM) --prefix packages/file-transfer-core ci --ignore-scripts --no-audit --no-fund
	$(NPM) --prefix packages/file-transfer-core run check

fmt:
	find $(GO_SOURCE_DIRS) -type f -name '*.go' -print0 | xargs -0 gofmt -w

fmt-check:
	@test -z "$$(gofmt -l $$(find $(GO_SOURCE_DIRS) -type f -name '*.go'))" || (gofmt -l $$(find $(GO_SOURCE_DIRS) -type f -name '*.go'); exit 1)

release-provenance-test:
	ops/test-release-provenance

test:
	$(GO) test $(GO_PACKAGES)

vet:
	$(GO) vet $(GO_PACKAGES)

tidy:
	cd acceptance/access-contract && $(GO) mod tidy
	cd acceptance/c6-cost-outsourcing-composition && $(GO) mod tidy
	cd acceptance/combined-delivery && $(GO) mod tidy
	cd acceptance/contract-ledger-demo && $(GO) mod tidy
	cd acceptance/design-operations-demo && $(GO) mod tidy
	cd acceptance/evidence-service-demo && $(GO) mod tidy
	cd acceptance/money-flow-demo && $(GO) mod tidy
	cd acceptance/people-groups-demo && $(GO) mod tidy
	cd acceptance/project-workspace-demo && $(GO) mod tidy
	cd acceptance/reference-oa-demo && $(GO) mod tidy
	cd acceptance/scenario-evidence && $(GO) mod tidy
	cd backend/services/approval-audit-results && $(GO) mod tidy
	cd backend/services/bonus-distribution && $(GO) mod tidy
	cd backend/services/contract-ledger && $(GO) mod tidy
	cd backend/services/cost-outsourcing && $(GO) mod tidy
	cd backend/services/billing-invoice && $(GO) mod tidy
	cd backend/services/business-engagement && $(GO) mod tidy
	cd backend/services/design-operations && $(GO) mod tidy
	cd backend/services/evidence-service && $(GO) mod tidy
	cd backend/platform/gateway && $(GO) mod tidy
	cd backend/platform/identity-access && $(GO) mod tidy
	cd backend/services/money-flow-ledger && $(GO) mod tidy
	cd backend/services/party-directory && $(GO) mod tidy
	cd backend/services/people-groups && $(GO) mod tidy
	cd backend/services/project-workspace && $(GO) mod tidy
	cd frontend/oms-workbench && $(GO) mod tidy
	$(GO) work sync

billing-invoice-core-acceptance:
	@test -n "$(TEST_DATABASE_URL)" || (echo "TEST_DATABASE_URL is required and must point to a disposable PostgreSQL test database"; exit 2)
	cd backend/services/billing-invoice && TEST_DATABASE_URL="$(TEST_DATABASE_URL)" $(GO) test -count=1 -tags=integration ./internal/invoice

approval-audit-results-core-acceptance:
	@test -n "$(TEST_DATABASE_URL)" || (echo "TEST_DATABASE_URL is required and must point to a disposable PostgreSQL 18 test database"; exit 2)
	cd backend/services/approval-audit-results && TEST_DATABASE_URL="$(TEST_DATABASE_URL)" $(GO) test -count=1 -tags=integration ./internal/results

approval-audit-results-protected-http-acceptance:
	@test -n "$(TEST_DATABASE_URL)" || (echo "TEST_DATABASE_URL is required and must point to a disposable PostgreSQL 18 test database"; exit 2)
	cd backend/services/approval-audit-results && TEST_DATABASE_URL="$(TEST_DATABASE_URL)" $(GO) test -count=1 -tags=integration ./internal/httpserver ./internal/results

cost-outsourcing-core-acceptance:
	@test -n "$(TEST_DATABASE_URL)" || (echo "TEST_DATABASE_URL is required and must point to a disposable PostgreSQL 18 test database"; exit 2)
	cd backend/services/cost-outsourcing && TEST_DATABASE_URL="$(TEST_DATABASE_URL)" $(GO) test -count=1 -tags=integration ./internal/costoutsourcing

c6-cost-outsourcing-composition-acceptance:
	acceptance/c6-cost-outsourcing-composition/run-isolated

scenario-evidence-check:
	$(GO) test ./acceptance/scenario-evidence/...
	$(GO) run ./acceptance/scenario-evidence/cmd/validate \
		contracts/scenario-evidence/v1/examples/reference-oa.json \
		contracts/scenario-evidence/v1/examples/reference-oms.json \
		docs/templates/customer-scenario-evidence-v1.json
	$(GO) run ./acceptance/scenario-evidence/cmd/readiness --require final_ready \
		contracts/scenario-evidence/v1/examples/reference-oa.json
	$(GO) run ./acceptance/scenario-evidence/cmd/readiness --require evidence_ready \
		contracts/scenario-evidence/v1/examples/reference-oms.json
	$(GO) run ./acceptance/scenario-evidence/cmd/readiness \
		docs/templates/customer-scenario-evidence-v1.json

scenario-evidence-readiness:
	@test -n "$(SCENARIO)" || (echo "SCENARIO is required, for example SCENARIO=docs/templates/customer-scenario-evidence-v1.json"; exit 2)
	$(GO) run ./acceptance/scenario-evidence/cmd/readiness $(if $(SCENARIO_TARGET),--require $(SCENARIO_TARGET),) "$(SCENARIO)"

scenario-evidence-compare:
	@test -n "$(BASELINE)" || (echo "BASELINE is required"; exit 2)
	@test -n "$(CANDIDATE)" || (echo "CANDIDATE is required"; exit 2)
	$(GO) run ./acceptance/scenario-evidence/cmd/compare "$(BASELINE)" "$(CANDIDATE)"

contract-ledger-release:
	@test -n "$(VERSION)" || (echo "VERSION is required, for example VERSION=v0.1.0"; exit 2)
	backend/services/contract-ledger/ops/build-release "$(VERSION)" "$(OUTPUT_DIR)"

contract-ledger-demo-static-check:
	sh -n deployments/demos/contract-ledger/ops/demo

money-flow-demo-static-check:
	sh -n deployments/demos/money-flow-ledger/ops/demo

p4-payroll-execution-acceptance:
	acceptance/p4-payroll-execution/run-isolated

people-groups-demo-static-check:
	sh -n deployments/demos/people-groups/ops/demo

evidence-service-demo-static-check:
	sh -n deployments/demos/evidence-service/ops/demo

project-workspace-demo-static-check:
	sh -n deployments/demos/project-workspace/ops/demo

design-operations-demo-static-check:
	sh -n deployments/demos/design-operations/ops/demo

level-a-demo-static-check:
	sh -n deployments/demos/ops/level-a
	@awk -F '\t' 'NF != 3 || $$1 !~ /^(contract-ledger|money-flow-ledger|people-groups|evidence-service|project-workspace|design-operations)$$/ || $$2 !~ /^(complete|partial)$$/ { exit 1 } END { if (NR != 6) exit 1 }' deployments/demos/level-a-capabilities.tsv

evidence-service-release:
	@test -n "$(VERSION)" || (echo "VERSION is required, for example VERSION=v0.1.0"; exit 2)
	backend/services/evidence-service/ops/build-release "$(VERSION)" "$(OUTPUT_DIR)"

evidence-service-backup-restore-acceptance:
	@test -n "$(TEST_DATABASE_URL)" || (echo "TEST_DATABASE_URL is required"; exit 2)
	cd backend/services/evidence-service && TEST_DATABASE_URL="$(TEST_DATABASE_URL)" $(GO) test -count=1 -tags=integration ./internal/systembackup

contract-ledger-clean-install-backup-restore-acceptance:
	@test -n "$(BUNDLE)" || (echo "BUNDLE must be an absolute release .tar.gz path"; exit 2)
	@test -n "$(CHECKSUM)" || (echo "CHECKSUM must be an absolute release .sha256 path"; exit 2)
	backend/services/contract-ledger/ops/verify-clean-install-backup-restore "$(BUNDLE)" "$(CHECKSUM)"

combined-delivery-acceptance:
	@test -n "$(CONTRACT_BUNDLE)" || (echo "CONTRACT_BUNDLE must be an absolute release .tar.gz path"; exit 2)
	@test -n "$(CONTRACT_CHECKSUM)" || (echo "CONTRACT_CHECKSUM must be an absolute release .sha256 path"; exit 2)
	@test -n "$(MONEY_BUNDLE)" || (echo "MONEY_BUNDLE must be an absolute release .tar.gz path"; exit 2)
	@test -n "$(MONEY_CHECKSUM)" || (echo "MONEY_CHECKSUM must be an absolute release .sha256 path"; exit 2)
	acceptance/combined-delivery/verify-same-host \
		"$(CONTRACT_BUNDLE)" "$(CONTRACT_CHECKSUM)" \
		"$(MONEY_BUNDLE)" "$(MONEY_CHECKSUM)"

d1-delivery-acceptance:
	acceptance/d1-delivery/verify-running

d1-authorization-acceptance:
	acceptance/d1-authorization/verify-running

d1-resource-authorization-acceptance:
	acceptance/d1-resource-authorization/verify-running

d1-business-fact-authorization-acceptance:
	acceptance/d1-business-fact-authorization/verify-running

d1-c1-authorization-acceptance:
	acceptance/d1-c1-authorization/verify-running

d1-c4-production-authorization-acceptance:
	acceptance/d1-c4-production-authorization/verify-running

d1-c5-results-journey-acceptance:
	acceptance/d1-c5-results-journey/verify-running

d1-c6-cost-outsourcing-acceptance:
	acceptance/d1-c6-cost-outsourcing/verify-running

d1-c7-bonus-distribution-acceptance:
	acceptance/d1-c7-bonus-distribution/verify-running

d1-host-bootstrap-test:
	sh -n deployments/d1/ops/bootstrap-host deployments/d1/ops/bootstrap-remote-host
	deployments/d1/ops/bootstrap-host self-test

d1-image-mode-test:
	sh -n deployments/d1/ops/d1 deployments/d1/ops/test-image-mode deployments/d1/ops/publish-images
	deployments/d1/ops/test-image-mode
	deployments/d1/ops/publish-images self-test

d1-image-promotion-test:
	sh -n deployments/d1/ops/promote-images
	deployments/d1/ops/promote-images self-test

d1-ingress-mode-test:
	sh -n deployments/d1/ops/d1 deployments/d1/ops/render-nginx-config deployments/d1/ops/test-ingress-mode acceptance/d1-delivery/verify-running acceptance/d1-authorization/verify-running acceptance/d1-resource-authorization/verify-running acceptance/d1-business-fact-authorization/verify-running acceptance/d1-c1-authorization/verify-running acceptance/d1-c4-production-authorization/run-isolated acceptance/d1-c4-production-authorization/verify-running acceptance/d1-c4-production-authorization/verify-restored deployments/d1/ops/grant-bootstrap-access
	deployments/d1/ops/test-ingress-mode

d1-upgrade-selection-test:
	sh -n deployments/d1/ops/d1 deployments/d1/ops/test-upgrade-selection
	deployments/d1/ops/test-upgrade-selection

d1-release:
	@test -n "$(VERSION)" || (echo "VERSION is required, for example VERSION=v0.1.0-test.3"; exit 2)
	@test -n "$(IMAGE_MANIFEST)" || (echo "IMAGE_MANIFEST is required"; exit 2)
	deployments/d1/ops/build-release "$(VERSION)" "$(IMAGE_MANIFEST)" "$(OUTPUT_DIR)"

d1-release-bundle-test:
	sh -n deployments/d1/ops/build-release deployments/d1/ops/test-release-bundle
	deployments/d1/ops/test-release-bundle

local-oa-release:
	@test -n "$(VERSION)" || (echo "VERSION is required, for example VERSION=v0.1.0-demo.1"; exit 2)
	deployments/local-oa/ops/build-release "$(VERSION)" "$(OUTPUT_DIR)"

local-oa-host-preflight-test:
	sh -n deployments/local-oa/ops/preflight-host deployments/local-oa/ops/test-preflight-host
	deployments/local-oa/ops/test-preflight-host

local-oa-release-bundle-test:
	sh -n deployments/local-oa/ops/local-oa deployments/local-oa/ops/build-release deployments/local-oa/ops/preflight-host deployments/local-oa/ops/test-preflight-host deployments/local-oa/ops/verify-release deployments/local-oa/ops/test-release-bundle deployments/local-oa/ops/verify-release-bundle deployments/demos/ops/level-a deployments/demos/ops/release-runtime deployments/demos/*/ops/demo acceptance/reference-oa-demo/verify-running
	deployments/local-oa/ops/test-release-bundle

local-oa-release-acceptance:
	@test -n "$(BUNDLE)" || (echo "BUNDLE must be an absolute local OA release .tar.gz path"; exit 2)
	@test -n "$(CHECKSUM)" || (echo "CHECKSUM must be an absolute local OA release .sha256 path"; exit 2)
	deployments/local-oa/ops/verify-release-bundle "$(BUNDLE)" "$(CHECKSUM)"

money-flow-ledger-release:
	@test -n "$(VERSION)" || (echo "VERSION is required, for example VERSION=v0.1.0"; exit 2)
	backend/services/money-flow-ledger/ops/build-release "$(VERSION)" "$(OUTPUT_DIR)"

design-operations-release:
	@test -n "$(VERSION)" || (echo "VERSION is required, for example VERSION=v0.1.0"; exit 2)
	backend/services/design-operations/ops/build-release "$(VERSION)" "$(OUTPUT_DIR)"

design-operations-backup-restore-acceptance:
	backend/services/design-operations/ops/verify-backup-restore

project-workspace-release:
	@test -n "$(VERSION)" || (echo "VERSION is required, for example VERSION=v0.1.0"; exit 2)
	backend/services/project-workspace/ops/build-release "$(VERSION)" "$(OUTPUT_DIR)"

money-flow-ledger-clean-install-acceptance:
	@test -n "$(BUNDLE)" || (echo "BUNDLE must be an absolute release .tar.gz path"; exit 2)
	@test -n "$(CHECKSUM)" || (echo "CHECKSUM must be an absolute release .sha256 path"; exit 2)
	backend/services/money-flow-ledger/ops/verify-clean-install "$(BUNDLE)" "$(CHECKSUM)"

money-flow-ledger-backup-restore-acceptance:
	@test -n "$(BUNDLE)" || (echo "BUNDLE must be an absolute release .tar.gz path"; exit 2)
	@test -n "$(CHECKSUM)" || (echo "CHECKSUM must be an absolute release .sha256 path"; exit 2)
	backend/services/money-flow-ledger/ops/verify-backup-restore "$(BUNDLE)" "$(CHECKSUM)"

party-directory-release:
	@test -n "$(VERSION)" || (echo "VERSION is required, for example VERSION=v0.1.0"; exit 2)
	backend/services/party-directory/ops/build-release "$(VERSION)" "$(OUTPUT_DIR)"

party-directory-delivery-recovery-acceptance:
	backend/services/party-directory/ops/verify-delivery-recovery
