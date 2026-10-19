#!/bin/sh
# Private no-host-port candidate only. All endpoints are runtime injected.
set -eu
umask 077
. /probe/config/fixture.env
export GOMAXPROCS=1
if [ "$1" = iam ]; then
  export WORKLOAD_ID=contract-reference-oa WORKLOAD_SUBJECT_ID=44444444-4444-4444-8444-444444444444
  export WORKLOAD_KEY_FILE=/probe/data/workload-private.json WORKLOAD_PUBLIC_KEY_FILE=/probe/data/workload-public.json
  /probe/bin/keyctl init
  exec env HTTP_ADDR=:8080 DATABASE_URL="$IAM_DATABASE_URL" MIGRATION_MODE=auto TENANT_ID=11111111-1111-4111-8111-111111111111 TENANT_NAME=contract-material-private PUBLIC_ISSUER="$COMBO_ISSUER" SIGNING_KEY_FILE=/probe/data/iam-key.json BOOTSTRAP_ADMIN_USERNAME=fixture-admin BOOTSTRAP_ADMIN_DISPLAY_NAME=fixture-admin BOOTSTRAP_ADMIN_PASSWORD="$IAM_ADMIN_PASSWORD" SESSION_COOKIE_SECURE=false CAPABILITY_MANIFEST_PATHS=/probe/config/contract-capabilities.json,/probe/config/evidence-capabilities.json WORKLOAD_IDENTITY_POLICY_PATH=/probe/config/workload-policy.json /probe/bin/iam
fi
mode="$1"
case "$mode" in
  source) contract_url="$CONTRACT_DATABASE_URL"; evidence_url="$EVIDENCE_DATABASE_URL"; objects=/probe/data/objects; legacy=/probe/data/legacy ;;
  restored) contract_url="$RESTORED_CONTRACT_DATABASE_URL"; evidence_url="$RESTORED_EVIDENCE_DATABASE_URL"; objects=/probe/data/restored-objects; legacy=/probe/data/restored-legacy ;;
  *) exit 2 ;;
esac
wget -q -O /probe/data/jwks.json http://127.0.0.1:8080/api/v1/.well-known/jwks.json
export IDENTITY_PROVIDER=iam IAM_ISSUER="$COMBO_ISSUER" IAM_TENANT_ID=11111111-1111-4111-8111-111111111111 IAM_PUBLIC_KEYS_PATH=/probe/data/jwks.json MIGRATION_MODE=verify
if [ "$mode" = source ]; then
  env DATABASE_URL="$contract_url" EVIDENCE_STORAGE_DIR="$legacy" /probe/bin/contract-ops migrate
  env DATABASE_URL="$evidence_url" STORAGE_PROVIDER=local PUBLIC_BASE_URL="$COMBO_EVIDENCE_PUBLIC_URL" LOCAL_STORAGE_ROOT="$objects" LOCAL_SIGNING_KEY="$LOCAL_SIGNING_KEY" /probe/bin/evidence-ops migrate
fi
pids=''
cleanup() {
  trap - EXIT INT TERM
  for pid in $pids; do kill -CONT "$pid" 2>/dev/null || true; kill "$pid" 2>/dev/null || true; done
  wait || true
}
trap cleanup EXIT INT TERM
env HTTP_ADDR=:8085 DATABASE_URL="$evidence_url" STORAGE_PROVIDER=local PUBLIC_BASE_URL="$COMBO_EVIDENCE_PUBLIC_URL" LOCAL_STORAGE_ROOT="$objects" LOCAL_SIGNING_KEY="$LOCAL_SIGNING_KEY" /probe/bin/evidence >/probe/data/evidence-$mode.log 2>&1 &
evidence_pid=$!; pids="$pids $evidence_pid"; echo "$evidence_pid" >/probe/data/evidence.pid
env HTTP_ADDR=:8081 DATABASE_URL="$contract_url" EVIDENCE_STORAGE_DIR="$legacy" EXTERNAL_EVIDENCE_BASE_URL=http://127.0.0.1:8085 IAM_SERVICE_CREDENTIAL_URL=http://127.0.0.1:8080/api/v1/service-credentials WORKLOAD_ID=contract-reference-oa WORKLOAD_SUBJECT_ID=44444444-4444-4444-8444-444444444444 WORKLOAD_KEY_FILE=/probe/data/workload-private.json REFERENCE_TIMEOUT=2s /probe/bin/contract >/probe/data/contract-$mode.log 2>&1 &
contract_pid=$!; pids="$pids $contract_pid"; echo "$contract_pid" >/probe/data/contract.pid
wait "$contract_pid" "$evidence_pid"
