// Read-only closing inventory. Never deploy, grant, migrate or start Docker.
import assert from 'node:assert/strict';
import {readFileSync, writeFileSync, existsSync} from 'node:fs';
import {createHash} from 'node:crypto';
import {execFileSync} from 'node:child_process';
import {dirname, resolve, isAbsolute} from 'node:path';

const destination = 'acceptance/oa-initiation-repair/evidence/production-stage-native-20261004';
const sha = path => createHash('sha256').update(readFileSync(path)).digest('hex');
const shellQuote = text => "'" + text.replaceAll("'", "'\\''") + "'";
const run = (program, args, cwd = '.') => execFileSync(program, args, {
  cwd, encoding: 'utf8', timeout: 30000, maxBuffer: 4 * 1024 * 1024,
});
const ssh = command => run('ssh', [
  '-o', 'BatchMode=yes', '-o', 'ConnectTimeout=8', '-o', 'StrictHostKeyChecking=yes',
  'ecs-user@47.98.156.166', command,
]);
function safe(name, value) {
  const text = JSON.stringify(value, null, 2) + '\n';
  assert(!/"(?:password|token|access_token|refresh_token|private_key|database_url|cookies|session)"\s*:/i.test(text));
  assert(!/Bearer [A-Za-z0-9._-]+|POSTGRES_PASSWORD=|BOOTSTRAP_ADMIN_PASSWORD=/i.test(text));
  writeFileSync(destination + '/' + name + '.json', text);
}

const startedAt = new Date().toISOString();
const mainCommand = 'docker ps --filter name=omsbizfoundry --no-trunc --format "{{.Names}}|{{.ID}}|{{.Image}}|{{.Status}}"';
const mainBefore = ssh(mainCommand);
const main = mainBefore.trim().split('\n').map(line => {
  const [name, id, image, status] = line.split('|');
  assert(name.startsWith('omsbizfoundry-') && status.endsWith('(healthy)'));
  return {name, id, image, status};
});
assert.equal(main.length, 17);

const sql = `BEGIN READ ONLY;
SELECT json_build_object(
  'schema_versions', (SELECT json_agg(version ORDER BY version) FROM schema_migrations),
  'work_item_groups', (SELECT json_agg(grouped ORDER BY type_key, type_definition_version, status)
    FROM (SELECT type_key, type_definition_version, status, count(*) AS count
      FROM design_work_items GROUP BY type_key, type_definition_version, status) grouped),
  'stage_material_table_present', to_regclass('public.production_stage_materials') IS NOT NULL
)::text;
COMMIT;`;
const production = JSON.parse(ssh('docker exec omsbizfoundry-postgres-1 psql -U oms_admin -d design_operations -XqAt -v ON_ERROR_STOP=1 -c ' + shellQuote(sql)));
const fixturesRunning = ssh('docker ps --filter label=oms.task=oa-stage-native-browser --format "{{.Names}}"')
  + ssh('docker ps --filter label=oms.task=oa-stage-material-validation --format "{{.Names}}"');
assert.equal(fixturesRunning.trim(), '');

const processes = run('ps', ['-axo', 'pid=,ppid=,rss=,command=']).trim().split('\n').map(line => {
  const parsed = line.trim().match(/^(\d+)\s+(\d+)\s+(\d+)\s+(.*)$/);
  return parsed && {pid: Number(parsed[1]), ppid: Number(parsed[2]), rss_kib: Number(parsed[3]), command: parsed[4]};
}).filter(Boolean);
const localTaskProcesses = processes.filter(({pid, command}) => pid !== process.pid && (
  command.startsWith('/Applications/Docker.app/Contents/MacOS/')
  || command.startsWith('/Applications/Docker.app/Contents/Resources/bin/cagent api')
  || command.startsWith('/tmp/oa-stage-suite.')
  || command.startsWith('/tmp/oa-stage-native.')
  || /--user-data-dir=\/tmp\/oa-(?:stage|production-stage)/.test(command)
));
assert.deepEqual(localTaskProcesses, []);

const provenance = JSON.parse(readFileSync(destination + '/provenance.json'));
for (const file of provenance.candidate_files) assert.equal(sha(file.path), file.sha256, file.path);
const originalRoot = '/Users/crispydog/Workspace/Hoewo/WebProjects/RoadSystemOA/road-design-oa-system';
for (const file of provenance.original_files) assert.equal(sha(resolve(originalRoot, 'frontend', file.path)), file.sha256, file.path);

const extraOriginalFiles = [
  'frontend/src/components/production/ConstructionStageEditModal.tsx',
  'frontend/src/components/production/ChangeFileList.tsx',
  'frontend/src/components/production/CompletionFileList.tsx',
  'frontend/src/components/production/AuxiliaryFilePanel.tsx',
  'frontend/src/components/production/ChangeFileUpload.tsx',
  'frontend/src/components/production/CompletionFileUpload.tsx',
].map(path => {
  const contents = run('git', ['show', provenance.source_commit + ':' + path], originalRoot);
  const sha256 = sha(resolve(originalRoot, path));
  assert.equal(createHash('sha256').update(contents).digest('hex'), sha256, path);
  return {path, sha256};
});
safe('later-stage-source', {
  source_commit: provenance.source_commit, files: extraOriginalFiles,
  original_source_writes: false, evidence_layer: 'read-only-source-inspection',
  native_later_stage_accepted: false, original_live_site_accepted: false,
});

const materialChecks = [];
for (const [module, filter] of [
  ['project-workspace', '^TestProductionMaterialConsumer'],
  ['design-operations', '^TestMaterialReference|^TestStageMaterial'],
]) {
  const output = execFileSync('go', ['test', '-p', '1', './internal/' +
    (module === 'project-workspace' ? 'references' : 'design'), '-run', filter, '-count=1', '-v'], {
    cwd: 'backend/services/' + module, env: {...process.env, GOMAXPROCS: '2'},
    encoding: 'utf8', timeout: 30000, maxBuffer: 4 * 1024 * 1024,
  });
  const passed = [...output.matchAll(/^\s*--- PASS: ([^ ]+)/gm)].map(match => match[1]);
  assert(passed.length > 0 && /^PASS$/m.test(output) && !/no tests to run|--- FAIL:|--- SKIP:/.test(output));
  materialChecks.push({module, filter, passed: true,
    pass_top_level: passed.filter(name => !name.includes('/')).length,
    pass_including_subtests: passed.length, skipped: 0});
}
safe('material-consumer-boundary', {
  checks: materialChecks, evidence_layer: 'logic-and-typed-HTTP-test-provider',
  shared_collection_consumer_implemented: false,
  original_v1_validation_weakened: false, main_changed: false,
  files: ['backend/services/design-operations/internal/design/material_reference.go',
    'backend/services/design-operations/internal/design/material_reference_test.go',
    'backend/services/project-workspace/internal/references/http.go',
    'backend/services/project-workspace/internal/references/production_material_test.go']
    .map(path => ({path, sha256: sha(path)})),
});

const documents = [
  'README.md',
  'docs/decisions.md', 'docs/architecture.md', 'docs/foundation.md',
  'docs/development-plan.md', 'docs/next-stage-action-plan.md', 'docs/testing.md',
  'docs/oa-full-scope-validation-plan.md', 'docs/progress.md',
  'docs/specs/assembly/oa-production-stage-materials-v1.md',
  'docs/specs/assembly/oa-production-stage-original-operation-map.md',
  'docs/specs/assembly/oa-complete-operation-delivery-ledger.md',
  'docs/reports/oa-production-stage-native-validation-20261004.md',
  'docs/reports/oa-complete-ui-checkpoint-20261004.md',
];
let localLinks = 0;
for (const path of documents) {
  const markdown = readFileSync(path, 'utf8').replace(/```[^\n]*\n[\s\S]*?```/g, '');
  for (const match of markdown.matchAll(/\]\(([^\n)]+)\)/g)) {
    let target = match[1].trim().replace(/^<|>$/g, '').replace(/\s+".*"$/, '');
    if (/^(?:[a-z][a-z0-9+.-]*:|#)/i.test(target)) continue;
    target = decodeURIComponent(target.split('#')[0]).replace(/:\d+$/, '');
    if (!target) continue;
    const absolute = isAbsolute(target) ? target : resolve(dirname(path), target);
    assert(existsSync(absolute), path + ' missing target ' + target);
    localLinks++;
  }
}
run('git', ['diff', '--check']);
// Compare stable identities/images, not the naturally increasing Up duration.
const mainAfter = ssh(mainCommand).trim().split('\n').map(line => line.split('|').slice(0, 3).join('|')).sort();
assert.deepEqual(mainAfter, mainBefore.trim().split('\n').map(line => line.split('|').slice(0, 3).join('|')).sort());

safe('closeout', {
  started_at: startedAt, finished_at: new Date().toISOString(), main,
  main_healthy_count: main.length, main_images_and_ids_unchanged_during_closeout: true,
  main_production_readonly_inventory: production, main_writes: false, main_grants_changed: false,
  local_docker_started: false, local_docker_or_task_processes: localTaskProcesses,
  remote_task_containers_running: [], private_archives_or_keys_removed: false,
  candidate_hashes_rechecked: provenance.candidate_files.length,
  original_hashes_rechecked: provenance.original_files.length,
  later_stage_original_hashes_rechecked: extraOriginalFiles.length,
  material_consumer_boundary_regression: materialChecks,
  documents_checked: documents.length, local_document_links_checked: localLinks,
  diff_check_passed: true, native_scheme_browser_scenarios: 16,
  stage_actual_d1_restore_accepted: false, main_stage_deployed: false, full_oa_accepted: false,
});
console.log(JSON.stringify({healthy: main.length, localLinks, candidate_hashes_unchanged: true, remote_task_containers_running: 0, full_oa_accepted: false}));
