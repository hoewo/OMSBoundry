#!/bin/sh
set -eu

: "${IDENTITY_DB_PASSWORD:?IDENTITY_DB_PASSWORD is required}"
: "${CONTRACT_DB_PASSWORD:?CONTRACT_DB_PASSWORD is required}"
: "${MONEY_DB_PASSWORD:?MONEY_DB_PASSWORD is required}"
: "${PEOPLE_DB_PASSWORD:?PEOPLE_DB_PASSWORD is required}"
: "${PARTY_DB_PASSWORD:?PARTY_DB_PASSWORD is required}"
: "${PROJECT_DB_PASSWORD:?PROJECT_DB_PASSWORD is required}"
: "${EVIDENCE_DB_PASSWORD:?EVIDENCE_DB_PASSWORD is required}"
: "${BUSINESS_ENGAGEMENT_DB_PASSWORD:?BUSINESS_ENGAGEMENT_DB_PASSWORD is required}"
: "${BILLING_DB_PASSWORD:?BILLING_DB_PASSWORD is required}"
: "${PRODUCTION_DB_PASSWORD:?PRODUCTION_DB_PASSWORD is required}"
: "${APPROVAL_AUDIT_RESULTS_DB_PASSWORD:?APPROVAL_AUDIT_RESULTS_DB_PASSWORD is required}"
: "${COST_OUTSOURCING_DB_PASSWORD:?COST_OUTSOURCING_DB_PASSWORD is required}"
: "${BONUS_DISTRIBUTION_DB_PASSWORD:?BONUS_DISTRIBUTION_DB_PASSWORD is required}"

# The same script runs both inside PostgreSQL's init phase and in the
# idempotent provisioning container.  On a brand-new Compose network the
# server can briefly pass its local health check before the TCP listener is
# reachable from that second container, so wait at the actual connection
# boundary instead of failing the whole first installation.
attempt=0
until pg_isready --username "$POSTGRES_USER" --dbname postgres >/dev/null 2>&1; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 30 ]; then
    echo "PostgreSQL did not become reachable for database provisioning" >&2
    exit 1
  fi
  sleep 1
done

create_role_and_database() {
  role_name="$1"
  database_name="$2"
  role_password="$3"
  psql --set=ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres \
    --set=role_name="$role_name" --set=role_password="$role_password" <<'SQL'
SELECT format('CREATE ROLE %I LOGIN PASSWORD %L', :'role_name', :'role_password')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = :'role_name')
\gexec
SQL
  if ! psql --set=ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres --tuples-only --no-align \
      --command "SELECT 1 FROM pg_database WHERE datname = '$database_name'" | grep -q '^1$'; then
    createdb --username "$POSTGRES_USER" --owner "$role_name" --template template0 "$database_name"
  fi
}

create_role_and_database identity_access identity_access "$IDENTITY_DB_PASSWORD"
create_role_and_database contract_ledger contract_ledger "$CONTRACT_DB_PASSWORD"
create_role_and_database money_flow money_flow "$MONEY_DB_PASSWORD"
create_role_and_database people_groups people_groups "$PEOPLE_DB_PASSWORD"
create_role_and_database party_directory party_directory "$PARTY_DB_PASSWORD"
create_role_and_database project_workspace project_workspace "$PROJECT_DB_PASSWORD"
create_role_and_database evidence_service evidence_service "$EVIDENCE_DB_PASSWORD"
create_role_and_database business_engagement business_engagement "$BUSINESS_ENGAGEMENT_DB_PASSWORD"
create_role_and_database billing_invoice billing_invoice "$BILLING_DB_PASSWORD"
create_role_and_database design_operations design_operations "$PRODUCTION_DB_PASSWORD"
create_role_and_database approval_audit_results approval_audit_results "$APPROVAL_AUDIT_RESULTS_DB_PASSWORD"
create_role_and_database cost_outsourcing cost_outsourcing "$COST_OUTSOURCING_DB_PASSWORD"
create_role_and_database bonus_distribution bonus_distribution "$BONUS_DISTRIBUTION_DB_PASSWORD"
