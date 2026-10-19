import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';
const context = vm.createContext({URLSearchParams});
vm.runInContext(readFileSync(new URL('./reference_oa_project_directory.js', import.meta.url), 'utf8'), context);
const directory = context.ReferenceOAProjectDirectory;
const tenant = '11111111-1111-4111-8111-111111111111';
const subject = '22222222-2222-4222-8222-222222222222';
const id = index => '33333333-3333-4333-8333-' + String(index).padStart(12, '0');
const item = (index, number = 'HNJH2026-001') => ({id: id(index), tenant_id: tenant, status: 'active', node_type: 'project', name: '项目' + index, project_number: number, children: [], assignments: []});
const plain = value => JSON.parse(JSON.stringify(value));

test('OA years come from variable-prefix number, never the start date', () => {
  for (const [number, year, sequence] of [[' HNJH2025-009J01 ', 2025, 9n], ['ABC2026-001', 2026, 1n], ['2020-17', 2020, 17n], ['ABC2026-abc', 2026, null], ['', null, null], ['OA-001', null, 1n], ['ABC2026-2BAD', 2026, null]]) {
    const parsed = directory.numberParts(number); assert.equal(parsed.year, year); assert.equal(parsed.sequence, sequence);
  }
  const groups = directory.yearGroups([{...item(1, 'HNJH2025-009'), planned_start_date: '2030-01-01'}, item(2, '')]);
  assert.equal(groups[0].year, 2025); assert.equal(groups[1].year, null);
});
test('same-year ordering uses exact sequence then date and ID; unnumbered stays visible', () => {
  const groups = directory.yearGroups([item(1, 'HNJH2026-9007199254740992'), item(2, 'HNJH2026-9007199254740993'), item(3, ''), item(4, 'HNJH2027-001')]);
  assert.deepEqual(plain(groups.map(group => group.year)), [2027, 2026, null]);
  assert.deepEqual(plain(groups[1].projects.map(project => project.id)), [id(2), id(1)]);
});
test('navigation storage is account/tenant isolated and never stores business facts or credentials', () => {
  const map = new Map(), storage = {getItem: key => map.get(key), setItem: (key, value) => map.set(key, value)};
  const key = directory.navigationKey(tenant, subject);
  directory.saveState(storage, key, {mode: 'paged', page: 3, pageSize: 20, keyword: '北环', expanded: [id(1)], scrollTop: 482, token: 'secret', projects: [item(1)], amount: 123});
  assert.equal(directory.readState(storage, key).page, 3);
  assert.equal(directory.readState(storage, directory.navigationKey(tenant, id(99))).page, 1);
  assert.equal(directory.readState(storage, directory.navigationKey(id(88), subject)).keyword, '');
  assert.doesNotMatch(map.get(key), /secret|amount|projects|tenant_id/);
  assert.equal(directory.navigationKey('*', subject), null);
});
test('corrupt or unavailable storage cannot break navigation; restored fields are bounded', () => {
  const throwing = {getItem: () => {throw Error('blocked');}, setItem: () => {throw Error('blocked');}};
  assert.equal(directory.readState(throwing, 'key').mode, 'timeline'); assert.doesNotThrow(() => directory.saveState(throwing, 'key', {}));
  const value = directory.listState({page: -4, pageSize: 700, mode: 'other', scrollTop: Infinity, expanded: ['*', id(1), id(1)], collapsedYears: ['script', '2026', 'unassigned'], keyword: 'x'.repeat(500)});
  assert.equal(value.page, 1); assert.equal(value.pageSize, 10); assert.equal(value.scrollTop, 0); assert.equal(value.keyword.length, 200);
  assert.deepEqual(plain(value.expanded), [id(1)]); assert.deepEqual(plain(value.collapsedYears), ['2026', 'unassigned']);
});
test('directory retrieves more than 100 roots serially, not a latest-100 preview', async () => {
  const all = Array.from({length: 127}, (_, i) => item(i + 1)); let active = 0, maximum = 0; const offsets = [];
  const result = await directory.readDirectory(async path => {
    active++; maximum = Math.max(maximum, active); const params = new URL('https://example.test' + path).searchParams;
    const offset = Number(params.get('offset')); offsets.push(offset); assert.equal(params.get('keyword'), '字面%_');
    await Promise.resolve(); active--;
    return {schema_version: '1', items: all.slice(offset, offset + 100), total: 127, offset, limit: 100, has_more: offset + 100 < 127};
  }, tenant, '字面%_');
  assert.equal(result.length, 127); assert.equal(maximum, 1); assert.deepEqual(offsets, [0, 100]);
});
test('a broken or drifting page is an explicit error, never partial success', async () => {
  for (const invalid of [{offset: 1}, {limit: 50}, {total: 1, has_more: true, items: []}, {items: [item(1), item(1)], total: 2}, {items: [{...item(1), tenant_id: id(3)}], total: 1}, {items: [{...item(1), status: 'draft'}], total: 1}]) {
    await assert.rejects(directory.readDirectory(async () => ({schema_version: '1', items: [], offset: 0, limit: 100, total: 0, has_more: false, ...invalid}), tenant, ''));
  }
  let count = 0;
  await assert.rejects(directory.readDirectory(async () => (++count === 1 ? {schema_version: '1', items: Array.from({length: 100}, (_, i) => item(i + 1)), offset: 0, limit: 100, total: 101, has_more: true} : {schema_version: '1', items: [], offset: 100, limit: 100, total: 100, has_more: false}), tenant, ''), /发生变化/);
});
test('child hierarchy and owner snapshots must agree with visible parent and Tenant', async () => {
  const parent = {...item(1, ''), node_type: 'package', children: [{...item(2), parent_project_id: id(3)}]};
  const response = () => ({schema_version: '1', items: [parent], total: 1, offset: 0, limit: 100, has_more: false});
  await assert.rejects(directory.readDirectory(async () => response(), tenant, ''), /项目包/);
  parent.children[0].parent_project_id = parent.id;
  assert.equal((await directory.readDirectory(async () => response(), tenant, ''))[0].children.length, 1);
  parent.assignments = [{project_id: id(3), tenant_id: tenant, role_key: 'business_owner', display_snapshot: 'wrong'}];
  await assert.rejects(directory.readDirectory(async () => response(), tenant, ''), /负责人/);
});
