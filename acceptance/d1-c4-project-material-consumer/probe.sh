#!/bin/sh
set -eu
umask 077
. /probe/config/fixture.env
export GOMAXPROCS=1 TEST_DATABASE_URL
export PRODUCTION_MATERIAL_CHECKPOINT_DIR=/probe/data/checkpoints
/probe/bin/migration.test -test.v -test.run '^TestProductionMaterialMigration7To8KeepsLegacyFactsAndIsIdempotent$' -test.timeout=90s
/probe/bin/consumer.test -test.v -test.run '^TestProductionMaterialProjectPostgresHTTPCompositionAndEmptyRestore$' -test.timeout=120s
