import assert from 'node:assert/strict';
import http from 'node:http';
import { spawn } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const cli = fileURLToPath(new URL('./oms-open-api-test', import.meta.url));
const key = 'bfk_testclient_abcdefghijklmnopqrstuvwxyz0123456789AB';

function readBody(request) {
  return new Promise((resolve, reject) => {
    const chunks = [];
    request.on('data', (chunk) => chunks.push(chunk));
    request.on('end', () => resolve(Buffer.concat(chunks)));
    request.on('error', reject);
  });
}

function json(response, code, value, headers = {}) {
  response.writeHead(code, { 'content-type': 'application/json', ...headers });
  response.end(JSON.stringify(value));
}

function run(args, baseURL) {
  return new Promise((resolve) => {
    const child = spawn(cli, [...args, '--base-url', `${baseURL}/open-api/v1`], {
      env: { ...process.env, OMS_API_KEY: key, OMS_PAYEE_ACCOUNT_NUMBER: '6222020000007890' },
    });
    let stdout = '';
    let stderr = '';
    child.stdout.setEncoding('utf8');
    child.stderr.setEncoding('utf8');
    child.stdout.on('data', (value) => { stdout += value; });
    child.stderr.on('data', (value) => { stderr += value; });
    child.on('close', (code) => resolve({ code, stdout, stderr }));
  });
}

const state = {
  person: 'missing',
  abortReasons: [],
  uploads: 0,
  drafts: 0,
  submissions: 0,
  reimbursementID: '22222222-2222-4222-8222-222222222222',
};

const server = http.createServer(async (request, response) => {
  const url = new URL(request.url, 'http://localhost');
  if (url.pathname !== '/signed-upload') {
    assert.equal(request.headers.authorization, `ApiKey ${key}`);
  }
  if (request.method === 'GET' && url.pathname === '/open-api/v1/') return json(response, 200, { ok: true });
  if (request.method === 'GET' && url.pathname === '/open-api/v1/me') return json(response, 200, { tenant_id: 'tenant', subject_id: 'subject', key_id: 'key' });
  if (request.method === 'GET' && url.pathname === '/open-api/v1/settings') return json(response, 200, { webhook: { available: false } });
  if (request.method === 'GET' && url.pathname === '/open-api/v1/reimbursements') {
    if (state.person === 'missing') return json(response, 403, { error: { code: 'PERSON_MAPPING_REQUIRED', message: 'link a member', retryable: false, recovery: { kind: 'member_account_link', method: 'GET', path: '/workbench/?view=oms-v1#member-account-link' } } });
    return json(response, 200, { items: state.drafts ? [{ id: state.reimbursementID, status: state.submissions ? 'payable' : 'draft' }] : [] });
  }
  if (request.method === 'POST' && url.pathname === '/open-api/v1/evidence/upload-sessions') {
    assert.ok(request.headers['idempotency-key']);
    return json(response, 201, { session_id: '11111111-1111-4111-8111-111111111111', authorization: { upload_url: `${baseURL}/signed-upload` } });
  }
  if (request.method === 'POST' && url.pathname.endsWith('/abort')) {
    assert.ok(request.headers['idempotency-key']);
    const body = JSON.parse((await readBody(request)).toString('utf8'));
    state.abortReasons.push(body.reason);
    response.writeHead(204).end();
    return;
  }
  if (request.method === 'PUT' && url.pathname === '/signed-upload') {
    state.uploads += 1;
    await readBody(request);
    response.writeHead(204, { etag: 'mock-etag' }).end();
    return;
  }
  if (request.method === 'POST' && url.pathname.endsWith('/complete')) {
    assert.ok(request.headers['idempotency-key']);
    return json(response, 200, { file_id: '33333333-3333-4333-8333-333333333333', version: 1, validation_profile: 'invoice-v1', security_status: 'passed', status: 'available' });
  }
  if (request.method === 'POST' && url.pathname === '/open-api/v1/reimbursements') {
    const body = JSON.parse((await readBody(request)).toString('utf8'));
    assert.equal(body.payee.account_number, '6222020000007890');
    state.drafts += 1;
    return json(response, 201, { id: state.reimbursementID, revision: 1, status: 'draft' });
  }
  if (request.method === 'POST' && url.pathname.endsWith('/submit')) {
    state.submissions += 1;
    return json(response, 200, { id: state.reimbursementID, revision: 2, status: 'payable' });
  }
  if (request.method === 'GET' && url.pathname === `/open-api/v1/reimbursements/${state.reimbursementID}`) {
    return json(response, 200, { id: state.reimbursementID, status: state.submissions ? 'payable' : 'draft' });
  }
  response.writeHead(404).end();
});

await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
const address = server.address();
const baseURL = `http://127.0.0.1:${address.port}`;

try {
  const smoke = await run(['smoke', '--expect-person', 'missing'], baseURL);
  assert.equal(smoke.code, 0, smoke.stderr);
  assert.equal(JSON.parse(smoke.stdout).evidence_session, 'created_and_aborted');
  assert.equal(JSON.parse(smoke.stdout).repair_path, '/workbench/?view=oms-v1#member-account-link');
  assert.deepEqual(state.abortReasons, ['client_shutdown']);
  assert.ok(!`${smoke.stdout}${smoke.stderr}`.includes(key));

  const wrongExpectation = await run(['check', '--expect-person', 'active'], baseURL);
  assert.notEqual(wrongExpectation.code, 0);
  assert.match(wrongExpectation.stderr, /expected active/);
  assert.ok(!`${wrongExpectation.stdout}${wrongExpectation.stderr}`.includes(key));

  state.person = 'active';
  const submit = await run([
    'submit', '--expect-person', 'active', '--confirm-business-write', '--confirm-submit',
    '--bank-name', '测试银行',
  ], baseURL);
  assert.equal(submit.code, 0, submit.stderr);
  const result = JSON.parse(submit.stdout);
  assert.equal(result.reimbursement.status, 'payable');
  assert.equal(result.repair_path, undefined);
  assert.equal(state.uploads, 1);
  assert.equal(state.drafts, 1);
  assert.equal(state.submissions, 1);
  assert.ok(!`${submit.stdout}${submit.stderr}`.includes(key));
  assert.ok(!`${submit.stdout}${submit.stderr}`.includes('6222020000007890'));
  assert.ok(!`${submit.stdout}${submit.stderr}`.includes('/signed-upload'));

  const missingConfirmation = await run(['draft', '--bank-name', '测试银行'], baseURL);
  assert.notEqual(missingConfirmation.code, 0);
  assert.match(missingConfirmation.stderr, /confirm-business-write/);
  assert.ok(!`${missingConfirmation.stdout}${missingConfirmation.stderr}`.includes(key));
  console.log('PASS oms-open-api-test safe smoke and explicit business-write modes');
} finally {
  await new Promise((resolve) => server.close(resolve));
}
