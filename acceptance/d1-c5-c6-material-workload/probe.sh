#!/bin/sh
set -eu
umask 077
. /probe/config/fixture.env
export GOMAXPROCS=1 TEST_DATABASE_URL MATERIAL_PROBE_ISSUER
export WORKLOAD_ID=project-workspace WORKLOAD_SUBJECT_ID=44444444-4444-4444-8444-444444444444
export WORKLOAD_KEY_FILE=/probe/data/project-workload-private.json WORKLOAD_PUBLIC_KEY_FILE=/probe/data/project-workload-public.json
/probe/bin/keyctl init
manifests=/probe/config/project-capabilities.json,/probe/config/evidence-capabilities.json,/probe/config/results-capabilities.json,/probe/config/cost-capabilities.json
production_base=''
if [ "$MATERIAL_PROBE_PRODUCTION" = yes ]; then
  manifests="$manifests,/probe/config/production-capabilities.json"
  production_base=http://127.0.0.1:8097
fi
pids=''
cleanup() {
  trap - EXIT INT TERM
  for pid in $pids; do kill -CONT "$pid" 2>/dev/null || true; kill "$pid" 2>/dev/null || true; done
  wait || true
}
trap cleanup EXIT INT TERM
env HTTP_ADDR=:8080 DATABASE_URL="$IAM_DATABASE_URL" MIGRATION_MODE=auto TENANT_ID=11111111-1111-4111-8111-111111111111 TENANT_NAME=material-workload-fixture PUBLIC_ISSUER="$MATERIAL_PROBE_ISSUER" SIGNING_KEY_FILE=/probe/data/iam-key.json BOOTSTRAP_ADMIN_USERNAME=fixture-admin BOOTSTRAP_ADMIN_DISPLAY_NAME=fixture-admin BOOTSTRAP_ADMIN_PASSWORD="$IAM_ADMIN_PASSWORD" SESSION_COOKIE_SECURE=false CAPABILITY_MANIFEST_PATHS="$manifests" WORKLOAD_IDENTITY_POLICY_PATH=/probe/config/workload-policy.json /probe/bin/iam >/probe/data/iam.log 2>&1 &
pids="$pids $!"
ready=false
for attempt in $(seq 1 45); do
  if wget -q -O /probe/data/jwks.json http://127.0.0.1:8080/api/v1/.well-known/jwks.json; then ready=true; break; fi
  sleep 1
done
[ "$ready" = true ]
export IDENTITY_PROVIDER=iam IAM_ISSUER="$MATERIAL_PROBE_ISSUER" IAM_TENANT_ID=11111111-1111-4111-8111-111111111111 IAM_PUBLIC_KEYS_PATH=/probe/data/jwks.json MIGRATION_MODE=auto
env HTTP_ADDR=:8085 DATABASE_URL="$EVIDENCE_DATABASE_URL" STORAGE_PROVIDER=local PUBLIC_BASE_URL=http://127.0.0.1:8085 LOCAL_STORAGE_ROOT=/probe/data/objects LOCAL_SIGNING_KEY="$LOCAL_SIGNING_KEY" /probe/bin/evidence >/probe/data/evidence.log 2>&1 &
evidence_pid=$!; pids="$pids $evidence_pid"; echo "$evidence_pid" >/probe/data/evidence.pid
env HTTP_ADDR=:8086 DATABASE_URL="$PROJECT_DATABASE_URL" IAM_SERVICE_CREDENTIAL_URL=http://127.0.0.1:8080/api/v1/service-credentials WORKLOAD_REFRESH_BEFORE=15s REFERENCE_TIMEOUT=2s EVIDENCE_BASE_URL=http://127.0.0.1:8085 RESULTS_BASE_URL=http://127.0.0.1:8095 COST_BASE_URL=http://127.0.0.1:8096 PRODUCTION_BASE_URL="$production_base" /probe/bin/project >/probe/data/project.log 2>&1 &
pids="$pids $!"
for attempt in $(seq 1 90); do [ ! -f /probe/data/project-id ] || break; sleep 1; done
[ -f /probe/data/project-id ]
export MATERIAL_PROBE_PROJECT_ID="$(cat /probe/data/project-id)" MATERIAL_PROBE_FILE_ID=c5000000-0000-4000-8000-000000000010 MATERIAL_PROBE_KEYS=/probe/data/jwks.json
env MATERIAL_PROBE_ADDR=:8095 MATERIAL_PROBE_OUTPUT=/probe/data/results-source.json /probe/bin/results.test -test.v -test.run '^TestServeResultsMaterialsForIsolatedWorkloadProbe$' -test.timeout=420s >/probe/data/results.log 2>&1 &
results_pid=$!; pids="$pids $results_pid"; echo "$results_pid" >/probe/data/results.pid
env MATERIAL_PROBE_ADDR=:8096 MATERIAL_PROBE_OUTPUT=/probe/data/cost-source.json /probe/bin/cost.test -test.v -test.run '^TestServeCostMaterialsForIsolatedWorkloadProbe$' -test.timeout=420s >/probe/data/cost.log 2>&1 &
cost_pid=$!; pids="$pids $cost_pid"; echo "$cost_pid" >/probe/data/cost.pid
if [ "$MATERIAL_PROBE_PRODUCTION" = yes ]; then
  env MATERIAL_PROBE_ADDR=:8097 MATERIAL_PROBE_OUTPUT=/probe/data/production-source.json /probe/bin/production.test -test.v -test.run '^TestServeProductionMaterialsForIsolatedWorkloadProbe$' -test.timeout=420s >/probe/data/production.log 2>&1 &
  production_pid=$!; pids="$pids $production_pid"; echo "$production_pid" >/probe/data/production.pid
fi
while :; do sleep 2; done
