// Task-isolated PG18 fixture: zero published ports, no Main database or compose operations.
import { spawnSync } from 'node:child_process';
import { mkdirSync, writeFileSync, readFileSync } from 'node:fs';
import { resolve, join } from 'node:path';
import { randomBytes, createHash } from 'node:crypto';

const output = resolve(process.argv[2] ?? '');
if (!/^\/tmp\/oa-stage-native\.[A-Za-z0-9]+$/.test(output)) throw new Error('explicit generated task output directory required');
const host = 'ecs-user@47.98.156.166';
const sshArgs = ['-o', 'BatchMode=yes', '-o', 'ConnectTimeout=8', '-o', 'StrictHostKeyChecking=yes', host];
const suffix = randomBytes(5).toString('hex');
const name = `oa-stage-native-${suffix}`;
const remoteDir = `/tmp/${name}`;
const label = 'oms.task=oa-stage-material-validation';
const postgresImage = 'crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/base-postgres:18.6-alpine@sha256:d8703cd7fba306b9fec9268ecedfa8a966846c053036a60e3635791957eb2f66';
const receipt = { schema_version: '1', started_at: new Date().toISOString(), scope: 'isolated-production-pg18', local_docker_started: false, main_mutated: false, fixture: name, fixture_limits: { memory_bytes: 268435456, cpu: 0.5, published_ports: 0 }, binaries: {}, tests: [], errors: [] };
function run(program, args, timeout = 60000) {
  const result = spawnSync(program, args, { encoding: 'utf8', timeout, maxBuffer: 12 * 1024 * 1024 });
  if (result.error || result.status !== 0) throw new Error(`${program} failed (${result.status}): ${result.stderr || result.stdout || result.error}`);
  return result.stdout.trim();
}
const ssh = command => run('ssh', [...sshArgs, command]);
let started = false;
mkdirSync(output, { recursive: true });
try {
  const stats = ssh('free -m; docker ps --format "{{.Names}} {{.Status}}"');
  writeFileSync(join(output, 'remote-before.txt'), stats + '\n');
  const mainBefore = ssh('docker ps --filter name=omsbizfoundry --format "{{.Names}} {{.Image}}"');
  ssh(`test ! -d '${remoteDir}' && mkdir -m 700 '${remoteDir}'`);
  for (const pkg of ['design', 'database', 'httpserver']) {
    const binary = join(output, `${pkg}.test`);
    receipt.binaries[pkg] = createHash('sha256').update(readFileSync(binary)).digest('hex');
    run('scp', ['-o', 'BatchMode=yes', '-o', 'StrictHostKeyChecking=yes', binary, `${host}:${remoteDir}/${pkg}.test`]);
  }
  receipt.postgres_image = postgresImage;
  ssh(`docker image inspect '${postgresImage}' --format '{{.Id}}'`);
  ssh(`docker run -d --name '${name}' --label '${label}' --network none --memory 256m --cpus 0.5 --pids-limit 128 --tmpfs /var/lib/postgresql:rw,size=268435456 -e POSTGRES_PASSWORD=task-fixture-only '${postgresImage}' -c shared_buffers=16MB -c work_mem=1MB -c maintenance_work_mem=16MB -c max_connections=30`);
  started = true;
  ssh(`for attempt in 1 2 3 4 5 6 7 8 9 10; do docker exec '${name}' pg_isready -U postgres >/dev/null 2>&1 && exit 0; sleep 1; done; exit 1`);
  receipt.fixture_identity = JSON.parse(ssh(`docker inspect '${name}' --format '{{json .}}'`));
  // Keep only safe, relevant identity data. Do not persist Docker environment arrays.
  receipt.fixture_identity = { id: receipt.fixture_identity.Id, image: receipt.fixture_identity.Image, labels: receipt.fixture_identity.Config.Labels, memory: receipt.fixture_identity.HostConfig.Memory, network: receipt.fixture_identity.HostConfig.NetworkMode, ports: receipt.fixture_identity.HostConfig.PortBindings };
  for (const pkg of ['database', 'design', 'httpserver']) {
    ssh(`docker cp '${remoteDir}/${pkg}.test' '${name}:/tmp/${pkg}.test'`);
    const text = ssh(`docker exec -e TEST_DATABASE_URL=postgres://postgres:task-fixture-only@127.0.0.1:5432/postgres?sslmode=disable -e GOMAXPROCS=2 '${name}' /tmp/${pkg}.test -test.v -test.timeout=45s`);
    writeFileSync(join(output, `${pkg}-pg18.log`), text + '\n');
    receipt.tests.push({ package: pkg, pass_top_level: (text.match(/^--- PASS:/gm) || []).length, pass_including_subtests: (text.match(/^\s*--- PASS:/gm) || []).length, skipped: (text.match(/^\s*--- SKIP:/gm) || []).length, exit_code: 0 });
  }
  receipt.main_images_unchanged = mainBefore === ssh('docker ps --filter name=omsbizfoundry --format "{{.Names}} {{.Image}}"');
  if (!receipt.main_images_unchanged) throw new Error('Main container inventory changed during isolated test');
} catch (error) {
  receipt.errors.push(String(error));
} finally {
  if (started) {
    try {
      const observed = JSON.parse(ssh(`docker inspect '${name}' --format '{{json .}}'`));
      if (observed.Config.Labels['oms.task'] !== 'oa-stage-material-validation' || observed.Name !== `/${name}`) throw new Error('fixture cleanup identity mismatch');
      ssh(`docker stop --time 10 '${name}'`);
      receipt.fixture_stopped = true;
      // Keep the stopped container identity; only tmpfs test databases disappear on stop.
    } catch (error) { receipt.errors.push(`cleanup: ${error}`); }
  }
  receipt.finished_at = new Date().toISOString();
  receipt.passed = receipt.errors.length === 0 && receipt.tests.length === 3 && receipt.fixture_stopped === true;
  writeFileSync(join(output, 'remote-pg18.json'), JSON.stringify(receipt, null, 2) + '\n');
}
console.log(JSON.stringify({ output, passed: receipt.passed, tests: receipt.tests, errors: receipt.errors, fixture_stopped: receipt.fixture_stopped }));
if (!receipt.passed) process.exitCode = 1;
