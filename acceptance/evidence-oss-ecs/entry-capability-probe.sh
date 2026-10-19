#!/bin/sh
# Three existing application processes in one bounded, private test container.
# Only synthetic IAM/Evidence databases and files are mounted here.
set -eu
umask 077
. /probe/config/fixture.env
export GOMAXPROCS=1
env HTTP_ADDR=:8080 DATABASE_URL="$IAM_DATABASE_URL" MIGRATION_MODE=auto \
  TENANT_ID="$FIXTURE_TENANT" TENANT_NAME=entry-capability-fixture \
  PUBLIC_ISSUER="$FIXTURE_ISSUER" SIGNING_KEY_FILE=/probe/data/iam-key.json \
  BOOTSTRAP_ADMIN_USERNAME=fixture-admin BOOTSTRAP_ADMIN_DISPLAY_NAME=fixture-admin \
  BOOTSTRAP_ADMIN_PASSWORD="$FIXTURE_PASSWORD" SESSION_COOKIE_SECURE=false \
  CAPABILITY_MANIFEST_PATHS=/probe/config/evidence-capabilities.json \
  /probe/bin/iam > /probe/data/iam.log 2>&1 &
iam_pid=$!
evidence_pid=
gateway_pid=
trap 'kill "$iam_pid" ${evidence_pid:-} ${gateway_pid:-} 2>/dev/null || true; wait || true' EXIT INT TERM
ready=false
for attempt in $(seq 1 45); do
  if wget -q -O /probe/data/jwks.json http://127.0.0.1:8080/api/v1/.well-known/jwks.json; then
    ready=true
    break
  fi
  sleep 1
done
[ "$ready" = true ]
address=$(hostname -i | cut -d ' ' -f 1)
env HTTP_ADDR=:8085 DATABASE_URL="$EVIDENCE_DATABASE_URL" MIGRATION_MODE=auto \
  IDENTITY_PROVIDER=iam IAM_ISSUER="$FIXTURE_ISSUER" IAM_TENANT_ID="$FIXTURE_TENANT" \
  IAM_PUBLIC_KEYS_PATH=/probe/data/jwks.json STORAGE_PROVIDER=local \
  PUBLIC_BASE_URL="http://$address:8085" LOCAL_STORAGE_ROOT=/probe/data/files \
  LOCAL_SIGNING_KEY="$FIXTURE_LOCAL_KEY" ACCESS_GRANT_TTL=1m \
  /probe/bin/evidence > /probe/data/evidence.log 2>&1 &
evidence_pid=$!
env HTTP_ADDR=:8088 SUITE_MANIFEST_PATH=/probe/config/suite.json \
  IAM_PUBLIC_KEYS_PATH=/probe/data/jwks.json IAM_ISSUER="$FIXTURE_ISSUER" \
  COOKIE_SECURE=false REQUESTS_PER_MINUTE=600 \
  /probe/bin/gateway > /probe/data/gateway.log 2>&1 &
gateway_pid=$!
wait "$gateway_pid"
