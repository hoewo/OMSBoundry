#!/bin/sh
set -eu
umask 077
. /probe/config/fixture.env
export GOMAXPROCS=1 TEST_DATABASE_URL
env HTTP_ADDR=:8080 DATABASE_URL="$IAM_DATABASE_URL" MIGRATION_MODE=auto \
  TENANT_ID=11111111-1111-4111-8111-111111111111 TENANT_NAME=cost-material-fixture \
  PUBLIC_ISSUER="$COST_MATERIAL_IAM_ISSUER" SIGNING_KEY_FILE=/probe/data/iam-key.json \
  BOOTSTRAP_ADMIN_USERNAME=fixture-admin BOOTSTRAP_ADMIN_DISPLAY_NAME=fixture-admin \
  BOOTSTRAP_ADMIN_PASSWORD="$COST_MATERIAL_IAM_PASSWORD" SESSION_COOKIE_SECURE=false \
  CAPABILITY_MANIFEST_PATHS=/probe/config/cost-capabilities.json \
  /probe/bin/iam > /probe/data/iam.log 2>&1 &
iam_pid=$!
trap 'kill "$iam_pid" 2>/dev/null || true; wait || true' EXIT INT TERM
ready=false
for attempt in $(seq 1 45); do
  if wget -q -O /probe/data/jwks.json http://127.0.0.1:8080/api/v1/.well-known/jwks.json; then
    ready=true
    break
  fi
  sleep 1
done
[ "$ready" = true ]
/probe/bin/domain.test -test.v -test.run '^TestCostMaterialPostgresBothAggregatesCurrentHistoryAndZeroWrites$' -test.timeout=90s
export COST_MATERIAL_IAM_URL COST_MATERIAL_IAM_ISSUER COST_MATERIAL_IAM_KEYS COST_MATERIAL_IAM_PASSWORD
/probe/bin/http.test -test.v -test.run '^TestCostMaterialHTTPPostgresNativeIAMBothAggregates$' -test.timeout=180s

