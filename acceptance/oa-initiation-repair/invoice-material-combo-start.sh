#!/bin/sh
# Private candidate; all application endpoints/DBs are injected.
set -eu
umask 077
. /probe/config/fixture.env
export GOMAXPROCS=1
if [ "$1" = iam ]; then
  export WORKLOAD_ID=billing-reference-oa WORKLOAD_SUBJECT_ID=44444444-4444-4444-8444-444444444444
  export WORKLOAD_KEY_FILE=/probe/data/workload-private.json WORKLOAD_PUBLIC_KEY_FILE=/probe/data/workload-public.json
  /probe/bin/keyctl init
  exec env HTTP_ADDR=:8080 DATABASE_URL="$IAM_DATABASE_URL" MIGRATION_MODE=auto TENANT_ID=11111111-1111-4111-8111-111111111111 TENANT_NAME=invoice-material-private PUBLIC_ISSUER="$COMBO_ISSUER" SIGNING_KEY_FILE=/probe/data/iam-key.json BOOTSTRAP_ADMIN_USERNAME=fixture-admin BOOTSTRAP_ADMIN_DISPLAY_NAME=fixture-admin BOOTSTRAP_ADMIN_PASSWORD="$IAM_ADMIN_PASSWORD" SESSION_COOKIE_SECURE=false CAPABILITY_MANIFEST_PATHS=/probe/config/billing-capabilities.json,/probe/config/evidence-capabilities.json,/probe/config/project-capabilities.json,/probe/config/party-capabilities.json WORKLOAD_IDENTITY_POLICY_PATH=/probe/config/workload-policy.json /probe/bin/iam
fi
case "$1" in
  source) billing_url="$BILLING_DATABASE_URL"; evidence_url="$EVIDENCE_DATABASE_URL"; objects=/probe/data/objects ;;
  restored) billing_url="$RESTORED_BILLING_DATABASE_URL"; evidence_url="$RESTORED_EVIDENCE_DATABASE_URL"; objects=/probe/data/restored-objects ;;
  *) exit 2 ;;
esac
wget -q -O /probe/data/jwks.json http://127.0.0.1:8080/api/v1/.well-known/jwks.json
export IDENTITY_PROVIDER=iam IAM_ISSUER="$COMBO_ISSUER" IAM_TENANT_ID=11111111-1111-4111-8111-111111111111 IAM_PUBLIC_KEYS_PATH=/probe/data/jwks.json MIGRATION_MODE=verify
export PARTY_BASE_URL="$COMBO_REFERENCE_URL" PROJECT_BASE_URL="$COMBO_REFERENCE_URL" CONTRACT_BASE_URL="$COMBO_REFERENCE_URL" MONEY_BASE_URL="$COMBO_REFERENCE_URL" EVIDENCE_BASE_URL=http://127.0.0.1:8085
export IAM_SERVICE_CREDENTIAL_URL=http://127.0.0.1:8080/api/v1/service-credentials WORKLOAD_ID=billing-reference-oa WORKLOAD_SUBJECT_ID=44444444-4444-4444-8444-444444444444 WORKLOAD_KEY_FILE=/probe/data/workload-private.json REFERENCE_TIMEOUT=2s
if [ "$1" = source ]; then
  env DATABASE_URL="$billing_url" /probe/bin/billing-ops migrate
  env DATABASE_URL="$evidence_url" STORAGE_PROVIDER=local PUBLIC_BASE_URL="$COMBO_EVIDENCE_PUBLIC_URL" LOCAL_STORAGE_ROOT="$objects" LOCAL_SIGNING_KEY="$LOCAL_SIGNING_KEY" /probe/bin/evidence-ops migrate
fi
pids=''
cleanup(){ trap - EXIT INT TERM; for pid in $pids; do kill -CONT "$pid" 2>/dev/null || true; kill "$pid" 2>/dev/null || true; done; wait || true; }
trap cleanup EXIT INT TERM
env HTTP_ADDR=:8085 DATABASE_URL="$evidence_url" STORAGE_PROVIDER=local PUBLIC_BASE_URL="$COMBO_EVIDENCE_PUBLIC_URL" LOCAL_STORAGE_ROOT="$objects" LOCAL_SIGNING_KEY="$LOCAL_SIGNING_KEY" /probe/bin/evidence >/probe/data/evidence-$1.log 2>&1 &
evidence_pid=$!; pids="$pids $evidence_pid"; echo "$evidence_pid" >/probe/data/evidence.pid
env HTTP_ADDR=:8081 DATABASE_URL="$billing_url" /probe/bin/billing >/probe/data/billing-$1.log 2>&1 &
billing_pid=$!; pids="$pids $billing_pid"; echo "$billing_pid" >/probe/data/billing.pid
wait "$billing_pid" "$evidence_pid"
